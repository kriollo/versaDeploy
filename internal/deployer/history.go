package deployer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/user"
	"path"
	"strings"
	"time"

	verserrors "github.com/user/versaDeploy/internal/errors"
	"github.com/user/versaDeploy/internal/ssh"
	"github.com/user/versaDeploy/internal/version"
)

// Deploy history lives in <remote_path>/history: history.jsonl (one entry per deploy
// attempt, newest last) and <id>.log with each deploy's full log.
const (
	historyEntries = 100 // entries kept in history.jsonl
	historyLogs    = 20  // newest entries that keep their log file
)

// HistoryEntry is one deploy attempt, successful or not.
type HistoryEntry struct {
	ID        string    `json:"id"` // start time; also the log file name
	Started   time.Time `json:"started"`
	DurationS float64   `json:"duration_s"`
	Status    string    `json:"status"` // success | failure
	Release   string    `json:"release,omitempty"`
	Commit    string    `json:"commit,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	Error     string    `json:"error,omitempty"`
	By        string    `json:"by"`      // user@host that ran versa
	Version   string    `json:"version"` // versa version
	Log       string    `json:"log,omitempty"`
}

// beginAttempt starts capturing this deploy's log (written to the server's history).
func (d *Deployer) beginAttempt() {
	d.started = time.Now()
	d.logBuf = &bytes.Buffer{}
	d.log = d.log.Tee(d.logBuf)
}

// endAttempt runs on_failure hooks after a failed deploy, then appends the attempt to
// the server's history. It runs while the deploy lock is still held.
func (d *Deployer) endAttempt(c *ssh.Client, release, commit string, deployErr error) {
	if deployErr != nil && len(d.env.OnFailure) > 0 {
		d.log.Info("Running on_failure hooks...")
		for _, h := range d.env.OnFailure {
			for _, cmd := range append([]string{h.Command}, h.Parallel...) {
				if cmd != "" {
					if err := d.execHook(c, d.remote("current"), cmd); err != nil {
						d.log.Warn("on_failure hook failed: %v", err)
					}
				}
			}
		}
	}
	e := HistoryEntry{
		ID:        d.started.UTC().Format("20060102-150405"), // UTC, like release names
		Started:   d.started.UTC(),
		DurationS: time.Since(d.started).Round(100 * time.Millisecond).Seconds(),
		Status:    "success",
		Release:   release,
		Commit:    commit,
		Tags:      d.tags,
		By:        whoami(),
		Version:   version.Version,
	}
	if deployErr != nil {
		e.Status, e.Error = "failure", deployErr.Error()
	}
	prev, err := d.appendHistory(c, e, d.logBuf.Bytes())
	if err != nil {
		d.log.Warn("Could not update the deploy history: %v", err)
	}
	d.prevStatus = prev
}

// appendHistory adds e (and its log) to the server's history, drops the oldest entries
// and logs, and returns the status of the attempt before e ("" if none).
func (d *Deployer) appendHistory(c *ssh.Client, e HistoryEntry, logData []byte) (string, error) {
	dir := d.remote("history")
	if err := c.MkdirAll(dir); err != nil {
		return "", err
	}
	entries, err := readHistory(c, dir)
	if err != nil {
		return "", err
	}
	prev := ""
	if len(entries) > 0 {
		prev = entries[len(entries)-1].Status
	}
	e.Log = e.ID + ".log"
	// logs can carry hook output (secrets): private, like the deploy artifacts
	if err := c.WriteRemoteFileMode(path.Join(dir, e.Log), logData, 0600); err != nil {
		e.Log = ""
	}
	entries = append(entries, e)
	for i := range len(entries) - historyLogs {
		if entries[i].Log != "" {
			c.Remove(path.Join(dir, entries[i].Log))
			entries[i].Log = ""
		}
	}
	if len(entries) > historyEntries {
		entries = entries[len(entries)-historyEntries:]
	}
	var buf bytes.Buffer
	for _, x := range entries {
		line, _ := json.Marshal(x)
		buf.Write(append(line, '\n'))
	}
	return prev, c.WriteRemoteFileMode(path.Join(dir, "history.jsonl"), buf.Bytes(), 0600)
}

// readHistory parses dir/history.jsonl, oldest first; no file = no history.
func readHistory(c *ssh.Client, dir string) ([]HistoryEntry, error) {
	f := path.Join(dir, "history.jsonl")
	if ok, err := c.FileExists(f); err != nil || !ok {
		return nil, err
	}
	data, err := c.ReadRemoteBytes(f, 16<<20)
	if err != nil {
		return nil, err
	}
	var out []HistoryEntry
	for _, line := range strings.Split(string(data), "\n") {
		var e HistoryEntry
		if json.Unmarshal([]byte(line), &e) == nil && e.ID != "" {
			out = append(out, e)
		}
	}
	return out, nil
}

// History returns the server's deploy history, oldest first.
func (d *Deployer) History(c *ssh.Client) ([]HistoryEntry, error) {
	return ReadHistory(c, d.env.RemotePath)
}

// ReadHistory returns the deploy history under remotePath, oldest first.
func ReadHistory(c *ssh.Client, remotePath string) ([]HistoryEntry, error) {
	return readHistory(c, path.Join(remotePath, "history"))
}

// FormatHistory renders the newest n entries, newest first, as table lines.
func FormatHistory(entries []HistoryEntry, n int) []string {
	lines := []string{fmt.Sprintf("%-15s  %-7s  %-6s  %-8s  %-12s  %s", "ID", "STATUS", "TIME", "COMMIT", "TAG", "BY")}
	for i := len(entries) - 1; i >= 0 && i >= len(entries)-n; i-- {
		e := entries[i]
		status := e.Status
		if status == "failure" {
			status = "FAILED"
		}
		commit := e.Commit
		if len(commit) > 8 {
			commit = commit[:8]
		}
		lines = append(lines, fmt.Sprintf("%-15s  %-7s  %5.0fs  %-8s  %-12s  %s", e.ID, status, e.DurationS, commit, strings.Join(e.Tags, ","), e.By))
		if e.Error != "" {
			msg := strings.SplitN(e.Error, "\n", 2)[0]
			if r := []rune(msg); len(r) > 110 {
				msg = string(r[:110]) + "…"
			}
			lines = append(lines, "                 └ "+msg)
		}
	}
	return lines
}

// HistoryLog returns the log of the attempt with this id.
func (d *Deployer) HistoryLog(c *ssh.Client, id string) ([]byte, error) {
	if !validHistoryID(id) {
		return nil, fmt.Errorf("invalid deploy id %q (e.g. 20261108-153000)", id)
	}
	return c.ReadRemoteBytes(d.remote("history", id+".log"), 64<<20)
}

func validHistoryID(id string) bool {
	_, err := time.Parse("20060102-150405", id)
	return err == nil
}

// requireTag enforces deploy_tags on d.tags.
func (d *Deployer) requireTag() error {
	if d.env.DeployTags == "" {
		return nil
	}
	for _, t := range d.tags {
		if ok, _ := path.Match(d.env.DeployTags, t); ok {
			d.log.Info("Tag: %s", t)
			return nil
		}
	}
	return fmt.Errorf("%s only deploys tagged commits (deploy_tags: %q) and this commit has no matching tag: tag it first, e.g. git tag v1.2.0 && git push --tags", d.envName, d.env.DeployTags)
}

// whoami names who ran the deploy: user@host.
func whoami() string {
	name := "unknown"
	if u, err := user.Current(); err == nil {
		name = u.Username
	}
	if h, err := os.Hostname(); err == nil {
		name += "@" + h
	}
	return name
}

// logTail returns the last n lines of this deploy's log.
func (d *Deployer) logTail(n int) string {
	if d.logBuf == nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(d.logBuf.String(), "\n"), "\n")
	return strings.Join(lines[max(0, len(lines)-n):], "\n")
}

// PrintHistory writes the newest n attempts to w, or the log of attempt id.
func (d *Deployer) PrintHistory(w io.Writer, id string, n int) error {
	if d.env.Local {
		return errLocalUnsupported("history")
	}
	c, err := ssh.NewClient(&d.env.SSH, d.log)
	if err != nil {
		return verserrors.Wrap(err)
	}
	defer c.Close()
	if id != "" {
		b, err := d.HistoryLog(c, id)
		if err != nil {
			return fmt.Errorf("no log for %s (only the newest %d deploys keep theirs): %w", id, historyLogs, err)
		}
		_, err = w.Write(b)
		return err
	}
	entries, err := d.History(c)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Fprintln(w, "No deploys recorded yet (the history starts with versa 1.15.0).")
		return nil
	}
	fmt.Fprintln(w, strings.Join(FormatHistory(entries, n), "\n"))
	fmt.Fprintf(w, "\nFull log: versa history %s <ID>\n", d.envName)
	return nil
}
