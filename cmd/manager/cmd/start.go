package cmd

import (
	"github.com/spf13/cobra"
)

// start cmd
func newStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Starts a Lattice component (controller, client, drp).",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.AddCommand(newControllerCmd())
	cmd.AddCommand(newLrpCmd())
	cmd.AddCommand(newManagementCmd())

	return cmd
}
