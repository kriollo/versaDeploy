package changeset

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/user/versaDeploy/internal/state"
)

// ChangeSet represents detected changes
type ChangeSet struct {
	PHPFiles        []string
	TwigFiles       []string
	GoFiles         []string
	FrontendFiles   []string
	ComposerChanged bool
	PackageChanged  bool
	GoModChanged    bool
	RoutesChanged   bool
	OtherFiles      []string          // Files not categorized as PHP, Go, or Frontend
	AllFileHashes   map[string]string // All current file hashes
	ComposerHash    string
	PackageHash     string
	GoModHash       string
	Force           bool // If true, ignore change detection and force full build
}

// Detector handles change detection
type Detector struct {
	repoPath     string
	ignoredPaths []string
	routeFiles   []string
	phpRoot      string
	goRoot       string
	frontendRoot string
	previousLock *state.DeployLock
}

// NewDetector creates a new change detector
func NewDetector(repoPath string, ignoredPaths, routeFiles []string, phpRoot, goRoot, frontendRoot string, previousLock *state.DeployLock) *Detector {
	return &Detector{
		repoPath:     repoPath,
		ignoredPaths: ignoredPaths,
		routeFiles:   routeFiles,
		phpRoot:      phpRoot,
		goRoot:       goRoot,
		frontendRoot: frontendRoot,
		previousLock: previousLock,
	}
}

// Detect calculates hashes and generates a ChangeSet
func (d *Detector) Detect() (*ChangeSet, error) {
	cs := &ChangeSet{
		PHPFiles:      []string{},
		TwigFiles:     []string{},
		GoFiles:       []string{},
		FrontendFiles: []string{},
		OtherFiles:    []string{},
		AllFileHashes: make(map[string]string),
	}

	// Collect all files to hash
	type fileToHash struct {
		path    string
		relPath string
		ext     string
	}

	var filesToHash []fileToHash
	var mu sync.Mutex

	// Walk the repository and collect files
	err := filepath.Walk(d.repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(d.repoPath, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		// 1. Hard-skip truly heavy/metadata directories that we NEVER want to walk
		if info.IsDir() {
			if relPath == ".git" || relPath == "node_modules" || relPath == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		// 2. For files, determine if we should skip hashing
		// We only skip if it's in ignoredPaths AND it's NOT a file type that triggers a build
		ignored := d.shouldIgnore(relPath)
		ext := strings.ToLower(filepath.Ext(relPath))
		isCritical := false

		// Check if it's a critical extension that might trigger a build
		switch ext {
		case ".php", ".twig", ".go", ".mod", ".sum", ".js", ".ts", ".vue", ".jsx", ".tsx", ".css", ".scss", ".sass", ".less":
			isCritical = true
		case ".json":
			base := filepath.Base(relPath)
			if base == "composer.json" || base == "package.json" || base == "composer.lock" || base == "package-lock.json" || base == "pnpm-lock.yaml" {
				isCritical = true
			}
		}

		if ignored && !isCritical {
			return nil
		}

		// Add to list for concurrent hashing
		mu.Lock()
		filesToHash = append(filesToHash, fileToHash{
			path:    path,
			relPath: relPath,
			ext:     ext,
		})
		mu.Unlock()

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk repository: %w", err)
	}

	// Concurrent hashing with worker pool
	numWorkers := runtime.NumCPU() * 2
	if numWorkers > len(filesToHash) {
		numWorkers = len(filesToHash)
	}
	if numWorkers < 1 {
		numWorkers = 1
	}

	type hashResult struct {
		relPath string
		hash    string
		ext     string
		err     error
	}

	jobs := make(chan fileToHash, len(filesToHash))
	results := make(chan hashResult, len(filesToHash))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range jobs {
				hash, err := hashFile(file.path)
				results <- hashResult{
					relPath: file.relPath,
					hash:    hash,
					ext:     file.ext,
					err:     err,
				}
			}
		}()
	}

	// Send jobs
	for _, file := range filesToHash {
		jobs <- file
	}
	close(jobs)

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	for result := range results {
		if result.err != nil {
			return nil, fmt.Errorf("failed to hash %s: %w", result.relPath, result.err)
		}

		cs.AllFileHashes[result.relPath] = result.hash

		// Check if file changed
		changed := d.isFileChanged(result.relPath, result.hash)

		// Categorize changed files by extension
		if changed {
			switch result.ext {
			case ".php":
				cs.PHPFiles = append(cs.PHPFiles, result.relPath)
			case ".twig":
				cs.TwigFiles = append(cs.TwigFiles, result.relPath)
			case ".go":
				cs.GoFiles = append(cs.GoFiles, result.relPath)
			case ".js", ".vue", ".ts", ".jsx", ".tsx", ".css", ".scss", ".less":
				cs.FrontendFiles = append(cs.FrontendFiles, result.relPath)
			default:
				cs.OtherFiles = append(cs.OtherFiles, result.relPath)
			}

			// Check if route file changed
			for _, rf := range d.routeFiles {
				if result.relPath == rf {
					cs.RoutesChanged = true
					break
				}
			}
		}
	}

	// Check dependency files
	composerPath := filepath.ToSlash(filepath.Join(d.phpRoot, "composer.json"))
	if strings.HasPrefix(composerPath, "./") {
		composerPath = composerPath[2:]
	}
	cs.ComposerHash = cs.AllFileHashes[composerPath]
	if d.previousLock != nil {
		cs.ComposerChanged = cs.ComposerHash != "" && cs.ComposerHash != d.previousLock.LastDeploy.ComposerHash
	} else {
		cs.ComposerChanged = cs.ComposerHash != ""
	}

	packagePath := filepath.ToSlash(filepath.Join(d.frontendRoot, "package.json"))
	if strings.HasPrefix(packagePath, "./") {
		packagePath = packagePath[2:]
	}
	cs.PackageHash = cs.AllFileHashes[packagePath]
	if d.previousLock != nil {
		cs.PackageChanged = cs.PackageHash != "" && cs.PackageHash != d.previousLock.LastDeploy.PackageJSONHash
	} else {
		cs.PackageChanged = cs.PackageHash != ""
	}

	goModPath := filepath.ToSlash(filepath.Join(d.goRoot, "go.mod"))
	if strings.HasPrefix(goModPath, "./") {
		goModPath = goModPath[2:]
	}
	cs.GoModHash = cs.AllFileHashes[goModPath]
	if d.previousLock != nil {
		cs.GoModChanged = cs.GoModHash != "" && cs.GoModHash != d.previousLock.LastDeploy.GoModHash
	} else {
		cs.GoModChanged = cs.GoModHash != ""
	}

	return cs, nil
}

// isFileChanged checks if a file has changed compared to previous deployment
func (d *Detector) isFileChanged(path, currentHash string) bool {
	if d.previousLock == nil {
		return true // First deploy, all files are "changed"
	}

	previousHash, exists := d.previousLock.GetFileHash(path)
	if !exists {
		return true // New file
	}

	return currentHash != previousHash
}

// shouldIgnore checks if a path should be ignored
func (d *Detector) shouldIgnore(path string) bool {
	path = filepath.ToSlash(path)
	for _, ignored := range d.ignoredPaths {
		ignored = filepath.ToSlash(ignored)
		if strings.HasPrefix(path, ignored) || path == ignored {
			return true
		}
	}
	return false
}

// hashFile calculates SHA256 hash of a file
func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("sha256:%x", hash.Sum(nil)), nil
}

// HasChanges returns true if any changes were detected
func (cs *ChangeSet) HasChanges() bool {
	return len(cs.PHPFiles) > 0 ||
		len(cs.TwigFiles) > 0 ||
		len(cs.GoFiles) > 0 ||
		len(cs.FrontendFiles) > 0 ||
		len(cs.OtherFiles) > 0 ||
		cs.ComposerChanged ||
		cs.PackageChanged ||
		cs.GoModChanged
}
