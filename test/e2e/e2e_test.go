package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/pkg/utils/resp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	sigclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	podA = "pod-a"
	podB = "pod-b"
)

var _ = Describe("Lattice Core Connectivity E2E", Ordered, func() {
	var (
		accessToken string
		workspaceId string
		joinToken   string
		httpClient  = &http.Client{Timeout: 15 * time.Second}
		ctx         = context.Background()
	)

	// Collect diagnostic logs on failure to aid troubleshooting
	AfterAll(func() {
		if CurrentSpecReport().Failed() {
			collectDiagnostics(ctx, ns)
		}
	})

	It("Full chain: Login -> Create Workspace -> Generate Token -> Deploy Pod -> Verify tunnel connectivity", func() {

		By("Step 1: Login to Manager, obtain Admin Access Token")
		loginBody, _ := json.Marshal(map[string]string{"username": "admin", "password": "123456"})
		respLogin, err := httpClient.Post(manageUrl+"/api/v1/users/login", "application/json", bytes.NewBuffer(loginBody))
		Expect(err).NotTo(HaveOccurred(), "login request failed")
		defer respLogin.Body.Close() //nolint:errcheck

		var loginData resp.Response
		Expect(json.NewDecoder(respLogin.Body).Decode(&loginData)).To(Succeed())
		Expect(respLogin.StatusCode).To(Equal(http.StatusOK), "login API returned non-200")

		dataMap, ok := loginData.Data.(map[string]any)
		Expect(ok).To(BeTrue(), "login response Data format error")
		accessToken, ok = dataMap["token"].(string)
		Expect(ok && accessToken != "").To(BeTrue(), "token not found in login response")

		wsName := fmt.Sprintf("e2e-%d", time.Now().UnixMilli())
		By("Step 2: Create Workspace (Namespace: " + ns + ", Name: " + wsName + ")")
		wsBody, _ := json.Marshal(dto.WorkspaceDto{
			Namespace:   ns,
			DisplayName: wsName,
			Slug:        wsName,
		})
		reqWs, _ := http.NewRequestWithContext(ctx, http.MethodPost, manageUrl+"/api/v1/workspaces/add", bytes.NewBuffer(wsBody))
		reqWs.Header.Set("Authorization", "Bearer "+accessToken)
		reqWs.Header.Set("Content-Type", "application/json")

		respWs, err := httpClient.Do(reqWs)
		Expect(err).NotTo(HaveOccurred(), "create Workspace request failed")
		defer respWs.Body.Close() //nolint:errcheck

		var wsData resp.Response
		Expect(json.NewDecoder(respWs.Body).Decode(&wsData)).To(Succeed())

		wsMap, ok := wsData.Data.(map[string]any)
		Expect(ok).To(BeTrue(), "Workspace response Data format error")
		workspaceId, ok = wsMap["id"].(string)
		Expect(ok && workspaceId != "").To(BeTrue(), "workspace id not found in Workspace response")

		ns = fmt.Sprintf("wf-%s", workspaceId)

		By("Step 3: Generate Agent Join Token for Workspace")
		reqTk, _ := http.NewRequestWithContext(ctx, http.MethodPost, manageUrl+"/api/v1/token/generate", nil)
		reqTk.Header.Set("Authorization", "Bearer "+accessToken)
		reqTk.Header.Set("X-workspace-id", workspaceId)

		respTk, err := httpClient.Do(reqTk)
		Expect(err).NotTo(HaveOccurred(), "generate Token request failed")
		defer respTk.Body.Close() //nolint:errcheck

		var tkData resp.Response
		Expect(json.NewDecoder(respTk.Body).Decode(&tkData)).To(Succeed())

		tkMap, ok := tkData.Data.(map[string]any)
		Expect(ok).To(BeTrue(), "Token response Data format error")
		joinToken, ok = tkMap["token"].(string)
		Expect(ok && joinToken != "").To(BeTrue(), "token not found in Token response")

		By("Step 4: Find NATS Service ClusterIP and create test Deployment with privileged access and kernel module mounts")
		svc, err := clientset.CoreV1().Services("lattice-system").Get(ctx, "lattice-nats-service", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred(), "lattice-nats-service not found")

		hostAliases := []corev1.HostAlias{{
			IP:        svc.Spec.ClusterIP,
			Hostnames: []string{"signaling.alattice.io"},
		}}

		privileged := true
		replicas := int32(1)
		hostPathType := corev1.HostPathDirectory

		for _, name := range []string{podA, podB} {
			role := name
			_, err := clientset.AppsV1().Deployments(ns).Create(ctx, &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"wf-role": role},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app":     "wf-e2e",
								"wf-role": role,
							},
						},
						Spec: corev1.PodSpec{
							Hostname:    name,
							HostAliases: hostAliases,
							Containers: []corev1.Container{{
								Name:            "agent",
								Image:           agentImage,
								ImagePullPolicy: corev1.PullIfNotPresent,
								SecurityContext: &corev1.SecurityContext{
									Privileged:               &privileged,
									AllowPrivilegeEscalation: &privileged,
									Capabilities: &corev1.Capabilities{
										Add: []corev1.Capability{"NET_ADMIN", "NET_RAW"},
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "lib-modules",
										MountPath: "/lib/modules",
										ReadOnly:  true,
									},
									{
										Name:      "xtables-lock",
										MountPath: "/run/xtables.lock",
									},
								},
								Command: []string{
									"/app/lattice", "up",
									"--token", joinToken,
									"--level", "debug",
									"--server-url", "http://lattice-api-service.lattice-system.svc.cluster.local:8080",
								},
							}},
							Volumes: []corev1.Volume{
								{
									Name: "lib-modules",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/lib/modules",
											Type: &hostPathType,
										},
									},
								},
								{
									Name: "xtables-lock",
									VolumeSource: corev1.VolumeSource{
										HostPath: &corev1.HostPathVolumeSource{
											Path: "/run/xtables.lock",
											Type: func() *corev1.HostPathType {
												t := corev1.HostPathFileOrCreate
												return &t
											}(),
										},
									},
								},
							},
						},
					},
				},
			}, metav1.CreateOptions{})

			Expect(err).NotTo(HaveOccurred(), "failed to create Deployment %s", name)
		}

		By("Step 5: Wait for both Deployment Pods to be Running and all containers Ready (max 180s)")
		for _, role := range []string{podA, podB} {
			Eventually(func() error {
				pods, err := clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
					LabelSelector: "wf-role=" + role,
				})
				if err != nil {
					return err
				}
				if len(pods.Items) == 0 {
					return fmt.Errorf("waiting for Pod %s to be scheduled", role)
				}
				pod := pods.Items[0]
				if pod.Status.Phase != corev1.PodRunning {
					return fmt.Errorf("Pod %s phase is %s, expected Running", pod.Name, pod.Status.Phase)
				}
				for _, cs := range pod.Status.ContainerStatuses {
					if !cs.Ready {
						return fmt.Errorf("Pod %s container %s not Ready yet (restarts=%d)", pod.Name, cs.Name, cs.RestartCount)
					}
				}
				return nil
			}, "180s", "3s").Should(Succeed(), "Deployment %s Pod failed to reach Running+Ready status", role)
		}

		// Get the actual Pod names for both Deployments for use in subsequent steps
		getPodName := func(role string) string {
			pods, err := clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
				LabelSelector: "wf-role=" + role,
			})
			Expect(err).NotTo(HaveOccurred(), "failed to list Pods for %s", role)
			Expect(pods.Items).NotTo(BeEmpty(), "Pod not found for %s", role)
			return pods.Items[0].Name
		}
		podAName := getPodName(podA)
		podBName := getPodName(podB)

		By("Step 6: Wait for control plane to assign WireGuard virtual IP for " + podA + " and " + podB + " (LatticePeer CRD)")
		var podBWGIP string
		for _, peerName := range []string{podA, podB} {
			name := peerName
			Eventually(func() error {
				peer := &v1alpha1.LatticePeer{}
				if err := latticeClient.Get(ctx, sigclient.ObjectKey{Namespace: ns, Name: name}, peer); err != nil {
					return fmt.Errorf("LatticePeer %s not yet created: %w", name, err)
				}
				if peer.Status.AllocatedAddress == nil || *peer.Status.AllocatedAddress == "" {
					return fmt.Errorf("LatticePeer %s created but control plane has not assigned address yet", name)
				}
				if name == podB {
					podBWGIP = *peer.Status.AllocatedAddress
					// The address may contain a CIDR prefix (e.g. "10.0.0.2/24"), ping only needs the IP part
					if idx := strings.Index(podBWGIP, "/"); idx != -1 {
						podBWGIP = podBWGIP[:idx]
					}
				}
				return nil
			}, "90s", "3s").Should(Succeed(), "timed out waiting for WireGuard IP of %s", name)
		}

		By("Step 7: Create LatticePolicy allowing pod-a ↔ pod-b communication")
		peerB := &v1alpha1.LatticePeer{}
		Expect(latticeClient.Get(ctx, sigclient.ObjectKey{Namespace: ns, Name: podB}, peerB)).To(Succeed())

		var networkName string
		if peerB.Spec.Network != nil && *peerB.Spec.Network != "" {
			networkName = *peerB.Spec.Network
		} else if peerB.Status.ActiveNetwork != nil {
			networkName = *peerB.Status.ActiveNetwork
		}
		Expect(networkName).NotTo(BeEmpty(), "failed to get network name from LatticePeer")

		networkLabel := fmt.Sprintf("alattice.io/network-%s", networkName)
		peerNetSelector := metav1.LabelSelector{
			MatchLabels: map[string]string{networkLabel: "true"},
		}
		allowPolicy := &v1alpha1.LatticePolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "e2e-allow-all",
				Namespace: ns,
			},
			Spec: v1alpha1.LatticePolicySpec{
				Network:      networkName,
				PeerSelector: peerNetSelector,
				Action:       "ALLOW",
				Ingress: []v1alpha1.IngressRule{
					{From: []v1alpha1.PeerSelection{{PeerSelector: &peerNetSelector}}},
				},
				Egress: []v1alpha1.EgressRule{
					{To: []v1alpha1.PeerSelection{{PeerSelector: &peerNetSelector}}},
				},
			},
		}
		Expect(latticeClient.Create(ctx, allowPolicy)).To(Succeed(), "failed to create LatticePolicy")

		By(fmt.Sprintf("Step 8: Verify tunnel connectivity (%s → %s @ %s)", podAName, podBName, podBWGIP))
		Eventually(func() error {
			output, err := execInPod(clientset, restConfig, ns, podAName, []string{"ping", "-c", "3", "-W", "2", podBWGIP})
			if err != nil {
				return fmt.Errorf("ping execution failed: %w", err)
			}
			if !strings.Contains(output, "0% packet loss") {
				return fmt.Errorf("ping has packet loss: %s", output)
			}
			return nil
		}, "60s", "5s").Should(Succeed(), "tunnel connectivity verification failed")
	})

	It("ACL rule verification: DENY blocking + port-level control + default-deny", func() {
		// Reuse variables from step 6 (within the same Describe scope)
		// Re-fetch necessary data
		peerA := &v1alpha1.LatticePeer{}
		Expect(latticeClient.Get(ctx, sigclient.ObjectKey{Namespace: ns, Name: podA}, peerA)).To(Succeed())
		peerB := &v1alpha1.LatticePeer{}
		Expect(latticeClient.Get(ctx, sigclient.ObjectKey{Namespace: ns, Name: podB}, peerB)).To(Succeed())

		var podAIP, podBIP string
		if peerA.Status.AllocatedAddress != nil {
			podAIP = *peerA.Status.AllocatedAddress
			if idx := strings.Index(podAIP, "/"); idx != -1 {
				podAIP = podAIP[:idx]
			}
		}
		if peerB.Status.AllocatedAddress != nil {
			podBIP = *peerB.Status.AllocatedAddress
			if idx := strings.Index(podBIP, "/"); idx != -1 {
				podBIP = podBIP[:idx]
			}
		}
		Expect(podAIP).NotTo(BeEmpty(), "failed to get WireGuard IP of pod-a")
		Expect(podBIP).NotTo(BeEmpty(), "failed to get WireGuard IP of pod-b")

		var networkName string
		if peerB.Spec.Network != nil && *peerB.Spec.Network != "" {
			networkName = *peerB.Spec.Network
		} else if peerB.Status.ActiveNetwork != nil {
			networkName = *peerB.Status.ActiveNetwork
		}
		Expect(networkName).NotTo(BeEmpty(), "failed to get network name from LatticePeer")

		networkLabel := fmt.Sprintf("alattice.io/network-%s", networkName)
		peerNetSelector := metav1.LabelSelector{
			MatchLabels: map[string]string{networkLabel: "true"},
		}

		getPodName := func(role string) string {
			pods, err := clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
				LabelSelector: "wf-role=" + role,
			})
			Expect(err).NotTo(HaveOccurred(), "failed to list Pods for %s", role)
			Expect(pods.Items).NotTo(BeEmpty(), "Pod not found for %s", role)
			return pods.Items[0].Name
		}
		podAName := getPodName(podA)
		podBName := getPodName(podB)

		By("ACL-1: Verify ping works under e2e-allow-all policy (baseline)")
		output, err := execInPod(clientset, restConfig, ns, podAName, []string{"ping", "-c", "3", "-W", "2", podBIP})
		Expect(err).NotTo(HaveOccurred(), "baseline ping failed")
		Expect(strings.Contains(output, "0% packet loss")).To(BeTrue(), "baseline ping has packet loss: %s", output)

		By("ACL-2: Update policy to DENY, verify pod-a → pod-b is blocked")
		allowPolicy := &v1alpha1.LatticePolicy{}
		Expect(latticeClient.Get(ctx, sigclient.ObjectKey{Namespace: ns, Name: "e2e-allow-all"}, allowPolicy)).To(Succeed())

		// Modify the policy to DENY (keep PeerSelector and rules unchanged)
		allowPolicy.Spec.Action = "DENY"
		Expect(latticeClient.Update(ctx, allowPolicy)).To(Succeed(), "failed to update LatticePolicy to DENY")

		// Wait for policy to take effect (agent receives updates via NATS and refreshes iptables)
		Eventually(func() error {
			out, execErr := execInPod(clientset, restConfig, ns, podAName, []string{"ping", "-c", "3", "-W", "2", podBIP})
			if execErr != nil {
				// ping command itself failed (e.g. sendto: EPERM) — blocked
				return nil
			}
			if strings.Contains(out, "0% packet loss") {
				return fmt.Errorf("DENY policy not effective, ping still works: %s", out)
			}
			return nil
		}, "30s", "2s").Should(Succeed(), "DENY policy should block ping within 30s")

		By("ACL-3: Restore ALLOW + port-level rules, only allow TCP 8080")
		allowPolicy.Spec.Action = "ALLOW"
		allowPolicy.Spec.Ingress = []v1alpha1.IngressRule{
			{
				From: []v1alpha1.PeerSelection{{PeerSelector: &peerNetSelector}},
				Ports: []v1alpha1.NetworkPolicyPort{
					{Port: 8080, Protocol: "tcp"},
				},
			},
		}
		allowPolicy.Spec.Egress = []v1alpha1.EgressRule{
			{
				To: []v1alpha1.PeerSelection{{PeerSelector: &peerNetSelector}},
				Ports: []v1alpha1.NetworkPolicyPort{
					{Port: 8080, Protocol: "tcp"},
				},
			},
		}
		Expect(latticeClient.Update(ctx, allowPolicy)).To(Succeed(), "failed to update port-level policy")

		// Wait for the policy to take effect
		Eventually(func() error {
			out, execErr := execInPod(clientset, restConfig, ns, podAName, []string{"ping", "-c", "3", "-W", "2", podBIP})
			if execErr != nil {
				// ping command itself failed (e.g. sendto: EPERM) — iptables blocking at low level
				return nil
			}
			if !strings.Contains(out, "0% packet loss") {
				// ping succeeded but got no replies — policy blocked ICMP
				return nil
			}
			return fmt.Errorf("ping should still be blocked under port-level policy: %s", out)
		}, "30s", "2s").Should(Succeed(), "port-level policy should block ICMP")

		By("ACL-4: Verify iptables rules allow TCP 8080 (in LATTICE-EGRESS chain)")
		Eventually(func() error {
			out, execErr := execInPod(clientset, restConfig, ns, podAName, []string{"iptables", "-L", "LATTICE-EGRESS", "-n"})
			if execErr != nil {
				return execErr
			}
			if !strings.Contains(out, "tcp dpt:8080") {
				return fmt.Errorf("LATTICE-EGRESS does not contain TCP 8080 rule:\n%s", out)
			}
			return nil
		}, "15s", "2s").Should(Succeed(), "iptables should contain TCP 8080 allow rule")

		By("ACL-5: Verify iptables does not have TCP 9999 rule")
		out, err := execInPod(clientset, restConfig, ns, podAName, []string{"sh", "-c", "iptables -L LATTICE-EGRESS -n 2>/dev/null | grep 9999 || true"})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(BeEmpty(), "should not have TCP 9999 iptables rule")

		By("ACL-6: Verify IPBlock CIDR rule — only allow pod-b's exact CIDR")
		// Reuse the existing allowPolicy, update in-place with CIDR rules (avoiding delete+create race)
		Expect(latticeClient.Get(ctx, sigclient.ObjectKey{Namespace: ns, Name: "e2e-allow-all"}, allowPolicy)).To(Succeed())
		podBCIDR := podBIP + "/32"
		podACIDR := podAIP + "/32"
		allowPolicy.Spec.Action = "ALLOW"
		// Need to include CIDRs for both directions: ingress allows IPs from both sides, egress allows IPs to both sides
		// This way the policy applied to pod-a/pod-b can each match the other's IP
		allowPolicy.Spec.Ingress = []v1alpha1.IngressRule{
			{From: []v1alpha1.PeerSelection{
				{IPBlock: &v1alpha1.IPBlock{CIDR: podACIDR}},
				{IPBlock: &v1alpha1.IPBlock{CIDR: podBCIDR}},
			}},
		}
		allowPolicy.Spec.Egress = []v1alpha1.EgressRule{
			{To: []v1alpha1.PeerSelection{
				{IPBlock: &v1alpha1.IPBlock{CIDR: podACIDR}},
				{IPBlock: &v1alpha1.IPBlock{CIDR: podBCIDR}},
			}},
		}
		Expect(latticeClient.Update(ctx, allowPolicy)).To(Succeed(), "failed to update CIDR policy")

		// First verify that iptables rules have been updated to CIDR (separate "policy not effective" from "policy effective but ping fails")
		Eventually(func() error {
			out, execErr := execInPod(clientset, restConfig, ns, podAName, []string{"iptables", "-L", "LATTICE-EGRESS", "-n"})
			if execErr != nil {
				return execErr
			}
			// iptables does not show /32 suffix, so use podBIP (without /32) for matching
			if !strings.Contains(out, podBIP) {
				return fmt.Errorf("LATTICE-EGRESS does not contain CIDR allow rule for %s:\n%s", podBIP, out)
			}
			return nil
		}, "15s", "2s").Should(Succeed(), "iptables should contain CIDR allow rule")

		By("ACL-6: Verify WireGuard handshake established (ensure tunnel is not interrupted)")
		Eventually(func() error {
			out, err := execInPod(clientset, restConfig, ns, podAName, []string{"wg", "show"})
			if err != nil {
				return err
			}
			if !strings.Contains(out, "latest handshake") {
				return fmt.Errorf("WireGuard handshake not yet completed:\n%s", out)
			}
			return nil
		}, "15s", "2s").Should(Succeed(), "WG handshake should have completed under CIDR policy")

		Eventually(func() error {
			out, execErr := execInPod(clientset, restConfig, ns, podAName, []string{"ping", "-c", "3", "-W", "2", podBIP})
			if execErr != nil {
				diag := collectBothPodsDiagnostics(clientset, restConfig, ns, podAName, podBName, podBIP)
				return fmt.Errorf("ping execution failed: %w\nDiagnostic summary: %s", execErr, diag)
			}
			if !strings.Contains(out, "0% packet loss") {
				return fmt.Errorf("ping should work under CIDR rule: %s", out)
			}
			return nil
		}, "30s", "2s").Should(Succeed(), "CIDR policy should allow pod-b's IP")

		By("ACL-7: Replace CIDR with non-matching subnet, verify traffic is blocked")
		wrongCIDR := "192.168.99.0/24"
		// Re-get to avoid resourceVersion conflict
		Expect(latticeClient.Get(ctx, sigclient.ObjectKey{Namespace: ns, Name: "e2e-allow-all"}, allowPolicy)).To(Succeed())
		allowPolicy.Spec.Egress = []v1alpha1.EgressRule{
			{To: []v1alpha1.PeerSelection{{IPBlock: &v1alpha1.IPBlock{CIDR: wrongCIDR}}}},
		}
		Expect(latticeClient.Update(ctx, allowPolicy)).To(Succeed(), "failed to update CIDR policy")

		Eventually(func() error {
			out, execErr := execInPod(clientset, restConfig, ns, podAName, []string{"ping", "-c", "3", "-W", "2", podBIP})
			if execErr != nil {
				// ping command itself failed (e.g. sendto: EPERM) — blocked
				return nil
			}
			if strings.Contains(out, "0% packet loss") {
				return fmt.Errorf("ping should still be blocked under wrong CIDR: %s", out)
			}
			return nil
		}, "30s", "2s").Should(Succeed(), "non-matching CIDR should block traffic")

		By("ACL test completed, cleaning up policy")
		_ = latticeClient.Delete(ctx, allowPolicy)
	})
})

// collectDiagnostics prints key logs on test failure to aid CI debugging
func collectDiagnostics(ctx context.Context, namespace string) {
	w := GinkgoWriter
	fprintf := func(format string, args ...any) { fmt.Fprintf(w, format, args...) } //nolint:errcheck

	fprintf("\n========== E2E Diagnostic Logs [ns=%s] ==========\n", namespace)

	// ── 1. LatticePeer CRD status ────────────────────────────────────────
	fprintf("\n[LatticePeer Status]\n")
	var peerList v1alpha1.LatticePeerList
	if err := latticeClient.List(ctx, &peerList, sigclient.InNamespace(namespace)); err != nil {
		fprintf("  [WARN] failed to list LatticePeer: %v\n", err)
	} else {
		for _, p := range peerList.Items {
			addr := "<nil>"
			if p.Status.AllocatedAddress != nil {
				addr = *p.Status.AllocatedAddress
			}
			activeNet := "<nil>"
			if p.Status.ActiveNetwork != nil {
				activeNet = *p.Status.ActiveNetwork
			}
			fprintf("  %-20s  phase=%-12s  ip=%-18s  activeNetwork=%-30s  hash=%s\n",
				p.Name, p.Status.Phase, addr, activeNet, p.Status.CurrentHash)
			for _, c := range p.Status.Conditions {
				fprintf("    condition %-25s  status=%-5s  reason=%-20s  msg=%s\n",
					c.Type, c.Status, c.Reason, c.Message)
			}
		}
	}

	// ── 2. LatticeNetwork status ─────────────────────────────────────────
	fprintf("\n[LatticeNetwork Status]\n")
	var netList v1alpha1.LatticeNetworkList
	if err := latticeClient.List(ctx, &netList, sigclient.InNamespace(namespace)); err != nil {
		fprintf("  [WARN] failed to list LatticeNetwork: %v\n", err)
	} else {
		for _, n := range netList.Items {
			fprintf("  %-30s  phase=%-10s  activeCIDR=%-20s  allocatedCount=%d\n",
				n.Name, n.Status.Phase, n.Status.ActiveCIDR, n.Status.AllocatedCount)
		}
	}

	// ── 3. ConfigMap contents (agent config) ─────────────────────────────
	fprintf("\n[ConfigMap Contents]\n")
	cms, err := clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=lattice-controller",
	})
	if err != nil {
		fprintf("  [WARN] failed to list ConfigMap: %v\n", err)
	} else {
		for _, cm := range cms.Items {
			fprintf("\n  --- ConfigMap: %s ---\n", cm.Name)
			for k, v := range cm.Data {
				fprintf("  [%s]\n%s\n", k, v)
			}
		}
	}

	// ── 4. Pod logs + WireGuard / Network status ─────────────────────────
	fprintf("\n[Pod Logs and Network Status]\n")
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		fprintf("  [WARN] failed to list Pod: %v\n", err)
	} else {
		for _, pod := range pods.Items {
			fprintf("\n--- Pod: %s  Phase: %s ---\n", pod.Name, pod.Status.Phase)
			for _, cs := range pod.Status.ContainerStatuses {
				fprintf("  Container %s: ready=%v restarts=%d\n", cs.Name, cs.Ready, cs.RestartCount)
			}

			if pod.Status.Phase == corev1.PodRunning {
				// lattice status: WireGuard tunnel connection status (peer handshake, traffic)
				if out, err := execInPod(clientset, restConfig, namespace, pod.Name,
					[]string{"/app/lattice", "status"}); err != nil {
					fprintf("  [lattice status] execution failed: %v\n", err)
				} else {
					fprintf("  [lattice status]\n%s\n", out)
				}

				// ip addr: confirm wf0 interface exists and its IP
				if out, err := execInPod(clientset, restConfig, namespace, pod.Name,
					[]string{"ip", "addr", "show"}); err != nil {
					fprintf("  [ip addr] execution failed: %v\n", err)
				} else {
					fprintf("  [ip addr]\n%s\n", out)
				}

				// ip route: routing table
				if out, err := execInPod(clientset, restConfig, namespace, pod.Name,
					[]string{"ip", "route", "show"}); err != nil {
					fprintf("  [ip route] execution failed: %v\n", err)
				} else {
					fprintf("  [ip route]\n%s\n", out)
				}
			}

			// Container logs (last 150 lines)
			tailLines := int64(150)
			logReq := clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
				TailLines: &tailLines,
			})
			logStream, err := logReq.Stream(ctx)
			if err != nil {
				fprintf("  [WARN] failed to get logs: %v\n", err)
				continue
			}
			var buf bytes.Buffer
			_, _ = buf.ReadFrom(logStream)
			_ = logStream.Close()
			fprintf("  [agent log]\n%s\n", buf.String())
		}
	}

	fprintf("===========================================\n")
}

// collectPodDiagnostics collects iptables, wg, and routing info from a pod for debugging
func collectPodDiagnostics(c *kubernetes.Clientset, config *rest.Config, namespace, podName, targetIP string) string {
	var buf bytes.Buffer
	cmds := []struct {
		name string
		args []string
	}{
		{"LATTICE-EGRESS", []string{"iptables", "-L", "LATTICE-EGRESS", "-n"}},
		{"LATTICE-INGRESS", []string{"iptables", "-L", "LATTICE-INGRESS", "-n"}},
		{"WG", []string{"wg", "show"}},
		{"WG dump", []string{"wg", "show", "wf0", "dump"}},
		{"WF0 stats", []string{"sh", "-c", "ip -s link show wf0"}},
		{"ROUTE", []string{"ip", "route", "get", targetIP}},
		{"ALL iptables", []string{"sh", "-c", "iptables -S; iptables -t nat -S"}},
		{"lattice status", []string{"/app/lattice", "status"}},
	}
	for _, cmd := range cmds {
		out, err := execInPod(c, config, namespace, podName, cmd.args)
		if err != nil {
			fmt.Fprintf(&buf, "[%s] error: %v\n", cmd.name, err)
		} else {
			fmt.Fprintf(&buf, "--- %s ---\n%s\n", cmd.name, out)
		}
	}

	// Collect agent container logs (last 50 lines)
	tailLines := int64(50)
	logReq := c.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		TailLines: &tailLines,
	})
	logStream, err := logReq.Stream(context.Background())
	if err != nil {
		fmt.Fprintf(&buf, "[agent log] error: %v\n", err)
	} else {
		var logBuf bytes.Buffer
		_, _ = logBuf.ReadFrom(logStream)
		_ = logStream.Close()
		fmt.Fprintf(&buf, "--- agent log (last 50 lines) ---\n%s\n", logBuf.String())
	}

	return buf.String()
}

// collectBothPodsDiagnostics collects diagnostics from both pods, writes to
// GinkgoWriter (avoids Gomega truncation), and returns a short summary.
func collectBothPodsDiagnostics(c *kubernetes.Clientset, config *rest.Config, namespace, srcPod, dstPod, targetIP string) string {
	w := GinkgoWriter
	fmt.Fprintf(w, "\n========== Pod diagnostics: %s ==========\n", srcPod) //nolint:errcheck
	diagA := collectPodDiagnostics(c, config, namespace, srcPod, targetIP)
	fmt.Fprintf(w, "%s\n", diagA) //nolint:errcheck

	fmt.Fprintf(w, "\n========== Pod diagnostics: %s ==========\n", dstPod) //nolint:errcheck
	diagB := collectPodDiagnostics(c, config, namespace, dstPod, targetIP)
	fmt.Fprintf(w, "%s\n", diagB) //nolint:errcheck

	return fmt.Sprintf("diagnostics written to GinkgoWriter for pods %s and %s", srcPod, dstPod)
}
