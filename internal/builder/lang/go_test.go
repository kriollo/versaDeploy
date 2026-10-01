package lang

import (
	"debug/elf"
	"os"
	"path/filepath"
	"testing"

	"github.com/user/versaDeploy/internal/changeset"
	"github.com/user/versaDeploy/internal/config"
	"github.com/user/versaDeploy/internal/logger"
)

// The default build must be a static linux binary: no dynamic loader means no
// dependency on the build machine's glibc.
func TestGoBuilderStaticByDefault(t *testing.T) {
	repo, out := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module hello\n\ngo 1.21\n"), 0644)
	// net forces cgo when CGO_ENABLED=1, so a dynamic binary would show up here
	os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main\nimport \"net\"\nfunc main() { net.LookupHost(\"x\") }\n"), 0644)

	log, _ := logger.NewLogger("", false, false)
	ctx := &BuilderContext{
		RepoPath:    repo,
		ArtifactDir: out,
		Config: &config.Environment{Builds: config.BuildsConfig{Go: config.GoBuildConfig{
			Enabled: true, TargetOS: "linux", TargetArch: "amd64", BinaryName: "app", DeployPath: "bin",
		}}},
		Changeset: &changeset.ChangeSet{GoFiles: []string{"main.go"}},
		Log:       log,
	}
	if _, _, err := (&GoBuilder{}).Build(ctx); err != nil {
		t.Fatal(err)
	}

	f, err := elf.Open(filepath.Join(out, "bin", "app"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, p := range f.Progs {
		if p.Type == elf.PT_INTERP {
			t.Fatal("binary is dynamically linked (has PT_INTERP)")
		}
	}
}
