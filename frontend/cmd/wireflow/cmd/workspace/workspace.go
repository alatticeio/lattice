package workspace

import (
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "workspace",
	Short: "Workspace management commands",
	Long:  `Manage Lattice workspaces, including creation, repair, and other operations`,
}

func init() {
	// Register subcommands
}
