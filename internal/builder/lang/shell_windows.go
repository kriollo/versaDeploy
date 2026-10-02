package lang

import (
	"os"
	"os/exec"
	"syscall"
)

// shellCommand runs command through cmd.exe. The command line is passed raw: letting
// exec quote it as an argument escapes inner quotes as \", which cmd.exe doesn't
// understand (`go build -o "C:\out"` reached go as `"C:\out\"`). /s /c "..." makes
// cmd strip only the outer quotes and run the rest as typed.
func shellCommand(command string) *exec.Cmd {
	shell := os.Getenv("COMSPEC")
	if shell == "" {
		shell = "cmd.exe"
	}
	cmd := exec.Command(shell)
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: syscall.EscapeArg(shell) + ` /d /s /c "` + command + `"`}
	return cmd
}
