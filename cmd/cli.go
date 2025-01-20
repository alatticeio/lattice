package cmd

import "github.com/spf13/cobra"

func cli() *cobra.Command {
	return &cobra.Command{
		Short:        "any",
		Use:          "any [command]",
		SilenceUsage: true,
		Long:         `start controller`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCli()
		},
	}
}

func runCli() error {
	return nil
}
