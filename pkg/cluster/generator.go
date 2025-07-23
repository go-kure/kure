package cluster

import "github.com/go-kure/kure/pkg/layout"

// NewClusterLayouts generates the layouts for the given Cluster.
func NewClusterLayouts(c *Cluster) ([]*layout.ManifestLayout, []*layout.FluxLayout, *layout.FluxLayout, error) {

    if c == nil {
        return nil, nil, nil, nil
    }
    return c.BuildLayout(LayoutRules{})
}
