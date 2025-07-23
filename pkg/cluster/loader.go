package cluster

import (
	"os"

	"gopkg.in/yaml.v3"
)

// LoadClusterFromYAML reads and parses a cluster configuration from a YAML file.
func LoadClusterFromYAML(configPath string) (*Cluster, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var c Cluster
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}
