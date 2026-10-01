package version

// Version is the current version of versaDeploy. Overridden at build time via
// -ldflags "-X github.com/user/versaDeploy/internal/version.Version=vX.Y.Z"
// in the release workflow.
var Version = "1.6.1"
