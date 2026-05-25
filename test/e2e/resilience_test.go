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
	resilientPeer = "res-pod"
	resilientPol  = "e2e-resilient-allow"
)

var _ = Describe("Agent Restart Resilience Test", Ordered, func() {
	var (
		testNS      string
		podName     string
		wgIP        string
		accessToken string
		ctxR        = context.Background()
	)

	BeforeAll(func() {
		accessToken = login(manageUrl)

		nsName := fmt.Sprintf("wf-e2e-resilient-%d", time.Now().UnixMilli())
		workspaceID := createWorkspace(manageUrl, accessToken, nsName)
		testNS = workspaceID

		joinToken := generateJoinToken(manageUrl, accessToken, workspaceID)
		hostAliases := hostAliasesForNATS(clientset)

		deployAgentDeployment(clientset, testNS, resilientPeer, agentImage, joinToken, hostAliases)
		podName = waitForPodRunningReady(clientset, testNS, resilientPeer, "180s")
		wgIP = waitForWGIP(latticeClient, testNS, resilientPeer, "90s")

		By(fmt.Sprintf("Pod ready: name=%s, ip=%s", podName, wgIP))
	})

	It("Baseline: Create ALLOW policy and verify tunnel status", func() {
		// Get network name
		peer := &latticev1.LatticePeer{}
		Expect(latticeClient.Get(ctxR, sigclient.ObjectKey{Namespace: testNS, Name: resilientPeer}, peer)).To(Succeed())
		networkName := getNetworkName(peer)
		Expect(networkName).NotTo(BeEmpty())

		createAllowAllPolicy(latticeClient, testNS, resilientPol, networkName)

		By("Verify WireGuard interface was created")
		Eventually(func() error {
			out, err := execInPod(clientset, restConfig, testNS, podName, []string{"wg", "show"})
			if err != nil {
				return err
			}
			if !strings.Contains(out, "interface:") {
				return fmt.Errorf("WireGuard interface not ready:\n%s", out)
			}
			return nil
		}, "15s", "2s").Should(Succeed(), "WireGuard interface should have been created")
	})

	It("Agent process crash recovery: tunnel should be automatically rebuilt", func() {
		By("Recording current Pod name: " + podName)

		By("Simulate Agent crash: delete Pod, trigger Deployment rebuild")
		err := clientset.CoreV1().Pods(testNS).Delete(ctxR, podName, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred(), "failed to delete Pod")

		By("Wait for Deployment to create new Pod and enter Ready")
		podName = waitForPodRunningReady(clientset, testNS, resilientPeer, "120s")

		By(fmt.Sprintf("New Pod ready: %s", podName))

		By("Wait for control plane to confirm LatticePeer status (re-allocate/confirm IP)")
		_ = waitForWGIP(latticeClient, testNS, resilientPeer, "90s")

		By("Verify wg interface has recovered")
		Eventually(func() error {
			out, err := execInPod(clientset, restConfig, testNS, podName, []string{"wg", "show"})
			if err != nil {
				return fmt.Errorf("wg show failed: %w", err)
			}
			if !strings.Contains(out, "interface:") {
				return fmt.Errorf("WireGuard interface not rebuilt:\n%s", out)
			}
			return nil
		}, "45s", "2s").Should(Succeed(), "WireGuard interface should be rebuilt after restart")

		By("Verify IP address is consistent (agent should get the same IP after re-registration)")
		Eventually(func() error {
			out, err := execInPod(clientset, restConfig, testNS, podName, []string{"ip", "addr", "show", "wf0"})
			if err != nil {
				return fmt.Errorf("ip addr failed: %w", err)
			}
			if !strings.Contains(out, wgIP) {
				return fmt.Errorf("IP mismatch after restart: expected %s, got:\n%s", wgIP, out)
			}
			return nil
		}, "15s", "2s").Should(Succeed(), "Agent should retain the same WireGuard IP after restart")

		By("Verify iptables policies have been re-applied")
		Eventually(func() error {
			out, err := execInPod(clientset, restConfig, testNS, podName, []string{"iptables", "-L", "LATTICE-EGRESS", "-n"})
			if err != nil {
				return err
			}
			if !strings.Contains(out, "ACCEPT") {
				return fmt.Errorf("LATTICE-EGRESS chain does not contain ACCEPT rule:\n%s", out)
			}
			return nil
		}, "15s", "2s").Should(Succeed(), "iptables policies should be re-applied after restart")
	})

	AfterAll(func() {
		if CurrentSpecReport().Failed() {
			collectDiagnostics(ctxR, testNS)
		}
		// Cleanup
		pol := &latticev1.LatticePolicy{ObjectMeta: metav1.ObjectMeta{Name: resilientPol, Namespace: testNS}}
		_ = latticeClient.Delete(ctxR, pol)
		cleanupWorkspace(clientset, testNS)
	})
})
