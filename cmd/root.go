package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"cli2http/internal/runner"
	"cli2http/internal/server"

	"github.com/spf13/cobra"
)

func newRootCommand() *cobra.Command {
	return &cobra.Command{
		Use:          "cli2http <command>",
		Short:        "A lightweight CLI-to-HTTP bridge",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE:         runRoot,
	}
}

// Execute runs the CLI until completion or an interrupt.
func Execute() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return newRootCommand().ExecuteContext(ctx)
}

func runRoot(cmd *cobra.Command, args []string) error {
	target, err := runner.Resolve(args[0])
	if err != nil {
		return err
	}
	srv, err := server.New(target)
	if err != nil {
		return err
	}
	defer srv.Close()
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Command : %s\nAddress : %s\nToken   : %s\n\nPOST /exec\n", srv.Command(), srv.Address(), srv.Token()); err != nil {
		return err
	}
	return srv.Run(cmd.Context())
}
