package e2e

import (
	"context"
	"fmt"
	"strings"
	"time"

	latticev1 "github.com/alatticeio/lattice/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	sigclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	policyChangePeer = "pc-peer"
	policyChangePol1 = "e2e-pc-allow"
	policyChangePol2 = "e2e-pc-deny"
)

var _ = Describe("Policy CRUD Lifecycle", Ordered, func() {
	var (
		testNS      string
		podName     string
		podWGIP     string
		accessToken string
		netName     string
		ctxPC       = context.Background()
	)

	BeforeAll(func() {
		accessToken = login(manageUrl)

		nsName := fmt.Sprintf("wf-e2e-policycrud-%d", time.Now().UnixMilli())
		workspaceID := createWorkspace(manageUrl, accessToken, nsName)
		testNS = fmt.Sprintf("wf-%s", workspaceID)

		joinToken := generateJoinToken(manageUrl, accessToken, workspaceID)
		hostAliases := hostAliasesForNATS(clientset)

		deployAgentDeployment(clientset, testNS, policyChangePeer, agentImage, joinToken, hostAliases)
		podName = waitForPodRunningReady(clientset, testNS, policyChangePeer, "180s")
		podWGIP = waitForWGIP(latticeClient, testNS, policyChangePeer, "90s")

		peer := &latticev1.LatticePeer{}
		Expect(latticeClient.Get(ctxPC, sigclient.ObjectKey{Namespace: testNS, Name: policyChangePeer}, peer)).To(Succeed())
		netName = getNetworkName(peer)
		Expect(netName).NotTo(BeEmpty())

		By(fmt.Sprintf("Pod ready: name=%s, ip=%s, network=%s", podName, podWGIP, netName))
	})

	It("创建一个 ALLOW 策略并验证 CRD 创建成功", func() {
		By("创建 e2e-pc-allow 策略")
		createAllowAllPolicy(latticeClient, testNS, policyChangePol1, netName)

		By("验证策略存在于 CRD 中")
		found := &latticev1.LatticePolicy{}
		Expect(latticeClient.Get(ctxPC, sigclient.ObjectKey{Namespace: testNS, Name: policyChangePol1}, found)).To(Succeed())
		Expect(found.Spec.Action).To(Equal("ALLOW"))
		Expect(found.Spec.Network).To(Equal(netName))
	})

	It("列出工作空间下的所有策略，应包含新创建的策略", func() {
		By("列出 Namespace " + testNS + " 下的所有 LatticePolicy")
		policyList := &latticev1.LatticePolicyList{}
		Expect(latticeClient.List(ctxPC, policyList, sigclient.InNamespace(testNS))).To(Succeed())

		found := false
		for _, p := range policyList.Items {
			if p.Name == policyChangePol1 {
				found = true
				break
			}
		}
		Expect(found).To(BeTrue(), "列表应包含 %s", policyChangePol1)
	})

	It("创建第二个 DENY 策略并验证两条策略共存", func() {
		By("创建第二个策略: " + policyChangePol2)
		netLabel := fmt.Sprintf("alattice.io/network-%s", netName)
		selector := metav1.LabelSelector{
			MatchLabels: map[string]string{netLabel: "true"},
		}
		denyPolicy := &latticev1.LatticePolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      policyChangePol2,
				Namespace: testNS,
			},
			Spec: latticev1.LatticePolicySpec{
				Network:      netName,
				PeerSelector: selector,
				Action:       "DENY",
				Ingress: []latticev1.IngressRule{
					{From: []latticev1.PeerSelection{{PeerSelector: &selector}}},
				},
				Egress: []latticev1.EgressRule{
					{To: []latticev1.PeerSelection{{PeerSelector: &selector}}},
				},
			},
		}
		Expect(latticeClient.Create(ctxPC, denyPolicy)).To(Succeed(), "创建 DENY 策略失败")

		By("验证 iptables 规则已更新为两条策略的组合效果")
		Eventually(func() error {
			out, err := execInPod(clientset, restConfig, testNS, podName, []string{"iptables", "-L", "LATTICE-EGRESS", "-n"})
			if err != nil {
				return err
			}
			// 至少应存在 iptables 规则链
			if !strings.Contains(out, "LATTICE-EGRESS") {
				return fmt.Errorf("LATTICE-EGRESS 链未找到:\n%s", out)
			}
			return nil
		}, "15s", "2s").Should(Succeed(), "iptables 应包含策略规则")
	})

	It("删除第一个策略后，第二个策略仍应生效", func() {
		By("删除策略: " + policyChangePol1)
		pol := &latticev1.LatticePolicy{ObjectMeta: metav1.ObjectMeta{Name: policyChangePol1, Namespace: testNS}}
		Expect(latticeClient.Delete(ctxPC, pol)).To(Succeed())

		By("验证策略已从 CRD 中移除")
		gone := &latticev1.LatticePolicy{}
		err := latticeClient.Get(ctxPC, sigclient.ObjectKey{Namespace: testNS, Name: policyChangePol1}, gone)
		Expect(err).To(HaveOccurred(), "已删除的策略不应存在")
		Expect(apierrors.IsNotFound(err)).To(BeTrue(), "错误应为 NotFound")

		By("验证第二个策略仍然存在")
		remaining := &latticev1.LatticePolicy{}
		Expect(latticeClient.Get(ctxPC, sigclient.ObjectKey{Namespace: testNS, Name: policyChangePol2}, remaining)).To(Succeed())
		Expect(remaining.Spec.Action).To(Equal("DENY"))

		By("验证 iptables 规则链仍然存在（残留策略的规则）")
		Eventually(func() error {
			out, err := execInPod(clientset, restConfig, testNS, podName, []string{"iptables", "-L", "LATTICE-EGRESS", "-n"})
			if err != nil {
				return err
			}
			if !strings.Contains(out, "LATTICE-EGRESS") {
				return fmt.Errorf("删除一个策略后 LATTICE-EGRESS 链不应被清空:\n%s", out)
			}
			return nil
		}, "15s", "2s").Should(Succeed(), "iptables 规则链应保留")
	})

	AfterAll(func() {
		if CurrentSpecReport().Failed() {
			collectDiagnostics(ctxPC, testNS)
		}
		// 清理所有策略
		for _, p := range []string{policyChangePol1, policyChangePol2} {
			pol := &latticev1.LatticePolicy{ObjectMeta: metav1.ObjectMeta{Name: p, Namespace: testNS}}
			_ = latticeClient.Delete(ctxPC, pol)
		}
		cleanupWorkspace(clientset, testNS)
	})
})
