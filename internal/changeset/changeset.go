package changeset

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/user/versaDeploy/internal/state"
)

// ChangeSet represents detected changes
type ChangeSet struct {
	PHPFiles            []string
	TwigFiles           []string
	GoFiles             []string
	FrontendFiles       []string
	PythonFiles         []string
	ComposerChanged     bool
	PackageChanged      bool
	GoModChanged        bool
	RequirementsChanged bool
	RoutesChanged       bool
	OtherFiles          []string          // Files not categorized as PHP, Go, or Frontend
	AllFileHashes       map[string]string // All current file hashes
	ComposerHash        string
	PackageHash         string
	GoModHash           string
	RequirementsHash    string
	Force               bool // If true, ignore change detection and force full build
}

// Detector handles change detection
type Detector struct {
	repoPath         string
	ignoredPaths     []string
	ignoredMap       map[string]struct{} // exact-match set for O(1) lookup
	routeFiles       []string
	phpRoot          string
	goRoot           string
	frontendRoot     string
	pythonRoot       string
	requirementsFile string
	previousLock     *state.DeployLock
}

// NewDetector creates a new change detector
func NewDetector(repoPath string, ignoredPaths, routeFiles []string, phpRoot, goRoot, frontendRoot, pythonRoot, requirementsFile string, previousLock *state.DeployLock) *Detector {
	// Pre-normalize ignored paths once and build a map for O(1) exact-match lookups
	normalized := make([]string, len(ignoredPaths))
	ignoredMap := make(map[string]struct{}, len(ignoredPaths))
	for i, p := range ignoredPaths {
		n := filepath.ToSlash(p)
		normalized[i] = n
		ignoredMap[n] = struct{}{}
	}

	return &Detector{
		repoPath:         repoPath,
		ignoredPaths:     normalized,
		ignoredMap:       ignoredMap,
		routeFiles:       routeFiles,
		phpRoot:          phpRoot,
		goRoot:           goRoot,
		frontendRoot:     frontendRoot,
		pythonRoot:       pythonRoot,
		requirementsFile: requirementsFile,
		previousLock:     previousLock,
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

	filesToHash := make([]fileToHash, 0, 512)
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
		case ".php", ".twig", ".go", ".mod", ".sum", ".js", ".ts", ".vue", ".jsx", ".tsx", ".css", ".scss", ".sass", ".less", ".py":
			isCritical = true
		case ".json":
			base := filepath.Base(relPath)
			if base == "composer.json" || base == "package.json" || base == "composer.lock" || base == "package-lock.json" || base == "pnpm-lock.yaml" || base == "pyproject.toml" || base == "poetry.lock" {
				isCritical = true
			}
		case ".txt":
			base := filepath.Base(relPath)
			if base == "requirements.txt" || base == "Pipfile" {
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

	bufSize := len(filesToHash)
	if bufSize > 1000 {
		bufSize = 1000
	}
	jobs := make(chan fileToHash, bufSize)
	results := make(chan hashResult, bufSize)

	// Start workers
	const hashTimeout = 30 * time.Second
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range jobs {
				ctx, cancel := context.WithTimeout(context.Background(), hashTimeout)
				hash, err := hashFileCtx(ctx, file.path)
				cancel()
				results <- hashResult{
					relPath: file.relPath,
					hash:    hash,
					ext:     file.ext,
					err:     err,
				}
			}
		}()
	}

	// Send jobs in goroutine to avoid blocking when buffer is smaller than filesToHash
	go func() {
		for _, file := range filesToHash {
			jobs <- file
		}
		close(jobs)
	}()

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
			case ".py":
				cs.PythonFiles = append(cs.PythonFiles, result.relPath)
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
	composerPath = strings.TrimPrefix(composerPath, "./")
	cs.ComposerHash = cs.AllFileHashes[composerPath]
	if d.previousLock != nil {
		cs.ComposerChanged = cs.ComposerHash != "" && cs.ComposerHash != d.previousLock.LastDeploy.ComposerHash
	} else {
		cs.ComposerChanged = cs.ComposerHash != ""
	}

	packagePath := filepath.ToSlash(filepath.Join(d.frontendRoot, "package.json"))
	packagePath = strings.TrimPrefix(packagePath, "./")
	cs.PackageHash = cs.AllFileHashes[packagePath]
	if d.previousLock != nil {
		cs.PackageChanged = cs.PackageHash != "" && cs.PackageHash != d.previousLock.LastDeploy.PackageJSONHash
	} else {
		cs.PackageChanged = cs.PackageHash != ""
	}

	goModPath := filepath.ToSlash(filepath.Join(d.goRoot, "go.mod"))
	goModPath = strings.TrimPrefix(goModPath, "./")
	cs.GoModHash = cs.AllFileHashes[goModPath]
	if d.previousLock != nil {
		cs.GoModChanged = cs.GoModHash != "" && cs.GoModHash != d.previousLock.LastDeploy.GoModHash
	} else {
		cs.GoModChanged = cs.GoModHash != ""
	}

	// Check Python dependency files
	requirementsPath := filepath.ToSlash(filepath.Join(d.pythonRoot, d.requirementsFile))
	requirementsPath = strings.TrimPrefix(requirementsPath, "./")
	cs.RequirementsHash = cs.AllFileHashes[requirementsPath]
	if d.previousLock != nil {
		cs.RequirementsChanged = cs.RequirementsHash != "" && cs.RequirementsHash != d.previousLock.LastDeploy.RequirementsHash
	} else {
		cs.RequirementsChanged = cs.RequirementsHash != ""
	}

	// Also check pyproject.toml and Pipfile
	pyprojectPath := filepath.ToSlash(filepath.Join(d.pythonRoot, "pyproject.toml"))
	if _, ok := cs.AllFileHashes[pyprojectPath]; ok {
		cs.RequirementsChanged = true
	}
	pipfilePath := filepath.ToSlash(filepath.Join(d.pythonRoot, "Pipfile"))
	if _, ok := cs.AllFileHashes[pipfilePath]; ok {
		cs.RequirementsChanged = true
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
	// paths in ignoredPaths are already normalized in NewDetector
	if _, ok := d.ignoredMap[path]; ok {
		return true
	}
	for _, ignored := range d.ignoredPaths {
		if strings.HasPrefix(path, ignored+"/") {
			return true
		}
	}
	return false
}

// hashFileCtx calculates SHA256 hash of a file, respecting context cancellation/timeout.
// If the context expires while hashing, the function returns ctx.Err().
func hashFileCtx(ctx context.Context, path string) (string, error) {
	type result struct {
		hash string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		h, e := hashFile(path)
		ch <- result{h, e}
	}()
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("hashing %s: %w", path, ctx.Err())
	case r := <-ch:
		return r.hash, r.err
	}
}

// hashFile calculates SHA256 hash of a file efficiently using a fixed buffer
func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()

	// Create a 32KB buffer to avoid massive memory allocations during io.Copy
	// for very large files (e.g. video files, large datasets)
	buf := make([]byte, 32*1024)
	if _, err := io.CopyBuffer(hash, file, buf); err != nil {
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
		len(cs.PythonFiles) > 0 ||
		len(cs.OtherFiles) > 0 ||
		cs.ComposerChanged ||
		cs.PackageChanged ||
		cs.GoModChanged ||
		cs.RequirementsChanged
}
