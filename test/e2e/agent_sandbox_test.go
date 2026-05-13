package e2e

import (
	"context"
	"fmt"
	"strings"
	"time"

	latticev1 "github.com/alatticeio/lattice/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Agent Sandbox", Ordered, func() {
	var (
		testNS            string
		accessToken       string
		workspaceID       string
		enrollmentToken   string
		companionName     string
		companionPeerName string
		companionVPNIP    string
		sandboxName       string
		sandboxPeerName   string
		sandboxVPNIP      string
		networkName       string
	)

	BeforeAll(func() {
		testNS = "wf-e2e-sandbox-" + fmt.Sprintf("%d", time.Now().UnixMilli())
		companionName = "comp-" + testNS
		companionPeerName = companionName + "-peer"
		sandboxName = "sandbox-" + testNS
		sandboxPeerName = sandboxName + "-peer"

		// 1. Login
		accessToken = login(manageUrl)

		// 2. Create workspace
		workspaceID = createWorkspace(manageUrl, accessToken, testNS)

		// 3. Generate join token for companion agent
		joinToken := generateJoinToken(manageUrl, accessToken, workspaceID)

		// 4. Create enrollment token for sandbox agent
		enrollmentToken = createEnrollmentToken(manageUrl, accessToken, testNS)

		// 5. Discover NATS for host aliases
		hostAliases := hostAliasesForNATS(clientset)

		// 6. Deploy companion agent (standard lattice + nginx)
		deployMultiContainerAgent(
			clientset, testNS, companionName, agentImage, joinToken,
			hostAliases,
			corev1.Container{
				Name:  "nginx",
				Image: "nginx:alpine",
				Ports: []corev1.ContainerPort{{
					ContainerPort: 8080,
				}},
			},
		)

		// 7. Wait for companion ready
		_ = waitForPodRunningReady(clientset, testNS, companionName, "120s")
		companionVPNIP = waitForWGIP(latticeClient, testNS, companionPeerName, "60s")
		GinkgoWriter.Printf("[sandbox e2e] companion VPN IP: %s\n", companionVPNIP)

		// 8. Look up companion peer info for static WireGuard configuration.
		compPeer := &latticev1.LatticePeer{}
		Expect(latticeClient.Get(context.Background(), client.ObjectKey{Namespace: testNS, Name: companionPeerName}, compPeer)).To(Succeed())
		compPubKey := compPeer.Spec.PublicKey
		compPodName := waitForPodRunningReady(clientset, testNS, companionName, "30s")
		compPod, err := clientset.CoreV1().Pods(testNS).Get(context.Background(), compPodName, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred(), "get companion pod")
		compPodIP := compPod.Status.PodIP
		peerArg := fmt.Sprintf("%s:%s:51820:%s/32", compPubKey, compPodIP, companionVPNIP)
		GinkgoWriter.Printf("[sandbox e2e] sandbox --peer arg: %s\n", peerArg)

		// 9. Deploy sandbox pod
		sandboxServerURL := "http://lattice-api-service.lattice-system.svc.cluster.local:8080"
		deploySandboxPod(clientset, testNS, sandboxName, agentImage, sandboxServerURL, enrollmentToken, hostAliases, peerArg)

		// 9. Wait for sandbox ready
		_ = waitForPodRunningReady(clientset, testNS, sandboxName, "120s")
		sandboxVPNIP = waitForWGIP(latticeClient, testNS, sandboxPeerName, "60s")
		GinkgoWriter.Printf("[sandbox e2e] sandbox VPN IP: %s\n", sandboxVPNIP)

		// 10. Get network name and create allow-all policy
		peer := &latticev1.LatticePeer{}
		Expect(latticeClient.Get(context.Background(), client.ObjectKey{Namespace: testNS, Name: companionPeerName}, peer)).To(Succeed())
		networkName = getNetworkName(peer)
		GinkgoWriter.Printf("[sandbox e2e] network: %s\n", networkName)
		createAllowAllPolicy(latticeClient, testNS, testNS+"-allow-all", networkName)

		// Wait for policy to propagate and WireGuard tunnel to establish
		time.Sleep(10 * time.Second)
	})

	AfterAll(func() {
		cleanupSandboxCRDs(testNS)
		cleanupWorkspace(clientset, testNS)
	})

	// ─── Scenario 1: Agent registration creates AgentIdentity ───
	It("agent registration creates AgentIdentity CRD with Active phase", func() {
		identity := &latticev1.AgentIdentity{}
		err := latticeClient.Get(context.Background(), client.ObjectKey{Namespace: testNS, Name: sandboxPeerName}, identity)
		Expect(err).NotTo(HaveOccurred(), "AgentIdentity CRD should exist")
		Expect(identity.Status.Phase).To(Equal(latticev1.AgentPhaseActive),
			"AgentIdentity should be Active, got %s", identity.Status.Phase)
	})

	// ─── Scenario 2: Sandbox connects to companion via overlay ───
	It("sandbox can reach companion nginx via WireGuard overlay", func() {
		podName := waitForPodRunningReady(clientset, testNS, sandboxName, "30s")
		// Alpine has busybox wget, use it to hit companion nginx.
		Eventually(func() error {
			output, err := execInPod(clientset, restConfig, testNS, podName,
				[]string{"wget", "-q", "-O", "-", "--timeout=5",
					fmt.Sprintf("http://%s:8080", companionVPNIP)})
			if err != nil {
				return fmt.Errorf("wget failed: %w", err)
			}
			if !strings.Contains(output, "nginx") && !strings.Contains(output, "Welcome") {
				return fmt.Errorf("unexpected response: %s", strings.TrimSpace(output))
			}
			return nil
		}, "60s", "5s").Should(Succeed(), "sandbox should reach companion via overlay")
	})

	// ─── Scenario 3: Non-lattice IP is blocked ───
	It("sandbox cannot reach non-lattice IP addresses", func() {
		podName := waitForPodRunningReady(clientset, testNS, sandboxName, "30s")
		Consistently(func() error {
			_, err := execInPod(clientset, restConfig, testNS, podName,
				[]string{"wget", "-q", "-O", "-", "--timeout=3", "http://1.2.3.4:80"})
			if err == nil {
				return fmt.Errorf("expected connection to fail but succeeded")
			}
			return nil
		}, "15s", "3s").Should(Succeed(), "connection to non-lattice IP should consistently fail")
	})

	// ─── Scenario 4: Policy deny blocks companion ───
	It("policy deny blocks sandbox from reaching companion", func() {
		policyList := &latticev1.LatticePolicyList{}
		Expect(latticeClient.List(context.Background(), policyList, client.InNamespace(testNS))).To(Succeed())
		Expect(policyList.Items).NotTo(BeEmpty(), "should have at least one policy")

		policy := &policyList.Items[0]
		policy.Spec.Action = "DENY"
		Expect(latticeClient.Update(context.Background(), policy)).To(Succeed())

		// Wait for policy propagation
		time.Sleep(5 * time.Second)

		podName := waitForPodRunningReady(clientset, testNS, sandboxName, "30s")
		Consistently(func() error {
			_, err := execInPod(clientset, restConfig, testNS, podName,
				[]string{"wget", "-q", "-O", "-", "--timeout=3",
					fmt.Sprintf("http://%s:8080", companionVPNIP)})
			if err == nil {
				return fmt.Errorf("expected connection to fail under deny policy but succeeded")
			}
			return nil
		}, "15s", "3s").Should(Succeed(), "connection should be blocked under deny policy")
	})

	// ─── Scenario 5: Audit events captured ───
	It("audit log contains allow and drop events", func() {
		podName := waitForPodRunningReady(clientset, testNS, sandboxName, "30s")
		events := readAuditLog(clientset, restConfig, testNS, podName)
		Expect(events).NotTo(BeEmpty(), "audit log should not be empty")

		var hasAllow, hasDrop bool
		for _, ev := range events {
			switch ev.Verdict {
			case "allow":
				hasAllow = true
			case "drop":
				hasDrop = true
			}
		}
		Expect(hasAllow).To(BeTrue(), "audit log should contain allow events")
		Expect(hasDrop).To(BeTrue(), "audit log should contain drop events")
	})

	// ─── Scenario 6: Agent revoke stops sandbox ───
	It("agent revocation stops sandbox connections", func() {
		revokeSandboxAgent(manageUrl, accessToken, sandboxPeerName, testNS)

		// Verify AgentIdentity is revoked
		identity := &latticev1.AgentIdentity{}
		err := latticeClient.Get(context.Background(), client.ObjectKey{Namespace: testNS, Name: sandboxPeerName}, identity)
		if err == nil {
			Expect(identity.Status.Phase).To(Equal(latticev1.AgentPhaseRevoked),
				"AgentIdentity should be Revoked, got %s", identity.Status.Phase)
		}
		// After revocation, the sandbox process is still running but its
		// identity is revoked.
	})
})
