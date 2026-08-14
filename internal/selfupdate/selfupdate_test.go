package selfupdate

import (
	"fmt"
	"net/http"
	"net/http/httptest"
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "abc123  versa_linux_amd64\n")
	}))
	defer srv.Close()

	got, err := fetchChecksum(srv.URL)
	if err != nil {
		t.Fatalf("fetchChecksum failed: %v", err)
	}
	if got != "abc123" {
		t.Errorf("expected 'abc123', got %q", got)
	}
}

func TestFetchChecksum_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	if _, err := fetchChecksum(srv.URL); err == nil {
		t.Error("expected error for non-200 response")
	}
}
