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

	cmd.AddCommand(NewControllerCmd())
	cmd.AddCommand(NewDrpCmd())
	cmd.AddCommand(NewTurnCmd())
	cmd.AddCommand(NewManagementCmd())

	return cmd
}
