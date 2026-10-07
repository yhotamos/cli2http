package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"

	"github.com/yhotamos/cli2http/internal/runner"
	"github.com/yhotamos/cli2http/internal/server"

	"github.com/spf13/cobra"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

var version = "dev"

func newRootCommand() *cobra.Command {
	command := &cobra.Command{
		Use:          "cli2http <command>",
		Short:        "A lightweight CLI-to-HTTP bridge",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		Version:      buildVersion(),
		RunE:         runRoot,
	}
	command.Flags().Int("port", 0, "Port to listen on (0 selects an available port)")
	command.Flags().String("token", "", "Authentication token (at least 16 characters; ASCII letters, digits, - and _)")
	return command
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
	port, err := cmd.Flags().GetInt("port")
	if err != nil {
		return err
	}
	token, err := cmd.Flags().GetString("token")
	if err != nil {
		return err
	}
	if cmd.Flags().Changed("token") && token == "" {
		return fmt.Errorf("invalid token: must not be empty")
	}
	srv, err := server.Listen(target, port, token)
	if err != nil {
		return err
	}
	defer srv.Close()
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Command : %s\nURL     : %s\nToken   : %s\n", srv.Command(), srv.URL(), srv.Token()); err != nil {
		return err
	}
	return srv.Run(cmd.Context())
}

func buildVersion() string {
	if version != "dev" {
		return version
	}
	info, ok := debug.ReadBuildInfo()
	if ok && semver.IsValid(info.Main.Version) && !module.IsPseudoVersion(info.Main.Version) {
		return info.Main.Version
	}
	return version
}
