package cmd

import (
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	"os"
)

var rootCmd = &cobra.Command{
	Use:          "up [command]",
	SilenceUsage: true,
	Short:        "any",
	Long:         `start up up, login, logout, register and also will use https to serve DRP`,
}

func Execute() {
	rootCmd.AddCommand(up(), loginCmd(), drpCmd(), turnCmd(), managementCmd())
	if err := rootCmd.Execute(); err != nil {
		klog.Errorf("rootCmd execute failed: %v", err)
		os.Exit(-1)
	}
}
