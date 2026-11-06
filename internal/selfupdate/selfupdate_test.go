package selfupdate

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/user/versaDeploy/internal/logger"
)

func withMockGithubAPI(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	original := githubAPIBase
	githubAPIBase = srv.URL
	t.Cleanup(func() { githubAPIBase = original })
}

func TestGetLatestRelease(t *testing.T) {
	withMockGithubAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"tag_name":"v9.9.9","assets":[{"name":"versa_linux_amd64","browser_download_url":"http://example.com/versa_linux_amd64"}]}`)
	})

	log, _ := logger.NewLogger("", false, false)
	u := NewUpdater(log)

	release, err := u.getLatestRelease()
	if err != nil {
		t.Fatalf("getLatestRelease failed: %v", err)
	}
	if release.TagName != "v9.9.9" {
		t.Errorf("expected tag v9.9.9, got %s", release.TagName)
	}
	if len(release.Assets) != 1 || release.Assets[0].Name != "versa_linux_amd64" {
		t.Errorf("unexpected assets: %+v", release.Assets)
	}
}

func TestGetLatestRelease_NonOKStatus(t *testing.T) {
	withMockGithubAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	log, _ := logger.NewLogger("", false, false)
	u := NewUpdater(log)

	if _, err := u.getLatestRelease(); err == nil {
		t.Error("expected error for non-200 response")
	}
}

func TestFetchChecksum(t *testing.T) {
	sum := strings.Repeat("ab", 32)
	other := strings.Repeat("cd", 32)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/bad":
			fmt.Fprint(w, "abc123  versa_linux_amd64\n")
		default: // CI runs sha256sum on dist/<name>
			fmt.Fprintf(w, "%s  dist/versa_darwin_arm64\n%s  dist/versa_linux_amd64\n", other, sum)
		}
	}))
	defer srv.Close()
	original := githubAPIBase
	githubAPIBase = srv.URL
	defer func() { githubAPIBase = original }()

	got, err := fetchChecksum(srv.URL, "versa_linux_amd64")
	if err != nil || got != sum {
		t.Fatalf("fetchChecksum = %q, %v; want the digest listed for the asset", got, err)
	}
	if _, err := fetchChecksum(srv.URL+"/bad", "versa_linux_amd64"); err == nil {
		t.Error("a non-sha256 digest must be refused")
	}
	if _, err := fetchChecksum(srv.URL, "versa_windows_amd64.exe"); err == nil {
		t.Error("a missing entry must be refused")
	}
}

func TestDownloadURLsMustBeGitHubHTTPS(t *testing.T) {
	for raw, ok := range map[string]bool{
		"https://github.com/kriollo/versaDeploy/releases/download/v1/versa": true,
		"https://objects.githubusercontent.com/x":                           true,
		"http://github.com/kriollo/versaDeploy/releases/download/v1/versa":  false,
		"https://evil.example/versa":                                        false,
		"https://github.com.evil.example/versa":                             false,
		"https://release-assets.githubusercontent.com.evil.example/versa":   false,
	} {
		if err := checkURL(raw); (err == nil) != ok {
			t.Errorf("checkURL(%q) = %v, want ok=%v", raw, err, ok)
		}
	}
}

func TestIsOlder(t *testing.T) {
	cases := []struct {
		tag, current string
		older        bool
	}{
		{"v1.9.1", "1.10.0", true},
		{"v1.10.0", "1.10.0", false},
		{"v1.11.0", "1.10.0", false},
		{"v2.0.0-beta", "1.10.0", false},
		{"v1.0.0", "dev", false},
	}
	for _, c := range cases {
		if got := isOlder(c.tag, c.current); got != c.older {
			t.Errorf("isOlder(%s, %s) = %v", c.tag, c.current, got)
		}
	}
}

func TestFetchChecksum_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	if _, err := fetchChecksum(srv.URL, "x"); err == nil {
		t.Error("expected error for non-200 response")
	}
}

func TestUpdate_RefusesReleaseWithoutChecksum(t *testing.T) {
	name := fmt.Sprintf("versa_%s_%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	withMockGithubAPI(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"tag_name":"v9.9.9","assets":[{"name":%q,"browser_download_url":"http://127.0.0.1:1/bin"}]}`, name)
	})
	log, _ := logger.NewLogger("", false, false)
	if err := NewUpdater(log).Update(); err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("expected refusal without checksum, got %v", err)
	}
}
