package verserrors

import (
	"fmt"
	"strings"
)

// ErrorCode is a custom error code type
type ErrorCode string

const (
	CodeConfigInvalid    ErrorCode = "CONFIG_INVALID"
	CodeSSHAuthFailed    ErrorCode = "SSH_AUTH_FAILED"
	CodeSSHConnectFailed ErrorCode = "SSH_CONNECT_FAILED"
	CodeBuildFailed      ErrorCode = "BUILD_FAILED"
	CodeGitDirty         ErrorCode = "GIT_DIRTY"
	CodeStateMissing     ErrorCode = "STATE_MISSING"
	CodeUploadFailed     ErrorCode = "UPLOAD_FAILED"
	CodeDeploymentFailed ErrorCode = "DEPLOYMENT_FAILED"
	CodeUnknown          ErrorCode = "UNKNOWN"
)

// VersaError represents a structured error in the system
type VersaError struct {
	Code       ErrorCode
	Message    string
	Suggestion string
	WrappedErr error
}

func (e *VersaError) Error() string {
	if e.WrappedErr != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.WrappedErr)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// New creates a new VersaError
func New(code ErrorCode, msg, suggestion string, err error) *VersaError {
	return &VersaError{
		Code:       code,
		Message:    msg,
		Suggestion: suggestion,
		WrappedErr: err,
	}
}

// FormatError pretty-prints the error with suggestions
func FormatError(err error) string {
	if vErr, ok := err.(*VersaError); ok {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("\n\x1b[31m[ERROR] %s\x1b[0m\n", vErr.Message))
		sb.WriteString(fmt.Sprintf("\x1b[33mCode:\x1b[0m %s\n", vErr.Code))
		if vErr.WrappedErr != nil {
			sb.WriteString(fmt.Sprintf("\x1b[33mDetails:\x1b[0m %v\n", vErr.WrappedErr))
		}
		if vErr.Suggestion != "" {
			sb.WriteString(fmt.Sprintf("\n\x1b[32mSuggestion:\x1b[0m %s\n", vErr.Suggestion))
		}
		return sb.String()
	}
	return fmt.Sprintf("\x1b[31m[ERROR]\x1b[0m %v", err)
}

// Wrap maps common Go errors to VersaErrors
func Wrap(err error) error {
	if err == nil {
		return nil
	}

	errMsg := err.Error()

	// SSH common errors
	if strings.Contains(errMsg, "ssh: handshake failed") || strings.Contains(errMsg, "unable to authenticate") {
		return New(CodeSSHAuthFailed, "SSH Authentication failed", "Check your SSH private key path and ensure it's added to the remote server's authorized_keys.", err)
	}
	if strings.Contains(errMsg, "dial tcp") && strings.Contains(errMsg, "i/o timeout") {
		return New(CodeSSHConnectFailed, "SSH Connection timed out", "Ensure the remote host is reachable and the port (default 22) is open in the firewall.", err)
	}
	if strings.Contains(errMsg, "connection refused") {
		return New(CodeSSHConnectFailed, "SSH Connection refused", "Check if the SSH service is running on the remote host and you're using the correct port.", err)
	}

	// Git common errors
	if strings.Contains(errMsg, "uncommitted changes") {
		return New(CodeGitDirty, "Working directory is not clean", "Commit or stash your changes before deploying to ensure a reproducible build.", err)
	}

	// State errors
	if strings.Contains(errMsg, "deploy.lock not found") {
		return New(CodeStateMissing, "Missing deploy.lock on remote", "This seems to be the first deployment. Run the command with the --initial-deploy flag.", err)
	}

	// Config errors
	if strings.Contains(errMsg, "config validation failed") {
		return New(CodeConfigInvalid, "Invalid configuration", "Revise your deploy.yml file and ensure all required fields are present and correctly formatted.", err)
	}

	return err
}
