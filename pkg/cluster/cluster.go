package cluster

import (
	"github.com/go-kure/kure/pkg/api"
	"github.com/go-kure/kure/pkg/cluster/layout"
)

// LayoutRules control how layouts are generated.
type LayoutRules struct {
	// FilePer sets the default file export mode for resources.
	FilePer api.FileExportMode
}

// AppSet represents a group of applications that share common settings.
type AppSet struct {
	api.AppGroup `yaml:",inline"`
}

// Cluster describes a target cluster configuration.
type Cluster struct {
	Name      string                   `yaml:"name"`
	Interval  string                   `yaml:"interval"`
	SourceRef string                   `yaml:"sourceRef"`
	FilePer   api.FileExportMode       `yaml:"filePer,omitempty"`
	OCIRepo   *api.OCIRepositoryConfig `yaml:"ociRepo,omitempty"`
	AppSets   []AppSet                 `yaml:"appGroups,omitempty"`
}

// NewCluster creates a Cluster with the provided metadata.
func NewCluster(name, interval, sourceRef string, repo *api.OCIRepositoryConfig) *Cluster {
	return &Cluster{Name: name, Interval: interval, SourceRef: sourceRef, OCIRepo: repo}
}

// AddAppSet appends an application set to the cluster.
func (c *Cluster) AddAppSet(set AppSet) { c.AppSets = append(c.AppSets, set) }

// SetOCIRepository sets the OCI repository configuration.
func (c *Cluster) SetOCIRepository(repo *api.OCIRepositoryConfig) { c.OCIRepo = repo }

// BuildLayout generates manifest and Flux layouts for the cluster.
func (c *Cluster) BuildLayout(r LayoutRules) ([]*layout.ManifestLayout, []*layout.FluxLayout, *layout.FluxLayout, error) {
	if r.FilePer == "" {
		r.FilePer = c.FilePer
	}

	var manifests []*layout.ManifestLayout
	var fluxes []*layout.FluxLayout

	for _, set := range c.AppSets {
		group := set.AppGroup
		if r.FilePer != "" && group.FilePer == "" {
			group.FilePer = r.FilePer
		}
		manifest, flux, err := layout.NewAppGroup(group)
		if err != nil {
			return nil, nil, nil, err
		}
		manifests = append(manifests, manifest)
		fluxes = append(fluxes, flux)
	}

	bootstrapFlux, err := layout.NewFluxBootstrap(c.Name, c.SourceRef, c.Interval, "flux-system")
	if err != nil {
		return nil, nil, nil, err
	}
	return manifests, fluxes, bootstrapFlux, nil
}

// Helper getters.
func (c *Cluster) GetName() string                            { return c.Name }
func (c *Cluster) GetInterval() string                        { return c.Interval }
func (c *Cluster) GetSourceRef() string                       { return c.SourceRef }
func (c *Cluster) GetOCIRepository() *api.OCIRepositoryConfig { return c.OCIRepo }
func (c *Cluster) GetAppSets() []AppSet                       { return c.AppSets }

// Setters for metadata fields.
func (c *Cluster) SetName(n string)                { c.Name = n }
func (c *Cluster) SetInterval(i string)            { c.Interval = i }
func (c *Cluster) SetSourceRef(s string)           { c.SourceRef = s }
func (c *Cluster) SetFilePer(f api.FileExportMode) { c.FilePer = f }
