package runner

import "golang.org/x/sys/windows"

func splitCommandLine(commandLine string) ([]string, error) {
	return windows.DecomposeCommandLine(commandLine)
}
