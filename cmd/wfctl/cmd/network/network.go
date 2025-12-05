package network

import (
	"github.com/spf13/cobra"
)

// start cmd
func NewNetworkCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "network",
		Short: "all network operations, (join, leave)",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.AddCommand(newCreateCmd())
	cmd.AddCommand(newRemoveCmd())
	cmd.AddCommand(newJoinCmd())
	cmd.AddCommand(newLeaveCmd())
	cmd.AddCommand(newUpdateCmd())

	return cmd
}
