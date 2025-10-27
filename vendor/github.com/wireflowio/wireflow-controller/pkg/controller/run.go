package controller

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/wireflowio/wireflow-controller/pkg/http"
	"github.com/wireflowio/wireflow-controller/pkg/signals"
	"k8s.io/client-go/rest"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	clientset "github.com/wireflowio/wireflow-controller/pkg/generated/clientset/versioned"
	informers "github.com/wireflowio/wireflow-controller/pkg/generated/informers/externalversions"
)

type PushOptions struct {
	Server string
}

func Run(options *PushOptions) error {
	klog.InitFlags(nil)
	flag.Parse()

	var kubeconfig string
	// if running in mac os x, the kubeconfig file is in /Users/username/.kube/config
	if runtime.GOOS == "darwin" {
		dir, _ := os.UserHomeDir()
		kubeconfig = dir + "/.kube/config"
	} else {
		kubeconfig = "/root/.kube/config"
	}

	// set up signals so we handle the shutdown signal gracefully
	ctx := signals.SetupSignalHandler()
	logger := klog.FromContext(ctx)

	// 尝试使用 kubeconfig 文件
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		klog.Warningf("using in-cluster configuration: %v", err)
		// 如果失败，尝试使用 in-cluster 配置
		cfg, err = rest.InClusterConfig()
		if err != nil {
			return fmt.Errorf("Error when createing kubernetes config: %v", err)
		}
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logger.Error(err, "Error building kubernetes clientset")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	client, err := clientset.NewForConfig(cfg)
	if err != nil {
		logger.Error(err, "Error building kubernetes clientset")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	informerFactory := informers.NewSharedInformerFactory(client, time.Second*30)

	httpClient := http.NewHttpClient(
		http.WithTimtout(10*time.Second),
		http.WithBaseURL(options.Server),
		http.WithHeaders(map[string]string{
			"User-Agent": "wireflow-controller",
		}),
	)

	controller := NewController(ctx, kubeClient, client,
		httpClient,
		informerFactory.Wireflowcontroller().V1alpha1().Nodes(),
		informerFactory.Wireflowcontroller().V1alpha1().Networks())

	// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(ctx.done())
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	kubeInformerFactory.Start(ctx.Done())
	informerFactory.Start(ctx.Done())

	if err = controller.Run(ctx, 2); err != nil {
		logger.Error(err, "Error running controller")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	return nil
}
