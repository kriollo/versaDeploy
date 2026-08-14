package deployer

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/user/versaDeploy/internal/fsutil"
	"golang.org/x/sync/errgroup"
)

// errLocalUnsupported reports that an SSH-only operation was called against a
// local environment, which has no remote server, release history, or symlink
// to act on.
func errLocalUnsupported(op string) error {
	return fmt.Errorf("%s is not supported for local environments (local deploys are a full flat replace with no release history — use a config with 'local: true' only for this)", op)
}

// deployLocal builds the release and writes a full, flat copy of it to
// env.LocalPath, replacing whatever was there before. There is no release
// history, no symlink switching, and no deploy.lock: the output is meant to
// be copied as-is (e.g. via FTP or a hosting file manager) to an environment
// where SSH deploys aren't possible.
func (d *Deployer) deployLocal() error {
	startTime := time.Now()
	d.log.Info("Building local release for %s...", d.envName)

	artifactDir, tmpRepo, releaseVersion, commitHash, _, err := d.buildRelease()
	if err != nil {
		return err
	}
	defer func() {
		os.RemoveAll(tmpRepo)
		os.RemoveAll(artifactDir)
	}()

	d.log.Info("Writing release %s (%s) to %s...", releaseVersion, commitHash[:8], d.env.LocalPath)
	if err := fsutil.CopyDirReplace(artifactDir, d.env.LocalPath); err != nil {
		return fmt.Errorf("failed to write local release: %w", err)
	}

	if err := d.executeLocalPostDeployHooks(); err != nil {
		return err
	}

	d.log.Success("Local deploy finished in %s. Upload the contents of %s/app to your hosting (replacing everything there).", time.Since(startTime).Round(time.Second), d.env.LocalPath)
	return nil
}

// executeLocalPostDeployHooks runs post_deploy hooks locally with the local
// release directory as cwd, since there is no remote to run them against.
func (d *Deployer) executeLocalPostDeployHooks() error {
	if len(d.env.PostDeploy) == 0 {
		return nil
	}

	d.log.Info("Running post_deploy hooks locally...")
	for _, hookConfig := range d.env.PostDeploy {
		if hookConfig.Command != "" {
			if err := d.runLocalHook(hookConfig.Command); err != nil {
				return err
			}
		} else if len(hookConfig.Parallel) > 0 {
			var g errgroup.Group
			for _, h := range hookConfig.Parallel {
				cmd := h
				g.Go(func() error {
					return d.runLocalHook(cmd)
				})
			}
			if err := g.Wait(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *Deployer) runLocalHook(cmd string) error {
	d.log.Info("  Local: %s", cmd)
	var outBuf bytes.Buffer
	c := exec.Command("sh", "-c", cmd)
	c.Dir = filepath.Join(d.env.LocalPath, "app")
	c.Stdout = &outBuf
	c.Stderr = &outBuf
	if err := c.Run(); err != nil {
		d.log.Error("post_deploy hook failed: %s\nOutput: %s", cmd, outBuf.String())
		return fmt.Errorf("post_deploy hook failed: %w", err)
	}
	d.log.Info("  Output: %s", strings.TrimSpace(outBuf.String()))
	return nil
}
