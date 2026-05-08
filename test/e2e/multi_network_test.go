package e2e

import (
	"context"
	"fmt"
	"strings"
	"time"

	latticev1 "github.com/alatticeio/lattice/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	sigclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	netPeerA  = "mnet-a"
	netPeerB  = "mnet-b"
	netPeerC  = "mnet-c"
	policyAll = "e2e-mnet-allow"
)

var _ = Describe("Multi-Network Isolation Test", Ordered, func() {
	var (
		accessToken string
		workspaceID string
		joinToken   string
		testNS      string
		podNames    map[string]string
		podIPs      map[string]string
		net2Name    string
		ctxB        = context.Background()
	)

	// Create network 1 (default) + network 2, deploy 3 Pods
	BeforeAll(func() {
		accessToken = login(manageUrl)

		nsName := fmt.Sprintf("wf-e2e-multinet-%d", time.Now().UnixMilli())
		workspaceID = createWorkspace(manageUrl, accessToken, nsName)
		testNS = fmt.Sprintf("wf-%s", workspaceID)

		joinToken = generateJoinToken(manageUrl, accessToken, workspaceID)
		hostAliases := hostAliasesForNATS(clientset)

		// Create a 2nd LatticeNetwork (via CRD)
		net2Name = fmt.Sprintf("e2e-net2-%d", time.Now().UnixMilli())
		net2 := &latticev1.LatticeNetwork{
			ObjectMeta: metav1.ObjectMeta{
				Name:      net2Name,
				Namespace: testNS,
			},
			Spec: latticev1.LatticeNetworkSpec{
				Name: net2Name,
			},
		}
		Expect(latticeClient.Create(ctxB, net2)).To(Succeed(), "failed to create 2nd LatticeNetwork")

		// Deploy 3 Pods, all joined to the default network
		for _, role := range []string{netPeerA, netPeerB, netPeerC} {
			deployAgentDeployment(clientset, testNS, role, agentImage, joinToken, hostAliases)
		}

		podNames = make(map[string]string)
		podIPs = make(map[string]string)
		for _, role := range []string{netPeerA, netPeerB, netPeerC} {
			podNames[role] = waitForPodRunningReady(clientset, testNS, role, "180s")
			podIPs[role] = waitForWGIP(latticeClient, testNS, role, "90s")
		}

		By(fmt.Sprintf("All Pods ready: A=%s(%s), B=%s(%s), C=%s(%s)",
			podNames[netPeerA], podIPs[netPeerA],
			podNames[netPeerB], podIPs[netPeerB],
			podNames[netPeerC], podIPs[netPeerC]))
	})

	// Move mnet-c to network 2
	It("Switch mnet-c to network 2, verify controller re-allocates IP", func() {
		peerC := &latticev1.LatticePeer{}
		Expect(latticeClient.Get(ctxB, sigclient.ObjectKey{Namespace: testNS, Name: netPeerC}, peerC)).To(Succeed())

		By("Update mnet-c Spec.Network to point to network 2: " + net2Name)
		peerC.Spec.Network = &net2Name
		Expect(latticeClient.Update(ctxB, peerC)).To(Succeed(), "failed to update LatticePeer network")

		// Wait for the controller to move mnet-c to the new network and re-allocate IP
		By("Wait for mnet-c to obtain a new IP in the new network")
		Eventually(func() error {
			updated := &latticev1.LatticePeer{}
			if err := latticeClient.Get(ctxB, sigclient.ObjectKey{Namespace: testNS, Name: netPeerC}, updated); err != nil {
				return err
			}
			if updated.Status.ActiveNetwork == nil || *updated.Status.ActiveNetwork != net2Name {
				return fmt.Errorf("ActiveNetwork not switched yet, current: %v", updated.Status.ActiveNetwork)
			}
			if updated.Status.AllocatedAddress == nil || *updated.Status.AllocatedAddress == "" {
				return fmt.Errorf("new IP not yet allocated")
			}
			// Update cached IP
			ip := *updated.Status.AllocatedAddress
			if idx := strings.Index(ip, "/"); idx != -1 {
				ip = ip[:idx]
			}
			podIPs[netPeerC] = ip
			return nil
		}, "60s", "3s").Should(Succeed(), "mnet-c failed to switch to network 2")

		By(fmt.Sprintf("mnet-c switched to network %s, new IP: %s", net2Name, podIPs[netPeerC]))
	})

	// Verify intra-network connectivity (mnet-a ↔ mnet-b)
	It("Same network connectivity: mnet-a → mnet-b ping should succeed", func() {
		By("Create ALLOW policy for the default network")
		peerA := &latticev1.LatticePeer{}
		Expect(latticeClient.Get(ctxB, sigclient.ObjectKey{Namespace: testNS, Name: netPeerA}, peerA)).To(Succeed())
		net1Name := getNetworkName(peerA)
		Expect(net1Name).NotTo(BeEmpty(), "failed to get network 1 name")
		createAllowAllPolicy(latticeClient, testNS, policyAll, net1Name)
		// Also create a policy for network 2 so mnet-c has its own policy
		createAllowAllPolicy(latticeClient, testNS, policyAll+"-net2", net2Name)

		pingWithRetry(clientset, restConfig, testNS, podNames[netPeerA], podIPs[netPeerB], "60s")
	})

	// Verify cross-network isolation (mnet-a → mnet-c unreachable)
	It("Cross-network isolation: Pods in different networks should not communicate", func() {
		assertPingBlocked(clientset, restConfig, testNS, podNames[netPeerA], podIPs[netPeerC], "30s")
	})

	// Verify mnet-b → mnet-c is also blocked
	It("Cross-network isolation: mnet-b → mnet-c should also be blocked", func() {
		assertPingBlocked(clientset, restConfig, testNS, podNames[netPeerB], podIPs[netPeerC], "30s")
	})

	// Cleanup
	AfterAll(func() {
		if CurrentSpecReport().Failed() {
			collectDiagnostics(ctxB, testNS)
		}
		// Delete policies to avoid leftovers
		for _, p := range []string{policyAll, policyAll + "-net2"} {
			pol := &latticev1.LatticePolicy{ObjectMeta: metav1.ObjectMeta{Name: p, Namespace: testNS}}
			_ = latticeClient.Delete(ctxB, pol)
		}
		cleanupWorkspace(clientset, testNS)
	})
})
