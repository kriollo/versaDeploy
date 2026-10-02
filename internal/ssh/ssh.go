package ssh

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
	agentConn  net.Conn
	config     *config.SSHConfig
	log        *logger.Logger
	done       chan struct{} // stops the keepalive goroutine
}

// NewClient creates a new SSH client
func NewClient(cfg *config.SSHConfig, log *logger.Logger) (*Client, error) {
	authMethods := []ssh.AuthMethod{}

	// Support SSH Agent
	var agentConn net.Conn
	if cfg.UseSSHAgent {
		sock := os.Getenv("SSH_AUTH_SOCK")
		if sock != "" {
			if conn, err := net.Dial("unix", sock); err == nil {
				agentConn = conn
				authMethods = append(authMethods, ssh.PublicKeysCallback(agent.NewClient(conn).Signers))
			}
		}
	}

	// Try reading private key if path is provided
	if cfg.KeyPath != "" {
		keyData, err := os.ReadFile(cfg.KeyPath)
		if err != nil {
			if len(authMethods) == 0 {
				return nil, verserrors.New(verserrors.CodeSSHAuthFailed, fmt.Sprintf("Failed to read SSH key: %s", cfg.KeyPath), "Check that ssh.key_path in your config points to a readable private key file.", err)
			}
		} else {
			signer, err := ssh.ParsePrivateKey(keyData)
			if err != nil {
				if len(authMethods) == 0 {
					return nil, verserrors.New(verserrors.CodeSSHAuthFailed, fmt.Sprintf("Failed to parse SSH key: %s", cfg.KeyPath), "Ensure the key is a valid unencrypted private key (OpenSSH/PEM format). Encrypted keys are not supported.", err)
				}
			} else {
				authMethods = append(authMethods, ssh.PublicKeys(signer))
			}
		}
	}

	if len(authMethods) == 0 {
		return nil, verserrors.New(verserrors.CodeSSHAuthFailed, "No valid SSH authentication methods found", "Set ssh.key_path to a valid private key, or enable ssh.use_ssh_agent and ensure SSH_AUTH_SOCK is set.", nil)
	}

	// Configure SSH client
	sshConfig := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: createHostKeyCallback(cfg),
		Timeout:         10 * time.Second,
	}
	if cfg.LegacyAlgorithms {
		sup, ins := ssh.SupportedAlgorithms(), ssh.InsecureAlgorithms()
		sshConfig.KeyExchanges = append(sup.KeyExchanges, ins.KeyExchanges...)
		sshConfig.Ciphers = append(sup.Ciphers, ins.Ciphers...)
		sshConfig.MACs = append(sup.MACs, ins.MACs...)
		sshConfig.HostKeyAlgorithms = append(sup.HostKeys, ins.HostKeys...)
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
		return nil, verserrors.New(verserrors.CodeSSHConnectFailed, "Failed to create SFTP client", "Ensure the SFTP subsystem is enabled on the remote server (check 'Subsystem sftp' in /etc/ssh/sshd_config).", err)
	}

	c := &Client{
		sshClient:  sshClient,
		sftpClient: sftpClient,
		agentConn:  agentConn,
		config:     cfg,
		log:        log,
		done:       make(chan struct{}),
	}
	go c.keepAlive(30 * time.Second)
	return c, nil
}

// keepAlive pings the server so long uploads/hooks survive NAT and firewall idle timeouts.
// Servers that don't know the request reply with a failure, which still counts as alive.
func (c *Client) keepAlive(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-t.C:
			if _, _, err := c.sshClient.SendRequest("keepalive@openssh.com", true, nil); err != nil {
				return
			}
		}
	}
}

// ShellQuote single-quotes s for POSIX sh.
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// shWrap runs cmd under /bin/sh regardless of the remote user's login shell (csh/tcsh
// would otherwise reject sh syntax like $(...), 2>/dev/null or [ ]). Only for versa's
// own commands: user hooks run in the login shell (dash as /bin/sh lacks `source`, [[ ]]).
func shWrap(cmd string) string {
	return "/bin/sh -c " + ShellQuote(cmd)
}

// Close closes the SSH and SFTP connections
func (c *Client) Close() error {
	if c.done != nil {
		select {
		case <-c.done:
		default:
			close(c.done)
		}
	}
	if c.sftpClient != nil {
		c.sftpClient.Close()
	}
	if c.agentConn != nil {
		c.agentConn.Close()
	}
	if c.sshClient != nil {
		return c.sshClient.Close()
	}
	return nil
}

// UploadDirectory uploads a directory recursively.
// Directories are created sequentially (to preserve parent-before-child ordering),
// then files are uploaded in parallel using a pool of 4 workers.
func (c *Client) UploadDirectory(localDir, remoteDir string) error {
	// Create remote root directory
	if err := c.sftpClient.MkdirAll(remoteDir); err != nil {
		return fmt.Errorf("failed to create remote directory: %w", err)
	}

	type filePair struct {
		local  string
		remote string
	}

	// Pass 1: collect directories and create them sequentially, collect files for parallel upload
	var files []filePair
	err := filepath.Walk(localDir, func(localPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(localDir, localPath)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)
		remotePath := filepath.ToSlash(filepath.Join(remoteDir, relPath))

		if info.IsDir() {
			return c.sftpClient.MkdirAll(remotePath)
		}

		files = append(files, filePair{local: localPath, remote: remotePath})
		return nil
	})
	if err != nil {
		return err
	}

	// Pass 2: upload files in parallel (4 workers, conservative for VPS bandwidth)
	const uploadWorkers = 4
	jobs := make(chan filePair, len(files))
	for _, f := range files {
		jobs <- f
	}
	close(jobs)

	var g errgroup.Group
	for i := 0; i < uploadWorkers; i++ {
		g.Go(func() error {
			for f := range jobs {
				if err := c.uploadFile(f.local, f.remote, nil); err != nil {
					return err
				}
			}
			return nil
		})
	}
	return g.Wait()
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

// uploadFile uploads a single file, optionally reporting progress to a writer.
// Uses a 256 KB buffer to reduce syscall overhead for large files.
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

	// Copy contents with an explicit buffer to reduce syscall overhead
	buf := make([]byte, 256*1024)
	var writer io.Writer = remoteFile
	if progress != nil {
		writer = io.MultiWriter(remoteFile, progress)
	}

	if _, err := io.CopyBuffer(writer, localFile, buf); err != nil {
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
	cmd := "tar -xzf " + ShellQuote(archivePath) + " -C " + ShellQuote(targetDir)
	output, err := c.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to extract archive: %w (output: %s)", err, output)
	}

	return nil
}

// ExecuteCommand executes one of versa's own commands under /bin/sh, with no timeout.
func (c *Client) ExecuteCommand(cmd string) (string, error) {
	return c.ExecuteCommandWithTimeout(shWrap(cmd), 0)
}

// ExecuteCommandWithTimeout executes a command with a specific timeout in the remote
// user's login shell, so user hooks keep the shell features they were written for.
func (c *Client) ExecuteCommandWithTimeout(cmd string, timeout time.Duration) (string, error) {
	session, err := c.sshClient.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	var outBuf, errBuf bytes.Buffer
	session.Stdout = &outBuf
	session.Stderr = &errBuf

	if err := session.Start(cmd); err != nil {
		return "", fmt.Errorf("failed to start command %q: %w", cmd, err)
	}

	done := make(chan error, 1)
	go func() {
		done <- session.Wait()
	}()

	var waitErr error
	if timeout > 0 {
		select {
		case <-time.After(timeout):
			session.Signal(ssh.SIGKILL)
			return outBuf.String(), verserrors.New(verserrors.CodeCommandTimeout, fmt.Sprintf("Remote command timed out after %v", timeout), "Increase hook_timeout (or deploy_timeout) in your environment config if this command legitimately needs more time.", nil)
		case waitErr = <-done:
		}
	} else {
		waitErr = <-done
	}

	// Combine stdout and stderr for full context on failure
	output := outBuf.String()
	if waitErr != nil {
		errMsg := errBuf.String()
		if errMsg != "" {
			return output, fmt.Errorf("command failed: %w (stderr: %s)", waitErr, strings.TrimSpace(errMsg))
		}
		return output, fmt.Errorf("command failed: %w", waitErr)
	}

	return output, nil
}

// ExecuteCommandStreaming runs a command and streams stdout/stderr to the provided writers in real-time.
// It allocates a PTY so that remote programs produce line-buffered output.
func (c *Client) ExecuteCommandStreaming(cmd string, stdout, stderr io.Writer) error {
	session, err := c.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Request PTY for proper line-buffered output
	modes := ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm", 80, 200, modes); err != nil {
		// Fall back to non-PTY if terminal allocation fails
		c.log.Debug("PTY allocation failed, falling back to pipe mode: %v", err)
	}

	session.Stdout = stdout
	session.Stderr = stderr

	if err := session.Run(cmd); err != nil {
		return fmt.Errorf("command failed: %w", err)
	}
	return nil
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

// CreateSymlink creates a symlink atomically using a single SSH round-trip.
// It creates a temporary symlink, atomically renames it to the final location,
// then reads back the target for verification — all in one shell command.
func (c *Client) CreateSymlink(target, linkPath string) error {
	cmd := symlinkSwapCmd(target, linkPath)
	output, err := c.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	// Verify the symlink points to the expected target
	actualTarget := strings.TrimSpace(output)
	if !strings.HasSuffix(actualTarget, target) && actualTarget != target {
		return fmt.Errorf("symlink verification failed: expected %s, got %s", target, actualTarget)
	}

	return nil
}

// symlinkSwapCmd replaces linkPath with a symlink to target. `mv -T` (GNU, recent
// busybox) is tried first; perl/python call rename(2) directly where mv lacks -T
// (old busybox). The last resort, rm + mv, is not atomic but works everywhere.
func symlinkSwapCmd(target, linkPath string) string {
	t, l, tmp := ShellQuote(target), ShellQuote(linkPath), ShellQuote(linkPath+".tmp")
	rename := `import os,sys;os.rename(sys.argv[1],sys.argv[2])`
	return fmt.Sprintf("ln -sfn %[1]s %[3]s && { mv -Tf %[3]s %[2]s 2>/dev/null"+
		" || perl -e 'rename $ARGV[0],$ARGV[1] or exit 1' %[3]s %[2]s 2>/dev/null"+
		" || python3 -c '%[4]s' %[3]s %[2]s 2>/dev/null"+
		" || python -c '%[4]s' %[3]s %[2]s 2>/dev/null"+
		" || { rm -f %[2]s && mv -f %[3]s %[2]s; }; } && readlink %[2]s", t, l, tmp, rename)
}

// CleanupOldReleases removes old releases, keeping only the specified number
func (c *Client) CleanupOldReleases(releasesDir string, keepCount int) error {
	releases, err := c.ListReleases(releasesDir)
	if err != nil {
		return err
	}

	// Never delete the newest release, even with a bad keepCount
	if keepCount < 1 {
		keepCount = 1
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
		cmd := "rm -rf -- " + ShellQuote(releaseDir)
		output, err := c.ExecuteCommand(cmd)
		if err != nil {
			return fmt.Errorf("failed to delete old release %s: %w (output: %s)", releases[i], err, output)
		}
	}

	return nil
}

// AvailableDiskBytes returns the free space of the filesystem holding path.
func (c *Client) AvailableDiskBytes(path string) (int64, error) {
	out, err := c.ExecuteCommand("df -Pk " + ShellQuote(path))
	if err != nil {
		return 0, err
	}
	return parseDfAvailable(out)
}

// parseDfAvailable reads the "Available" column of `df -Pk` output (in bytes).
// -P (POSIX) keeps each filesystem on one line; without it old coreutils (RHEL5) wrap
// long LVM device names and the columns shift. -k is portable, -B1 is GNU-only.
func parseDfAvailable(out string) (int64, error) {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	fields := strings.Fields(lines[len(lines)-1])
	if len(lines) < 2 || len(fields) < 6 {
		return 0, fmt.Errorf("unexpected df output: %q", out)
	}
	kb, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("unexpected df output: %q", out)
	}
	return kb * 1024, nil
}

// DirSizesKB runs `du -sk` over paths in order. du counts each hardlinked inode only
// once, so a path's size excludes data already counted for an earlier path.
func (c *Client) DirSizesKB(paths []string) (map[string]int64, error) {
	quoted := make([]string, len(paths))
	for i, p := range paths {
		quoted[i] = ShellQuote(p)
	}
	out, err := c.ExecuteCommand("du -sk " + strings.Join(quoted, " ") + " 2>/dev/null")
	sizes := map[string]int64{}
	for _, line := range strings.Split(out, "\n") {
		if f := strings.SplitN(line, "\t", 2); len(f) == 2 {
			if kb, perr := strconv.ParseInt(f[0], 10, 64); perr == nil {
				sizes[f[1]] = kb
			}
		}
	}
	if len(sizes) == 0 && err != nil {
		return nil, err
	}
	return sizes, nil
}

// Rename renames a remote path via SFTP (fails if newPath exists; no `mv -T` needed).
func (c *Client) Rename(oldPath, newPath string) error {
	return c.sftpClient.Rename(oldPath, newPath)
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

// ReadDir lists the contents of a remote directory via SFTP.
func (c *Client) ReadDir(path string) ([]os.FileInfo, error) {
	return c.sftpClient.ReadDir(path)
}

// ReadRemoteBytes reads up to maxBytes from a remote file into memory.
func (c *Client) ReadRemoteBytes(path string, maxBytes int64) ([]byte, error) {
	f, err := c.sftpClient.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open remote file: %w", err)
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, maxBytes))
}

// WriteRemoteBytes writes data to a remote file, preserving the original permissions
// when the file already exists. If the file does not exist it is created with mode 0644.
func (c *Client) WriteRemoteBytes(path string, data []byte) error {
	// Capture existing permissions if the file exists
	var existingMode os.FileMode = 0644
	if info, err := c.sftpClient.Stat(path); err == nil {
		existingMode = info.Mode()
	}

	f, err := c.sftpClient.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("failed to write remote file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close remote file: %w", err)
	}

	if err := c.sftpClient.Chmod(path, existingMode); err != nil {
		// Non-fatal: log but don't fail the write
		c.log.Warn("failed to restore file permissions on %s: %v", path, err)
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
