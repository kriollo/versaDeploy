package tui

import (
	"errors"

	verserrors "github.com/user/versaDeploy/internal/errors"
)

// errDisplay renders an error for the TUI's single-line status/log widgets.
// Structured *verserrors.VersaError errors get their Suggestion appended so
// the TUI carries the same actionable hints as the CLI (verserrors.FormatError),
// without the ANSI multi-line block that format uses for full terminal output.
func errDisplay(err error) string {
	if err == nil {
		return ""
	}
	var vErr *verserrors.VersaError
	if errors.As(err, &vErr) {
		if vErr.Suggestion != "" {
			return vErr.Message + " — " + vErr.Suggestion
		}
		return vErr.Message
	}
	return err.Error()
}
