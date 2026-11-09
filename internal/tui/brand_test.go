package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/user/versaDeploy/internal/config"
	versassh "github.com/user/versaDeploy/internal/ssh"
)

func TestBrandMarkFitsHeader(t *testing.T) {
	if got := lipgloss.Width(brandTitle()); got != len("versaDeploy") {
		t.Fatalf("brandTitle width = %d, want %d", got, len("versaDeploy"))
	}
	if got := lipgloss.Width(brandMark()); got != 2+len("versaDeploy") {
		t.Fatalf("brandMark width = %d, want %d", got, 2+len("versaDeploy"))
	}
}

func TestBrandBannerAligned(t *testing.T) {
	lines := brandBanner()
	if len(lines) != 11 {
		t.Fatalf("banner lines = %d, want 11", len(lines))
	}
	for i, line := range lines {
		if got := lipgloss.Width(line); got != brandBannerWidth {
			t.Fatalf("line %d width = %d, want %d (%q)", i, got, brandBannerWidth, line)
		}
	}
}

func TestRenderHeaderKeepsEnvAndConnection(t *testing.T) {
	got := renderHeader(80, "v1.10.0", "production", true)
	for _, want := range []string{"production", "CONNECTED", "v1.10.0", "versa", "ploy"} {
		if !strings.Contains(got, want) {
			t.Fatalf("header missing %q:\n%s", want, got)
		}
	}
	if lipgloss.Width(got) > 80 {
		t.Fatalf("header wider than the terminal: %d", lipgloss.Width(got))
	}
}

func TestDashboardBannerSplashThenSide(t *testing.T) {
	logo := "╭──────────────╮"

	d := dashboardModel{}
	if got := d.view(80, 30); !strings.Contains(got, logo) || !strings.Contains(got, "Loading") {
		t.Fatalf("expected logo splash while loading:\n%s", got)
	}
	if got := d.view(20, 30); strings.Contains(got, logo) {
		t.Fatal("splash should hide on a narrow pane")
	}

	d.loaded = true
	d.server = &versassh.ServerInfo{Hostname: "web1", CPUModel: "a very long CPU model name that would push the logo off the pane"}
	got := d.view(100, 30)
	lines := strings.Split(got, "\n")
	if len(lines) > 30 || !strings.Contains(got, logo) {
		t.Fatalf("expected side logo within the pane:\n%s", got)
	}
	for i, l := range lines {
		if w := lipgloss.Width(l); w > 100 {
			t.Fatalf("line %d wider than the pane: %d", i, w)
		}
	}
	if got := d.view(80, 30); strings.Contains(got, logo) {
		t.Fatal("side logo should hide when the stats need the width")
	}
}

func TestStartupSplashThenPicker(t *testing.T) {
	m := newAppModel(nil, t.TempDir())
	m.currentView = viewConfigSelector
	m.width, m.height = 100, 40
	if got := m.View(); !strings.Contains(got, "╭──────────────╮") || strings.Contains(got, "Select Configuration File") {
		t.Fatalf("expected the logo splash first:\n%s", got)
	}

	next, _ := m.Update(msgSplashDone{})
	if got := next.View(); !strings.Contains(got, "Select Configuration File") {
		t.Fatalf("expected the config picker after the splash:\n%s", got)
	}

	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // any key skips the splash
	if next.(appModel).splash {
		t.Fatal("a key press should skip the splash")
	}
}

// Passing through the Terminal view must not grab the keyboard: ←/→ switch views
// until Enter starts typing; Esc stops typing, a second Esc leaves.
func TestTerminalTypingOnlyAfterEnter(t *testing.T) {
	m := newAppModel(nil, t.TempDir())
	m.splash = false
	m.width, m.height = 120, 40
	press := func(m appModel, k tea.KeyMsg) appModel {
		next, _ := m.Update(k)
		return next.(appModel)
	}

	m.currentView = viewOperations
	m = press(m, tea.KeyMsg{Type: tea.KeyRight})
	if m.currentView != viewTerminal || m.terminal.typing || m.terminal.input.Focused() {
		t.Fatalf("entering Terminal: view=%v typing=%v", m.currentView, m.terminal.typing)
	}
	if got := press(m, tea.KeyMsg{Type: tea.KeyRight}).currentView; got != viewServices {
		t.Fatalf("Right through Terminal went to %v, want Services", got)
	}

	m = press(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.terminal.typing || !m.terminal.input.Focused() {
		t.Fatal("Enter must start typing")
	}
	m = press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ls")})
	if m.terminal.input.Value() != "ls" {
		t.Fatalf("typed %q", m.terminal.input.Value())
	}
	if got := press(m, tea.KeyMsg{Type: tea.KeyRight}).currentView; got != viewTerminal {
		t.Fatalf("Right while typing must stay in the Terminal, got %v", got)
	}

	m = press(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.currentView != viewTerminal || m.terminal.typing {
		t.Fatal("first Esc stops typing and stays")
	}
	if got := press(m, tea.KeyMsg{Type: tea.KeyEsc}).currentView; got != viewDashboard {
		t.Fatalf("second Esc goes back, got %v", got)
	}
}

// --config wins over the picker; an auto-discovered default keeps it when the
// directory has other configs.
func TestExplicitConfigSkipsPicker(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"deploy.yml", "deploy_other.yml"} {
		os.WriteFile(filepath.Join(dir, f), []byte("project: x\n"), 0o644)
	}
	t.Chdir(dir)
	for _, explicit := range []bool{true, false} {
		m := newAppModel(&config.Config{Project: "x"}, dir)
		m.discoverConfigs()
		m.useConfig("deploy.yml", explicit)
		if got := m.currentView == viewConfigSelector; got == explicit {
			t.Errorf("explicit=%v: picker shown=%v", explicit, got)
		}
		if m.config.configPath != filepath.Join(dir, "deploy.yml") {
			t.Errorf("configPath = %q", m.config.configPath)
		}
	}
}

// A long status never wraps the status bar (it pushed the header off the screen).
func TestStatusbarOneLine(t *testing.T) {
	hints := "←/→:views  Tab:sidebar  ↑/↓:navigate  ↵:select  F5:refresh  D:deploy  R:rollback  c:connect  q:quit"
	long := "Connection failed: SSH Authentication failed — Check your SSH private key path and ensure it's added to the remote server's authorized_keys. — fresh server? Services → P prepares it for deploy"
	for _, w := range []int{40, 80, 120, 200} {
		out := renderStatusbar(w, hints, long)
		if n := strings.Count(out, "\n"); n != 0 || lipgloss.Width(out) > w {
			t.Errorf("width %d: %d extra lines, width %d", w, n, lipgloss.Width(out))
		}
		if !strings.Contains(out, "Connection failed") {
			t.Errorf("width %d: the status must win over the hints: %q", w, out)
		}
	}
}
