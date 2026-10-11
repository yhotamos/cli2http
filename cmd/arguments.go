package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yhotamos/cli2http/internal/runner"
	"github.com/yhotamos/cli2http/internal/server"
)

// parseOptions leaves publication arguments untouched and lets Cobra's flags parse option values.
func parseOptions(cmd *cobra.Command, args []string) ([]string, error) {
	end := 0
	for end < len(args) {
		argument := args[end]
		if argument == "--path" || strings.HasPrefix(argument, "--path=") || !strings.HasPrefix(argument, "-") {
			break
		}
		if argument == "--" {
			break
		}
		name, _, hasValue := strings.Cut(argument, "=")
		flag := cmd.Flags().Lookup(strings.TrimPrefix(name, "--"))
		if !strings.HasPrefix(name, "--") {
			flag = cmd.Flags().ShorthandLookup(strings.TrimPrefix(name, "-"))
		}
		if flag == nil {
			return nil, fmt.Errorf("unknown option %q", name)
		}
		end++
		if flag.NoOptDefVal == "" && !hasValue && end < len(args) {
			end++
		}
	}
	if err := cmd.Flags().Parse(args[:end]); err != nil {
		return nil, err
	}
	if end < len(args) && args[end] == "--" {
		end++
	}
	return args[end:], nil
}

func parseEndpoints(args []string) ([]server.Endpoint, error) {
	var endpoints []server.Endpoint
	for len(args) > 0 {
		path := "/exec"
		if args[0] == "--path" {
			if len(args) < 3 {
				return nil, fmt.Errorf("--path requires a path and a command string")
			}
			path = args[1]
			args = args[2:]
		} else if value, found := strings.CutPrefix(args[0], "--path="); found {
			if len(args) < 2 {
				return nil, fmt.Errorf("--path requires a command string")
			}
			path = value
			args = args[1:]
		} else if len(endpoints) != 0 || strings.HasPrefix(args[0], "-") {
			return nil, fmt.Errorf("unexpected argument %q: use --path <path> <command-string> for another endpoint", args[0])
		}
		target, err := runner.ResolveCommand(args[0])
		if err != nil {
			return nil, err
		}
		endpoints = append(endpoints, server.Endpoint{Path: path, Target: target})
		args = args[1:]
	}
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("a command string is required")
	}
	return endpoints, nil
}
