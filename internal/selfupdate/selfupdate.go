package selfupdate

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/user/versaDeploy/internal/logger"
	"github.com/user/versaDeploy/internal/version"
)

const (
	githubOwner = "kriollo" // Corrected owner based on remote config
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
	// Resolve current binary path first so we can place the temp file on the
	// same filesystem, avoiding cross-device rename errors.
	currentPath, err := os.Executable()
	if err != nil {
		return err
	}
	currentDir := filepath.Dir(currentPath)

	// Download the new binary to a temp file in the same directory as the binary.
	tmpFile, err := os.CreateTemp(currentDir, "versa-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	resp, err := http.Get(url)
	if err != nil {
		tmpFile.Close()
		return err
	}
	defer resp.Body.Close()

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to download update: %w", err)
	}
	tmpFile.Close()

	// Set execution bits before replacing (Linux/Mac)
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpPath, 0775); err != nil {
			return fmt.Errorf("failed to set permissions on update: %w", err)
		}
	}

	// Backup current binary then atomically swap in the new one.
	oldPath := currentPath + ".old"
	_ = os.Remove(oldPath)

	if err := os.Rename(currentPath, oldPath); err != nil {
		return fmt.Errorf("failed to move current binary: %w", err)
	}

	if err := os.Rename(tmpPath, currentPath); err != nil {
		// Cross-device rename should not happen now (same dir), but fall back
		// to a manual copy just in case.
		if copyErr := copyFile(tmpPath, currentPath); copyErr != nil {
			_ = os.Rename(oldPath, currentPath) // restore on failure
			return fmt.Errorf("failed to replace binary: %w", err)
		}
	}

	// Remove the old backup (best-effort; Windows may skip this).
	_ = os.Remove(oldPath)

	return nil
}

// copyFile copies src to dst, preserving executable permissions.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
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
