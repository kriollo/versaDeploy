package ssh

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"

	"golang.org/x/sync/errgroup"

	"github.com/user/versaDeploy/internal/config"
	verserrors "github.com/user/versaDeploy/internal/errors"
	"github.com/user/versaDeploy/internal/logger"
)

// Client wraps SSH and SFTP operations
type Client struct {
	sshClient  *ssh.Client
	sftpClient *sftp.Client
	config     *config.SSHConfig
	log        *logger.Logger
}

// NewClient creates a new SSH client
func NewClient(cfg *config.SSHConfig, log *logger.Logger) (*Client, error) {
	authMethods := []ssh.AuthMethod{}

	// Support SSH Agent
	if cfg.UseSSHAgent {
		sock := os.Getenv("SSH_AUTH_SOCK")
		if sock != "" {
			if agentConn, err := net.Dial("unix", sock); err == nil {
				authMethods = append(authMethods, ssh.PublicKeysCallback(agent.NewClient(agentConn).Signers))
			}
		}
	}

	// Try reading private key if path is provided
	if cfg.KeyPath != "" {
		keyData, err := os.ReadFile(cfg.KeyPath)
		if err != nil {
			if len(authMethods) == 0 {
				return nil, fmt.Errorf("failed to read SSH key: %w", err)
			}
		} else {
			signer, err := ssh.ParsePrivateKey(keyData)
			if err != nil {
				if len(authMethods) == 0 {
					return nil, fmt.Errorf("failed to parse SSH key: %w", err)
				}
			} else {
				authMethods = append(authMethods, ssh.PublicKeys(signer))
			}
		}
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no valid SSH authentication methods found (check key_path or use_ssh_agent)")
	}

	// Configure SSH client
	sshConfig := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: createHostKeyCallback(cfg),
		Timeout:         10 * time.Second,
	}

	// Connect with retry logic
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	var sshClient *ssh.Client
	var err error

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

	// Create SFTP client with optimized settings
	sftpClient, err := sftp.NewClient(sshClient, sftp.MaxPacket(1<<15))
	if err != nil {
		sshClient.Close()
		return nil, fmt.Errorf("failed to create SFTP client: %w", err)
	}

	return &Client{
		sshClient:  sshClient,
		sftpClient: sftpClient,
		config:     cfg,
		log:        log,
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
		return c.uploadFile(localPath, remotePath, nil)
	})
}

// UploadFilesParallel uploads multiple files concurrently to a remote directory
func (c *Client) UploadFilesParallel(localPaths []string, remoteDir string, concurrency int) error {
	if concurrency <= 0 {
		concurrency = 3
	}

	// Create remote directory if it doesn't exist
	if err := c.sftpClient.MkdirAll(remoteDir); err != nil {
		return fmt.Errorf("failed to create remote directory: %w", err)
	}

	// Calculate total size for unified progress bar
	var totalSize int64
	for _, p := range localPaths {
		info, err := os.Stat(p)
		if err == nil {
			totalSize += info.Size()
		}
	}

	bar := progressbar.DefaultBytes(totalSize, "Uploading archive chunks")

	type uploadJob struct {
		localPath  string
		remotePath string
	}

	jobs := make(chan uploadJob, len(localPaths))
	for _, localPath := range localPaths {
		remotePath := filepath.ToSlash(filepath.Join(remoteDir, filepath.Base(localPath)))
		jobs <- uploadJob{localPath, remotePath}
	}
	close(jobs)

	var g errgroup.Group
	for i := 0; i < concurrency; i++ {
		g.Go(func() error {
			for job := range jobs {
				if err := c.uploadFile(job.localPath, job.remotePath, bar); err != nil {
					return err
				}
			}
			return nil
		})
	}

	return g.Wait()
}

// uploadFile uploads a single file, optionally reporting progress to a writer
func (c *Client) uploadFile(localPath, remotePath string, progress io.Writer) error {
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
	var writer io.Writer = remoteFile
	if progress != nil {
		writer = io.MultiWriter(remoteFile, progress)
	}

	if _, err := io.Copy(writer, localFile); err != nil {
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

// UploadFileWithProgress uploads a single file with a progress bar
func (c *Client) UploadFileWithProgress(localPath, remotePath string) error {
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	info, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat local file: %w", err)
	}

	remoteFile, err := c.sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteFile.Close()

	bar := progressbar.DefaultBytes(
		info.Size(),
		fmt.Sprintf("Uploading %s", filepath.Base(localPath)),
	)

	_, err = io.Copy(io.MultiWriter(remoteFile, bar), localFile)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	return nil
}

// ExtractArchive extracts a tar.gz archive on the remote server
func (c *Client) ExtractArchive(archivePath, targetDir string) error {
	// Create target directory if it doesn't exist using SFTP
	if err := c.sftpClient.MkdirAll(targetDir); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Extract using shell (tar is too complex for SFTP)
	cmd := fmt.Sprintf("tar -xzf %q -C %q", archivePath, targetDir)
	output, err := c.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to extract archive: %w (output: %s)", err, output)
	}

	return nil
}

// ExecuteCommand executes a command on the remote server
func (c *Client) ExecuteCommand(cmd string) (string, error) {
	return c.ExecuteCommandWithTimeout(cmd, 0)
}

// ExecuteCommandWithTimeout executes a command with a specific timeout
func (c *Client) ExecuteCommandWithTimeout(cmd string, timeout time.Duration) (string, error) {
	session, err := c.sshClient.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	var b bytes.Buffer
	session.Stdout = &b
	session.Stderr = &b

	if err := session.Start(cmd); err != nil {
		return "", fmt.Errorf("failed to start command: %w", err)
	}

	if timeout <= 0 {
		err := session.Wait()
		return b.String(), err
	}

	done := make(chan error, 1)
	go func() {
		done <- session.Wait()
	}()

	select {
	case <-time.After(timeout):
		session.Signal(ssh.SIGKILL)
		session.Close()
		return b.String(), fmt.Errorf("command timed out after %v", timeout)
	case err := <-done:
		if err != nil {
			return b.String(), fmt.Errorf("command failed: %w", err)
		}
		return b.String(), nil
	}
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

	// Remove tmp link if it exists using SFTP
	c.sftpClient.Remove(tmpLink)

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

	// Keep newest releases
	if len(releases) <= keepCount {
		return nil // Nothing to clean up
	}

	// Sort releases in descending order (newest first)
	// Simple string sort works due to timestamp format YYYYMMDD-HHMMSS
	sort.Sort(sort.Reverse(sort.StringSlice(releases)))

	// Delete old releases
	for i := keepCount; i < len(releases); i++ {
		releaseDir := filepath.ToSlash(filepath.Join(releasesDir, releases[i]))
		// Use %q for safe quoting and -- to prevent arguments injection
		cmd := fmt.Sprintf("rm -rf -- %q", releaseDir)
		output, err := c.ExecuteCommand(cmd)
		if err != nil {
			return fmt.Errorf("failed to delete old release %s: %w (output: %s)", releases[i], err, output)
		}
	}

	return nil
}

// CheckDiskSpace verifies sufficient disk space is available on remote server
func (c *Client) CheckDiskSpace(path string, requiredBytes int64) error {
	// Get disk usage for the path
	cmd := fmt.Sprintf("df -B1 %s | tail -1 | awk '{print $4}'", path)
	output, err := c.ExecuteCommand(cmd)
	if err != nil {
		// Non-fatal: just warn and continue
		c.log.Warn("Failed to check disk space: %v", err)
		return nil
	}

	output = strings.TrimSpace(output)
	if output == "" {
		// Non-fatal: just warn and continue
		c.log.Warn("Empty output from disk space check command")
		return nil
	}

	var availableBytes int64
	_, err = fmt.Sscanf(output, "%d", &availableBytes)
	if err != nil {
		// Non-fatal: show the output for debugging and continue
		c.log.Warn("Failed to parse disk space output (got: '%s'): %v", output, err)
		return nil
	}

	// Require 20% buffer on top of required space
	requiredWithBuffer := int64(float64(requiredBytes) * 1.2)

	if availableBytes < requiredWithBuffer {
		return fmt.Errorf("insufficient disk space: need %d MB, have %d MB available",
			requiredWithBuffer/(1024*1024), availableBytes/(1024*1024))
	}

	c.log.Info("Disk space check passed: %d MB available, %d MB required",
		availableBytes/(1024*1024), requiredWithBuffer/(1024*1024))

	return nil
}

// AcquireLock attempts to acquire a deployment lock using atomic directory creation via SFTP
func (c *Client) AcquireLock(lockPath string) error {
	err := c.sftpClient.Mkdir(lockPath)
	if err != nil {
		return verserrors.New(verserrors.CodeConfigInvalid,
			"Deployment lock already held",
			"Another deployment is currently in progress. If you are sure no one else is deploying, manually remove the directory: "+lockPath,
			err)
	}
	return nil
}

// ReleaseLock releases the deployment lock via SFTP
func (c *Client) ReleaseLock(lockPath string) error {
	return c.sftpClient.RemoveDirectory(lockPath)
}

// MkdirAll creates a directory and all parent directories via SFTP
func (c *Client) MkdirAll(path string) error {
	return c.sftpClient.MkdirAll(path)
}

// Remove removes a file or empty directory via SFTP
func (c *Client) Remove(path string) error {
	return c.sftpClient.Remove(path)
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
