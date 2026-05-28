//go:build ignore

package e2e

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	latticev1 "github.com/alatticeio/lattice/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// These tests validate the runsc gVisor isolation mode without requiring
// runsc to be installed on the test node. Flag validation and dispatch
// are exercised by running `lattice sandbox start --mode gvisor` with
// various flag combinations and checking the error messages.
//
// Full end-to-end runsc tests (with a real gVisor container and overlay
// connectivity) require:
//   1. runsc installed on the k3d node (`make e2e-install-runsc`)
//   2. A rootfs image with the lattice binary and a test agent
//      (`make e2e-build-rootfs`)
// Those tests are in the separate Describe block below and are skipped
// when runsc is not available.

var _ = Describe("Agent Sandbox Gvisor Mode", Ordered, func() {
	var (
		testNS     string
		sandboxImg string
	)

	BeforeAll(func() {
		if sandboxImage == "" {
			Skip("sandbox image not provided")
		}
		sandboxImg = sandboxImage

		// Create a dedicated namespace for validation pods.
		testNS = createE2ETestNamespace()
	})

	AfterAll(func() {
		cleanupNamespace(testNS)
	})

	// ─── Scenario 1: gvisor mode requires --agent-rootfs ───
	It("gvisor mode returns error when --agent-rootfs is missing", func() {
		// Run `lattice sandbox start --mode gvisor` without --agent-rootfs.
		// The command should fail with a validation error before attempting
		// to launch runsc.
		ts := fmt.Sprintf("%d", time.Now().UnixMilli())
		podName := "gvisor-validate-rootfs-" + ts
		_, err := clientset.CoreV1().Pods(testNS).Create(context.Background(), gvisorValidationPod(
			sandboxImg, podName, gvisorValidationArgs{
				mode:        "gvisor",
				name:        podName,
				serverURL:   "http://x",
				token:       "t",
				agentRootFS: "", // intentionally empty
				agentBinary: "/bin/sleep",
			},
		), metav1.CreateOptions{})
		if err != nil {
			Skip(fmt.Sprintf("cannot create test pod (namespace may not exist): %v", err))
		}
		defer deletePod(testNS, podName)

		Eventually(func() string {
			return podLogs(testNS, podName)
		}, "30s", "2s").Should(
			ContainSubstring("--agent-rootfs is required for gvisor mode"),
			"should require --agent-rootfs",
		)
	})

	// ─── Scenario 2: gvisor mode requires --agent-binary ───
	It("gvisor mode returns error when --agent-binary is missing", func() {
		ts := fmt.Sprintf("%d", time.Now().UnixMilli())
		podName := "gvisor-validate-binary-" + ts
		_, err := clientset.CoreV1().Pods(testNS).Create(context.Background(), gvisorValidationPod(
			sandboxImg, podName, gvisorValidationArgs{
				mode:        "gvisor",
				name:        podName,
				serverURL:   "http://x",
				token:       "t",
				agentRootFS: "/tmp/rootfs",
				agentBinary: "", // intentionally empty
			},
		), metav1.CreateOptions{})
		if err != nil {
			Skip(fmt.Sprintf("cannot create test pod: %v", err))
		}
		defer deletePod(testNS, podName)

		Eventually(func() string {
			return podLogs(testNS, podName)
		}, "30s", "2s").Should(
			ContainSubstring("--agent-binary is required for gvisor mode"),
			"should require --agent-binary",
		)
	})

	// ─── Scenario 3: gvisor mode validates egress CIDRs ───
	It("gvisor mode returns error for invalid egress CIDR", func() {
		ts := fmt.Sprintf("%d", time.Now().UnixMilli())
		podName := "gvisor-validate-cidr-" + ts
		_, err := clientset.CoreV1().Pods(testNS).Create(context.Background(), gvisorValidationPod(
			sandboxImg, podName, gvisorValidationArgs{
				mode:        "gvisor",
				name:        podName,
				serverURL:   "http://x",
				token:       "t",
				agentRootFS: "/tmp/rootfs",
				agentBinary: "/bin/sleep",
				egressAllow: "not-a-cidr",
			},
		), metav1.CreateOptions{})
		if err != nil {
			Skip(fmt.Sprintf("cannot create test pod: %v", err))
		}
		defer deletePod(testNS, podName)

		Eventually(func() string {
			return podLogs(testNS, podName)
		}, "30s", "2s").Should(
			ContainSubstring("invalid egress CIDR"),
			"should reject invalid egress CIDR",
		)
	})

	// ─── Scenario 4: gvisor mode dispatches correctly ───
	It("gvisor mode with valid flags dispatches to RunscDriver", func() {
		ts := fmt.Sprintf("%d", time.Now().UnixMilli())
		podName := "gvisor-dispatch-" + ts
		_, err := clientset.CoreV1().Pods(testNS).Create(context.Background(), gvisorValidationPod(
			sandboxImg, podName, gvisorValidationArgs{
				mode:        "gvisor",
				name:        podName,
				serverURL:   "http://lattice-api-service.lattice-system.svc.cluster.local:8080",
				token:       "lt-test12345",
				agentRootFS: "/tmp/nonexistent-rootfs",
				agentBinary: "/bin/sleep",
			},
		), metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred(), "failed to create gVisor dispatch test pod")
		defer deletePod(testNS, podName)

		// The pod will fail because the rootfs doesn't exist, but the key
		// assertion is that it got PAST flag validation. Flag validation errors
		// (--agent-rootfs required, --agent-binary required) must NOT appear.
		Consistently(func() string {
			return podLogs(testNS, podName)
		}, "10s", "2s").ShouldNot(
			Or(
				ContainSubstring("--agent-rootfs is required for gvisor mode"),
				ContainSubstring("--agent-binary is required for gvisor mode"),
			),
			"should pass flag validation and reach RunscDriver.Start()",
		)
	})
})

// ─── GVisor full integration tests ───
// These require runsc + rootfs on the k3d node. They are skipped when
// runsc is not detected on the node.

var _ = Describe("Agent Sandbox GVisor Integration", Ordered, func() {
	var (
		testNS            string
		accessToken       string
		workspaceID       string
		enrollmentToken   string
		companionName     string
		companionPeerName string
		companionVPNIP    string
		sandboxName       string
		sandboxID         string
		sandboxPeerName   string
		networkName       string
	)

	BeforeAll(func() {
		// Only run when sandbox image is provided (PRO builds).
		if sandboxImage == "" {
			Skip("sandbox image not provided")
		}
		if !runscAvailable(sandboxImage) {
			Skip("runsc not available in sandbox image")
		}

		ts := fmt.Sprintf("%d", time.Now().UnixMilli())
		companionName = "gv-comp-" + ts
		companionPeerName = companionName
		sandboxName = "gv-sandbox-" + ts
		sandboxID = sandboxName
		sandboxPeerName = sandboxName

		// 1. Login and create workspace
		accessToken = login(manageUrl)
		nsName := "wf-e2e-gvisor-" + ts
		workspaceID = createWorkspace(manageUrl, accessToken, nsName)
		testNS = workspaceID
		GinkgoWriter.Printf("[gvisor e2e] testNS=%s\n", testNS)

		// 2. Generate join token for companion agent
		joinToken := generateJoinToken(manageUrl, accessToken, workspaceID)

		// 3. Create enrollment token for sandbox
		enrollmentToken = createEnrollmentToken(manageUrl, accessToken, testNS)

		// 4. Host aliases for NATS discovery
		hostAliases := hostAliasesForNATS(clientset)

		// 5. Deploy companion agent (standard lattice + nginx)
		deployMultiContainerAgent(
			clientset, testNS, companionName, agentImage, joinToken,
			hostAliases,
			corev1.Container{
				Name:  "nginx",
				Image: "nginx:alpine",
				Ports: []corev1.ContainerPort{{ContainerPort: 8080}},
				Command: []string{"sh", "-c",
					`sed -i 's/listen\s*80/listen 8080/g' /etc/nginx/conf.d/default.conf && nginx -g 'daemon off;'`,
				},
			},
		)

		// 6. Wait for companion ready
		_ = waitForPodRunningReady(clientset, testNS, companionName, "120s")
		companionVPNIP = waitForWGIP(latticeClient, testNS, companionPeerName, "60s")
		GinkgoWriter.Printf("[gvisor e2e] companion VPN IP: %s\n", companionVPNIP)
		_ = waitForPodRunningReady(clientset, testNS, companionName, "30s")

		// 7. Resolve lattice API service IP (gVisor sandbox has no K8s DNS).
		apiIP := getServiceClusterIP("lattice-system", "lattice-api-service")
		Expect(apiIP).NotTo(BeEmpty(), "failed to resolve lattice-api-service ClusterIP")
		sandboxServerURL := fmt.Sprintf("http://%s:8080", apiIP)
		GinkgoWriter.Printf("[gvisor e2e] server URL: %s\n", sandboxServerURL)

		// 8. Deploy gvisor sandbox pod
		deployGvisorSandboxPod(clientset, testNS, sandboxName, sandboxImage,
			sandboxServerURL, enrollmentToken, sandboxID, hostAliases)

		// 9. Wait for sandbox ready
		_ = waitForPodRunningReady(clientset, testNS, sandboxName, "180s")
		GinkgoWriter.Printf("[gvisor e2e] sandbox pod running\n")

		// 10. Create allow-all policy and wait for WG handshake
		peer := &latticev1.LatticePeer{}
		Expect(latticeClient.Get(context.Background(),
			client.ObjectKey{Namespace: testNS, Name: companionPeerName}, peer)).To(Succeed())
		networkName = getNetworkName(peer)
		Expect(networkName).NotTo(BeEmpty(), "failed to get network name")
		GinkgoWriter.Printf("[gvisor e2e] network: %s\n", networkName)
		createAllowAllPolicy(latticeClient, testNS, "e2e-gvisor-allow-all", networkName)

		// 10. Wait for WireGuard handshake
		Eventually(func() error {
			cPodName := waitForPodRunningReady(clientset, testNS, companionName, "5s")
			output, err := execInContainer(clientset, restConfig, testNS, cPodName, "agent",
				[]string{"sh", "-c", fmt.Sprintf(
					`wget -q -O - --timeout=5 "http://%s:8080"`, companionVPNIP)})
			if err != nil {
				return fmt.Errorf("WG not ready: %w", err)
			}
			if !strings.Contains(output, "nginx") && !strings.Contains(output, "Welcome") {
				return fmt.Errorf("unexpected response: %s", strings.TrimSpace(output))
			}
			return nil
		}, "180s", "5s").Should(Succeed())
	})

	AfterAll(func() {
		if CurrentSpecReport().Failed() {
			collectDiagnostics(context.Background(), testNS)
		}
		cleanupSandboxCRDs(testNS)
		cleanupWorkspace(clientset, testNS)
	})

	// ─── Scenario 1: Sandbox registers with AgentIdentity ───
	It("gvisor sandbox creates AgentIdentity CRD in Active phase", func() {
		identity := &latticev1.AgentIdentity{}
		err := latticeClient.Get(context.Background(),
			client.ObjectKey{Namespace: testNS, Name: sandboxPeerName}, identity)
		Expect(err).NotTo(HaveOccurred(), "AgentIdentity CRD should exist")
		Expect(identity.Status.Phase).To(Equal(latticev1.AgentPhaseActive),
			"AgentIdentity should be Active, got %s", identity.Status.Phase)
	})

	// ─── Scenario 2: GVisor sandbox reaches companion via WireGuard overlay ───
	It("gvisor sandbox can reach companion nginx via WireGuard overlay (direct, no SOCKS5)", func() {
		podName := waitForPodRunningReady(clientset, testNS, sandboxName, "30s")
		// In gvisor mode, the AI agent has direct overlay access via wg0.
		// Use runsc exec to run curl inside the gVisor container.
		Eventually(func() error {
			output, err := execInPod(clientset, restConfig, testNS, podName,
				[]string{"sh", "-c", fmt.Sprintf(
					`runsc exec %s curl -s --max-time 5 "http://%s:8080"`,
					sandboxID, companionVPNIP)})
			if err != nil {
				return fmt.Errorf("runsc exec failed: %w", err)
			}
			if !strings.Contains(output, "nginx") && !strings.Contains(output, "Welcome") {
				return fmt.Errorf("unexpected response: %s", strings.TrimSpace(output))
			}
			return nil
		}, "120s", "10s").Should(Succeed(), "gvisor sandbox should reach companion via overlay")
	})

	// ─── Scenario 3: Companion can reach gVisor sandbox ───
	It("companion can reach gvisor sandbox test-agent via overlay", func() {
		// The gVisor sandbox has an overlay IP. The companion (kernel WG)
		// should be able to reach it. The test-agent inside the gVisor
		// container is sleep-ing, but the overlay IP should be pingable.
		sandboxVPNIP := waitForWGIP(latticeClient, testNS, sandboxPeerName, "60s")
		Expect(sandboxVPNIP).NotTo(BeEmpty(), "gvisor sandbox should get an overlay IP")
		GinkgoWriter.Printf("[gvisor e2e] sandbox VPN IP: %s\n", sandboxVPNIP)

		// Companion → sandbox: verify WireGuard route exists
		cPodName := waitForPodRunningReady(clientset, testNS, companionName, "10s")
		output, err := execInContainer(clientset, restConfig, testNS, cPodName, "agent",
			[]string{"sh", "-c", fmt.Sprintf(
				`ping -c 1 -W 3 %s 2>&1`, sandboxVPNIP)})
		Expect(err).NotTo(HaveOccurred(), "ping from companion to sandbox failed: %s", output)
		GinkgoWriter.Printf("[gvisor e2e] ping result: %s\n", output)
	})
})

// gvisorValidationArgs holds the flags passed to `lattice sandbox start`.
type gvisorValidationArgs struct {
	mode         string
	name         string
	serverURL    string
	token        string
	agentRootFS  string
	agentBinary  string
	agentArgs    []string
	egressAllow  string
	egressDeny   bool
	proxyAddr    string
	forwardRules []string
}

// gvisorValidationPod returns a Pod spec that runs `lattice sandbox start`
// with the given gvisor mode flags. The pod is configured to restart never
// so we can read its termination message.
func gvisorValidationPod(image, podName string, args gvisorValidationArgs) *corev1.Pod {
	cmd := []string{
		"/app/lattice", "sandbox", "start",
		"--mode", args.mode,
		"--name", args.name,
		"--server-url", args.serverURL,
		"--token", args.token,
	}
	if args.agentRootFS != "" {
		cmd = append(cmd, "--agent-rootfs", args.agentRootFS)
	}
	if args.agentBinary != "" {
		cmd = append(cmd, "--agent-binary", args.agentBinary)
	}
	for _, a := range args.agentArgs {
		cmd = append(cmd, "--agent-args", a)
	}
	if args.egressAllow != "" {
		cmd = append(cmd, "--egress-allow", args.egressAllow)
	}
	if args.egressDeny {
		cmd = append(cmd, "--egress-default-deny")
	}
	if args.proxyAddr != "" {
		cmd = append(cmd, "--proxy-addr", args.proxyAddr)
	}
	for _, f := range args.forwardRules {
		cmd = append(cmd, "--forward", f)
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				"app":     "wf-e2e",
				"wf-role": "gvisor-validation",
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:            "sandbox",
				Image:           image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         cmd,
			}},
		},
	}
}

// podLogs reads the container logs from a terminated pod. Returns "" if
// the pod hasn't terminated yet or logs are not yet available.
func podLogs(ns, podName string) string {
	req := clientset.CoreV1().Pods(ns).GetLogs(podName, &corev1.PodLogOptions{})
	stream, err := req.Stream(context.Background())
	if err != nil {
		return ""
	}
	defer stream.Close()
	data, _ := io.ReadAll(stream)
	return string(data)
}

// deletePod removes a pod, ignoring errors (best-effort cleanup).
func deletePod(ns, podName string) {
	clientset.CoreV1().Pods(ns).Delete(context.Background(), podName, metav1.DeleteOptions{}) //nolint:errcheck
}

// createE2ETestNamespace creates a dedicated namespace for gvisor validation tests.
func createE2ETestNamespace() string {
	ts := fmt.Sprintf("%d", time.Now().UnixMilli())
	ns := fmt.Sprintf("wf-test-gvisor-%s", ts)
	_, err := clientset.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: ns},
	}, metav1.CreateOptions{})
	if err != nil {
		// Return the name anyway; the caller will skip if pods can't be created.
		GinkgoWriter.Printf("Failed to create namespace %s: %v\n", ns, err)
	}
	return ns
}

// cleanupNamespace deletes the namespace (cascading delete of all resources).
func cleanupNamespace(ns string) {
	if ns == "" {
		return
	}
	clientset.CoreV1().Namespaces().Delete(context.Background(), ns, metav1.DeleteOptions{}) //nolint:errcheck
}

// ─── GVisor integration helpers ───

// runscAvailable checks whether runsc is present inside the given sandbox image
// by running `runsc --version` in a short-lived test pod.
func runscAvailable(image string) bool {
	if image == "" {
		return false
	}
	ts := fmt.Sprintf("%d", time.Now().UnixMilli())
	podName := "gvisor-check-runsc-" + ts
	ns := createE2ETestNamespace()
	if ns == "" {
		return false
	}
	defer cleanupNamespace(ns)

	_, err := clientset.CoreV1().Pods(ns).Create(context.Background(),
		gvisorCheckPod(image, podName), metav1.CreateOptions{})
	if err != nil {
		return false
	}
	defer deletePod(ns, podName)

	// Wait for the pod to complete or timeout.
	var logs string
	Eventually(func() string {
		logs = podLogs(ns, podName)
		return logs
	}, "30s", "2s").ShouldNot(BeEmpty())

	return strings.Contains(logs, "runsc version")
}

// gvisorCheckPod returns a pod spec that runs `runsc --version`.
func gvisorCheckPod(image, podName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				"app":     "wf-e2e",
				"wf-role": "gvisor-check",
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:            "check",
				Image:           image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         []string{"runsc", "--version"},
			}},
		},
	}
}

// deployGvisorSandboxPod creates a Pod that runs `lattice sandbox start --mode gvisor`.
// The pod must be privileged for runsc to create gVisor sandboxes.
func deployGvisorSandboxPod(clientset *kubernetes.Clientset, ns, name, sandboxImage, serverURL, enrollmentToken, sandboxID string, hostAliases []corev1.HostAlias) {
	gvRootfsDir := "/tmp/lattice-gvisor-rootfs"
	args := []string{
		"/app/lattice", "sandbox", "start",
		"--mode", "gvisor",
		"--name", sandboxID,
		"--server-url", serverURL,
		"--token", enrollmentToken,
		"--agent-rootfs", gvRootfsDir,
		"--agent-binary", "/usr/local/bin/test-agent",
	}

	privileged := true
	_, err := clientset.CoreV1().Pods(ns).Create(context.Background(), &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app":     "wf-e2e",
				"wf-role": name,
			},
		},
		Spec: corev1.PodSpec{
			Hostname:    name,
			HostAliases: hostAliases,
			Volumes: []corev1.Volume{
				{
					Name: "lattice-config",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "gvisor-rootfs",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: gvRootfsDir,
							Type: ptrHostPathDir(),
						},
					},
				},
			},
			Containers: []corev1.Container{{
				Name:            "sandbox",
				Image:           sandboxImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				SecurityContext: &corev1.SecurityContext{
					Privileged: &privileged,
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "lattice-config",
						MountPath: "/etc/lattice",
					},
					{
						Name:      "gvisor-rootfs",
						MountPath: gvRootfsDir,
					},
				},
				Command: args,
			}},
		},
	}, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred(), "failed to create gVisor sandbox Pod %s", name)
}

// getServiceClusterIP returns the ClusterIP of the given K8s service.
func getServiceClusterIP(namespace, svcName string) string {
	svc, err := clientset.CoreV1().Services(namespace).Get(
		context.Background(), svcName, metav1.GetOptions{})
	if err != nil || svc == nil {
		return ""
	}
	return svc.Spec.ClusterIP
}

// ptrHostPathDir returns a pointer to corev1.HostPathDirectoryOrCreate.
func ptrHostPathDir() *corev1.HostPathType {
	t := corev1.HostPathDirectoryOrCreate
	return &t
}
