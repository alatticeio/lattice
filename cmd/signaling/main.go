package main

import (
	"github.com/spf13/cobra"
	"linkany/cmd/signaling/command"
	"linkany/pkg/log"
	"os"
)

func main() {
	logger := log.NewLogger(log.Loglevel, "linkany")
	rootCmd := &cobra.Command{Use: "linkany [command]", SilenceUsage: true, Short: "any", Long: `linkany support up, login, logout, register, manager, turn command,`}
	rootCmd.AddCommand(command.SignalingCmd())
	if err := rootCmd.Execute(); err != nil {
		logger.Errorf("signaling cmd execute failed: %v", err)
		os.Exit(-1)
	}
}
