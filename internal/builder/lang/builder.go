package lang

import (
	"github.com/user/versaDeploy/internal/changeset"
	"github.com/user/versaDeploy/internal/config"
	"github.com/user/versaDeploy/internal/logger"
)

// BuilderContext holds the shared state needed by all language builders
type BuilderContext struct {
	RepoPath    string
	ArtifactDir string
	Config      *config.Environment
	Changeset   *changeset.ChangeSet
	Log         *logger.Logger
}

// LanguageBuilder defines the interface for language-specific build strategies
type LanguageBuilder interface {
	// Build executes the build process for a specific language
	// returns: filesProcessed, isRebuilt/isUpdated, error
	Build(ctx *BuilderContext) (int, bool, error)
}
