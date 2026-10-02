//go:build !windows

package lang

import "os/exec"

// shellCommand runs command through sh.
func shellCommand(command string) *exec.Cmd {
	return exec.Command("sh", "-c", command)
}
