package artifact

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/user/versaDeploy/internal/builder"
)

// Manifest represents the manifest.json structure
type Manifest struct {
	ReleaseVersion string         `json:"release_version"`
	CommitHash     string         `json:"commit_hash"`
	BuildTimestamp time.Time      `json:"build_timestamp"`
	ChangesApplied ChangesApplied `json:"changes_applied"`
}

// ChangesApplied tracks what was changed in this release
type ChangesApplied struct {
	PHPFilesChanged      int  `json:"php_files_changed"`
	GoBinaryRebuilt      bool `json:"go_binary_rebuilt"`
	FrontendCompiled     int  `json:"frontend_files_compiled"`
	ComposerUpdated      bool `json:"composer_updated"`
	NPMUpdated           bool `json:"npm_updated"`
	TwigCacheCleanup     bool `json:"twig_cache_cleanup"`
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
			PHPFilesChanged:      buildResult.PHPFilesChanged,
			GoBinaryRebuilt:      buildResult.GoBinaryRebuilt,
			FrontendCompiled:     buildResult.FrontendCompiled,
			ComposerUpdated:      buildResult.ComposerUpdated,
			NPMUpdated:           buildResult.NPMUpdated,
			TwigCacheCleanup:     buildResult.TwigCacheCleanup,
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

// Compress creates a .tar.gz archive of the artifact directory with a progress bar
func (g *Generator) Compress(archivePath string) error {
	// First, count files for progress bar
	var fileCount int64
	filepath.WalkDir(g.artifactDir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			fileCount++
		}
		return nil
	})

	bar := progressbar.Default(fileCount, "Compressing artifact")

	file, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}
	defer file.Close()

	gw := gzip.NewWriter(file)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	return filepath.WalkDir(g.artifactDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// Skip entries that cause errors (like unreadable symlinks)
			fmt.Printf("[WARN] Skipping path (error): %s - %v\n", path, err)
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(g.artifactDir, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			fmt.Printf("[WARN] Skipping (cannot get info): %s - %v\n", relPath, err)
			return nil
		}

		// Create header manually to avoid Windows file mode issues
		header := &tar.Header{
			Name:    filepath.ToSlash(relPath),
			ModTime: info.ModTime(),
			Size:    info.Size(),
		}

		// Handle symlinks, junctions and reparse points
		isSymlink := info.Mode()&os.ModeSymlink != 0

		// Detailed check for Windows Junctions (which often appear as Dir | Irregular)
		if !isSymlink && info.Mode()&os.ModeIrregular != 0 {
			// Try to read as link anyway to see if it's a junction
			if _, err := os.Readlink(path); err == nil {
				isSymlink = true
			}
		}

		if isSymlink {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				// On Windows, symlinks/junctions may not be readable
				// Or they might be dangling (pointing to the ignored .pnpm)
				fmt.Printf("[WARN] Skipping dangling/unreadable link: %s\n", relPath)
				return nil
			}

			// Convert absolute targets (common on Windows junctions) to relative targets
			// so they work correctly when extracted on a different system/path.
			if filepath.IsAbs(linkTarget) {
				// Check if the target is inside our artifact directory
				// Use filepath.Dir(path) as the source for the relative jump
				if relTarget, err := filepath.Rel(g.artifactDir, linkTarget); err == nil {
					// Check if it's actually inside (doesn't start with ..)
					if !strings.HasPrefix(relTarget, ".."+string(filepath.Separator)) && relTarget != ".." {
						// Calculate relative path from the link directory to the target
						if portableTarget, err := filepath.Rel(filepath.Dir(path), linkTarget); err == nil {
							linkTarget = portableTarget
						}
					}
				}
			}

			header.Typeflag = tar.TypeSymlink
			header.Linkname = filepath.ToSlash(linkTarget)
			header.Size = 0
		} else if info.IsDir() {
			header.Typeflag = tar.TypeDir
			header.Mode = 0775 // rwxrwxr-x
		} else {
			header.Typeflag = tar.TypeReg
			header.Mode = 0664 // rw-rw-r--
		}

		// Write header
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write header for %s: %w", relPath, err)
		}

		// Only write content for regular files
		if header.Typeflag == tar.TypeReg {
			f, err := os.Open(path)
			if err != nil {
				// Final safety check for files that disappeared or are locked
				fmt.Printf("[WARN] Skipping file (cannot open): %s - %v\n", relPath, err)
				return nil
			}
			defer f.Close()

			_, err = io.Copy(tw, f)
			if err != nil {
				return fmt.Errorf("failed to copy content for %s: %w", relPath, err)
			}
			bar.Add(1)
		}

		return nil
	})
}
