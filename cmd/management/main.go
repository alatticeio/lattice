package main

import (
	"github.com/spf13/cobra"
	"linkany/cmd/management/command"
	"linkany/pkg/log"
	"os"
)

func main() {
	logger := log.NewLogger(log.Loglevel, "linkany")
	rootCmd := &cobra.Command{Use: "linkany [command]", SilenceUsage: true, Short: "any", Long: `linkany support up, login, logout, register, manager, turn command,`}
	rootCmd.AddCommand(command.ManagementCmd())
	if err := rootCmd.Execute(); err != nil {
		logger.Errorf("management cmd execute failed: %v", err)
		os.Exit(-1)
	}
}
