package cluster

import (
	"github.com/go-kure/kure/pkg/application"
	"github.com/go-kure/kure/pkg/fluxcd"
)

// Cluster describes a target cluster configuration.
type Cluster struct {
	Name             string                         `yaml:"name"`
	Interval         string                         `yaml:"interval"`
	SourceRef        string                         `yaml:"sourceRef"`
	OCIRepo          *fluxcd.OCIRepositoryConfig    `yaml:"ociRepo,omitempty"`
	ApplictionGroups []application.ApplicationGroup `yaml:"appGroups,omitempty"`
}

// NewCluster creates a Cluster with the provided metadata.
func NewCluster(name, interval, sourceRef string, repo *fluxcd.OCIRepositoryConfig) *Cluster {
	return &Cluster{Name: name, Interval: interval, SourceRef: sourceRef, OCIRepo: repo}
}

// AddAppSet appends an application set to the cluster.
func (c *Cluster) AddApplicationGroup(group application.ApplicationGroup) {
	c.ApplictionGroups = append(c.ApplictionGroups, group)
}

// SetOCIRepository sets the OCI repository configuration.
func (c *Cluster) SetOCIRepository(repo *fluxcd.OCIRepositoryConfig) { c.OCIRepo = repo }

// Helper getters.
func (c *Cluster) GetName() string                                      { return c.Name }
func (c *Cluster) GetInterval() string                                  { return c.Interval }
func (c *Cluster) GetSourceRef() string                                 { return c.SourceRef }
func (c *Cluster) GetOCIRepository() *fluxcd.OCIRepositoryConfig        { return c.OCIRepo }
func (c *Cluster) GetApplicationGroups() []application.ApplicationGroup { return c.ApplictionGroups }

// Setters for metadata fields.
func (c *Cluster) SetName(n string)      { c.Name = n }
func (c *Cluster) SetInterval(i string)  { c.Interval = i }
func (c *Cluster) SetSourceRef(s string) { c.SourceRef = s }
