package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func Execute() error {
	rootCmd := &cobra.Command{
		Use:   "cli2http",
		Short: "A lightweight CLI-to-HTTP bridge",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "helloworld")
			return err
		},
	}

	return rootCmd.Execute()
}
