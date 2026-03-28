// Copyright 2025 The Wireflow Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"wireflow/internal/config"
	"wireflow/internal/controller"

	"github.com/spf13/cobra"
	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	// +kubebuilder:scaffold:imports
)

func newControllerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Short:        "controller",
		Use:          "controller [command]",
		SilenceUsage: true,
		Long:         `wireflow core controller for CRDs reconcile`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. 检查用户是否传了 --save
			save, _ := cmd.Flags().GetBool("save")
			if save {
				// 2. 执行保存
				fmt.Println("Saving configuration...")

				if err := cfgManager.Save(); err != nil {
					return fmt.Errorf("failed to save config: %w", err)
				}

				fmt.Printf("Config saved to: %s\n", config.GetConfigFilePath())
			}

			return runController(config.Conf)
		},
	}

	fs := cmd.Flags()
	fs.StringP("metrics-bind-address", "", "0", "The address the metrics endpoint binds to. "+
		"Use :8443 for HTTPS or :8080 for HTTP, or leave as 0 to disable the metrics service.")
	fs.StringP("health-probe-bind-address", "", ":8081", "The address the probe endpoint binds to.")
	fs.BoolP("leader-elect", "", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	fs.BoolP("metrics-secure", "", true,
		"If set, the metrics endpoint is served securely via HTTPS. Use --metrics-secure=false to use HTTP instead.")
	fs.StringP("webhook-cert-path", "", "", "The directory that contains the webhook certificate.")
	fs.StringP("webhook-cert-name", "", "tls.crt", "The name of the webhook certificate file.")
	fs.StringP("webhook-cert-key", "", "tls.key", "The name of the webhook key file.")
	fs.StringP("metrics-cert-path", "", "",
		"The directory that contains the metrics server certificate.")
	fs.StringP("metrics-cert-name", "", "tls.crt", "The name of the metrics server certificate file.")
	fs.StringP("metrics-cert-key", ",", "tls.key", "The name of the metrics server key file.")
	fs.BoolP("enable-http2", "", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")

	return cmd
}

// nolint:all
type ControllerFlags struct {
	metricsAddr          string
	webhookCertPath      string
	webhookCertName      string
	webhookCertKey       string
	metricsCertPath      string
	metricsCertName      string
	metricsCertKey       string
	enableLeaderElection bool
	//probeAddr            string
	//secureMetrics        bool
	//enableHTTP2          bool
}

// nolint:gocyclo
func runController(flags *config.Config) error {
	return controller.Start(flags)
}
