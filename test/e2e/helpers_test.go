package e2e

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	latticev1 "github.com/alatticeio/lattice/api/v1alpha1"
	"github.com/alatticeio/lattice/internal/server/dto"
	"github.com/alatticeio/lattice/pkg/utils/resp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	sigclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// ---- HTTP helpers ----

// apiPOST performs an HTTP POST with optional Bearer auth and JSON body.
// Returns HTTP status code and parsed response body. Response body is consumed
// and closed inside this function. The raw request and response are always
// logged to GinkgoWriter to aid debugging on failure.
func apiPOST(url, token string, body any) (int, *resp.Response) {
	var reqBody []byte
	if body != nil {
		reqBody, _ = json.Marshal(body)
	}
	GinkgoWriter.Printf("[apiPOST] POST %s  body=%s\n", url, reqBody)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewBuffer(reqBody))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	httpClient := &http.Client{Timeout: 15 * time.Second}
	httpResp, err := httpClient.Do(req)
	Expect(err).NotTo(HaveOccurred(), "HTTP POST %s failed", url)
	defer httpResp.Body.Close() //nolint:errcheck

	rawBody, _ := io.ReadAll(httpResp.Body)
	GinkgoWriter.Printf("[apiPOST] %d  raw=%s\n", httpResp.StatusCode, rawBody)

	var data resp.Response
	_ = json.Unmarshal(rawBody, &data)
	return httpResp.StatusCode, &data
}

// apiDELETE sends an HTTP DELETE with optional Bearer auth. Returns status code and parsed response body.
func apiDELETE(url, token string) (int, *resp.Response) {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodDelete, url, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	httpClient := &http.Client{Timeout: 15 * time.Second}
	httpResp, err := httpClient.Do(req)
	Expect(err).NotTo(HaveOccurred(), "HTTP DELETE %s failed", url)
	defer httpResp.Body.Close() //nolint:errcheck

	var data resp.Response
	_ = json.NewDecoder(httpResp.Body).Decode(&data)
	return httpResp.StatusCode, &data
}

// ---- Test setup helpers ----

// login returns the admin access token.
func login(manageURL string) string {
	By("Login to Manager to obtain Admin Token")
	statusCode, data := apiPOST(manageURL+"/api/v1/users/login", "", map[string]string{"username": "admin", "password": "123456"})
	Expect(statusCode).To(Equal(http.StatusOK), "login API returned non-200: code=%d msg=%s", statusCode, data.Msg)

	dataMap, ok := data.Data.(map[string]any)
	Expect(ok).To(BeTrue(), "login response Data is not a map: code=%d msg=%q data=%v", data.Code, data.Msg, data.Data)
	token, ok := dataMap["token"].(string)
	Expect(ok && token != "").To(BeTrue(), "token field missing or empty in login response: %v", dataMap)
	return token
}

// createEnrollmentToken creates a one-time enrollment token for agent sandbox registration.
// Calls Skip() automatically if the server reports agent isolation is not enabled (code 402),
// so sandbox tests are skipped gracefully on community builds instead of failing.
func createEnrollmentToken(manageURL, accessToken, namespace string) string {
	By("Create Enrollment Token for sandbox agent")
	statusCode, data := apiPOST(manageURL+"/api/v1/agent-isolation/enrollment-tokens", accessToken, map[string]any{
		"namespace":    namespace,
		"allowedTools": []string{"http", "exec"},
		"ttlSeconds":   600,
	})

	// Community build or feature disabled: skip instead of failing.
	if data.Code == 402 {
		Skip(fmt.Sprintf("agent isolation not available on this server (%s) — skipping sandbox tests", data.Msg))
	}

	Expect(statusCode).To(Equal(http.StatusOK), "create enrollment token failed: code=%d msg=%s", statusCode, data.Msg)

	dataMap, ok := data.Data.(map[string]any)
	Expect(ok).To(BeTrue(), "enrollment token response Data is not a map: code=%d msg=%q data=%v", data.Code, data.Msg, data.Data)
	token, ok := dataMap["token"].(string)
	Expect(ok && token != "").To(BeTrue(), "token field missing or empty in enrollment response: %v", dataMap)
	return token
}

// registerSandboxAgent calls POST /api/v1/agent-isolation/register and
// returns the JWT and allocated VPN IP.
func registerSandboxAgent(manageURL, accessToken, enrollmentToken, agentName, sandboxMode string) (jwt, localIP string) {
	By("Register sandbox agent via enrollment token")

	rawKey := make([]byte, 32)
	_, err := rand.Read(rawKey)
	Expect(err).NotTo(HaveOccurred(), "generate sandbox WG key failed")
	pubKey := hex.EncodeToString(rawKey)

	statusCode, data := apiPOST(manageURL+"/api/v1/agent-isolation/register", accessToken, map[string]string{
		"enrollmentToken": enrollmentToken,
		"agentName":       agentName,
		"publicKey":       pubKey,
		"sandbox":         sandboxMode,
	})
	Expect(statusCode).To(Equal(http.StatusOK), "register sandbox agent failed: %+v", data)

	dataMap, ok := data.Data.(map[string]any)
	Expect(ok).To(BeTrue(), "register response Data format error")
	jwt, _ = dataMap["jwt"].(string)
	return jwt, localIP
}

// createWorkspace creates a workspace and returns the workspace ID.
func createWorkspace(manageURL, accessToken, nsName string) string {
	wsName := fmt.Sprintf("e2e-%d", time.Now().UnixMilli())
	By("Create Workspace: " + wsName)
	statusCode, data := apiPOST(manageURL+"/api/v1/workspaces/add", accessToken, dto.WorkspaceDto{
		Namespace:    nsName,
		DisplayName:  wsName,
		Slug:         wsName,
		MaxNodeCount: 10,
	})
	Expect(statusCode).To(Equal(http.StatusOK), "create Workspace failed: code=%d msg=%s", statusCode, data.Msg)

	dataMap, ok := data.Data.(map[string]any)
	Expect(ok).To(BeTrue(), "workspace response Data is not a map: code=%d msg=%q data=%v", data.Code, data.Msg, data.Data)
	workspaceID, ok := dataMap["id"].(string)
	Expect(ok && workspaceID != "").To(BeTrue(), "workspace id missing or empty in response: %v", dataMap)
	return workspaceID
}

// generateJoinToken generates an agent join token for the given workspace.
func generateJoinToken(manageURL, accessToken, workspaceID string) string {
	By("Generate Agent Join Token")
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, manageURL+"/api/v1/token/generate", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-workspace-id", workspaceID)
	httpClient := &http.Client{Timeout: 15 * time.Second}
	httpResp, err := httpClient.Do(req)
	Expect(err).NotTo(HaveOccurred(), "generate Token request failed")
	defer httpResp.Body.Close() //nolint:errcheck

	var data resp.Response
	Expect(json.NewDecoder(httpResp.Body).Decode(&data)).To(Succeed())

	tkMap, ok := data.Data.(map[string]any)
	Expect(ok).To(BeTrue(), "Token response Data format error")
	token, ok := tkMap["token"].(string)
	Expect(ok && token != "").To(BeTrue(), "token not found in Token response")
	return token
}

// ---- K8s pod helpers ----

// hostAliasesForNATS discovers the NATS service ClusterIP for hostAliases.
func hostAliasesForNATS(clientset *kubernetes.Clientset) []corev1.HostAlias {
	svc, err := clientset.CoreV1().Services("lattice-system").Get(context.Background(), "lattice-nats-service", metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred(), "lattice-nats-service not found")
	return []corev1.HostAlias{{
		IP:        svc.Spec.ClusterIP,
		Hostnames: []string{"signaling.alattice.io"},
	}}
}

// deployAgentDeployment creates a Deployment running the lattice agent.
func deployAgentDeployment(clientset *kubernetes.Clientset, ns, name, agentImage, joinToken string, hostAliases []corev1.HostAlias) {
	privileged := true
	replicas := int32(1)
	hostPathType := corev1.HostPathDirectory

	_, err := clientset.AppsV1().Deployments(ns).Create(context.Background(), &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"wf-role": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":     "wf-e2e",
						"wf-role": name,
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
							{Name: "lib-modules", MountPath: "/lib/modules", ReadOnly: true},
							{Name: "xtables-lock", MountPath: "/run/xtables.lock"},
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

// deployMultiContainerAgent creates a Deployment running the lattice agent plus additional containers.
func deployMultiContainerAgent(clientset *kubernetes.Clientset, ns, name, agentImage, joinToken string, hostAliases []corev1.HostAlias, extraContainers ...corev1.Container) {
	privileged := true
	replicas := int32(1)
	hostPathType := corev1.HostPathDirectory

	containers := []corev1.Container{{
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
			{Name: "lib-modules", MountPath: "/lib/modules", ReadOnly: true},
			{Name: "xtables-lock", MountPath: "/run/xtables.lock"},
		},
		Command: []string{
			"/app/lattice", "up",
			"--token", joinToken,
			"--level", "debug",
			"--server-url", "http://lattice-api-service.lattice-system.svc.cluster.local:8080",
		},
	}}
	containers = append(containers, extraContainers...)

	_, err := clientset.AppsV1().Deployments(ns).Create(context.Background(), &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"wf-role": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":     "wf-e2e",
						"wf-role": name,
					},
				},
				Spec: corev1.PodSpec{
					Hostname:    name,
					HostAliases: hostAliases,
					Containers:  containers,
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

// waitForPodRunningReady waits until a pod matching the role label is Running and all containers Ready.
func waitForPodRunningReady(clientset *kubernetes.Clientset, ns, role string, timeout string) string {
	var podName string
	Eventually(func() error {
		pods, err := clientset.CoreV1().Pods(ns).List(context.Background(), metav1.ListOptions{
			LabelSelector: "wf-role=" + role,
		})
		if err != nil {
			return err
		}
		if len(pods.Items) == 0 {
			return fmt.Errorf("waiting for Pod %s to be scheduled", role)
		}
		var pod *corev1.Pod
		for i := range pods.Items {
			if pods.Items[i].DeletionTimestamp == nil {
				pod = &pods.Items[i]
				break
			}
		}
		if pod == nil {
			return fmt.Errorf("waiting for Pod %s: all current Pods are in Terminating state", role)
		}
		if pod.Status.Phase != corev1.PodRunning {
			return fmt.Errorf("pod %s phase is %s, expected Running", pod.Name, pod.Status.Phase)
		}
		for _, cs := range pod.Status.ContainerStatuses {
			if !cs.Ready {
				return fmt.Errorf("pod %s container %s not Ready yet (restarts=%d)", pod.Name, cs.Name, cs.RestartCount)
			}
		}
		podName = pod.Name
		return nil
	}, timeout, "3s").Should(Succeed(), "Deployment %s Pod failed to reach Running+Ready status", role)
	return podName
}

// getPodName returns the name of the first pod matching the role label.
// waitForWGIP waits for the LatticePeer CRD to have an AllocatedAddress and returns the IP (without CIDR suffix).
func waitForWGIP(latticeClient sigclient.Client, ns, peerName string, timeout string) string {
	var ip string
	Eventually(func() error {
		peer := &latticev1.LatticePeer{}
		if err := latticeClient.Get(context.Background(), sigclient.ObjectKey{Namespace: ns, Name: peerName}, peer); err != nil {
			return fmt.Errorf("LatticePeer %s not yet created: %w", peerName, err)
		}
		if peer.Status.AllocatedAddress == nil || *peer.Status.AllocatedAddress == "" {
			return fmt.Errorf("LatticePeer %s created but control plane has not assigned address yet", peerName)
		}
		ip = *peer.Status.AllocatedAddress
		if idx := strings.Index(ip, "/"); idx != -1 {
			ip = ip[:idx]
		}
		return nil
	}, timeout, "3s").Should(Succeed(), "timed out waiting for WireGuard IP of %s", peerName)
	return ip
}

// getNetworkName extracts the active network name from a LatticePeer.
func getNetworkName(peer *latticev1.LatticePeer) string {
	if peer.Spec.Network != nil && *peer.Spec.Network != "" {
		return *peer.Spec.Network
	}
	if peer.Status.ActiveNetwork != nil {
		return *peer.Status.ActiveNetwork
	}
	return ""
}

// ---- Sandbox helpers ----

// deploySandboxPod creates a Pod running lattice agent sidecar with an init container
// that sets up iptables REDIRECT rules. The sidecar runs as UID 1337 (exempted from
// iptables redirect) and transparently proxies all TCP from the workload container
// through the WireGuard overlay.
//
// The pod includes an nginx workload on port 8080 to serve inbound overlay connections.
func deploySandboxPod(clientset *kubernetes.Clientset, ns, name, sandboxImage, serverURL, enrollmentToken string, hostAliases []corev1.HostAlias) {
	sidecarArgs := []string{
		"/app/lattice", "agent", "sidecar", name,
		"--server-url", serverURL,
		"--token", enrollmentToken,
		// Allow overlay traffic (any 10.x.x.x) and deny everything else.
		"--egress-allow", "10.0.0.0/8",
		"--egress-default-deny",
	}

	runAsUserID := int64(1337)

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
			Volumes: []corev1.Volume{{
				Name: "lattice-config",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}},
			InitContainers: []corev1.Container{
				{
					Name:            "lattice-init",
					Image:           sandboxImage,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         []string{"/app/lattice", "agent", "init"},
					SecurityContext: &corev1.SecurityContext{
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{"NET_ADMIN"},
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:            "lattice-sidecar",
					Image:           sandboxImage,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Ports: []corev1.ContainerPort{{
						ContainerPort: 51820,
						Protocol:      corev1.ProtocolUDP,
					}},
					VolumeMounts: []corev1.VolumeMount{{
						Name:      "lattice-config",
						MountPath: "/etc/lattice",
					}},
					Command: sidecarArgs,
					SecurityContext: &corev1.SecurityContext{
						RunAsUser: &runAsUserID,
					},
				},
				{
					// nginx workload: reachable from the overlay via the transparent proxy.
					Name:            "nginx",
					Image:           "nginx:alpine",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Ports: []corev1.ContainerPort{{
						ContainerPort: 8080,
					}},
					// Override nginx listen port from default 80 to 8080.
					Command: []string{"sh", "-c",
						`sed -i 's/listen\s*80/listen 8080/g' /etc/nginx/conf.d/default.conf && nginx -g 'daemon off;'`,
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred(), "failed to create sandbox Pod %s", name)
}

// revokeSandboxAgent calls DELETE /api/v1/agent-isolation/agents/:name to revoke an agent.
func revokeSandboxAgent(manageURL, accessToken, agentName, namespace string) {
	By("Revoke sandbox agent: " + agentName)
	url := fmt.Sprintf("%s/api/v1/agent-isolation/agents/%s?namespace=%s", manageURL, agentName, namespace)
	statusCode, data := apiDELETE(url, accessToken)
	Expect(statusCode).To(Equal(http.StatusOK), "revoke agent failed: %+v", data)
}

// auditEvent is a single audit event read from the sandbox JSONL log.
type auditEvent struct {
	Identity string `json:"identity"`
	SrcIP    string `json:"src_ip"`
	DstIP    string `json:"dst_ip"`
	DstPort  uint16 `json:"dst_port"`
	Protocol string `json:"protocol"`
	Verdict  string `json:"verdict"`
}

// readAuditLog reads and parses the sandbox audit JSONL file.
func readAuditLog(clientset *kubernetes.Clientset, config *rest.Config, ns, podName string) []auditEvent {
	output, err := execInPod(clientset, config, ns, podName, []string{"cat", "/tmp/lattice-audit.jsonl"})
	Expect(err).NotTo(HaveOccurred(), "read audit log failed")

	var events []auditEvent
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		var ev auditEvent
		Expect(json.Unmarshal([]byte(line), &ev)).To(Succeed(), "parse audit event: %s", line)
		events = append(events, ev)
	}
	return events
}

// ---- Policy helpers ----

// createAllowAllPolicy creates a LatticePolicy that allows all traffic between peers on the given network.
func createAllowAllPolicy(latticeClient sigclient.Client, ns, policyName, networkName string) {
	networkLabel := fmt.Sprintf("alattice.io/network-%s", networkName)
	peerSelector := metav1.LabelSelector{
		MatchLabels: map[string]string{networkLabel: "true"},
	}
	policy := &latticev1.LatticePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      policyName,
			Namespace: ns,
		},
		Spec: latticev1.LatticePolicySpec{
			Network:      networkName,
			PeerSelector: peerSelector,
			Action:       "ALLOW",
			Ingress: []latticev1.IngressRule{
				{From: []latticev1.PeerSelection{{PeerSelector: &peerSelector}}},
			},
			Egress: []latticev1.EgressRule{
				{To: []latticev1.PeerSelection{{PeerSelector: &peerSelector}}},
			},
		},
	}
	Expect(latticeClient.Create(context.Background(), policy)).To(Succeed(), "failed to create LatticePolicy %s", policyName)
}

// ---- Connectivity helpers ----

// pingWithRetry pings targetIP from the specified pod. Returns success when 0% packet loss is seen.
func pingWithRetry(clientset *kubernetes.Clientset, config *rest.Config, ns, srcPod, targetIP string, timeout string) {
	Eventually(func() error {
		output, err := execInPod(clientset, config, ns, srcPod, []string{"ping", "-c", "3", "-W", "2", targetIP})
		if err != nil {
			return fmt.Errorf("ping execution failed: %w", err)
		}
		if !strings.Contains(output, "0% packet loss") {
			return fmt.Errorf("ping has packet loss: %s", output)
		}
		return nil
	}, timeout, "5s").Should(Succeed(), "ping from %s to %s failed", srcPod, targetIP)
}

// assertPingBlocked verifies that ping from srcPod to targetIP is blocked (no 0% packet loss).
func assertPingBlocked(clientset *kubernetes.Clientset, config *rest.Config, ns, srcPod, targetIP string, timeout string) {
	Eventually(func() error {
		out, execErr := execInPod(clientset, config, ns, srcPod, []string{"ping", "-c", "3", "-W", "2", targetIP})
		if execErr != nil {
			// ping command itself failed — blocked by iptables
			return nil
		}
		if !strings.Contains(out, "0% packet loss") {
			// ping ran but got no replies — blocked
			return nil
		}
		return fmt.Errorf("ping should have been blocked but succeeded: %s", out)
	}, timeout, "2s").Should(Succeed(), "ping should be blocked but was not")
}

// ---- Workspace lifecycle helpers ----

// cleanupWorkspace tries to clean up Lattice CRDs and delete the namespace.
// Best-effort: warns on failure but does not fail the test (namespace stuck in
// Terminating is an infra issue, not a logic bug).
func cleanupWorkspace(clientset *kubernetes.Clientset, ns string) {
	if ns == "" {
		return
	}
	By("Cleanup Namespace: " + ns)

	ctx := context.Background()

	// Removal of finalizers from CRDs before namespace deletion — the controller
	// may be busy/unavailable, leaving finalizers that block namespace termination.
	removeFinalizers := func(obj any) {
		switch o := obj.(type) {
		case *latticev1.LatticePeer:
			o.SetFinalizers(nil)
			_ = latticeClient.Update(ctx, o)
		case *latticev1.LatticeNetwork:
			o.SetFinalizers(nil)
			_ = latticeClient.Update(ctx, o)
		case *latticev1.LatticePolicy:
			o.SetFinalizers(nil)
			_ = latticeClient.Update(ctx, o)
		}
	}

	// LatticePolicy
	policyList := &latticev1.LatticePolicyList{}
	if err := latticeClient.List(ctx, policyList, sigclient.InNamespace(ns)); err == nil {
		for _, p := range policyList.Items {
			removeFinalizers(&p)
			_ = latticeClient.Delete(ctx, &latticev1.LatticePolicy{ObjectMeta: metav1.ObjectMeta{Name: p.Name, Namespace: ns}})
		}
	}
	// LatticePeer
	peerList := &latticev1.LatticePeerList{}
	if err := latticeClient.List(ctx, peerList, sigclient.InNamespace(ns)); err == nil {
		for _, p := range peerList.Items {
			removeFinalizers(&p)
			_ = latticeClient.Delete(ctx, &latticev1.LatticePeer{ObjectMeta: metav1.ObjectMeta{Name: p.Name, Namespace: ns}})
		}
	}
	// LatticeNetwork
	netList := &latticev1.LatticeNetworkList{}
	if err := latticeClient.List(ctx, netList, sigclient.InNamespace(ns)); err == nil {
		for _, n := range netList.Items {
			removeFinalizers(&n)
			_ = latticeClient.Delete(ctx, &latticev1.LatticeNetwork{ObjectMeta: metav1.ObjectMeta{Name: n.Name, Namespace: ns}})
		}
	}

	deletePolicy := metav1.DeletePropagationBackground
	if err := clientset.CoreV1().Namespaces().Delete(ctx, ns, metav1.DeleteOptions{PropagationPolicy: &deletePolicy}); err != nil {
		if !k8serrors.IsNotFound(err) {
			fmt.Fprintf(GinkgoWriter, "[WARN] failed to clean up Namespace %s: %v\n", ns, err) //nolint:errcheck
		}
		return
	}
	// Best-effort wait (don't fail the suite if namespace gets stuck in Terminating).
	for i := 0; i < 20; i++ {
		time.Sleep(3 * time.Second)
		if _, err := clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{}); k8serrors.IsNotFound(err) {
			return
		}
	}
	fmt.Fprintf(GinkgoWriter, "[WARN] Namespace %s deletion timed out (skipped)\n", ns) //nolint:errcheck
}

// execInPod executes a command inside the first container of a Pod via SPDY.
func execInPod(c *kubernetes.Clientset, config *rest.Config, namespace, podName string, command []string) (string, error) {
	container := ""
	pod, err := c.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
	if err == nil && len(pod.Spec.Containers) > 0 {
		container = pod.Spec.Containers[0].Name
	}
	return execInContainer(c, config, namespace, podName, container, command)
}

// execInContainer executes a command inside a specific container of a Pod via SPDY.
// If container is empty, Kubernetes picks the first container in the pod spec.
func execInContainer(c *kubernetes.Clientset, config *rest.Config, namespace, podName, container string, command []string) (string, error) {
	req := c.CoreV1().RESTClient().Post().
		Resource("pods").Name(podName).Namespace(namespace).SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Container: container,
		Command:   command,
		Stdout:    true,
		Stderr:    true,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("failed to create SPDY executor: %w", err)
	}

	var stdout, stderr bytes.Buffer
	if err = exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	}); err != nil {
		return "", fmt.Errorf("command execution failed [%v]: stderr=%s", err, stderr.String())
	}
	return stdout.String(), nil
}

// cleanupSandboxCRDs removes AgentIdentity CRDs that might block namespace deletion.
func cleanupSandboxCRDs(ns string) {
	ctx := context.Background()
	list := &latticev1.AgentIdentityList{}
	if err := latticeClient.List(ctx, list, sigclient.InNamespace(ns)); err == nil {
		for _, id := range list.Items {
			id.SetFinalizers(nil)
			_ = latticeClient.Update(ctx, &id)
			_ = latticeClient.Delete(ctx, &latticev1.AgentIdentity{ObjectMeta: metav1.ObjectMeta{Name: id.Name, Namespace: ns}})
		}
	}
}
