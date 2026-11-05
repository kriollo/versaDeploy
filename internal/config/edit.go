package config

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// SaveProvision writes p as the provision block of envName in the YAML file at path,
// keeping comments and the rest of the file. db_password is never written: an existing
// value (usually ${VAR}) is kept as is.
func SaveProvision(path, envName string, p ProvisionConfig) error {
	return editEnv(path, envName, func(env *yaml.Node) error {
		p.DBPassword = ""
		var node yaml.Node
		if err := node.Encode(p); err != nil {
			return err
		}
		if old := mapGet(env, "provision"); old != nil {
			if pw := mapGet(old, "db_password"); pw != nil {
				node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: "db_password"}, pw)
			}
		}
		mapSet(env, "provision", &node)
		return nil
	})
}

// AddServicesReload appends the commands missing from envName's services_reload.
func AddServicesReload(path, envName string, cmds []string) error {
	return editEnv(path, envName, func(env *yaml.Node) error {
		seq := mapGet(env, "services_reload")
		if seq == nil || seq.Kind != yaml.SequenceNode {
			seq = &yaml.Node{Kind: yaml.SequenceNode}
			mapSet(env, "services_reload", seq)
		}
		var have []string
		for _, n := range seq.Content {
			have = append(have, n.Value)
		}
		for _, c := range cmds {
			if !slices.Contains(have, c) {
				seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: c})
			}
		}
		return nil
	})
}

// SetServices writes the services list of envName.
func SetServices(path, envName string, svcs []ServiceConfig) error {
	// only the fields that are set: ServiceConfig has no omitempty
	list := make([]map[string]any, 0, len(svcs))
	for _, sv := range svcs {
		m := map[string]any{"name": sv.Name}
		for k, v := range map[string]string{"exec": sv.Exec, "working_dir": sv.WorkingDir, "user": sv.User, "env_file": sv.EnvFile} {
			if v != "" {
				m[k] = v
			}
		}
		if len(sv.Environment) > 0 {
			m["environment"] = sv.Environment
		}
		list = append(list, m)
	}
	return editEnv(path, envName, func(env *yaml.Node) error {
		var node yaml.Node
		if err := node.Encode(list); err != nil {
			return err
		}
		mapSet(env, "services", &node)
		return nil
	})
}

func editEnv(path, envName string, edit func(env *yaml.Node) error) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("failed to parse %s: %w", path, err)
	}
	if len(doc.Content) == 0 {
		return fmt.Errorf("%s is empty", path)
	}
	env := mapGet(mapGet(doc.Content[0], "environments"), envName)
	if env == nil || env.Kind != yaml.MappingNode {
		return fmt.Errorf("environment %q not found in %s", envName, path)
	}
	if err := edit(env); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(detectIndent(data))
	if err := enc.Encode(&doc); err != nil {
		return err
	}
	enc.Close()
	return os.WriteFile(path, buf.Bytes(), 0644)
}

// detectIndent returns the indentation of the first indented line (default 4).
func detectIndent(data []byte) int {
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimLeft(line, " ")
		if n := len(line) - len(trimmed); n > 0 && trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			return n
		}
	}
	return 4
}

func mapGet(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func mapSet(m *yaml.Node, key string, val *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = val
			return
		}
	}
	m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, val)
}
