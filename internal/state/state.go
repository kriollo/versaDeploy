package state

import (
	"encoding/json"
	"fmt"
	"time"
)

const LockFileVersion = "1.0"

// DeployLock represents the deploy.lock structure
type DeployLock struct {
	Version    string     `json:"version"`
	LastDeploy DeployInfo `json:"last_deploy"`
}

// DeployInfo holds information about the last deployment
type DeployInfo struct {
	Timestamp       time.Time         `json:"timestamp"`
	CommitHash      string            `json:"commit_hash"`
	ReleaseDir      string            `json:"release_dir"`
	FileHashes      map[string]string `json:"file_hashes"`       // path -> sha256
	ComposerHash    string            `json:"composer_hash"`     // composer.json hash
	PackageJSONHash string            `json:"package_json_hash"` // package.json hash
	GoModHash       string            `json:"go_mod_hash"`       // go.mod hash
}

// New creates a new DeployLock with current deployment info
func New(commitHash, releaseDir string, fileHashes map[string]string, composerHash, packageHash, goModHash string) *DeployLock {
	return &DeployLock{
		Version: LockFileVersion,
		LastDeploy: DeployInfo{
			Timestamp:       time.Now().UTC(),
			CommitHash:      commitHash,
			ReleaseDir:      releaseDir,
			FileHashes:      fileHashes,
			ComposerHash:    composerHash,
			PackageJSONHash: packageHash,
			GoModHash:       goModHash,
		},
	}
}

// Parse parses deploy.lock JSON content
func Parse(data []byte) (*DeployLock, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("deploy.lock is empty")
	}

	var lock DeployLock
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("failed to parse deploy.lock: %w", err)
	}

	if lock.Version != LockFileVersion {
		return nil, fmt.Errorf("unsupported deploy.lock version: %s (expected %s)", lock.Version, LockFileVersion)
	}

	return &lock, nil
}

// ToJSON serializes DeployLock to JSON
func (d *DeployLock) ToJSON() ([]byte, error) {
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to serialize deploy.lock: %w", err)
	}
	return data, nil
}

// GetFileHash retrieves the hash for a specific file from the last deployment
func (d *DeployLock) GetFileHash(path string) (string, bool) {
	hash, exists := d.LastDeploy.FileHashes[path]
	return hash, exists
}

// IsFirstDeploy checks if this is the first deployment (no previous state)
func IsFirstDeploy(lock *DeployLock) bool {
	return lock == nil || lock.LastDeploy.FileHashes == nil || len(lock.LastDeploy.FileHashes) == 0
}
