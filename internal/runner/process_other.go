//go:build !unix && !windows

package runner

import (
	"errors"
	"os/exec"
)

func runProcess(cmd *exec.Cmd) error {
	return errors.New("process tree management requires Windows or Unix")
}
