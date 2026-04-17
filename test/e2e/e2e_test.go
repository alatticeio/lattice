package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"wireflow/api/v1alpha1"
	"wireflow/management/dto"
	"wireflow/pkg/utils/resp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	sigclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	podA = "pod-a"
	podB = "pod-b"
)

var _ = Describe("Wireflow 核心连通性 E2E", Ordered, func() {
	var (
		accessToken string
		workspaceId string
		joinToken   string
		httpClient  = &http.Client{Timeout: 15 * time.Second}
		ctx         = context.Background()
	)

	// 失败时收集诊断日志，帮助排查问题
	AfterAll(func() {
		if CurrentSpecReport().Failed() {
			collectDiagnostics(ctx, ns)
		}
	})

	It("全链路：登录 → 建 Workspace → 生成 Token → 拉起 Pod → 验证隧道互通", func() {

		By("步骤 1: 登录 Manager，获取 Admin Access Token")
		loginBody, _ := json.Marshal(map[string]string{"username": "admin", "password": "123456"})
		respLogin, err := httpClient.Post(manageUrl+"/api/v1/users/login", "application/json", bytes.NewBuffer(loginBody))
		Expect(err).NotTo(HaveOccurred(), "登录请求失败")
		defer respLogin.Body.Close() //nolint:errcheck

		var loginData resp.Response
		Expect(json.NewDecoder(respLogin.Body).Decode(&loginData)).To(Succeed())
		Expect(respLogin.StatusCode).To(Equal(http.StatusOK), "登录接口返回非 200")

		dataMap, ok := loginData.Data.(map[string]any)
		Expect(ok).To(BeTrue(), "登录响应 Data 格式错误")
		accessToken, ok = dataMap["token"].(string)
		Expect(ok && accessToken != "").To(BeTrue(), "登录响应中未找到 token")

		By("步骤 2: 创建 Workspace (Namespace: " + ns + ")")
		wsBody, _ := json.Marshal(dto.WorkspaceDto{
			Namespace:   ns,
			DisplayName: "E2E-Workspace",
		})
		reqWs, _ := http.NewRequestWithContext(ctx, http.MethodPost, manageUrl+"/api/v1/workspaces/add", bytes.NewBuffer(wsBody))
		reqWs.Header.Set("Authorization", "Bearer "+accessToken)
		reqWs.Header.Set("Content-Type", "application/json")

		respWs, err := httpClient.Do(reqWs)
		Expect(err).NotTo(HaveOccurred(), "创建 Workspace 请求失败")
		defer respWs.Body.Close() //nolint:errcheck

		var wsData resp.Response
		Expect(json.NewDecoder(respWs.Body).Decode(&wsData)).To(Succeed())

		wsMap, ok := wsData.Data.(map[string]any)
		Expect(ok).To(BeTrue(), "Workspace 响应 Data 格式错误")
		workspaceId, ok = wsMap["id"].(string)
		Expect(ok && workspaceId != "").To(BeTrue(), "Workspace 响应中未找到 id")

		ns := fmt.Sprintf("wf-%s", workspaceId)

		By("步骤 3: 为 Workspace 生成 Agent Join Token")
		reqTk, _ := http.NewRequestWithContext(ctx, http.MethodPost, manageUrl+"/api/v1/token/generate", nil)
		reqTk.Header.Set("Authorization", "Bearer "+accessToken)
		reqTk.Header.Set("X-workspace-id", workspaceId)

		respTk, err := httpClient.Do(reqTk)
		Expect(err).NotTo(HaveOccurred(), "生成 Token 请求失败")
		defer respTk.Body.Close() //nolint:errcheck

		var tkData resp.Response
		Expect(json.NewDecoder(respTk.Body).Decode(&tkData)).To(Succeed())

		tkMap, ok := tkData.Data.(map[string]any)
		Expect(ok).To(BeTrue(), "Token 响应 Data 格式错误")
		joinToken, ok = tkMap["token"].(string)
		Expect(ok && joinToken != "").To(BeTrue(), "Token 响应中未找到 token")

		By("步骤 4: 查找 NATS Service ClusterIP 并创建具备特权和内核模块挂载的测试 Deployment")
		svc, err := clientset.CoreV1().Services("wireflow-system").Get(ctx, "wireflow-nats-service", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred(), "未找到 wireflow-nats-service")

		hostAliases := []corev1.HostAlias{{
			IP:        svc.Spec.ClusterIP,
			Hostnames: []string{"signaling.wireflow.run"},
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
								ImagePullPolicy: corev1.PullAlways,
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
									"/app/wireflow", "up",
									"--token", joinToken,
									"--level", "debug",
									"--server-url", "wireflow-api-service.wireflow-system.svc.cluster.local:8080",
									"--signaling-url", "nats://signaling.wireflow.run:4222",
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

		By("步骤 5: 等待两个 Deployment 的 Pod 进入 Running 且容器全部 Ready (最长 180s)")
		for _, role := range []string{podA, podB} {
			Eventually(func() error {
				pods, err := clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
					LabelSelector: "wf-role=" + role,
				})
				if err != nil {
					return err
				}
				if len(pods.Items) == 0 {
					return fmt.Errorf("等待 %s 的 Pod 被调度", role)
				}
				pod := pods.Items[0]
				if pod.Status.Phase != corev1.PodRunning {
					return fmt.Errorf("Pod %s 阶段为 %s，期望 Running", pod.Name, pod.Status.Phase)
				}
				for _, cs := range pod.Status.ContainerStatuses {
					if !cs.Ready {
						return fmt.Errorf("Pod %s 容器 %s 尚未 Ready (restarts=%d)", pod.Name, cs.Name, cs.RestartCount)
					}
				}
				return nil
			}, "180s", "3s").Should(Succeed(), "Deployment %s 的 Pod 未能进入 Running+Ready 状态", role)
		}

		// 获取两个 Deployment 实际的 Pod 名称，供后续步骤使用
		getPodName := func(role string) string {
			pods, err := clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
				LabelSelector: "wf-role=" + role,
			})
			Expect(err).NotTo(HaveOccurred(), "列出 %s 的 Pod 失败", role)
			Expect(pods.Items).NotTo(BeEmpty(), "未找到 %s 的 Pod", role)
			return pods.Items[0].Name
		}
		podAName := getPodName(podA)
		podBName := getPodName(podB)

		By("步骤 6: 等待控制面为 " + podA + " 和 " + podB + " 分配 WireGuard 虚拟 IP (WireflowPeer CRD)")
		var podBWGIP string
		for _, peerName := range []string{podA, podB} {
			name := peerName
			Eventually(func() error {
				peer := &v1alpha1.WireflowPeer{}
				if err := wireflowClient.Get(ctx, sigclient.ObjectKey{Namespace: ns, Name: name}, peer); err != nil {
					return fmt.Errorf("WireflowPeer %s 尚未创建: %w", name, err)
				}
				if peer.Status.AllocatedAddress == nil || *peer.Status.AllocatedAddress == "" {
					return fmt.Errorf("WireflowPeer %s 已创建，控制面尚未分配地址", name)
				}
				if name == podB {
					podBWGIP = *peer.Status.AllocatedAddress
					// 地址可能包含 CIDR 前缀 (e.g. "10.0.0.2/24")，ping 只需要 IP 部分
					if idx := strings.Index(podBWGIP, "/"); idx != -1 {
						podBWGIP = podBWGIP[:idx]
					}
				}
				return nil
			}, "90s", "3s").Should(Succeed(), "超时未能获取 %s 的 WireGuard IP", name)
		}

		By("步骤 7: 创建 WireflowPolicy 允许 pod-a ↔ pod-b 互通")
		peerB := &v1alpha1.WireflowPeer{}
		Expect(wireflowClient.Get(ctx, sigclient.ObjectKey{Namespace: ns, Name: podB}, peerB)).To(Succeed())

		var networkName string
		if peerB.Spec.Network != nil && *peerB.Spec.Network != "" {
			networkName = *peerB.Spec.Network
		} else if peerB.Status.ActiveNetwork != nil {
			networkName = *peerB.Status.ActiveNetwork
		}
		Expect(networkName).NotTo(BeEmpty(), "无法从 WireflowPeer 获取网络名称")

		networkLabel := fmt.Sprintf("wireflow.run/network-%s", networkName)
		peerNetSelector := metav1.LabelSelector{
			MatchLabels: map[string]string{networkLabel: "true"},
		}
		allowPolicy := &v1alpha1.WireflowPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "e2e-allow-all",
				Namespace: ns,
			},
			Spec: v1alpha1.WireflowPolicySpec{
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
		Expect(wireflowClient.Create(ctx, allowPolicy)).To(Succeed(), "创建 WireflowPolicy 失败")

		By(fmt.Sprintf("步骤 8: 验证隧道连通性 (%s → %s @ %s)", podAName, podBName, podBWGIP))
		Eventually(func() error {
			output, err := execInPod(clientset, restConfig, ns, podAName, []string{"ping", "-c", "3", "-W", "2", podBWGIP})
			if err != nil {
				return fmt.Errorf("ping 执行失败: %w", err)
			}
			if !strings.Contains(output, "0% packet loss") {
				return fmt.Errorf("ping 存在丢包: %s", output)
			}
			return nil
		}, "60s", "5s").Should(Succeed(), "隧道连通性验证失败")
	})
})

// collectDiagnostics 在测试失败时打印关键日志，方便 CI 排查
func collectDiagnostics(ctx context.Context, namespace string) {
	w := GinkgoWriter
	fprintf := func(format string, args ...any) { fmt.Fprintf(w, format, args...) } //nolint:errcheck

	fprintf("\n========== E2E 诊断日志 [ns=%s] ==========\n", namespace)

	// ── 1. WireflowPeer CRD 状态 ──────────────────────────────────────────
	fprintf("\n[WireflowPeer 状态]\n")
	var peerList v1alpha1.WireflowPeerList
	if err := wireflowClient.List(ctx, &peerList, sigclient.InNamespace(namespace)); err != nil {
		fprintf("  [WARN] 无法列出 WireflowPeer: %v\n", err)
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

	// ── 2. WireflowNetwork 状态 ───────────────────────────────────────────
	fprintf("\n[WireflowNetwork 状态]\n")
	var netList v1alpha1.WireflowNetworkList
	if err := wireflowClient.List(ctx, &netList, sigclient.InNamespace(namespace)); err != nil {
		fprintf("  [WARN] 无法列出 WireflowNetwork: %v\n", err)
	} else {
		for _, n := range netList.Items {
			fprintf("  %-30s  phase=%-10s  activeCIDR=%-20s  allocatedCount=%d\n",
				n.Name, n.Status.Phase, n.Status.ActiveCIDR, n.Status.AllocatedCount)
		}
	}

	// ── 3. ConfigMap 内容（agent 配置） ───────────────────────────────────
	fprintf("\n[ConfigMap 内容]\n")
	cms, err := clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=wireflow-controller",
	})
	if err != nil {
		fprintf("  [WARN] 无法列出 ConfigMap: %v\n", err)
	} else {
		for _, cm := range cms.Items {
			fprintf("\n  --- ConfigMap: %s ---\n", cm.Name)
			for k, v := range cm.Data {
				fprintf("  [%s]\n%s\n", k, v)
			}
		}
	}

	// ── 4. Pod 日志 + WireGuard / 网络状态 ───────────────────────────────
	fprintf("\n[Pod 日志及网络状态]\n")
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		fprintf("  [WARN] 无法列出 Pod: %v\n", err)
	} else {
		for _, pod := range pods.Items {
			fprintf("\n--- Pod: %s  Phase: %s ---\n", pod.Name, pod.Status.Phase)
			for _, cs := range pod.Status.ContainerStatuses {
				fprintf("  Container %s: ready=%v restarts=%d\n", cs.Name, cs.Ready, cs.RestartCount)
			}

			if pod.Status.Phase == corev1.PodRunning {
				// wireflow status：WireGuard 隧道连接状态（对端握手、流量）
				if out, err := execInPod(clientset, restConfig, namespace, pod.Name,
					[]string{"/app/wireflow", "status"}); err != nil {
					fprintf("  [wireflow status] 执行失败: %v\n", err)
				} else {
					fprintf("  [wireflow status]\n%s\n", out)
				}

				// ip addr：确认 wf0 接口是否存在及 IP
				if out, err := execInPod(clientset, restConfig, namespace, pod.Name,
					[]string{"ip", "addr", "show"}); err != nil {
					fprintf("  [ip addr] 执行失败: %v\n", err)
				} else {
					fprintf("  [ip addr]\n%s\n", out)
				}

				// ip route：路由表
				if out, err := execInPod(clientset, restConfig, namespace, pod.Name,
					[]string{"ip", "route", "show"}); err != nil {
					fprintf("  [ip route] 执行失败: %v\n", err)
				} else {
					fprintf("  [ip route]\n%s\n", out)
				}
			}

			// 容器日志（最近 150 行）
			tailLines := int64(150)
			logReq := clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
				TailLines: &tailLines,
			})
			logStream, err := logReq.Stream(ctx)
			if err != nil {
				fprintf("  [WARN] 无法获取日志: %v\n", err)
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
