package cmd

import (
	"wireflow/cmd/wfctl/controller"
	"wireflow/cmd/wfctl/drp"
	"wireflow/cmd/wfctl/management"
	"wireflow/cmd/wfctl/turn"

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
