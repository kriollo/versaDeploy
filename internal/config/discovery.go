package config

import (
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
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

// EnvironmentNames returns the sorted environment names of a config file, without
// interpolating or validating it: the TUI's sidebar lists files that may not load yet.
func EnvironmentNames(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Environments map[string]yaml.Node `yaml:"environments"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(doc.Environments))
	for name := range doc.Environments {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}
