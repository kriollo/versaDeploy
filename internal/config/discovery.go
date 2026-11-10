package config

import (
	"path/filepath"
)

// FindConfigFiles looks for *deploy*.yml and *deploy*.yaml in the given directory: deploy.yml,
// deploy.docker.yml, deploy_staging.yml, versa_deploy.yml, api_deploy.yml…
func FindConfigFiles(dir string) ([]string, error) {
	var matches []string
	for _, pattern := range []string{"*deploy*.yml", "*deploy*.yaml"} {
		files, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			return nil, err
		}
		matches = append(matches, files...)
	}
	return matches, nil
}
