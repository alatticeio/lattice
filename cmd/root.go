package cmd

import (
	"github.com/spf13/cobra"
	"linkany/pkg/log"
	"os"
)

var rootCmd = &cobra.Command{
	Use:          "linkany [command]",
	SilenceUsage: true,
	Short:        "any",
	Long:         `linkany up, login, logout, register and also so on`,
}

func Execute() {
	logger := log.NewLogger(log.Loglevel, "linkany")
	rootCmd.AddCommand(up(), loginCmd(), drpCmd(), turnCmd(), managementCmd())
	if err := rootCmd.Execute(); err != nil {
		logger.Errorf("rootCmd execute failed: %v", err)
		os.Exit(-1)
	}
}
