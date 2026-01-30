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

// Compress creates a single-part .tar.gz archive of the artifact directory
func (g *Generator) Compress(archivePath string) error {
	// Use 1GB chunk size to ensure a single part for standard compression
	chunks, err := g.CompressChunked(archivePath, 1024*1024*1024)
	if err != nil {
		return err
	}

	if len(chunks) == 1 {
		// Rename the first chunk to the target archive path
		return os.Rename(chunks[0], archivePath)
	}

	return nil
}

// chunkWriter is a custom writer that splits output into chunks
type chunkWriter struct {
	basePath  string
	chunkSize int64
	current   *os.File
	currentID int
	totalSize int64
	bar       *progressbar.ProgressBar
}

func (cw *chunkWriter) Write(p []byte) (n int, err error) {
	if cw.current == nil {
		if err := cw.nextChunk(); err != nil {
			return 0, err
		}
	}

	remaining := cw.chunkSize - cw.totalSize
	if int64(len(p)) <= remaining {
		n, err = cw.current.Write(p)
		cw.totalSize += int64(n)
		return n, err
	}

	// Write what fits
	n1, err := cw.current.Write(p[:remaining])
	if err != nil {
		return n1, err
	}
	cw.totalSize += int64(n1)

	// Switch to next chunk
	if err := cw.nextChunk(); err != nil {
		return n1, err
	}

	// Write the rest (recursively if needed for very large p)
	n2, err := cw.Write(p[remaining:])
	return n1 + n2, err
}

func (cw *chunkWriter) nextChunk() error {
	if cw.current != nil {
		cw.current.Close()
	}

	cw.currentID++
	cw.totalSize = 0
	path := fmt.Sprintf("%s.%03d", cw.basePath, cw.currentID)
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	cw.current = f
	return nil
}

func (cw *chunkWriter) Close() error {
	if cw.current != nil {
		return cw.current.Close()
	}
	return nil
}

func (cw *chunkWriter) ChunkPaths() []string {
	paths := []string{}
	for i := 1; i <= cw.currentID; i++ {
		paths = append(paths, fmt.Sprintf("%s.%03d", cw.basePath, i))
	}
	return paths
}

// CompressChunked creates a multi-part .tar.gz archive of the artifact directory
func (g *Generator) CompressChunked(archivePath string, chunkSize int64) ([]string, error) {
	// First, count files for progress bar
	var fileCount int64
	filepath.WalkDir(g.artifactDir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			fileCount++
		}
		return nil
	})

	bar := progressbar.Default(fileCount, "Compressing artifact (chunked)")

	cw := &chunkWriter{
		basePath:  archivePath,
		chunkSize: chunkSize,
		bar:       bar,
	}
	defer cw.Close()

	gw := gzip.NewWriter(cw)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	err := filepath.WalkDir(g.artifactDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			fmt.Printf("[WARN] Skipping path (error): %s - %v\n", path, err)
			return nil
		}

		relPath, err := filepath.Rel(g.artifactDir, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			fmt.Printf("[WARN] Skipping (cannot get info): %s - %v\n", relPath, err)
			return nil
		}

		header := &tar.Header{
			Name:    filepath.ToSlash(relPath),
			ModTime: info.ModTime(),
			Size:    info.Size(),
		}

		isSymlink := info.Mode()&os.ModeSymlink != 0
		if !isSymlink && info.Mode()&os.ModeIrregular != 0 {
			if _, err := os.Readlink(path); err == nil {
				isSymlink = true
			}
		}

		if isSymlink {
			linkTarget, _ := os.Readlink(path)
			if filepath.IsAbs(linkTarget) {
				if relTarget, err := filepath.Rel(g.artifactDir, linkTarget); err == nil {
					if !strings.HasPrefix(relTarget, ".."+string(filepath.Separator)) && relTarget != ".." {
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
			header.Mode = 0775
		} else {
			header.Typeflag = tar.TypeReg
			header.Mode = 0774
		}

		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write header for %s: %w", relPath, err)
		}

		if header.Typeflag == tar.TypeReg {
			f, err := os.Open(path)
			if err != nil {
				fmt.Printf("[WARN] Skipping file (cannot open): %s - %v\n", relPath, err)
				return nil
			}
			defer f.Close()

			if _, err = io.Copy(tw, f); err != nil {
				return fmt.Errorf("failed to copy content for %s: %w", relPath, err)
			}
			bar.Add(1)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Close tar and gzip before returning paths to ensure flushing
	tw.Close()
	gw.Close()
	cw.Close()

	return cw.ChunkPaths(), nil
}
