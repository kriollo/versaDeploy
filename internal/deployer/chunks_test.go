package deployer

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/user/versaDeploy/internal/logger"
)

// Two builds with the same release version (deploys started in the same second, of
// different projects) compress side by side: chunk names come from each private dir.
func TestChunksSameReleaseVersion(t *testing.T) {
	log, _ := logger.NewLogger("", false, false)
	arts := make([]*PrebuiltArtifact, 2)
	for i := range arts {
		dir, _ := os.MkdirTemp("", "versadeploy-artifact-test-")
		os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0644)
		arts[i] = &PrebuiltArtifact{ReleaseVersion: "20261108-210606", artifactDir: dir}
		t.Cleanup(arts[i].Cleanup)
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, a := range arts {
		wg.Add(1)
		go func() { defer wg.Done(); _, errs[i] = a.chunks(1<<20, log) }()
	}
	wg.Wait()
	if errs[0] != nil || errs[1] != nil || arts[0].ChunkPaths[0] == arts[1].ChunkPaths[0] {
		t.Fatalf("errs %v, chunks %v %v", errs, arts[0].ChunkPaths, arts[1].ChunkPaths)
	}
}
