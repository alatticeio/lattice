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
	restartPeerA = "restart-a"
	restartPeerB = "restart-b"
	restartPol   = "e2e-restart-allow"
)

var _ = Describe("多 Peer 重启韧性", Ordered, func() {
	var (
		testNS      string
		podNames    map[string]string
		podIPs      map[string]string
		accessToken string
		netName     string
		ctxRS       = context.Background()
	)

	BeforeAll(func() {
		accessToken = login(manageUrl)

		nsName := fmt.Sprintf("wf-e2e-restart-%d", time.Now().UnixMilli())
		workspaceID := createWorkspace(manageUrl, accessToken, nsName)
		testNS = fmt.Sprintf("wf-%s", workspaceID)

		joinToken := generateJoinToken(manageUrl, accessToken, workspaceID)
		hostAliases := hostAliasesForNATS(clientset)

		// 部署两个 peer
		for _, role := range []string{restartPeerA, restartPeerB} {
			deployAgentDeployment(clientset, testNS, role, agentImage, joinToken, hostAliases)
		}

		podNames = make(map[string]string)
		podIPs = make(map[string]string)
		for _, role := range []string{restartPeerA, restartPeerB} {
			podNames[role] = waitForPodRunningReady(clientset, testNS, role, "180s")
			podIPs[role] = waitForWGIP(latticeClient, testNS, role, "90s")
		}

		// 获取网络名称并创建 ALLOW 策略
		peer := &latticev1.LatticePeer{}
		Expect(latticeClient.Get(ctxRS, sigclient.ObjectKey{Namespace: testNS, Name: restartPeerA}, peer)).To(Succeed())
		netName = getNetworkName(peer)
		Expect(netName).NotTo(BeEmpty())
		createAllowAllPolicy(latticeClient, testNS, restartPol, netName)

		By(fmt.Sprintf("全部就绪: A=%s(%s), B=%s(%s), network=%s",
			podNames[restartPeerA], podIPs[restartPeerA],
			podNames[restartPeerB], podIPs[restartPeerB], netName))
	})

	It("基线: 两个 Peer 之间连通性正常", func() {
		pingWithRetry(clientset, restConfig, testNS, podNames[restartPeerA], podIPs[restartPeerB], "60s")
		pingWithRetry(clientset, restConfig, testNS, podNames[restartPeerB], podIPs[restartPeerA], "60s")

		By("双向 ping 均成功")
	})

	It("重启 restart-a: Pod 删除后重建，隧道应恢复", func() {
		By("删除 restart-a 的 Pod，触发 Deployment 重建")
		err := clientset.CoreV1().Pods(testNS).Delete(ctxRS, podNames[restartPeerA], metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred(), "删除 Pod 失败")

		By("等待 restart-a 新 Pod 就绪")
		podNames[restartPeerA] = waitForPodRunningReady(clientset, testNS, restartPeerA, "120s")

		By("等待 WireGuard IP 重新确认")
		podIPs[restartPeerA] = waitForWGIP(latticeClient, testNS, restartPeerA, "90s")

		By("验证 WG 接口已重建")
		Eventually(func() error {
			out, err := execInPod(clientset, restConfig, testNS, podNames[restartPeerA], []string{"wg", "show"})
			if err != nil {
				return fmt.Errorf("wg show 失败: %w", err)
			}
			if !strings.Contains(out, "interface:") {
				return fmt.Errorf("WireGuard 接口未重建:\n%s", out)
			}
			return nil
		}, "45s", "2s").Should(Succeed(), "WG 接口应在重启后重建")

		By("验证重启后 restart-a 到 restart-b 的连通性")
		pingWithRetry(clientset, restConfig, testNS, podNames[restartPeerA], podIPs[restartPeerB], "60s")
	})

	It("同时重启两个 Peer: 双方均重建后隧道仍能恢复", func() {
		By("同时删除两个 Peer 的 Pod")
		for _, role := range []string{restartPeerA, restartPeerB} {
			err := clientset.CoreV1().Pods(testNS).Delete(ctxRS, podNames[role], metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred(), "删除 %s 的 Pod 失败", role)
		}

		By("等待两个 Peer 的新 Pod 就绪")
		for _, role := range []string{restartPeerA, restartPeerB} {
			podNames[role] = waitForPodRunningReady(clientset, testNS, role, "120s")
			podIPs[role] = waitForWGIP(latticeClient, testNS, role, "90s")

			Eventually(func() error {
				out, err := execInPod(clientset, restConfig, testNS, podNames[role], []string{"wg", "show"})
				if err != nil {
					return fmt.Errorf("wg show 失败: %w", err)
				}
				if !strings.Contains(out, "interface:") {
					return fmt.Errorf("WireGuard 接口未重建 [%s]:\n%s", role, out)
				}
				return nil
			}, "45s", "2s").Should(Succeed(), "%s 的 WG 接口应在重启后重建", role)
		}

		By("验证双向连通性已恢复")
		pingWithRetry(clientset, restConfig, testNS, podNames[restartPeerA], podIPs[restartPeerB], "90s")
		pingWithRetry(clientset, restConfig, testNS, podNames[restartPeerB], podIPs[restartPeerA], "90s")
	})

	AfterAll(func() {
		if CurrentSpecReport().Failed() {
			collectDiagnostics(ctxRS, testNS)
		}
		// 清理
		pol := &latticev1.LatticePolicy{ObjectMeta: metav1.ObjectMeta{Name: restartPol, Namespace: testNS}}
		_ = latticeClient.Delete(ctxRS, pol)
		cleanupWorkspace(clientset, testNS)
	})
})
