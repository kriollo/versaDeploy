package ssh

import (
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/pkg/sftp"
	"github.com/user/versaDeploy/internal/config"
)

// memClient returns a Client whose SFTP side talks to an in-memory SFTP server.
func memClient(t *testing.T) *Client {
	t.Helper()
	serverConn, clientConn := net.Pipe()
	server := sftp.NewRequestServer(serverConn, sftp.InMemHandler())
	go server.Serve()
	client, err := sftp.NewClientPipe(clientConn, clientConn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		client.Close()
		server.Close()
	})
	return &Client{sftpClient: client, config: &config.SSHConfig{User: "deploy"}}
}

// Files go into folders that may not exist yet (e.g. <remote_path>/.versa/).
func TestWriteRemoteBytesCreatesMissingFolders(t *testing.T) {
	c := memClient(t)
	if err := c.MkdirAll("/srv"); err != nil {
		t.Fatal(err)
	}
	if err := c.WriteRemoteBytes("/srv/app/.versa/api.sh", []byte("hi")); err != nil {
		t.Fatalf("write into missing folders: %v", err)
	}
	if b, err := c.ReadRemoteBytes("/srv/app/.versa/api.sh", 10); err != nil || string(b) != "hi" {
		t.Errorf("read back %q, %v", b, err)
	}
	if fi, err := c.sftpClient.Stat("/srv/app/.versa"); err != nil || !fi.IsDir() {
		t.Errorf("parent folder not created: %v", err)
	}
	// The in-memory server must refuse files in missing folders, or this test proves nothing
	if f, err := c.sftpClient.Create("/nope/x"); err == nil {
		f.Close()
		t.Fatal("in-memory SFTP server accepts files in missing folders")
	}
}

// A missing remote_path must not be reported as "lock already held".
func TestAcquireLockErrors(t *testing.T) {
	c := memClient(t)

	err := c.AcquireLock("/var/www/web/.versa.lock")
	if err == nil || !strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "already held") {
		t.Fatalf("missing remote_path: got %v", err)
	}

	if err := c.MkdirAll("/var/www/web"); err != nil {
		t.Fatal(err)
	}
	if err := c.AcquireLock("/var/www/web/.versa.lock"); err != nil {
		t.Fatalf("first lock: %v", err)
	}
	if err := c.AcquireLock("/var/www/web/.versa.lock"); err == nil || !strings.Contains(err.Error(), "already held") {
		t.Fatalf("second lock must report it is held, got %v", err)
	}
}

func TestRenameErrorsNamePaths(t *testing.T) {
	c := memClient(t)
	err := c.Rename("/srv/releases/x.staging", "/srv/releases/x")
	if err == nil || !strings.Contains(err.Error(), "/srv/releases/x.staging does not exist") {
		t.Fatalf("got %v", err)
	}
}

// Files holding secrets get exactly the requested mode, even over an existing 0644
// file. Uses the disk-backed server: the in-memory one ignores chmod.
func TestWriteRemoteFileModeSetsMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX modes")
	}
	serverConn, clientConn := net.Pipe()
	server, err := sftp.NewServer(serverConn)
	if err != nil {
		t.Fatal(err)
	}
	go server.Serve()
	sc, err := sftp.NewClientPipe(clientConn, clientConn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sc.Close(); server.Close() })
	c := &Client{sftpClient: sc, config: &config.SSHConfig{User: "deploy"}}

	p := filepath.ToSlash(filepath.Join(t.TempDir(), "api.sh"))
	if err := c.WriteRemoteBytes(p, []byte("old")); err != nil {
		t.Fatal(err)
	}
	if err := c.WriteRemoteFileMode(p, []byte("export DB_PASS='x'"), 0600); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(p)
	if err != nil || fi.Mode().Perm() != 0600 {
		t.Fatalf("mode = %v, %v; want 0600", fi.Mode().Perm(), err)
	}
}
