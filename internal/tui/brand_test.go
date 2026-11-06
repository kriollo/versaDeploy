package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

// ←/→ must not get stuck in the Terminal view: on an empty prompt they switch views.
func TestTerminalArrowsSwitchViewsWhenEmpty(t *testing.T) {
	m := newAppModel(nil, t.TempDir())
	m.splash = false
	m.width, m.height = 120, 40
	m.currentView = viewTerminal
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if got := next.(appModel).currentView; got != viewServices {
		t.Fatalf("Right from an empty Terminal went to %v, want Services", got)
	}
	m.terminal.input.SetValue("ls")
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	if got := next.(appModel).currentView; got != viewTerminal {
		t.Fatalf("Right while typing must stay in the Terminal, got %v", got)
	}
}
