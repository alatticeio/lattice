package e2e

import (
	"context"
	"flag"
	"fmt"
	latticev1 "github.com/alatticeio/lattice/api/v1alpha1"
	"testing"

	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	restConfig    *rest.Config
	clientset     *kubernetes.Clientset
	latticeClient client.Client
	ns            string
	agentImage    string
	manageUrl     string
	kubeconfig    string
)

func init() {
	flag.StringVar(&agentImage, "agent-image", "ghcr.io/winstonfly/lattice:e2e", "Docker image for the lattice agent")
	flag.StringVar(&manageUrl, "manage-url", "http://localhost:8080", "Lattice manager API base URL")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig (defaults to ~/.kube/config)")
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Lattice E2E Suite")
}

var _ = BeforeSuite(func() {
	By("初始化测试环境")

	kubecfgPath := kubeconfig
	if kubecfgPath == "" {
		kubecfgPath = filepath.Join(homedir.HomeDir(), ".kube", "config")
	}

	var err error
	restConfig, err = clientcmd.BuildConfigFromFlags("", kubecfgPath)
	Expect(err).NotTo(HaveOccurred(), "无法加载 kubeconfig: %s", kubecfgPath)

	clientset, err = kubernetes.NewForConfig(restConfig)
	Expect(err).NotTo(HaveOccurred(), "无法创建 Clientset")

	s := scheme.Scheme
	err = latticev1.AddToScheme(s)
	Expect(err).NotTo(HaveOccurred(), "无法注册 LatticePeer Scheme")

	latticeClient, err = client.New(restConfig, client.Options{Scheme: s})
	Expect(err).NotTo(HaveOccurred(), "无法创建 CRD Client")

	By("测试环境就绪，Namespace: " + ns)
})

// ReportAfterSuite 是 Ginkgo v2 中获取套件整体通过/失败状态的正确方式
var _ = ReportAfterSuite("e2e cleanup", func(report Report) {
	if clientset == nil || ns == "" {
		return
	}

	ctx := context.Background()

	if report.SuiteSucceeded {
		By("测试全部通过，清理 Namespace: " + ns)

		deletePolicy := metav1.DeletePropagationBackground
		err := clientset.CoreV1().Namespaces().Delete(ctx, ns, metav1.DeleteOptions{
			PropagationPolicy: &deletePolicy,
		})
		if err != nil && !errors.IsNotFound(err) {
			fmt.Printf("[WARN] 清理 Namespace 失败: %v\n", err)
			return
		}

		Eventually(func() bool {
			_, err := clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
			return errors.IsNotFound(err)
		}, "120s", "3s").Should(BeTrue(), "Namespace 删除超时: %s", ns)

		By("资源清理完成")
	} else {
		fmt.Printf("\n[E2E FAILED] 保留现场以供排查。\n  kubectl get pods -n %s\n  kubectl delete ns %s\n\n", ns, ns)
	}
})
