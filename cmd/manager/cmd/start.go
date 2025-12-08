package cmd

import (
	"wireflow/cmd/manager/controller"
	"wireflow/cmd/manager/drp"
	"wireflow/cmd/manager/management"
	"wireflow/cmd/manager/turn"

	"github.com/spf13/cobra"
)

// start cmd
func newStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Starts a Wireflow component (controller, client, drp, turn).",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.AddCommand(controller.NewControllerCmd())
	cmd.AddCommand(drp.NewDrpCmd())
	cmd.AddCommand(turn.NewTurnCmd())
	cmd.AddCommand(management.NewManagementCmd())

	return cmd
}
