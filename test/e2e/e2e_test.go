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

var _ = Describe("Lattice 核心连通性 E2E", Ordered, func() {
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

		wsName := fmt.Sprintf("e2e-%d", time.Now().UnixMilli())
		By("步骤 2: 创建 Workspace (Namespace: " + ns + ", Name: " + wsName + ")")
		wsBody, _ := json.Marshal(dto.WorkspaceDto{
			Namespace:   ns,
			DisplayName: wsName,
			Slug:        wsName,
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

		ns = fmt.Sprintf("wf-%s", workspaceId)

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
		svc, err := clientset.CoreV1().Services("lattice-system").Get(ctx, "lattice-nats-service", metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred(), "未找到 lattice-nats-service")

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

		By("步骤 6: 等待控制面为 " + podA + " 和 " + podB + " 分配 WireGuard 虚拟 IP (LatticePeer CRD)")
		var podBWGIP string
		for _, peerName := range []string{podA, podB} {
			name := peerName
			Eventually(func() error {
				peer := &v1alpha1.LatticePeer{}
				if err := latticeClient.Get(ctx, sigclient.ObjectKey{Namespace: ns, Name: name}, peer); err != nil {
					return fmt.Errorf("LatticePeer %s 尚未创建: %w", name, err)
				}
				if peer.Status.AllocatedAddress == nil || *peer.Status.AllocatedAddress == "" {
					return fmt.Errorf("LatticePeer %s 已创建，控制面尚未分配地址", name)
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

		By("步骤 7: 创建 LatticePolicy 允许 pod-a ↔ pod-b 互通")
		peerB := &v1alpha1.LatticePeer{}
		Expect(latticeClient.Get(ctx, sigclient.ObjectKey{Namespace: ns, Name: podB}, peerB)).To(Succeed())

		var networkName string
		if peerB.Spec.Network != nil && *peerB.Spec.Network != "" {
			networkName = *peerB.Spec.Network
		} else if peerB.Status.ActiveNetwork != nil {
			networkName = *peerB.Status.ActiveNetwork
		}
		Expect(networkName).NotTo(BeEmpty(), "无法从 LatticePeer 获取网络名称")

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
		Expect(latticeClient.Create(ctx, allowPolicy)).To(Succeed(), "创建 LatticePolicy 失败")

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

	It("ACL 规则验证：DENY 阻断 + 端口级控制 + default-deny", func() {
		// 复用步骤 6 中获取的变量（需在同一 Describe 作用域内）
		// 重新获取必要的数据
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
		Expect(podAIP).NotTo(BeEmpty(), "无法获取 pod-a 的 WireGuard IP")
		Expect(podBIP).NotTo(BeEmpty(), "无法获取 pod-b 的 WireGuard IP")

		var networkName string
		if peerB.Spec.Network != nil && *peerB.Spec.Network != "" {
			networkName = *peerB.Spec.Network
		} else if peerB.Status.ActiveNetwork != nil {
			networkName = *peerB.Status.ActiveNetwork
		}
		Expect(networkName).NotTo(BeEmpty(), "无法从 LatticePeer 获取网络名称")

		networkLabel := fmt.Sprintf("alattice.io/network-%s", networkName)
		peerNetSelector := metav1.LabelSelector{
			MatchLabels: map[string]string{networkLabel: "true"},
		}

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

		By("ACL-1: 验证 e2e-allow-all 策略下 ping 通（基线确认）")
		output, err := execInPod(clientset, restConfig, ns, podAName, []string{"ping", "-c", "3", "-W", "2", podBIP})
		Expect(err).NotTo(HaveOccurred(), "基线 ping 失败")
		Expect(strings.Contains(output, "0% packet loss")).To(BeTrue(), "基线 ping 存在丢包: %s", output)

		By("ACL-2: 更新策略为 DENY，验证 pod-a → pod-b 被阻断")
		allowPolicy := &v1alpha1.LatticePolicy{}
		Expect(latticeClient.Get(ctx, sigclient.ObjectKey{Namespace: ns, Name: "e2e-allow-all"}, allowPolicy)).To(Succeed())

		// 修改策略为 DENY（保持 PeerSelector 和规则不变）
		allowPolicy.Spec.Action = "DENY"
		Expect(latticeClient.Update(ctx, allowPolicy)).To(Succeed(), "更新 LatticePolicy 为 DENY 失败")

		// 等待策略生效（agent 通过 NATS 接收更新并刷新 iptables）
		Eventually(func() error {
			out, execErr := execInPod(clientset, restConfig, ns, podAName, []string{"ping", "-c", "3", "-W", "2", podBIP})
			if execErr != nil {
				// ping command itself failed (e.g. sendto: EPERM) — blocked
				return nil
			}
			if strings.Contains(out, "0% packet loss") {
				return fmt.Errorf("DENY 策略未生效，ping 仍然通: %s", out)
			}
			return nil
		}, "30s", "2s").Should(Succeed(), "DENY 策略应在 30s 内阻断 ping")

		By("ACL-3: 恢复 ALLOW + 端口级规则，仅允许 TCP 8080")
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
		Expect(latticeClient.Update(ctx, allowPolicy)).To(Succeed(), "更新端口级策略失败")

		// 等待策略生效
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
			return fmt.Errorf("端口级策略下 ping 仍应被阻断: %s", out)
		}, "30s", "2s").Should(Succeed(), "端口级策略应阻断 ICMP")

		By("ACL-4: 验证 iptables 规则已放行 TCP 8080（在 LATTICE-EGRESS chain 中）")
		Eventually(func() error {
			out, execErr := execInPod(clientset, restConfig, ns, podAName, []string{"iptables", "-L", "LATTICE-EGRESS", "-n"})
			if execErr != nil {
				return execErr
			}
			if !strings.Contains(out, "tcp dpt:8080") {
				return fmt.Errorf("LATTICE-EGRESS 未包含 TCP 8080 规则:\n%s", out)
			}
			return nil
		}, "15s", "2s").Should(Succeed(), "iptables 应包含 TCP 8080 放行规则")

		By("ACL-5: 验证 iptables 没有 TCP 9999 的规则")
		out, err := execInPod(clientset, restConfig, ns, podAName, []string{"sh", "-c", "iptables -L LATTICE-EGRESS -n 2>/dev/null | grep 9999 || true"})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(BeEmpty(), "不应存在 TCP 9999 的 iptables 规则")

		By("ACL-6: 验证 IPBlock CIDR 规则 — 仅允许 pod-b 的精确 CIDR")
		// 复用已有 allowPolicy，原地更新为 CIDR 规则（避免 delete+create 的竞态）
		Expect(latticeClient.Get(ctx, sigclient.ObjectKey{Namespace: ns, Name: "e2e-allow-all"}, allowPolicy)).To(Succeed())
		podBCIDR := podBIP + "/32"
		podACIDR := podAIP + "/32"
		allowPolicy.Spec.Action = "ALLOW"
		// 需要同时包含两个方向的 CIDR：ingress 允许来自双方的 IP，egress 允许到双方的 IP
		// 这样策略应用到 pod-a/pod-b 时各自都能匹配到对方的 IP
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
		Expect(latticeClient.Update(ctx, allowPolicy)).To(Succeed(), "更新 CIDR 策略失败")

		// 先验证 iptables 规则已被更新为 CIDR（分离「策略未生效」和「策略生效但 ping 不通」）
		Eventually(func() error {
			out, execErr := execInPod(clientset, restConfig, ns, podAName, []string{"iptables", "-L", "LATTICE-EGRESS", "-n"})
			if execErr != nil {
				return execErr
			}
			// iptables 不显示 /32 后缀，所以用 podBIP（不带 /32）做匹配
			if !strings.Contains(out, podBIP) {
				return fmt.Errorf("LATTICE-EGRESS 未包含 %s 的 CIDR 放行规则:\n%s", podBIP, out)
			}
			return nil
		}, "15s", "2s").Should(Succeed(), "iptables 应包含 CIDR 放行规则")

		By("ACL-6: 验证 WireGuard 握手已建立（确保隧道未中断）")
		Eventually(func() error {
			out, err := execInPod(clientset, restConfig, ns, podAName, []string{"wg", "show"})
			if err != nil {
				return err
			}
			if !strings.Contains(out, "latest handshake") {
				return fmt.Errorf("WireGuard 尚未完成握手:\n%s", out)
			}
			return nil
		}, "15s", "2s").Should(Succeed(), "CIDR 策略下 WG 应已完成握手")

		Eventually(func() error {
			out, execErr := execInPod(clientset, restConfig, ns, podAName, []string{"ping", "-c", "3", "-W", "2", podBIP})
			if execErr != nil {
				diag := collectBothPodsDiagnostics(clientset, restConfig, ns, podAName, podBName, podBIP)
				return fmt.Errorf("ping 执行失败: %w\n诊断摘要: %s", execErr, diag)
			}
			if !strings.Contains(out, "0% packet loss") {
				return fmt.Errorf("CIDR 规则下 ping 应通: %s", out)
			}
			return nil
		}, "30s", "2s").Should(Succeed(), "CIDR 策略应放行 pod-b 的 IP")

		By("ACL-7: 替换 CIDR 为不匹配的网段，验证流量被阻断")
		wrongCIDR := "192.168.99.0/24"
		// 重新 Get 避免 resourceVersion 冲突
		Expect(latticeClient.Get(ctx, sigclient.ObjectKey{Namespace: ns, Name: "e2e-allow-all"}, allowPolicy)).To(Succeed())
		allowPolicy.Spec.Egress = []v1alpha1.EgressRule{
			{To: []v1alpha1.PeerSelection{{IPBlock: &v1alpha1.IPBlock{CIDR: wrongCIDR}}}},
		}
		Expect(latticeClient.Update(ctx, allowPolicy)).To(Succeed(), "更新 CIDR 策略失败")

		Eventually(func() error {
			out, execErr := execInPod(clientset, restConfig, ns, podAName, []string{"ping", "-c", "3", "-W", "2", podBIP})
			if execErr != nil {
				// ping command itself failed (e.g. sendto: EPERM) — blocked
				return nil
			}
			if strings.Contains(out, "0% packet loss") {
				return fmt.Errorf("错误 CIDR 下 ping 仍应被阻断: %s", out)
			}
			return nil
		}, "30s", "2s").Should(Succeed(), "不匹配的 CIDR 应阻断流量")

		By("ACL 测试完成，清理策略")
		_ = latticeClient.Delete(ctx, allowPolicy)
	})
})

// collectDiagnostics 在测试失败时打印关键日志，方便 CI 排查
func collectDiagnostics(ctx context.Context, namespace string) {
	w := GinkgoWriter
	fprintf := func(format string, args ...any) { fmt.Fprintf(w, format, args...) } //nolint:errcheck

	fprintf("\n========== E2E 诊断日志 [ns=%s] ==========\n", namespace)

	// ── 1. LatticePeer CRD 状态 ──────────────────────────────────────────
	fprintf("\n[LatticePeer 状态]\n")
	var peerList v1alpha1.LatticePeerList
	if err := latticeClient.List(ctx, &peerList, sigclient.InNamespace(namespace)); err != nil {
		fprintf("  [WARN] 无法列出 LatticePeer: %v\n", err)
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

	// ── 2. LatticeNetwork 状态 ───────────────────────────────────────────
	fprintf("\n[LatticeNetwork 状态]\n")
	var netList v1alpha1.LatticeNetworkList
	if err := latticeClient.List(ctx, &netList, sigclient.InNamespace(namespace)); err != nil {
		fprintf("  [WARN] 无法列出 LatticeNetwork: %v\n", err)
	} else {
		for _, n := range netList.Items {
			fprintf("  %-30s  phase=%-10s  activeCIDR=%-20s  allocatedCount=%d\n",
				n.Name, n.Status.Phase, n.Status.ActiveCIDR, n.Status.AllocatedCount)
		}
	}

	// ── 3. ConfigMap 内容（agent 配置） ───────────────────────────────────
	fprintf("\n[ConfigMap 内容]\n")
	cms, err := clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=lattice-controller",
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
				// lattice status：WireGuard 隧道连接状态（对端握手、流量）
				if out, err := execInPod(clientset, restConfig, namespace, pod.Name,
					[]string{"/app/lattice", "status"}); err != nil {
					fprintf("  [lattice status] 执行失败: %v\n", err)
				} else {
					fprintf("  [lattice status]\n%s\n", out)
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

// collectPodDiagnostics 收集 pod 上的 iptables、wg 和路由信息，用于调试
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

	// 收集 agent 容器日志（最近 50 行）
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
