package changeset

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

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

	// Walk the repository and hash all files
	err := filepath.Walk(d.repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			// Check if this directory should be ignored
			relPath, _ := filepath.Rel(d.repoPath, path)
			if d.shouldIgnore(relPath) {
				return filepath.SkipDir
			}
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(d.repoPath, path)
		if err != nil {
			return err
		}

		// Skip ignored paths
		if d.shouldIgnore(relPath) {
			return nil
		}

		// Calculate hash
		hash, err := hashFile(path)
		if err != nil {
			return fmt.Errorf("failed to hash %s: %w", relPath, err)
		}

		// Normalize path separators to forward slashes for consistency
		relPath = filepath.ToSlash(relPath)
		cs.AllFileHashes[relPath] = hash

		// Check if file changed
		changed := d.isFileChanged(relPath, hash)

		// Categorize changed files by extension
		if changed {
			ext := strings.ToLower(filepath.Ext(relPath))
			switch ext {
			case ".php":
				cs.PHPFiles = append(cs.PHPFiles, relPath)
			case ".twig":
				cs.TwigFiles = append(cs.TwigFiles, relPath)
			case ".go":
				cs.GoFiles = append(cs.GoFiles, relPath)
			case ".js", ".vue", ".ts", ".jsx", ".tsx", ".css", ".scss", ".less":
				cs.FrontendFiles = append(cs.FrontendFiles, relPath)
			default:
				cs.OtherFiles = append(cs.OtherFiles, relPath)
			}

			// Check if route file changed
			for _, rf := range d.routeFiles {
				if relPath == rf {
					cs.RoutesChanged = true
					break
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk repository: %w", err)
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
