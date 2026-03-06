package config

import (
	"path/filepath"
)

// FindConfigFiles looks for deploy.yml, deploy_*.yml, *.yaml, versa_deploy*.yml in the given directory
func FindConfigFiles(dir string) ([]string, error) {
	patterns := []string{
		"deploy.yml",
		"deploy_*.yml",
		"versa_deploy.yml",
		"versa_deploy_*.yml",
		"deploy.yaml",
		"deploy_*.yaml",
		"versa_deploy.yaml",
		"versa_deploy_*.yaml",
	}

	var matches []string
	seen := make(map[string]bool)

	for _, pattern := range patterns {
		files, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			base := filepath.Base(f)
			if !seen[base] {
				seen[base] = true
				matches = append(matches, f)
			}
		}
	}
	return matches, nil
}
