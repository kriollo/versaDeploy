package main

import (
	"os"
	"testing"
)

func TestRootCommand(t *testing.T) {
	// We just want to check if it's correctly defined
	if rootCmd.Use != "versa" {
		t.Errorf("expected versa, got %s", rootCmd.Use)
	}
}

func TestInitCommand(t *testing.T) {
	// versa init creates a deploy.yml
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	err := initCmd.RunE(initCmd, []string{})
	if err != nil {
		t.Fatalf("versa init failed: %v", err)
	}

	if _, err := os.Stat("deploy.yml"); os.IsNotExist(err) {
		t.Error("deploy.yml was not created by versa init")
	}

	// Running again should fail
	err = initCmd.RunE(initCmd, []string{})
	if err == nil {
		t.Error("versa init should fail if deploy.yml already exists")
	}
}
