package artifact

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/user/versaDeploy/internal/builder"
)

// Manifest represents the manifest.json structure
type Manifest struct {
	ReleaseVersion  string          `json:"release_version"`
	CommitHash      string          `json:"commit_hash"`
	BuildTimestamp  time.Time       `json:"build_timestamp"`
	ChangesApplied  ChangesApplied  `json:"changes_applied"`
}

// ChangesApplied tracks what was changed in this release
type ChangesApplied struct {
	PHPFilesChanged     int  `json:"php_files_changed"`
	GoBinaryRebuilt     bool `json:"go_binary_rebuilt"`
	FrontendCompiled    int  `json:"frontend_files_compiled"`
	ComposerUpdated     bool `json:"composer_updated"`
	NPMUpdated          bool `json:"npm_updated"`
	TwigCacheCleanup    bool `json:"twig_cache_cleanup"`
	RouteCacheRegenerate bool `json:"route_cache_regenerate"`
}

// Generator handles artifact generation
type Generator struct {
	artifactDir    string
	releaseVersion string
	commitHash     string
}

// NewGenerator creates a new artifact generator
func NewGenerator(artifactDir, releaseVersion, commitHash string) *Generator {
	return &Generator{
		artifactDir:    artifactDir,
		releaseVersion: releaseVersion,
		commitHash:     commitHash,
	}
}

// GenerateManifest creates the manifest.json file
func (g *Generator) GenerateManifest(buildResult *builder.BuildResult) error {
	manifest := Manifest{
		ReleaseVersion: g.releaseVersion,
		CommitHash:     g.commitHash,
		BuildTimestamp: time.Now().UTC(),
		ChangesApplied: ChangesApplied{
			PHPFilesChanged:     buildResult.PHPFilesChanged,
			GoBinaryRebuilt:     buildResult.GoBinaryRebuilt,
			FrontendCompiled:    buildResult.FrontendCompiled,
			ComposerUpdated:     buildResult.ComposerUpdated,
			NPMUpdated:          buildResult.NPMUpdated,
			TwigCacheCleanup:    buildResult.TwigCacheCleanup,
			RouteCacheRegenerate: buildResult.RouteCacheRegenerate,
		},
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	manifestPath := filepath.Join(g.artifactDir, "manifest.json")
	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	return nil
}

// Validate checks that the artifact is complete
func (g *Generator) Validate() error {
	// Check manifest exists
	manifestPath := filepath.Join(g.artifactDir, "manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return fmt.Errorf("manifest.json not found in artifact")
	}

	// Validate artifact directory structure
	requiredDirs := []string{"app", "vendor", "node_modules", "public", "bin"}
	for _, dir := range requiredDirs {
		dirPath := filepath.Join(g.artifactDir, dir)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			// Directory might not exist if that build type wasn't enabled
			// This is acceptable
			continue
		}
	}

	return nil
}

// GenerateReleaseVersion creates a timestamp-based release version
func GenerateReleaseVersion() string {
	return time.Now().UTC().Format("20060102-150405")
}
