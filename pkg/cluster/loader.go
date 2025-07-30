package cluster

import (
	"github.com/go-kure/kure/pkg/kio"
)

// LoadClusterFromYAML reads and parses a cluster configuration from a YAML file.
func LoadClusterFromYAML(configPath string) (*Cluster, error) {
	var c Cluster
	if err := kio.LoadFile(configPath, &c); err != nil {
		return nil, err
	}
	return &c, nil
}
