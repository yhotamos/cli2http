//go:build !windows

package runner

import "github.com/kballard/go-shellquote"

func splitCommandLine(commandLine string) ([]string, error) {
	return shellquote.Split(commandLine)
}
