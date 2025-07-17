package cluster

import (
	"os"

	"gopkg.in/yaml.v3"

	"github.com/go-kure/kure/pkg/api"
)

// LoadClusterConfigFromYAML reads and parses a cluster configuration from a YAML file
func LoadClusterConfigFromYAML(configPath string) (*api.ClusterConfig, error) {
	// Read the YAML file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	// Parse YAML into ClusterConfig struct
	var config api.ClusterConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}
