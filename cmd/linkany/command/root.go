package command

import (
	"github.com/spf13/cobra"
	"linkany/pkg/log"
	"os"
)

var rootCmd = &cobra.Command{
	Use:          "linkany [command]",
	SilenceUsage: true,
	Short:        "any",
	Long:         `linkany support UP, login, logout, register, manager, turn command,`,
}

func Execute() {
	logger := log.NewLogger(log.Loglevel, "linkany")
	rootCmd.AddCommand(UP(), LoginCmd())
	if err := rootCmd.Execute(); err != nil {
		logger.Errorf("rootCmd execute failed: %v", err)
		os.Exit(-1)
	}
}
