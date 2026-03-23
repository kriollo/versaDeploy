package builder

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/user/versaDeploy/internal/builder/lang"
	"github.com/user/versaDeploy/internal/changeset"
	"github.com/user/versaDeploy/internal/config"
	"github.com/user/versaDeploy/internal/fsutil"
	"github.com/user/versaDeploy/internal/logger"
	"golang.org/x/sync/errgroup"
)

// BuildResult tracks what was built
type BuildResult struct {
	PHPFilesChanged      int
	GoBinaryRebuilt      bool
	FrontendCompiled     int
	PythonFilesBuilt     int
	ComposerUpdated      bool
	NPMUpdated           bool
	PipUpdated           bool
	TwigCacheCleanup     bool
	RouteCacheRegenerate bool
}

// Builder orchestrates all build operations
type Builder struct {
	repoPath    string
	artifactDir string
	config      *config.Environment
	changeset   *changeset.ChangeSet
	result      *BuildResult
	log         *logger.Logger
}

// NewBuilder creates a new builder
func NewBuilder(repoPath, artifactDir string, cfg *config.Environment, cs *changeset.ChangeSet, log *logger.Logger) *Builder {
	return &Builder{
		repoPath:    repoPath,
		artifactDir: artifactDir,
		config:      cfg,
		changeset:   cs,
		result:      &BuildResult{},
		log:         log,
	}
}

// Build executes all necessary builds based on the changeset
func (b *Builder) Build() (*BuildResult, error) {
	// Step 1: Copy entire repository to app/ directory (including ignored paths for build)
	b.log.Info("Copying project files to artifact...")
	if err := b.copyEntireRepo(); err != nil {
		return nil, fmt.Errorf("failed to copy repository: %w", err)
	}

	// Step 2-4: Build PHP, Go, and Frontend concurrently
	b.log.Info("Running builds concurrently...")

	// Create context for language builders
	buildCtx := &lang.BuilderContext{
		RepoPath:    b.repoPath,
		ArtifactDir: b.artifactDir,
		Config:      b.config,
		Changeset:   b.changeset,
		Log:         b.log,
	}

	var g errgroup.Group

	if b.config.Builds.PHP.Enabled {
		g.Go(func() error {
			builder := &lang.PHPBuilder{}
			count, updated, err := builder.Build(buildCtx)
			if err != nil {
				return err
			}
			b.result.PHPFilesChanged = count
			b.result.ComposerUpdated = updated
			b.result.TwigCacheCleanup = len(b.changeset.TwigFiles) > 0
			b.result.RouteCacheRegenerate = b.changeset.RoutesChanged
			return nil
		})
	}

	if b.config.Builds.Go.Enabled {
		g.Go(func() error {
			builder := &lang.GoBuilder{}
			_, updated, err := builder.Build(buildCtx)
			if err != nil {
				return err
			}
			b.result.GoBinaryRebuilt = updated
			return nil
		})
	}

	if b.config.Builds.Frontend.Enabled {
		g.Go(func() error {
			builder := &lang.FrontendBuilder{}
			count, updated, err := builder.Build(buildCtx)
			if err != nil {
				return err
			}
			b.result.FrontendCompiled = count
			b.result.NPMUpdated = updated
			return nil
		})
	}

	if b.config.Builds.Python.Enabled {
		g.Go(func() error {
			builder := &lang.PythonBuilder{}
			count, updated, err := builder.Build(buildCtx)
			if err != nil {
				return err
			}
			b.result.PythonFilesBuilt = count
			b.result.PipUpdated = updated
			return nil
		})
	}

	// Wait for all builds to complete
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Step 5: Cleanup ignored paths after builds complete
	b.log.Info("Cleaning up build-time dependencies...")
	if err := b.cleanupIgnoredPaths(); err != nil {
		return nil, fmt.Errorf("failed to cleanup ignored paths: %w", err)
	}

	return b.result, nil
}

// copyEntireRepo copies the entire repository to app/ directory (including ignored paths for build).
// Directories are created sequentially (to satisfy parent-before-child ordering), then files are
// copied in parallel using a worker pool of runtime.NumCPU() goroutines.
func (b *Builder) copyEntireRepo() error {
	appDir := filepath.Join(b.artifactDir, "app")
	if err := os.MkdirAll(appDir, 0775); err != nil {
		return fmt.Errorf("failed to create app directory: %w", err)
	}

	type filePair struct {
		src string
		dst string
	}

	// Collect files; create directories inline (sequential, preserves order).
	var files []filePair
	err := filepath.Walk(b.repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(b.repoPath, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		if strings.HasPrefix(relPath, ".git") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		dstPath := filepath.Join(appDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		files = append(files, filePair{src: path, dst: dstPath})
		return nil
	})
	if err != nil {
		return err
	}

	// Copy files in parallel.
	numWorkers := runtime.NumCPU()
	if numWorkers > len(files) {
		numWorkers = len(files)
	}
	if numWorkers < 1 {
		numWorkers = 1
	}

	jobs := make(chan filePair, len(files))
	for _, f := range files {
		jobs <- f
	}
	close(jobs)

	var (
		wg      sync.WaitGroup
		copyErr error
		errMu   sync.Mutex
	)
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range jobs {
				if err := copyFile(f.src, f.dst); err != nil {
					errMu.Lock()
					if copyErr == nil {
						copyErr = err
					}
					errMu.Unlock()
				}
			}
		}()
	}
	wg.Wait()
	return copyErr
}

// cleanupIgnoredPaths removes ignored paths from artifact after builds complete
func (b *Builder) cleanupIgnoredPaths() error {
	appDir := filepath.Join(b.artifactDir, "app")

	for _, ignored := range b.config.Ignored {
		// Normalize to forward slashes for internal matching/lookup
		cleanIgnored := filepath.ToSlash(filepath.Clean(ignored))
		if cleanIgnored == ".git" {
			continue
		}

		ignoredPath := filepath.Join(appDir, cleanIgnored)

		// Check if path exists
		if _, err := os.Stat(ignoredPath); os.IsNotExist(err) {
			b.log.Debug("   Skipping ignored path (not found): %s", cleanIgnored)
			continue
		}

		// Remove the path
		if err := os.RemoveAll(ignoredPath); err != nil {
			return fmt.Errorf("failed to remove ignored path %s: %w", cleanIgnored, err)
		}
		b.log.Debug("   Removed ignored path: %s", cleanIgnored)
	}

	return nil
}

// calculateDirSize calculates the total size of a directory recursively
func (b *Builder) calculateDirSize(path string) (int64, error) {
	return fsutil.CalculateDirSize(path)
}

// executeCommand runs a command in a shell based on the current OS
func (b *Builder) executeCommand(command, dir string) ([]byte, error) {
	var shell, flag string
	if runtime.GOOS == "windows" {
		shell = os.Getenv("COMSPEC")
		if shell == "" {
			shell = "cmd.exe"
		}
		flag = "/c"
	} else {
		shell = "sh"
		flag = "-c"
	}

	cmd := exec.Command(shell, flag, command)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

// copyFile copies a single file using io.Copy for efficiency and reliability
func copyFile(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}

	// Double check it's a regular file. We should NEVER try to read directories as files.
	// This prevents "Función incorrecta" errors on Windows for junctions/reparse points.
	if !info.Mode().IsRegular() {
		// If it's a symlink that made it here, evaluate it
		if info.Mode()&os.ModeSymlink != 0 {
			realPath, err := filepath.EvalSymlinks(src)
			if err != nil {
				return nil // Skip if broken
			}
			return copyFile(realPath, dst)
		}
		return nil // Skip non-regular files
	}

	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	// Copy permissions
	os.Chmod(dst, info.Mode())

	return nil
}
