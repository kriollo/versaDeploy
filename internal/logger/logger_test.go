package logger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLogger_NewLogger(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.log")

	l, err := NewLogger(tmpFile, true, true)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	defer l.Close()

	if l.file == nil {
		t.Error("expected logger file to be initialized")
	}

	l.Info("test message")

	// Check if file contains the message
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatal(err)
	}

	if entry.Message != "test message" {
		t.Errorf("expected message 'test message', got %s", entry.Message)
	}
	if entry.Level != LevelInfo {
		t.Errorf("expected level INFO, got %s", entry.Level)
	}
}

func TestLogger_Levels(t *testing.T) {
	// Simple test to ensure level methods don't panic
	l, _ := NewLogger("", false, false)
	l.Debug("debug")
	l.Info("info")
	l.Warning("warning")
	l.Warn("warn")
	l.Error("error")
	l.Success("success")
}

func TestLogger_Close(t *testing.T) {
	l, _ := NewLogger("", false, false)
	if err := l.Close(); err != nil {
		t.Errorf("Close() on nil file error = %v", err)
	}

	tmpFile := filepath.Join(t.TempDir(), "close.log")
	l2, _ := NewLogger(tmpFile, false, false)
	if err := l2.Close(); err != nil {
		t.Errorf("Close() on valid file error = %v", err)
	}
}
