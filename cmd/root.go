package cmd

import (
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	"os"
)

var rootCmd = &cobra.Command{
	Use:          "linkany [command]",
	SilenceUsage: true,
	Short:        "any",
	Long:         `linkany up, login, logout, register and also so on`,
}

func Execute() {
	rootCmd.AddCommand(up(), loginCmd(), drpCmd(), turnCmd(), managementCmd())
	if err := rootCmd.Execute(); err != nil {
		klog.Errorf("rootCmd execute failed: %v", err)
		os.Exit(-1)
	}
}
