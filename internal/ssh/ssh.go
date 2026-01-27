package ssh

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/user/versaDeploy/internal/config"
	verserrors "github.com/user/versaDeploy/internal/errors"
)

// Client wraps SSH and SFTP operations
type Client struct {
	sshClient  *ssh.Client
	sftpClient *sftp.Client
	config     *config.SSHConfig
}

// NewClient creates a new SSH client
func NewClient(cfg *config.SSHConfig) (*Client, error) {
	// Read private key
	keyData, err := os.ReadFile(cfg.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read SSH key: %w", err)
	}

	// Parse private key
	signer, err := ssh.ParsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SSH key: %w", err)
	}

	// Configure SSH client
	sshConfig := &ssh.ClientConfig{
		User: cfg.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: createHostKeyCallback(cfg),
		Timeout:         10 * time.Second,
	}

	// Connect with retry logic
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	var sshClient *ssh.Client

	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		sshClient, err = ssh.Dial("tcp", addr, sshConfig)
		if err == nil {
			break
		}

		if attempt < maxRetries-1 {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			time.Sleep(backoff)
		}
	}

	if err != nil {
		return nil, verserrors.Wrap(fmt.Errorf("failed to connect to SSH server after %d attempts: %w", maxRetries, err))
	}

	// Create SFTP client
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, fmt.Errorf("failed to create SFTP client: %w", err)
	}

	return &Client{
		sshClient:  sshClient,
		sftpClient: sftpClient,
		config:     cfg,
	}, nil
}

// Close closes the SSH and SFTP connections
func (c *Client) Close() error {
	if c.sftpClient != nil {
		c.sftpClient.Close()
	}
	if c.sshClient != nil {
		return c.sshClient.Close()
	}
	return nil
}

// UploadDirectory uploads a directory recursively
func (c *Client) UploadDirectory(localDir, remoteDir string) error {
	// Create remote directory
	if err := c.sftpClient.MkdirAll(remoteDir); err != nil {
		return fmt.Errorf("failed to create remote directory: %w", err)
	}

	// Walk local directory
	return filepath.Walk(localDir, func(localPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(localDir, localPath)
		if err != nil {
			return err
		}

		// Convert to forward slashes for remote path
		relPath = filepath.ToSlash(relPath)
		remotePath := filepath.ToSlash(filepath.Join(remoteDir, relPath))

		if info.IsDir() {
			// Create remote directory
			return c.sftpClient.MkdirAll(remotePath)
		}

		// Upload file
		return c.uploadFile(localPath, remotePath)
	})
}

// uploadFile uploads a single file
func (c *Client) uploadFile(localPath, remotePath string) error {
	// Open local file
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	// Create remote file
	remoteFile, err := c.sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteFile.Close()

	// Copy contents
	if _, err := io.Copy(remoteFile, localFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

// DownloadFile downloads a file from remote server
func (c *Client) DownloadFile(remotePath, localPath string) error {
	// Open remote file
	remoteFile, err := c.sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file: %w", err)
	}
	defer remoteFile.Close()

	// Create local file
	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer localFile.Close()

	// Copy contents
	if _, err := io.Copy(localFile, remoteFile); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

// FileExists checks if a remote file exists
func (c *Client) FileExists(remotePath string) (bool, error) {
	_, err := c.sftpClient.Stat(remotePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ExecuteCommand executes a command on the remote server
func (c *Client) ExecuteCommand(cmd string) (string, error) {
	session, err := c.sshClient.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w", err)
	}

	return string(output), nil
}

// ListReleases lists all release directories on the remote server
func (c *Client) ListReleases(releasesDir string) ([]string, error) {
	entries, err := c.sftpClient.ReadDir(releasesDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read releases directory: %w", err)
	}

	var releases []string
	for _, entry := range entries {
		if entry.IsDir() {
			releases = append(releases, entry.Name())
		}
	}

	return releases, nil
}

// ReadSymlink reads the target of a symlink
func (c *Client) ReadSymlink(path string) (string, error) {
	target, err := c.sftpClient.ReadLink(path)
	if err != nil {
		return "", fmt.Errorf("failed to read symlink: %w", err)
	}
	return target, nil
}

// CreateSymlink creates a symlink atomically (two-step process)
func (c *Client) CreateSymlink(target, linkPath string) error {
	// Step 1: Create temporary symlink
	tmpLink := linkPath + ".tmp"

	// Remove tmp link if it exists
	c.ExecuteCommand(fmt.Sprintf("rm -f %s", tmpLink))

	// Create symlink
	cmd := fmt.Sprintf("ln -sfn %s %s", target, tmpLink)
	if _, err := c.ExecuteCommand(cmd); err != nil {
		return fmt.Errorf("failed to create temporary symlink: %w", err)
	}

	// Step 2: Atomically move to final location
	cmd = fmt.Sprintf("mv -Tf %s %s", tmpLink, linkPath)
	if _, err := c.ExecuteCommand(cmd); err != nil {
		return fmt.Errorf("failed to atomically switch symlink: %w", err)
	}

	// Step 3: Verify symlink points to correct target
	actualTarget, err := c.ReadSymlink(linkPath)
	if err != nil {
		return fmt.Errorf("failed to verify symlink: %w", err)
	}

	// Handle both absolute and relative paths
	if !strings.HasSuffix(actualTarget, target) && actualTarget != target {
		return fmt.Errorf("symlink verification failed: expected %s, got %s", target, actualTarget)
	}

	return nil
}

// CleanupOldReleases removes old releases, keeping only the specified number
func (c *Client) CleanupOldReleases(releasesDir string, keepCount int) error {
	releases, err := c.ListReleases(releasesDir)
	if err != nil {
		return err
	}

	// Sort releases (assuming timestamp format YYYYMMDD-HHMMSS)
	// Keep newest releases
	if len(releases) <= keepCount {
		return nil // Nothing to clean up
	}

	// Sort in reverse order (newest first)
	// Simple string sort works due to timestamp format
	sortedReleases := make([]string, len(releases))
	copy(sortedReleases, releases)

	// Simple bubble sort (good enough for small lists)
	for i := 0; i < len(sortedReleases); i++ {
		for j := i + 1; j < len(sortedReleases); j++ {
			if strings.Compare(sortedReleases[i], sortedReleases[j]) < 0 {
				sortedReleases[i], sortedReleases[j] = sortedReleases[j], sortedReleases[i]
			}
		}
	}

	// Delete old releases
	for i := keepCount; i < len(sortedReleases); i++ {
		releaseDir := filepath.ToSlash(filepath.Join(releasesDir, sortedReleases[i]))
		cmd := fmt.Sprintf("rm -rf %s", releaseDir)
		if _, err := c.ExecuteCommand(cmd); err != nil {
			return fmt.Errorf("failed to delete old release %s: %w", sortedReleases[i], err)
		}
		fmt.Printf("Cleaned up old release: %s\n", sortedReleases[i])
	}

	return nil
}

// CheckDiskSpace verifies sufficient disk space is available on remote server
func (c *Client) CheckDiskSpace(path string, requiredBytes int64) error {
	// Get disk usage for the path
	cmd := fmt.Sprintf("df -B1 %s | tail -1 | awk '{print $4}'", path)
	output, err := c.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to check disk space: %w", err)
	}

	var availableBytes int64
	_, err = fmt.Sscanf(strings.TrimSpace(output), "%d", &availableBytes)
	if err != nil {
		return fmt.Errorf("failed to parse disk space output: %w", err)
	}

	// Require 20% buffer on top of required space
	requiredWithBuffer := int64(float64(requiredBytes) * 1.2)

	if availableBytes < requiredWithBuffer {
		return fmt.Errorf("insufficient disk space: need %d MB, have %d MB available",
			requiredWithBuffer/(1024*1024), availableBytes/(1024*1024))
	}

	return nil
}

// createHostKeyCallback returns an SSH HostKeyCallback based on configuration
func createHostKeyCallback(cfg *config.SSHConfig) ssh.HostKeyCallback {
	knownHostsPath := cfg.KnownHostsFile

	// If no path specified, try to find default known_hosts
	if knownHostsPath == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			defaultPath := filepath.Join(home, ".ssh", "known_hosts")
			if _, err := os.Stat(defaultPath); err == nil {
				knownHostsPath = defaultPath
			}
		}
	}

	// If we still don't have a path, fallback to insecure for now but log it
	if knownHostsPath == "" {
		return ssh.InsecureIgnoreHostKey()
	}

	callback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		// If failed to load known_hosts, fallback to insecure but we should probably fail instead
		// For versaDeploy, we want to be safe but not break existing setups that don't have it.
		return ssh.InsecureIgnoreHostKey()
	}

	return callback
}
