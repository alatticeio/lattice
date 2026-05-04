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

var _ = Describe("Agent 重启韧性测试", Ordered, func() {
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
		testNS = fmt.Sprintf("wf-%s", workspaceID)

		joinToken := generateJoinToken(manageUrl, accessToken, workspaceID)
		hostAliases := hostAliasesForNATS(clientset)

		deployAgentDeployment(clientset, testNS, resilientPeer, agentImage, joinToken, hostAliases)
		podName = waitForPodRunningReady(clientset, testNS, resilientPeer, "180s")
		wgIP = waitForWGIP(latticeClient, testNS, resilientPeer, "90s")

		By(fmt.Sprintf("Pod 就绪: name=%s, ip=%s", podName, wgIP))
	})

	It("基线: 创建 ALLOW 策略并验证隧道状态", func() {
		// 获取网络名称
		peer := &latticev1.LatticePeer{}
		Expect(latticeClient.Get(ctxR, sigclient.ObjectKey{Namespace: testNS, Name: resilientPeer}, peer)).To(Succeed())
		networkName := getNetworkName(peer)
		Expect(networkName).NotTo(BeEmpty())

		createAllowAllPolicy(latticeClient, testNS, resilientPol, networkName)

		By("验证 WireGuard 接口已创建")
		Eventually(func() error {
			out, err := execInPod(clientset, restConfig, testNS, podName, []string{"wg", "show"})
			if err != nil {
				return err
			}
			if !strings.Contains(out, "interface:") {
				return fmt.Errorf("WireGuard 接口未就绪:\n%s", out)
			}
			return nil
		}, "15s", "2s").Should(Succeed(), "WireGuard 接口应已创建")
	})

	It("Agent 进程崩溃恢复: 隧道应自动重建", func() {
		By("记录当前 Pod 名称: " + podName)

		By("模拟 Agent 崩溃: 删除 Pod，触发 Deployment 重建")
		err := clientset.CoreV1().Pods(testNS).Delete(ctxR, podName, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred(), "删除 Pod 失败")

		By("等待 Deployment 创建新 Pod 并进入 Ready")
		podName = waitForPodRunningReady(clientset, testNS, resilientPeer, "120s")

		By(fmt.Sprintf("新 Pod 已就绪: %s", podName))

		By("等待控制面确认 LatticePeer 状态（重新分配/确认 IP）")
		_ = waitForWGIP(latticeClient, testNS, resilientPeer, "90s")

		By("验证 wg 接口已恢复")
		Eventually(func() error {
			out, err := execInPod(clientset, restConfig, testNS, podName, []string{"wg", "show"})
			if err != nil {
				return fmt.Errorf("wg show 失败: %w", err)
			}
			if !strings.Contains(out, "interface:") {
				return fmt.Errorf("WireGuard 接口未重建:\n%s", out)
			}
			return nil
		}, "45s", "2s").Should(Succeed(), "WireGuard 接口应在重启后重建")

		By("验证 IP 地址一致（agent 重新注册后应拿到相同 IP）")
		Eventually(func() error {
			out, err := execInPod(clientset, restConfig, testNS, podName, []string{"ip", "addr", "show", "wf0"})
			if err != nil {
				return fmt.Errorf("ip addr 失败: %w", err)
			}
			if !strings.Contains(out, wgIP) {
				return fmt.Errorf("重启后 IP 不一致: 期望 %s, 输出:\n%s", wgIP, out)
			}
			return nil
		}, "15s", "2s").Should(Succeed(), "Agent 重启后应保持相同 WireGuard IP")

		By("验证 iptables 策略已重新应用")
		Eventually(func() error {
			out, err := execInPod(clientset, restConfig, testNS, podName, []string{"iptables", "-L", "LATTICE-EGRESS", "-n"})
			if err != nil {
				return err
			}
			if !strings.Contains(out, "ACCEPT") {
				return fmt.Errorf("LATTICE-EGRESS 链未包含 ACCEPT 规则:\n%s", out)
			}
			return nil
		}, "15s", "2s").Should(Succeed(), "重启后 iptables 策略应重新应用")
	})

	AfterAll(func() {
		if CurrentSpecReport().Failed() {
			collectDiagnostics(ctxR, testNS)
		}
		// 清理
		pol := &latticev1.LatticePolicy{ObjectMeta: metav1.ObjectMeta{Name: resilientPol, Namespace: testNS}}
		_ = latticeClient.Delete(ctxR, pol)
		cleanupWorkspace(clientset, testNS)
	})
})
