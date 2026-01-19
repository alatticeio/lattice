package cmd

import (
	"github.com/spf13/cobra"
)

// start cmd
func newStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Starts a Wireflow component (controller, client, drp, turn).",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.AddCommand(newControllerCmd())
	cmd.AddCommand(newWrrpCmd())
	cmd.AddCommand(newTurnCmd())
	cmd.AddCommand(newManagementCmd())

	return cmd
}
