package verserrors

import (
	"errors"
	"strings"
	"testing"
)

func TestVersaError_Error(t *testing.T) {
	err := New(CodeBuildFailed, "build failed", "check logs", nil)
	expected := "[BUILD_FAILED] build failed"
	if err.Error() != expected {
		t.Errorf("expected %s, got %s", expected, err.Error())
	}

	wrapped := errors.New("something went wrong")
	errWithWrap := New(CodeBuildFailed, "build failed", "check logs", wrapped)
	expectedWithWrap := "[BUILD_FAILED] build failed: something went wrong"
	if errWithWrap.Error() != expectedWithWrap {
		t.Errorf("expected %s, got %s", expectedWithWrap, errWithWrap.Error())
	}
}

func TestFormatError(t *testing.T) {
	// Test normal error
	err := errors.New("plain error")
	formatted := FormatError(err)
	if !strings.Contains(formatted, "plain error") {
		t.Errorf("expected formatted to contain 'plain error', got %s", formatted)
	}

	// Test VersaError
	vErr := New(CodeConfigInvalid, "invalid config", "fix it", errors.New("yaml error"))
	formattedVErr := FormatError(vErr)
	if !strings.Contains(formattedVErr, "invalid config") {
		t.Error("expected formatted to contain 'invalid config'")
	}
	if !strings.Contains(formattedVErr, "yaml error") {
		t.Error("expected formatted to contain 'yaml error'")
	}
	if !strings.Contains(formattedVErr, "fix it") {
		t.Error("expected formatted to contain 'fix it'")
	}
}

func TestWrap(t *testing.T) {
	tests := []struct {
		name     string
		input    error
		wantCode ErrorCode
	}{
		{
			name:     "Nil error",
			input:    nil,
			wantCode: "",
		},
		{
			name:     "SSH Auth Failed",
			input:    errors.New("ssh: handshake failed"),
			wantCode: CodeSSHAuthFailed,
		},
		{
			name:     "SSH Timeout",
			input:    errors.New("dial tcp: i/o timeout"),
			wantCode: CodeSSHConnectFailed,
		},
		{
			name:     "SSH Refused",
			input:    errors.New("connection refused"),
			wantCode: CodeSSHConnectFailed,
		},
		{
			name:     "Git Dirty",
			input:    errors.New("uncommitted changes found"),
			wantCode: CodeGitDirty,
		},
		{
			name:     "State Missing",
			input:    errors.New("deploy.lock not found"),
			wantCode: CodeStateMissing,
		},
		{
			name:     "Config Invalid",
			input:    errors.New("config validation failed"),
			wantCode: CodeConfigInvalid,
		},
		{
			name:     "Unknown error",
			input:    errors.New("just a random error"),
			wantCode: "", // Wrap returns original if not matched
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Wrap(tt.input)
			if tt.input == nil {
				if got != nil {
					t.Errorf("Wrap(nil) = %v, want nil", got)
				}
				return
			}

			vErr, ok := got.(*VersaError)
			if tt.wantCode == "" {
				if ok {
					t.Error("expected regular error, got VersaError")
				}
				return
			}

			if !ok {
				t.Fatalf("expected VersaError, got %T", got)
			}
			if vErr.Code != tt.wantCode {
				t.Errorf("expected code %s, got %s", tt.wantCode, vErr.Code)
			}
		})
	}
}
