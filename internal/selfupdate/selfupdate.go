package selfupdate

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"github.com/user/versaDeploy/internal/logger"
	"github.com/user/versaDeploy/internal/version"
)

const (
	githubOwner = "jjara" // Standardizing based on user env
	githubRepo  = "versaDeploy"
)

// Release represents a GitHub release
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

// Asset represents a GitHub release asset
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Updater handles the self-update process
type Updater struct {
	log *logger.Logger
}

// NewUpdater creates a new updater
func NewUpdater(log *logger.Logger) *Updater {
	return &Updater{log: log}
}

// Update checks for updates and performs the update if available
func (u *Updater) Update() error {
	u.log.Info("Checking for updates...")

	latest, err := u.getLatestRelease()
	if err != nil {
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	current := version.Version
	if latest.TagName == "v"+current || latest.TagName == current {
		u.log.Info("You are already on the latest version (%s)", current)
		return nil
	}

	u.log.Info("New version available: %s (Current: %s)", latest.TagName, current)

	// Find the matching asset for current OS/Arch
	targetAsset := ""
	expectedName := fmt.Sprintf("versa_%s_%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		expectedName += ".exe"
	}

	for _, asset := range latest.Assets {
		if asset.Name == expectedName {
			targetAsset = asset.BrowserDownloadURL
			break
		}
	}

	if targetAsset == "" {
		return fmt.Errorf("no binary found for %s/%s in the latest release", runtime.GOOS, runtime.GOARCH)
	}

	u.log.Info("Downloading update from %s...", targetAsset)

	if err := u.performUpdate(targetAsset); err != nil {
		return err
	}

	u.log.Info("Update successful! versaDeploy has been updated to %s.", latest.TagName)
	u.log.Info("Restarting application...")

	return u.restart()
}

func (u *Updater) getLatestRelease() (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", githubOwner, githubRepo)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github API returned status %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

func (u *Updater) performUpdate(url string) error {
	// Download the new binary to a temporary file
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	tmpFile, err := os.CreateTemp("", "versa-update-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()

	// Replace the current binary
	currentPath, err := os.Executable()
	if err != nil {
		return err
	}

	// On Windows, we must rename the current file before replacing it
	oldPath := currentPath + ".old"
	_ = os.Remove(oldPath) // Remove old backup if exists

	if err := os.Rename(currentPath, oldPath); err != nil {
		return fmt.Errorf("failed to move current binary: %w", err)
	}

	if err := os.Rename(tmpPath, currentPath); err != nil {
		// Try to rollback if possible
		_ = os.Rename(oldPath, currentPath)
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	// Set execution bits (for Linux/Mac)
	if runtime.GOOS != "windows" {
		_ = os.Chmod(currentPath, 0775)
	}

	// On Windows, we can't delete the .old file while we are running,
	// but it's okay, it will be cleaned up eventually or by next update.

	return nil
}

func (u *Updater) restart() error {
	cmdPath, err := os.Executable()
	if err != nil {
		return err
	}

	cmd := exec.Command(cmdPath, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Start(); err != nil {
		return err
	}

	os.Exit(0)
	return nil
}
