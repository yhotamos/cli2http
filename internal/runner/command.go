package runner

import (
	"fmt"
	"strings"
)

// ResolveCommand splits a command string and resolves its executable at startup.
// Quoting follows the host OS; shell syntax and expansions are never executed.
func ResolveCommand(commandLine string) (Target, error) {
	if strings.TrimSpace(commandLine) == "" {
		return Target{}, fmt.Errorf("command string must contain an executable")
	}
	if strings.ContainsRune(commandLine, 0) {
		return Target{}, fmt.Errorf("command string must not contain NUL")
	}
	args, err := splitCommandLine(commandLine)
	if err != nil {
		return Target{}, fmt.Errorf("parse command string: %w", err)
	}
	if len(args) == 0 || args[0] == "" {
		return Target{}, fmt.Errorf("command string must contain an executable")
	}
	target, err := Resolve(args[0])
	if err != nil {
		return Target{}, err
	}
	target.FixedArgs = args[1:]
	return target, nil
}
