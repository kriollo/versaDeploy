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
	if err := initCmd.RunE(initCmd, []string{}); err == nil {
		t.Error("versa init should fail if deploy.yml already exists")
	}
}

func TestDeployCommand_ConfigNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	err := deployCmd.RunE(deployCmd, []string{"prod"})
	if err == nil {
		t.Error("expected failure when deploy.yml is missing")
	}
}

func TestDeployCommand_WrongArgs(t *testing.T) {
	rootCmd.SetArgs([]string{"deploy"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected failure for missing environment argument")
	}
}

func TestRollbackCommand(t *testing.T) {
	rootCmd.SetArgs([]string{"rollback"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected failure for missing environment argument")
	}
}

func TestStatusCommand(t *testing.T) {
	rootCmd.SetArgs([]string{"status"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected failure for missing environment argument")
	}
}

func TestSSHTestCommand(t *testing.T) {
	rootCmd.SetArgs([]string{"ssh-test"})
	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected failure for missing environment argument")
	}
}
