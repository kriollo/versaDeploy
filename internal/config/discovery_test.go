package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Any *deploy*.yml / .yaml is found (deploy.docker.yml, versa_deploy.yml…); other files aren't.
func TestFindConfigFiles(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"deploy.yml", "deploy.docker.yml", "deploy_staging.yml", "api_deploy.yaml", "versa_deploy_front.yml", "docker-compose.yml", "deploy.lock", "deploy.json"} {
		os.WriteFile(filepath.Join(dir, f), nil, 0o644)
	}
	files, err := FindConfigFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range files {
		got = append(got, filepath.Base(f))
	}
	slices.Sort(got)
	if want := []string{"api_deploy.yaml", "deploy.docker.yml", "deploy.yml", "deploy_staging.yml", "versa_deploy_front.yml"}; !slices.Equal(got, want) {
		t.Errorf("found %v, want %v", got, want)
	}
}
