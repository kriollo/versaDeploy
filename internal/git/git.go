package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Clone creates a clean clone of the repository in a temporary directory
func Clone(repoPath, ref string) (string, error) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "versadeploy-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Get absolute path to repository
	absRepoPath, err := filepath.Abs(repoPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("failed to get absolute repo path: %w", err)
	}

	// Clone the repository
	if _, err := executeGitInternal(repoPath, "clone", absRepoPath, tmpDir); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("git clone failed: %w", err)
	}

	// Checkout specific ref if provided
	if ref != "" {
		if _, err := executeGitInternal(tmpDir, "checkout", ref); err != nil {
			os.RemoveAll(tmpDir)
			return "", fmt.Errorf("git checkout %s failed: %w", ref, err)
		}
	}

	return tmpDir, nil
}

// GetCurrentCommit returns the current commit hash
func GetCurrentCommit(repoPath string) (string, error) {
	output, err := executeGitInternal(repoPath, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get commit hash: %w", err)
	}
	return strings.TrimSpace(output), nil
}

// IsClean checks if the working directory has uncommitted changes
func IsClean(repoPath string) (bool, error) {
	output, err := executeGitInternal(repoPath, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}
	return len(strings.TrimSpace(output)) == 0, nil
}

// ValidateRepository checks if the path is a valid git repository
func ValidateRepository(repoPath string) error {
	_, err := executeGitInternal(repoPath, "rev-parse", "--git-dir")
	return err
}

// executeGitInternal runs a git command using the system shell or absolute path
func executeGitInternal(repoPath string, args ...string) (string, error) {
	gitPath := resolveGitPath()

	allArgs := append([]string{"-C", repoPath}, args...)

	// Run git directly (exec resolves "git" through PATH on every OS): going through
	// cmd /c would split repo paths that contain spaces.
	output, err := exec.Command(gitPath, allArgs...).CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("git command failed: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

// resolveGitPath attempts to find the git executable, especially on Windows
func resolveGitPath() string {
	gitPath := "git"

	if runtime.GOOS == "windows" {
		// Try to find git in PATH first
		if p, err := exec.LookPath("git"); err == nil {
			return p
		}

		// Fallback to common locations on Windows
		commonPaths := []string{
			"C:\\Program Files\\Git\\cmd\\git.exe",
			"C:\\Program Files\\Git\\bin\\git.exe",
			"C:\\Program Files (x86)\\Git\\cmd\\git.exe",
			filepath.Join(os.Getenv("LocalAppData"), "Programs", "Git", "cmd", "git.exe"),
		}
		for _, p := range commonPaths {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}

	return gitPath
}
