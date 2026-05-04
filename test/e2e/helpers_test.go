package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
// and closed inside this function.
func apiPOST(url, token string, body any) (int, *resp.Response) {
	var reqBody []byte
	if body != nil {
		reqBody, _ = json.Marshal(body)
	}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, url, bytes.NewBuffer(reqBody))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	httpClient := &http.Client{Timeout: 15 * time.Second}
	httpResp, err := httpClient.Do(req)
	Expect(err).NotTo(HaveOccurred(), "HTTP POST %s 失败", url)
	defer httpResp.Body.Close() //nolint:errcheck

	var data resp.Response
	_ = json.NewDecoder(httpResp.Body).Decode(&data)
	return httpResp.StatusCode, &data
}

// ---- Test setup helpers ----

// login returns the admin access token.
func login(manageURL string) string {
	By("登录 Manager 获取 Admin Token")
	statusCode, data := apiPOST(manageURL+"/api/v1/users/login", "", map[string]string{"username": "admin", "password": "123456"})
	Expect(statusCode).To(Equal(http.StatusOK), "登录接口返回非 200")

	dataMap, ok := data.Data.(map[string]any)
	Expect(ok).To(BeTrue(), "登录响应 Data 格式错误")
	token, ok := dataMap["token"].(string)
	Expect(ok && token != "").To(BeTrue(), "登录响应中未找到 token")
	return token
}

// createWorkspace creates a workspace and returns the workspace ID.
func createWorkspace(manageURL, accessToken, nsName string) string {
	wsName := fmt.Sprintf("e2e-%d", time.Now().UnixMilli())
	By("创建 Workspace: " + wsName)
	statusCode, data := apiPOST(manageURL+"/api/v1/workspaces/add", accessToken, dto.WorkspaceDto{
		Namespace:   nsName,
		DisplayName: wsName,
		Slug:        wsName,
	})
	Expect(statusCode).To(Equal(http.StatusOK), "创建 Workspace 失败: %+v", data)

	dataMap, ok := data.Data.(map[string]any)
	Expect(ok).To(BeTrue(), "Workspace 响应 Data 格式错误")
	workspaceID, ok := dataMap["id"].(string)
	Expect(ok && workspaceID != "").To(BeTrue(), "Workspace 响应中未找到 id")
	return workspaceID
}

// generateJoinToken generates an agent join token for the given workspace.
func generateJoinToken(manageURL, accessToken, workspaceID string) string {
	By("生成 Agent Join Token")
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, manageURL+"/api/v1/token/generate", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-workspace-id", workspaceID)
	httpClient := &http.Client{Timeout: 15 * time.Second}
	httpResp, err := httpClient.Do(req)
	Expect(err).NotTo(HaveOccurred(), "生成 Token 请求失败")
	defer httpResp.Body.Close() //nolint:errcheck

	var data resp.Response
	Expect(json.NewDecoder(httpResp.Body).Decode(&data)).To(Succeed())

	tkMap, ok := data.Data.(map[string]any)
	Expect(ok).To(BeTrue(), "Token 响应 Data 格式错误")
	token, ok := tkMap["token"].(string)
	Expect(ok && token != "").To(BeTrue(), "Token 响应中未找到 token")
	return token
}

// ---- K8s pod helpers ----

// hostAliasesForNATS discovers the NATS service ClusterIP for hostAliases.
func hostAliasesForNATS(clientset *kubernetes.Clientset) []corev1.HostAlias {
	svc, err := clientset.CoreV1().Services("lattice-system").Get(context.Background(), "lattice-nats-service", metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred(), "未找到 lattice-nats-service")
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

	Expect(err).NotTo(HaveOccurred(), "创建 Deployment %s 失败", name)
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
			return fmt.Errorf("等待 %s 的 Pod 被调度", role)
		}
		var pod *corev1.Pod
		for i := range pods.Items {
			if pods.Items[i].DeletionTimestamp == nil {
				pod = &pods.Items[i]
				break
			}
		}
		if pod == nil {
			return fmt.Errorf("等待 %s 的 Pod：当前 Pod 均处于 Terminating 状态", role)
		}
		if pod.Status.Phase != corev1.PodRunning {
			return fmt.Errorf("pod %s 阶段为 %s，期望 Running", pod.Name, pod.Status.Phase)
		}
		for _, cs := range pod.Status.ContainerStatuses {
			if !cs.Ready {
				return fmt.Errorf("pod %s 容器 %s 尚未 Ready (restarts=%d)", pod.Name, cs.Name, cs.RestartCount)
			}
		}
		podName = pod.Name
		return nil
	}, timeout, "3s").Should(Succeed(), "Deployment %s 的 Pod 未能进入 Running+Ready 状态", role)
	return podName
}

// getPodName returns the name of the first pod matching the role label.
// waitForWGIP waits for the LatticePeer CRD to have an AllocatedAddress and returns the IP (without CIDR suffix).
func waitForWGIP(latticeClient sigclient.Client, ns, peerName string, timeout string) string {
	var ip string
	Eventually(func() error {
		peer := &latticev1.LatticePeer{}
		if err := latticeClient.Get(context.Background(), sigclient.ObjectKey{Namespace: ns, Name: peerName}, peer); err != nil {
			return fmt.Errorf("LatticePeer %s 尚未创建: %w", peerName, err)
		}
		if peer.Status.AllocatedAddress == nil || *peer.Status.AllocatedAddress == "" {
			return fmt.Errorf("LatticePeer %s 已创建，控制面尚未分配地址", peerName)
		}
		ip = *peer.Status.AllocatedAddress
		if idx := strings.Index(ip, "/"); idx != -1 {
			ip = ip[:idx]
		}
		return nil
	}, timeout, "3s").Should(Succeed(), "超时未能获取 %s 的 WireGuard IP", peerName)
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
	Expect(latticeClient.Create(context.Background(), policy)).To(Succeed(), "创建 LatticePolicy %s 失败", policyName)
}

// ---- Connectivity helpers ----

// pingWithRetry pings targetIP from the specified pod. Returns success when 0% packet loss is seen.
func pingWithRetry(clientset *kubernetes.Clientset, config *rest.Config, ns, srcPod, targetIP string, timeout string) {
	Eventually(func() error {
		output, err := execInPod(clientset, config, ns, srcPod, []string{"ping", "-c", "3", "-W", "2", targetIP})
		if err != nil {
			return fmt.Errorf("ping 执行失败: %w", err)
		}
		if !strings.Contains(output, "0% packet loss") {
			return fmt.Errorf("ping 存在丢包: %s", output)
		}
		return nil
	}, timeout, "5s").Should(Succeed(), "从 %s ping %s 失败", srcPod, targetIP)
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
		return fmt.Errorf("ping 应该被阻断但成功了: %s", out)
	}, timeout, "2s").Should(Succeed(), "ping 应被阻断但未生效")
}

// ---- Workspace lifecycle helpers ----

// cleanupWorkspace tries to clean up Lattice CRDs and delete the namespace.
// Best-effort: warns on failure but does not fail the test (namespace stuck in
// Terminating is an infra issue, not a logic bug).
func cleanupWorkspace(clientset *kubernetes.Clientset, ns string) {
	if ns == "" {
		return
	}
	By("清理 Namespace: " + ns)

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
			fmt.Fprintf(GinkgoWriter, "[WARN] 清理 Namespace %s 失败: %v\n", ns, err) //nolint:errcheck
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
	fmt.Fprintf(GinkgoWriter, "[WARN] Namespace %s 删除超时（已跳过）\n", ns) //nolint:errcheck
}

// execInPod 通过 SPDY 在指定 Pod 内执行命令并返回 stdout 输出
func execInPod(c *kubernetes.Clientset, config *rest.Config, namespace, podName string, command []string) (string, error) {
	req := c.CoreV1().RESTClient().Post().
		Resource("pods").Name(podName).Namespace(namespace).SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Command: command,
		Stdout:  true,
		Stderr:  true,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("创建 SPDY executor 失败: %w", err)
	}

	var stdout, stderr bytes.Buffer
	if err = exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	}); err != nil {
		return "", fmt.Errorf("执行命令失败 [%v]: stderr=%s", err, stderr.String())
	}
	return stdout.String(), nil
}
