package selfupdate

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/user/versaDeploy/internal/logger"
	"github.com/user/versaDeploy/internal/version"
)

const (
	githubOwner = "kriollo" // Corrected owner based on remote config
	githubRepo  = "versaDeploy"
)

// githubAPIBase is overridden in tests to point at an httptest.Server instead
// of the real GitHub API.
var githubAPIBase = "https://api.github.com"

// httpClient bounds every request (a stalled connection must not hang the update)
// and refuses redirects off GitHub or to plain http.
var httpClient = &http.Client{
	Timeout: 5 * time.Minute,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		return checkURL(req.URL.String())
	},
}

// Size caps: release metadata, binary, checksum file.
const (
	maxMetadata = 1 << 20
	maxBinary   = 200 << 20
	maxChecksum = 4 << 10
)

// checkURL accepts only https GitHub hosts (or the API base itself, which tests
// point at a local server).
func checkURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if base, err := url.Parse(githubAPIBase); err == nil && u.Scheme == base.Scheme && u.Host == base.Host {
		return nil
	}
	h := u.Hostname()
	if u.Scheme != "https" || (h != "github.com" && h != "api.github.com" && !strings.HasSuffix(h, ".githubusercontent.com")) {
		return fmt.Errorf("refusing download from %s: only https GitHub URLs are allowed", raw)
	}
	return nil
}

// get fetches raw after checking it is a GitHub URL.
func get(raw string) (*http.Response, error) {
	if err := checkURL(raw); err != nil {
		return nil, err
	}
	return httpClient.Get(raw)
}

// semver parses vX.Y.Z (any -suffix ignored); ok is false for dev builds.
func semver(v string) (out [3]int, ok bool) {
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return out, false
		}
		out[i] = n
	}
	return out, true
}

// isOlder reports whether tag is an older release than current.
func isOlder(tag, current string) bool {
	a, okA := semver(tag)
	b, okB := semver(current)
	if !okA || !okB {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

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
	log               *logger.Logger
	restartBinaryPath string
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

	if isOlder(latest.TagName, current) {
		return fmt.Errorf("latest release %s is older than the installed %s: refusing to downgrade", latest.TagName, current)
	}

	u.log.Info("New version available: %s (Current: %s)", latest.TagName, current)

	// Find the matching asset for current OS/Arch
	targetAsset := ""
	checksumAsset := ""
	expectedName := fmt.Sprintf("versa_%s_%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		expectedName += ".exe"
	}

	for _, asset := range latest.Assets {
		if asset.Name == expectedName {
			targetAsset = asset.BrowserDownloadURL
		}
		if asset.Name == expectedName+".sha256" {
			checksumAsset = asset.BrowserDownloadURL
		}
	}

	if targetAsset == "" {
		return fmt.Errorf("no binary found for %s/%s in the latest release", runtime.GOOS, runtime.GOARCH)
	}
	// every release publishes <binary>.sha256: without it the download can't be verified
	if checksumAsset == "" {
		return fmt.Errorf("release %s has no %s.sha256: refusing an unverified binary", latest.TagName, expectedName)
	}

	u.log.Info("Downloading update from %s...", targetAsset)

	if err := u.performUpdate(targetAsset, checksumAsset, expectedName); err != nil {
		return err
	}

	u.log.Info("Update successful! versaDeploy has been updated to %s.", latest.TagName)
	u.log.Info("Restarting application...")

	return u.restart()
}

func (u *Updater) getLatestRelease() (*Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", githubAPIBase, githubOwner, githubRepo)

	resp, err := get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github API returned status %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxMetadata)).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

func (u *Updater) performUpdate(url, checksumURL, assetName string) error {
	// Resolve current binary path first so we can place the temp file on the
	// same filesystem, avoiding cross-device rename errors.
	currentPath, err := os.Executable()
	if err != nil {
		return err
	}
	u.restartBinaryPath = currentPath
	currentDir := filepath.Dir(currentPath)

	// Download the new binary to a temp file in the same directory as the binary.
	tmpFile, err := os.CreateTemp(currentDir, "versa-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	resp, err := get(url)
	if err != nil {
		tmpFile.Close()
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		return fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	hasher := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmpFile, hasher), io.LimitReader(resp.Body, maxBinary+1))
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to download update: %w", err)
	}
	if n > maxBinary {
		tmpFile.Close()
		return fmt.Errorf("update is larger than %d MB: refusing it", maxBinary>>20)
	}
	tmpFile.Close()

	expected, err := fetchChecksum(checksumURL, assetName)
	if err != nil {
		return fmt.Errorf("failed to fetch checksum: %w", err)
	}
	got := fmt.Sprintf("%x", hasher.Sum(nil))
	if !strings.EqualFold(got, expected) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s (downloaded binary may be corrupted or tampered with)", expected, got)
	}
	u.log.Info("Checksum verified.")

	// Set execution bits before replacing (Linux/Mac)
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpPath, 0755); err != nil {
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

// fetchChecksum downloads a sha256sum-format file ("<hex digest>  <filename>") and
// returns the digest listed for name, which must be 64 hex characters.
func fetchChecksum(url, name string) (string, error) {
	resp, err := get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checksum download returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxChecksum))
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		// sha256sum prints "dist/<name>" in CI: compare the base name
		if len(f) == 2 && path.Base(strings.TrimPrefix(f[1], "*")) == name {
			if !sha256Re.MatchString(f[0]) {
				return "", fmt.Errorf("checksum for %s is not a sha256 digest", name)
			}
			return f[0], nil
		}
	}
	return "", fmt.Errorf("checksum file lists no digest for %s", name)
}

var sha256Re = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

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
	var candidates []string

	if u.restartBinaryPath != "" {
		candidates = append(candidates, u.restartBinaryPath)
	}

	if exePath, err := os.Executable(); err == nil {
		candidates = append(candidates, exePath)
	}

	if len(os.Args) > 0 {
		if filepath.IsAbs(os.Args[0]) {
			candidates = append(candidates, os.Args[0])
		} else if lookPath, err := exec.LookPath(os.Args[0]); err == nil {
			candidates = append(candidates, lookPath)
		}
	}

	cmdPath := ""
	for _, c := range candidates {
		candidate := c
		if strings.HasSuffix(candidate, ".old") {
			candidate = strings.TrimSuffix(candidate, ".old")
		}
		if _, err := os.Stat(candidate); err == nil {
			cmdPath = candidate
			break
		}
	}

	if cmdPath == "" {
		return fmt.Errorf("could not determine restart binary path")
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
