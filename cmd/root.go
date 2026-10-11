package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"

	"github.com/yhotamos/cli2http/internal/server"

	"github.com/spf13/cobra"
	"golang.org/x/mod/module"
	"golang.org/x/mod/semver"
)

var version = "dev"

func newRootCommand() *cobra.Command {
	command := &cobra.Command{
		Use:                   `cli2http [options] [--path <path>] <command-string> ...`,
		DisableFlagsInUseLine: true,
		DisableFlagParsing:    true,
		Short:                 "A lightweight CLI-to-HTTP bridge",
		SilenceUsage:          true,
		Version:               buildVersion(),
		RunE:                  runRoot,
	}
	command.Flags().Duration("timeout", 0, "Command execution timeout (e.g. 30s or 2m; 0 disables the limit)")
	command.Flags().String("path", "/exec", "Publish a command string at this path (repeatable)")
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
	args, err := parseOptions(cmd, args)
	if err != nil {
		return err
	}
	if help, _ := cmd.Flags().GetBool("help"); help {
		return cmd.Help()
	}
	if showVersion, _ := cmd.Flags().GetBool("version"); showVersion {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s version %s\n", cmd.Name(), cmd.Version)
		return err
	}
	endpoints, err := parseEndpoints(args)
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
	timeout, err := cmd.Flags().GetDuration("timeout")
	if err != nil {
		return err
	}
	srv, err := server.Listen(endpoints, port, token, timeout)
	if err != nil {
		return err
	}
	defer srv.Close()
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "URL     : %s\nToken   : %s\n", srv.URL(), srv.Token()); err != nil {
		return err
	}
	for _, endpoint := range endpoints {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s : %s\n", endpoint.Path, endpoint.Target.Command); err != nil {
			return err
		}
	}
	return srv.Run(cmd.Context(), cmd.OutOrStdout())
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
