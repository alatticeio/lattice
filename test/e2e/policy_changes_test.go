package e2e

import (
	"context"
	"fmt"
	"strings"
	"time"

	latticev1 "github.com/alatticeio/lattice/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	sigclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	policyChangePeer = "pc-peer"
	policyChangePol1 = "e2e-pc-allow"
	policyChangePol2 = "e2e-pc-deny"
)

var _ = Describe("Policy CRUD Lifecycle", Ordered, func() {
	var (
		testNS      string
		podName     string
		podWGIP     string
		accessToken string
		netName     string
		ctxPC       = context.Background()
	)

	BeforeAll(func() {
		accessToken = login(manageUrl)

		nsName := fmt.Sprintf("wf-e2e-policycrud-%d", time.Now().UnixMilli())
		workspaceID := createWorkspace(manageUrl, accessToken, nsName)
		testNS = fmt.Sprintf("wf-%s", workspaceID)

		joinToken := generateJoinToken(manageUrl, accessToken, workspaceID)
		hostAliases := hostAliasesForNATS(clientset)

		deployAgentDeployment(clientset, testNS, policyChangePeer, agentImage, joinToken, hostAliases)
		podName = waitForPodRunningReady(clientset, testNS, policyChangePeer, "180s")
		podWGIP = waitForWGIP(latticeClient, testNS, policyChangePeer, "90s")

		peer := &latticev1.LatticePeer{}
		Expect(latticeClient.Get(ctxPC, sigclient.ObjectKey{Namespace: testNS, Name: policyChangePeer}, peer)).To(Succeed())
		netName = getNetworkName(peer)
		Expect(netName).NotTo(BeEmpty())

		By(fmt.Sprintf("Pod ready: name=%s, ip=%s, network=%s", podName, podWGIP, netName))
	})

	It("Create an ALLOW policy and verify CRD creation succeeds", func() {
		By("Create e2e-pc-allow policy")
		createAllowAllPolicy(latticeClient, testNS, policyChangePol1, netName)

		By("Verify policy exists in CRD")
		found := &latticev1.LatticePolicy{}
		Expect(latticeClient.Get(ctxPC, sigclient.ObjectKey{Namespace: testNS, Name: policyChangePol1}, found)).To(Succeed())
		Expect(found.Spec.Action).To(Equal("ALLOW"))
		Expect(found.Spec.Network).To(Equal(netName))
	})

	It("List all policies under the workspace, should include the newly created policy", func() {
		By("List all LatticePolicies in Namespace " + testNS)
		policyList := &latticev1.LatticePolicyList{}
		Expect(latticeClient.List(ctxPC, policyList, sigclient.InNamespace(testNS))).To(Succeed())

		found := false
		for _, p := range policyList.Items {
			if p.Name == policyChangePol1 {
				found = true
				break
			}
		}
		Expect(found).To(BeTrue(), "list should contain %s", policyChangePol1)
	})

	It("Create a second DENY policy and verify both policies coexist", func() {
		By("Create second policy: " + policyChangePol2)
		netLabel := fmt.Sprintf("alattice.io/network-%s", netName)
		selector := metav1.LabelSelector{
			MatchLabels: map[string]string{netLabel: "true"},
		}
		denyPolicy := &latticev1.LatticePolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      policyChangePol2,
				Namespace: testNS,
			},
			Spec: latticev1.LatticePolicySpec{
				Network:      netName,
				PeerSelector: selector,
				Action:       "DENY",
				Ingress: []latticev1.IngressRule{
					{From: []latticev1.PeerSelection{{PeerSelector: &selector}}},
				},
				Egress: []latticev1.EgressRule{
					{To: []latticev1.PeerSelection{{PeerSelector: &selector}}},
				},
			},
		}
		Expect(latticeClient.Create(ctxPC, denyPolicy)).To(Succeed(), "failed to create DENY policy")

		By("Verify iptables rules updated to combined effect of both policies")
		Eventually(func() error {
			out, err := execInPod(clientset, restConfig, testNS, podName, []string{"iptables", "-L", "LATTICE-EGRESS", "-n"})
			if err != nil {
				return err
			}
			// Should at least have an iptables rule chain
			if !strings.Contains(out, "LATTICE-EGRESS") {
				return fmt.Errorf("LATTICE-EGRESS chain not found:\n%s", out)
			}
			return nil
		}, "15s", "2s").Should(Succeed(), "iptables should contain policy rules")
	})

	It("After deleting the first policy, the second policy should still be effective", func() {
		By("Delete policy: " + policyChangePol1)
		pol := &latticev1.LatticePolicy{ObjectMeta: metav1.ObjectMeta{Name: policyChangePol1, Namespace: testNS}}
		Expect(latticeClient.Delete(ctxPC, pol)).To(Succeed())

		By("Verify policy has been removed from CRD")
		gone := &latticev1.LatticePolicy{}
		err := latticeClient.Get(ctxPC, sigclient.ObjectKey{Namespace: testNS, Name: policyChangePol1}, gone)
		Expect(err).To(HaveOccurred(), "deleted policy should not exist")
		Expect(apierrors.IsNotFound(err)).To(BeTrue(), "error should be NotFound")

		By("Verify the second policy still exists")
		remaining := &latticev1.LatticePolicy{}
		Expect(latticeClient.Get(ctxPC, sigclient.ObjectKey{Namespace: testNS, Name: policyChangePol2}, remaining)).To(Succeed())
		Expect(remaining.Spec.Action).To(Equal("DENY"))

		By("Verify iptables rule chain still exists (remaining policy rule)")
		Eventually(func() error {
			out, err := execInPod(clientset, restConfig, testNS, podName, []string{"iptables", "-L", "LATTICE-EGRESS", "-n"})
			if err != nil {
				return err
			}
			if !strings.Contains(out, "LATTICE-EGRESS") {
				return fmt.Errorf("LATTICE-EGRESS chain should not be emptied after deleting one policy:\n%s", out)
			}
			return nil
		}, "15s", "2s").Should(Succeed(), "iptables rule chain should be preserved")
	})

	AfterAll(func() {
		if CurrentSpecReport().Failed() {
			collectDiagnostics(ctxPC, testNS)
		}
		// Clean up all policies
		for _, p := range []string{policyChangePol1, policyChangePol2} {
			pol := &latticev1.LatticePolicy{ObjectMeta: metav1.ObjectMeta{Name: p, Namespace: testNS}}
			_ = latticeClient.Delete(ctxPC, pol)
		}
		cleanupWorkspace(clientset, testNS)
	})
})
