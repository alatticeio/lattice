package controller

import (
	"crypto/tls"
	"path/filepath"
	"wireflow/internal/controller"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	wireflowcontrollerv1alpha1 "github.com/wireflowio/wireflow-controller/api/v1alpha1"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(wireflowcontrollerv1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

func NewControllerCmd() *cobra.Command {
	flag := new(ControllerFlags)
	cmd := &cobra.Command{
		Short:        "controller",
		Use:          "controller [command]",
		SilenceUsage: true,
		Long:         `wireflow core controller for CRDs reconcile`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runController(flag)
		},
	}

	fs := cmd.Flags()
	fs.StringVarP(&flag.metricsAddr, "metrics-bind-address", "", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	fs.StringVarP(&flag.probeAddr, "health-probe-bind-address", "", ":8081", "The address the probe endpoint binds to.")
	fs.BoolVarP(&flag.enableLeaderElection, "leader-elect", "", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	fs.BoolVarP(&flag.secureMetrics, "metrics-secure", "", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	fs.StringVarP(&flag.webhookCertPath, "webhook-cert-path", "", "", "The directory that contains the webhook certificate.")
	fs.StringVarP(&flag.webhookCertName, "webhook-cert-name", "", "tls.crt", "The name of the webhook certificate file.")
	fs.StringVarP(&flag.webhookCertKey, "webhook-cert-key", "", "tls.key", "The name of the webhook key file.")
	fs.StringVarP(&flag.metricsCertPath, "metrics-cert-path", "", "",
		"The directory that contains the metrics server certificate.")
	fs.StringVarP(&flag.metricsCertName, "metrics-cert-name", "", "tls.crt", "The name of the metrics server certificate file.")
	fs.StringVarP(&flag.metricsCertKey, "metrics-cert-key", ",", "tls.key", "The name of the metrics server key file.")
	fs.BoolVarP(&flag.enableHTTP2, "enable-http2", "", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")

	return cmd
}

type ControllerFlags struct {
	metricsAddr          string
	webhookCertPath      string
	webhookCertName      string
	webhookCertKey       string
	metricsCertPath      string
	metricsCertName      string
	metricsCertKey       string
	enableLeaderElection bool
	probeAddr            string
	secureMetrics        bool
	enableHTTP2          bool
}

// nolint:gocyclo
func runController(flags *ControllerFlags) error {
	var tlsOpts []func(*tls.Config)

	opts := zap.Options{
		Development: true,
	}
	//opts.BindFlags(flag.CommandLine)
	//flag.Parse()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	logs.InitLogs()
	ctrl.SetLogger(klog.Background()) // 使用 klog 的 Background logr 实例
	defer logs.FlushLogs()            // 确保在程序退出时日志被刷新

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !flags.enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Create watchers for metrics and webhooks certificates
	var metricsCertWatcher, webhookCertWatcher *certwatcher.CertWatcher

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts

	if len(flags.webhookCertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", flags.webhookCertPath, "webhook-cert-name", flags.webhookCertName, "webhook-cert-key", flags.webhookCertKey)

		var err error
		webhookCertWatcher, err = certwatcher.New(
			filepath.Join(flags.webhookCertPath, flags.webhookCertName),
			filepath.Join(flags.webhookCertPath, flags.webhookCertKey),
		)
		if err != nil {
			setupLog.Error(err, "Failed to initialize webhook certificate watcher")
			return err
		}

		webhookTLSOpts = append(webhookTLSOpts, func(config *tls.Config) {
			config.GetCertificate = webhookCertWatcher.GetCertificate
		})
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: webhookTLSOpts,
	})

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   flags.metricsAddr,
		SecureServing: flags.secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if flags.secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	//
	// TODO(user): If you enable certManager, uncomment the following lines:
	// - [METRICS-WITH-CERTS] at config/default/kustomization.yaml to generate and use certificates
	// managed by cert-manager for the metrics server.
	// - [PROMETHEUS-WITH-CERTS] at config/prometheus/kustomization.yaml for TLS certification.
	if len(flags.metricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", flags.metricsCertPath, "metrics-cert-name", flags.metricsCertName, "metrics-cert-key", flags.metricsCertKey)

		var err error
		metricsCertWatcher, err = certwatcher.New(
			filepath.Join(flags.metricsCertPath, flags.metricsCertName),
			filepath.Join(flags.metricsCertPath, flags.metricsCertKey),
		)
		if err != nil {
			setupLog.Error(err, "to initialize metrics certificate watcher", "error", err)
			return err
		}

		metricsServerOptions.TLSOpts = append(metricsServerOptions.TLSOpts, func(config *tls.Config) {
			config.GetCertificate = metricsCertWatcher.GetCertificate
		})
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: flags.probeAddr,
		LeaderElection:         flags.enableLeaderElection,
		LeaderElectionID:       "05657094.wireflow.io",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		return err
	}

	if err := (&controller.NodeReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		Detector:     controller.NewChangeDetector(mgr.GetClient()),
		NodeCtxCache: make(map[types.NamespacedName]*controller.NodeContext),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Node")
		return err
	}
	if err := (&controller.NetworkReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		Allocator: controller.NewIPAllocator(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Networks")
		return err
	}
	if err := (&controller.NetworkPolicyReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "NetworkPolicy")
		return err
	}
	// +kubebuilder:scaffold:builder

	if metricsCertWatcher != nil {
		setupLog.Info("Adding metrics certificate watcher to manager")
		if err := mgr.Add(metricsCertWatcher); err != nil {
			setupLog.Error(err, "unable to add metrics certificate watcher to manager")
			return err
		}
	}

	if webhookCertWatcher != nil {
		setupLog.Info("Adding webhook certificate watcher to manager")
		if err := mgr.Add(webhookCertWatcher); err != nil {
			setupLog.Error(err, "unable to add webhook certificate watcher to manager")
			return err
		}
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		return err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		return err
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		return err
	}

	return nil
}
