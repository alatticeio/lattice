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
	restartPeerA = "restart-a"
	restartPeerB = "restart-b"
	restartPol   = "e2e-restart-allow"
)

var _ = Describe("Multi-Peer Restart Resilience", Ordered, func() {
	var (
		testNS      string
		podNames    map[string]string
		podIPs      map[string]string
		accessToken string
		netName     string
		ctxRS       = context.Background()
	)

	BeforeAll(func() {
		accessToken = login(manageUrl)

		nsName := fmt.Sprintf("wf-e2e-restart-%d", time.Now().UnixMilli())
		workspaceID := createWorkspace(manageUrl, accessToken, nsName)
		testNS = fmt.Sprintf("wf-%s", workspaceID)

		joinToken := generateJoinToken(manageUrl, accessToken, workspaceID)
		hostAliases := hostAliasesForNATS(clientset)

		// Deploy two peers
		for _, role := range []string{restartPeerA, restartPeerB} {
			deployAgentDeployment(clientset, testNS, role, agentImage, joinToken, hostAliases)
		}

		podNames = make(map[string]string)
		podIPs = make(map[string]string)
		for _, role := range []string{restartPeerA, restartPeerB} {
			podNames[role] = waitForPodRunningReady(clientset, testNS, role, "180s")
			podIPs[role] = waitForWGIP(latticeClient, testNS, role, "90s")
		}

		// Get network name and create ALLOW policy
		peer := &latticev1.LatticePeer{}
		Expect(latticeClient.Get(ctxRS, sigclient.ObjectKey{Namespace: testNS, Name: restartPeerA}, peer)).To(Succeed())
		netName = getNetworkName(peer)
		Expect(netName).NotTo(BeEmpty())
		createAllowAllPolicy(latticeClient, testNS, restartPol, netName)

		By(fmt.Sprintf("All ready: A=%s(%s), B=%s(%s), network=%s",
			podNames[restartPeerA], podIPs[restartPeerA],
			podNames[restartPeerB], podIPs[restartPeerB], netName))
	})

	It("Baseline: Connectivity between two peers is normal", func() {
		pingWithRetry(clientset, restConfig, testNS, podNames[restartPeerA], podIPs[restartPeerB], "60s")
		pingWithRetry(clientset, restConfig, testNS, podNames[restartPeerB], podIPs[restartPeerA], "60s")

		By("Bidirectional ping both succeeded")
	})

	It("Restart restart-a: Pod deleted and recreated, tunnel should recover", func() {
		By("Delete restart-a Pod, trigger Deployment rebuild")
		err := clientset.CoreV1().Pods(testNS).Delete(ctxRS, podNames[restartPeerA], metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred(), "failed to delete Pod")

		By("Wait for restart-a new Pod to be ready")
		podNames[restartPeerA] = waitForPodRunningReady(clientset, testNS, restartPeerA, "120s")

		By("Wait for WireGuard IP re-confirmation")
		podIPs[restartPeerA] = waitForWGIP(latticeClient, testNS, restartPeerA, "90s")

		By("Verify WG interface has been rebuilt")
		Eventually(func() error {
			out, err := execInPod(clientset, restConfig, testNS, podNames[restartPeerA], []string{"wg", "show"})
			if err != nil {
				return fmt.Errorf("wg show failed: %w", err)
			}
			if !strings.Contains(out, "interface:") {
				return fmt.Errorf("WireGuard interface not rebuilt:\n%s", out)
			}
			return nil
		}, "45s", "2s").Should(Succeed(), "WG interface should be rebuilt after restart")

		By("Verify connectivity from restart-a to restart-b after restart")
		pingWithRetry(clientset, restConfig, testNS, podNames[restartPeerA], podIPs[restartPeerB], "60s")
	})

	It("Restart both Peers simultaneously: tunnel recovers after both rebuild", func() {
		By("Delete both Peer Pods simultaneously")
		for _, role := range []string{restartPeerA, restartPeerB} {
			err := clientset.CoreV1().Pods(testNS).Delete(ctxRS, podNames[role], metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred(), "failed to delete Pod %s", role)
		}

		By("Wait for new Pods for both Peers to be ready")
		for _, role := range []string{restartPeerA, restartPeerB} {
			podNames[role] = waitForPodRunningReady(clientset, testNS, role, "120s")
			podIPs[role] = waitForWGIP(latticeClient, testNS, role, "90s")

			Eventually(func() error {
				out, err := execInPod(clientset, restConfig, testNS, podNames[role], []string{"wg", "show"})
				if err != nil {
					return fmt.Errorf("wg show failed: %w", err)
				}
				if !strings.Contains(out, "interface:") {
					return fmt.Errorf("WireGuard interface not rebuilt [%s]:\n%s", role, out)
				}
				return nil
			}, "45s", "2s").Should(Succeed(), "WG interface should be rebuilt after restart for %s", role)
		}

		By("Verify bidirectional connectivity has been restored")
		pingWithRetry(clientset, restConfig, testNS, podNames[restartPeerA], podIPs[restartPeerB], "90s")
		pingWithRetry(clientset, restConfig, testNS, podNames[restartPeerB], podIPs[restartPeerA], "90s")
	})

	AfterAll(func() {
		if CurrentSpecReport().Failed() {
			collectDiagnostics(ctxRS, testNS)
		}
		// Cleanup
		pol := &latticev1.LatticePolicy{ObjectMeta: metav1.ObjectMeta{Name: restartPol, Namespace: testNS}}
		_ = latticeClient.Delete(ctxRS, pol)
		cleanupWorkspace(clientset, testNS)
	})
})
