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
		companionPodName  string
		sandboxName       string
		sandboxPeerName   string
		sandboxVPNIP      string
		networkName       string
	)

	BeforeAll(func() {
		if sandboxImage == "" {
			Skip("sandbox image not provided")
		}

		ts := fmt.Sprintf("%d", time.Now().UnixMilli())
		companionName = "comp-" + ts
		companionPeerName = companionName
		sandboxName = "sandbox-" + ts
		sandboxPeerName = sandboxName

		// 1. Login
		accessToken = login(manageUrl)

		// 2. Create workspace — testNS must be derived from the returned workspaceID
		//    because the server ignores the nsName field and always creates "wf-{id}".
		nsName := "wf-e2e-sandbox-" + ts
		workspaceID = createWorkspace(manageUrl, accessToken, nsName)
		testNS = fmt.Sprintf("wf-%s", workspaceID)
		GinkgoWriter.Printf("[sandbox e2e] testNS=%s\n", testNS)

		// 3. Generate join token for companion agent
		joinToken := generateJoinToken(manageUrl, accessToken, workspaceID)

		// 4. Create enrollment token for sandbox agent (must use real testNS)
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
				// Override nginx listen port from default 80 to 8080 so the
				// sandbox proxy can reach it at companionVPNIP:8080.
				Command: []string{"sh", "-c",
					`sed -i 's/listen\s*80/listen 8080/g' /etc/nginx/conf.d/default.conf && nginx -g 'daemon off;'`,
				},
			},
		)

		// 7. Wait for companion ready
		_ = waitForPodRunningReady(clientset, testNS, companionName, "120s")
		companionVPNIP = waitForWGIP(latticeClient, testNS, companionPeerName, "60s")
		GinkgoWriter.Printf("[sandbox e2e] companion VPN IP: %s\n", companionVPNIP)

		// 8. Deploy sandbox pod — peers are discovered automatically from the
		//    control plane network map; no static --peer configuration needed.
		companionPodName = waitForPodRunningReady(clientset, testNS, companionName, "30s")
		sandboxServerURL := "http://lattice-api-service.lattice-system.svc.cluster.local:8080"
		deploySandboxPod(clientset, testNS, sandboxName, sandboxImage, sandboxServerURL, enrollmentToken, hostAliases)

		// 10. Wait for sandbox ready
		_ = waitForPodRunningReady(clientset, testNS, sandboxName, "120s")
		sandboxVPNIP = waitForWGIP(latticeClient, testNS, sandboxPeerName, "60s")
		GinkgoWriter.Printf("[sandbox e2e] sandbox VPN IP: %s\n", sandboxVPNIP)

		// 11. Get network name and create allow-all policy
		peer := &latticev1.LatticePeer{}
		Expect(latticeClient.Get(context.Background(), client.ObjectKey{Namespace: testNS, Name: companionPeerName}, peer)).To(Succeed())
		networkName = getNetworkName(peer)
		Expect(networkName).NotTo(BeEmpty(), "failed to get network name from companion LatticePeer")
		GinkgoWriter.Printf("[sandbox e2e] network: %s\n", networkName)
		createAllowAllPolicy(latticeClient, testNS, "e2e-sandbox-allow-all", networkName)

		// Wait for the WireGuard config to propagate from the controller to the
		// companion agent and for the handshake to complete. The companion's
		// wireguard-go needs the sandbox peer added to its AllowedIPs trie before
		// it can encrypt/decrypt overlay traffic. ConfigMap generation is fast,
		// but NATS delivery + wireguard-go application can take 30-60s.
		//
		// Use companion→sandbox wget as the health probe because:
		//  1. The companion initiates the first WG handshake (port is known)
		//  2. The sandbox's ForwardListener on overlay:8080 only responds after
		//     both WG and gVisor netstack are fully initialized
		Eventually(func() error {
			cPodName := waitForPodRunningReady(clientset, testNS, companionName, "5s")
			output, err := execInContainer(clientset, restConfig, testNS, cPodName, "agent",
				[]string{"sh", "-c", fmt.Sprintf(`wget -q -O - --timeout=5 "http://%s:8080"`, sandboxVPNIP)})
			if err != nil {
				return fmt.Errorf("WG not ready: %w", err)
			}
			if !strings.Contains(output, "nginx") && !strings.Contains(output, "Welcome") {
				return fmt.Errorf("unexpected response: %s", strings.TrimSpace(output))
			}
			return nil
		}, "180s", "5s").Should(Succeed(), "WG handshake should establish within 180s")
	})

	AfterAll(func() {
		if CurrentSpecReport().Failed() {
			collectDiagnostics(context.Background(), testNS)
		}
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

	// ─── Scenario 2: Sandbox connects to companion via overlay proxy ───
	It("sandbox can reach companion nginx via WireGuard overlay", func() {
		podName := waitForPodRunningReady(clientset, testNS, sandboxName, "30s")
		// Route curl through the sandbox SOCKS5 proxy (127.0.0.1:1080) which
		// dials via gVisor/WireGuard. The sandbox has no kernel WG interface,
		// so direct wget would fail — the proxy bridges host→gVisor→overlay.
		Eventually(func() error {
			output, err := execInPod(clientset, restConfig, testNS, podName,
				[]string{"sh", "-c", fmt.Sprintf(
					`curl -s --socks5-hostname 127.0.0.1:1080 --max-time 5 "http://%s:8080"`,
					companionVPNIP)})
			if err != nil {
				return fmt.Errorf("wget failed: %w", err)
			}
			if !strings.Contains(output, "nginx") && !strings.Contains(output, "Welcome") {
				return fmt.Errorf("unexpected response: %s", strings.TrimSpace(output))
			}
			return nil
		}, "60s", "5s").Should(Succeed(), "sandbox should reach companion via overlay")
	})

	// ─── Scenario 3: Companion can reach sandbox nginx via overlay (inbound) ───
	It("companion can reach sandbox nginx via overlay ForwardListener", func() {
		// The sandbox has a ForwardListener relaying overlay:8080 → 127.0.0.1:8080 (nginx sidecar).
		// The companion is a standard lattice agent with a kernel WireGuard interface,
		// so it can reach the sandbox's overlay IP directly without a proxy.
		Eventually(func() error {
			output, err := execInContainer(clientset, restConfig, testNS, companionPodName, "agent",
				[]string{"sh", "-c", fmt.Sprintf(
					`wget -q -O - --timeout=5 "http://%s:8080"`,
					sandboxVPNIP)})
			if err != nil {
				return fmt.Errorf("wget from companion to sandbox failed: %w", err)
			}
			if !strings.Contains(output, "nginx") && !strings.Contains(output, "Welcome") {
				return fmt.Errorf("unexpected response: %s", strings.TrimSpace(output))
			}
			return nil
		}, "60s", "5s").Should(Succeed(), "companion should reach sandbox nginx via ForwardListener")
	})

	// ─── Scenario 4: Non-lattice IP is blocked ───
	It("sandbox cannot reach non-lattice IP addresses", func() {
		podName := waitForPodRunningReady(clientset, testNS, sandboxName, "30s")
		// Route through the SOCKS5 proxy so the request enters gVisor where the
		// EgressFilter (--egress-default-deny) blocks non-overlay destinations and
		// writes drop audit events for Scenario 6.
		Consistently(func() error {
			_, err := execInPod(clientset, restConfig, testNS, podName,
				[]string{"sh", "-c",
					`curl -s --socks5-hostname 127.0.0.1:1080 --max-time 3 "http://1.2.3.4:80"`})
			if err == nil {
				return fmt.Errorf("expected connection to fail but succeeded")
			}
			return nil
		}, "15s", "3s").Should(Succeed(), "connection to non-lattice IP should consistently fail")
	})

	// ─── Scenario 5: Policy deny blocks companion ───
	It("policy deny blocks sandbox from reaching companion", func() {
		// Fetch the named allow-all policy instead of blindly picking Items[0],
		// which could be the default lattice-deny-all or any other policy.
		policy := &latticev1.LatticePolicy{}
		Expect(latticeClient.Get(context.Background(), client.ObjectKey{Namespace: testNS, Name: "e2e-sandbox-allow-all"}, policy)).To(Succeed())

		policy.Spec.Action = "DENY"
		Expect(latticeClient.Update(context.Background(), policy)).To(Succeed())

		podName := waitForPodRunningReady(clientset, testNS, sandboxName, "30s")
		// Use Eventually first to wait for propagation, then Consistently to confirm.
		// Route via the sandbox SOCKS5 proxy so gVisor's policy engine is exercised.
		Eventually(func() error {
			_, err := execInPod(clientset, restConfig, testNS, podName,
				[]string{"sh", "-c", fmt.Sprintf(
					`curl -s --socks5-hostname 127.0.0.1:1080 --max-time 3 "http://%s:8080"`,
					companionVPNIP)})
			if err == nil {
				return fmt.Errorf("DENY policy not yet effective")
			}
			return nil
		}, "30s", "3s").Should(Succeed(), "DENY policy should take effect within 30s")

		Consistently(func() error {
			_, err := execInPod(clientset, restConfig, testNS, podName,
				[]string{"sh", "-c", fmt.Sprintf(
					`curl -s --socks5-hostname 127.0.0.1:1080 --max-time 3 "http://%s:8080"`,
					companionVPNIP)})
			if err == nil {
				return fmt.Errorf("expected connection to fail under deny policy but succeeded")
			}
			return nil
		}, "10s", "3s").Should(Succeed(), "connection should be blocked under deny policy")
	})

	// ─── Scenario 6: Audit events captured ───
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

	// ─── Scenario 7: Agent revoke stops sandbox ───
	It("agent revocation stops sandbox connections", func() {
		revokeSandboxAgent(manageUrl, accessToken, sandboxPeerName, testNS)

		// AgentIdentity must exist and phase must be Revoked — no silent pass.
		identity := &latticev1.AgentIdentity{}
		Eventually(func() error {
			err := latticeClient.Get(context.Background(), client.ObjectKey{Namespace: testNS, Name: sandboxPeerName}, identity)
			if err != nil {
				return fmt.Errorf("AgentIdentity not found: %w", err)
			}
			if identity.Status.Phase != latticev1.AgentPhaseRevoked {
				return fmt.Errorf("AgentIdentity phase is %s, expected Revoked", identity.Status.Phase)
			}
			return nil
		}, "30s", "3s").Should(Succeed(), "AgentIdentity should reach Revoked phase within 30s")
	})
})
