package cluster

import (
	"github.com/go-kure/kure/pkg/application"
	"github.com/go-kure/kure/pkg/fluxcd"
)

// ClusterConfig is the root configuration for a cluster layout.
type ClusterConfig struct {
	Name              string                      `yaml:"name"`
	Interval          string                      `yaml:"interval"`
	SourceRef         string                      `yaml:"sourceRef"`
	OCIRepo           *fluxcd.OCIRepositoryConfig `yaml:"ociRepo,omitempty"`
	ApplicationGroups []application.Group         `yaml:"appGroups,omitempty"`
}
