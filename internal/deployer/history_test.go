package deployer

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"github.com/user/versaDeploy/internal/config"
	"github.com/user/versaDeploy/internal/logger"
	"github.com/user/versaDeploy/internal/ssh"
)

func memSFTP(t *testing.T) *ssh.Client {
	serverConn, clientConn := net.Pipe()
	server := sftp.NewRequestServer(serverConn, sftp.InMemHandler())
	go server.Serve()
	client, err := sftp.NewClientPipe(clientConn, clientConn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close(); server.Close() })
	c := ssh.NewSFTPOnly(client)
	if err := c.MkdirAll("/srv/app"); err != nil {
		t.Fatal(err)
	}
	return c
}

// Every attempt is appended with its log; old logs and entries are dropped.
func TestHistory(t *testing.T) {
	c := memSFTP(t)
	log, _ := logger.NewLogger("", false, false)
	cfg := &config.Config{Project: "p", Environments: map[string]config.Environment{"prod": {RemotePath: "/srv/app"}}}
	d, _ := NewDeployer(cfg, "prod", ".", false, false, false, false, log)

	start := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	for i := range historyEntries + 5 {
		d.beginAttempt()
		d.started = start.Add(time.Duration(i) * time.Minute)
		d.log.Info("attempt %d", i)
		var err error
		if i%2 == 1 {
			err = fmt.Errorf("boom %d", i)
		}
		d.endAttempt(c, "r", "abc", err)
		want := "" // the status of attempt i-1: odd attempts fail
		if i > 0 {
			want = "success"
			if (i-1)%2 == 1 {
				want = "failure"
			}
		}
		if d.prevStatus != want {
			t.Fatalf("attempt %d: prevStatus %q, want %q", i, d.prevStatus, want)
		}
	}
	h, err := d.History(c)
	if err != nil || len(h) != historyEntries {
		t.Fatalf("%d entries, %v", len(h), err)
	}
	last := h[len(h)-1]
	if last.Status != "success" || h[len(h)-2].Error != fmt.Sprintf("boom %d", historyEntries+3) || last.By == "" {
		t.Errorf("last entries: %+v %+v", h[len(h)-2], last)
	}
	b, err := d.HistoryLog(c, last.ID)
	if err != nil || !strings.Contains(string(b), fmt.Sprintf("attempt %d", historyEntries+4)) {
		t.Errorf("log: %q %v", b, err)
	}
	withLog := 0
	for _, e := range h {
		if e.Log != "" {
			withLog++
		}
	}
	files, _ := c.ReadDir("/srv/app/history")
	if withLog != historyLogs || len(files) != historyLogs+1 {
		t.Errorf("%d entries with a log, %d files: want %d", withLog, len(files), historyLogs)
	}
	if _, err := d.HistoryLog(c, "../deploy"); err == nil {
		t.Error("an id outside history/ must be refused")
	}
}

// deploy_tags refuses untagged commits.
func TestRequireTag(t *testing.T) {
	log, _ := logger.NewLogger("", false, false)
	cfg := &config.Config{Project: "p", Environments: map[string]config.Environment{"prod": {RemotePath: "/srv", DeployTags: "v*"}}}
	d, _ := NewDeployer(cfg, "prod", ".", false, false, false, false, log)
	for tags, ok := range map[string]bool{"": false, "nightly": false, "nightly v1.2.0": true} {
		d.tags = strings.Fields(tags)
		if err := d.requireTag(); (err == nil) != ok {
			t.Errorf("tags %q: %v", tags, err)
		}
	}
}
