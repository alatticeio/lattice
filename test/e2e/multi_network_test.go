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
	netPeerA  = "mnet-a"
	netPeerB  = "mnet-b"
	netPeerC  = "mnet-c"
	policyAll = "e2e-mnet-allow"
)

var _ = Describe("多网络隔离测试", Ordered, func() {
	var (
		accessToken string
		workspaceID string
		joinToken   string
		testNS      string
		podNames    map[string]string
		podIPs      map[string]string
		net2Name    string
		ctxB        = context.Background()
	)

	// 创建网络1（默认）+ 网络2，部署 3 个 Pod
	BeforeAll(func() {
		accessToken = login(manageUrl)

		nsName := fmt.Sprintf("wf-e2e-multinet-%d", time.Now().UnixMilli())
		workspaceID = createWorkspace(manageUrl, accessToken, nsName)
		testNS = fmt.Sprintf("wf-%s", workspaceID)

		joinToken = generateJoinToken(manageUrl, accessToken, workspaceID)
		hostAliases := hostAliasesForNATS(clientset)

		// 创建第 2 个 LatticeNetwork（通过 CRD）
		net2Name = fmt.Sprintf("e2e-net2-%d", time.Now().UnixMilli())
		net2 := &latticev1.LatticeNetwork{
			ObjectMeta: metav1.ObjectMeta{
				Name:      net2Name,
				Namespace: testNS,
			},
			Spec: latticev1.LatticeNetworkSpec{
				Name: net2Name,
			},
		}
		Expect(latticeClient.Create(ctxB, net2)).To(Succeed(), "创建第 2 个 LatticeNetwork 失败")

		// 部署 3 个 Pod，全部加入默认网络
		for _, role := range []string{netPeerA, netPeerB, netPeerC} {
			deployAgentDeployment(clientset, testNS, role, agentImage, joinToken, hostAliases)
		}

		podNames = make(map[string]string)
		podIPs = make(map[string]string)
		for _, role := range []string{netPeerA, netPeerB, netPeerC} {
			podNames[role] = waitForPodRunningReady(clientset, testNS, role, "180s")
			podIPs[role] = waitForWGIP(latticeClient, testNS, role, "90s")
		}

		By(fmt.Sprintf("全部 Pod 就绪: A=%s(%s), B=%s(%s), C=%s(%s)",
			podNames[netPeerA], podIPs[netPeerA],
			podNames[netPeerB], podIPs[netPeerB],
			podNames[netPeerC], podIPs[netPeerC]))
	})

	// 将 mnet-c 移到网络 2
	It("将 mnet-c 切换到第 2 个网络，验证控制器重新分配 IP", func() {
		peerC := &latticev1.LatticePeer{}
		Expect(latticeClient.Get(ctxB, sigclient.ObjectKey{Namespace: testNS, Name: netPeerC}, peerC)).To(Succeed())

		By("更新 mnet-c 的 Spec.Network 指向第 2 个网络: " + net2Name)
		peerC.Spec.Network = &net2Name
		Expect(latticeClient.Update(ctxB, peerC)).To(Succeed(), "更新 LatticePeer 网络失败")

		// 等待控制器将 mnet-c 移到新网络并重新分配 IP
		By("等待 mnet-c 在新网络中获得新 IP")
		Eventually(func() error {
			updated := &latticev1.LatticePeer{}
			if err := latticeClient.Get(ctxB, sigclient.ObjectKey{Namespace: testNS, Name: netPeerC}, updated); err != nil {
				return err
			}
			if updated.Status.ActiveNetwork == nil || *updated.Status.ActiveNetwork != net2Name {
				return fmt.Errorf("ActiveNetwork 尚未切换，当前: %v", updated.Status.ActiveNetwork)
			}
			if updated.Status.AllocatedAddress == nil || *updated.Status.AllocatedAddress == "" {
				return fmt.Errorf("新 IP 尚未分配")
			}
			// 更新缓存中的 IP
			ip := *updated.Status.AllocatedAddress
			if idx := strings.Index(ip, "/"); idx != -1 {
				ip = ip[:idx]
			}
			podIPs[netPeerC] = ip
			return nil
		}, "60s", "3s").Should(Succeed(), "mnet-c 未能切换到第 2 个网络")

		By(fmt.Sprintf("mnet-c 已切换到网络 %s，新 IP: %s", net2Name, podIPs[netPeerC]))
	})

	// 验证同一网络内互通（mnet-a ↔ mnet-b）
	It("同网络互通: mnet-a → mnet-b ping 应成功", func() {
		By("为默认网络创建 ALLOW 策略")
		peerA := &latticev1.LatticePeer{}
		Expect(latticeClient.Get(ctxB, sigclient.ObjectKey{Namespace: testNS, Name: netPeerA}, peerA)).To(Succeed())
		net1Name := getNetworkName(peerA)
		Expect(net1Name).NotTo(BeEmpty(), "无法获取网络1的名称")
		createAllowAllPolicy(latticeClient, testNS, policyAll, net1Name)
		// 也创建网络 2 的策略，让 mnet-c 自身有策略
		createAllowAllPolicy(latticeClient, testNS, policyAll+"-net2", net2Name)

		pingWithRetry(clientset, restConfig, testNS, podNames[netPeerA], podIPs[netPeerB], "60s")
	})

	// 验证跨网络隔离（mnet-a → mnet-c 不通）
	It("跨网络隔离: 不同网络的 Pod 应无法通信", func() {
		assertPingBlocked(clientset, restConfig, testNS, podNames[netPeerA], podIPs[netPeerC], "30s")
	})

	// 验证 mnet-b → mnet-c 也不通
	It("跨网络隔离: mnet-b → mnet-c 也应阻断", func() {
		assertPingBlocked(clientset, restConfig, testNS, podNames[netPeerB], podIPs[netPeerC], "30s")
	})

	// 清理
	AfterAll(func() {
		if CurrentSpecReport().Failed() {
			collectDiagnostics(ctxB, testNS)
		}
		// 删除策略，避免残留
		for _, p := range []string{policyAll, policyAll + "-net2"} {
			pol := &latticev1.LatticePolicy{ObjectMeta: metav1.ObjectMeta{Name: p, Namespace: testNS}}
			_ = latticeClient.Delete(ctxB, pol)
		}
		cleanupWorkspace(clientset, testNS)
	})
})
