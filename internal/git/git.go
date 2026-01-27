package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	cmd := exec.Command("git", "clone", absRepoPath, tmpDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("git clone failed: %s", string(output))
	}

	// Checkout specific ref if provided
	if ref != "" {
		cmd = exec.Command("git", "-C", tmpDir, "checkout", ref)
		output, err = cmd.CombinedOutput()
		if err != nil {
			os.RemoveAll(tmpDir)
			return "", fmt.Errorf("git checkout %s failed: %s", ref, string(output))
		}
	}

	return tmpDir, nil
}

// GetCurrentCommit returns the current commit hash
func GetCurrentCommit(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get commit hash: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// IsClean checks if the working directory has uncommitted changes
func IsClean(repoPath string) (bool, error) {
	cmd := exec.Command("git", "-C", repoPath, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}
	return len(strings.TrimSpace(string(output))) == 0, nil
}

// ValidateRepository checks if the path is a valid git repository
func ValidateRepository(repoPath string) error {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--git-dir")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("not a valid git repository: %s", repoPath)
	}
	return nil
}
