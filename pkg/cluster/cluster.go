package cluster

// NewCluster creates a Cluster with the provided metadata.
func NewCluster(name string, tree *Tree) *Cluster {
	return &Cluster{Name: name, Tree: tree}
}

// Helper getters.
func (c *Cluster) GetName() string { return c.Name }
func (c *Cluster) GetTree() *Tree  { return c.Tree }

// Setters for metadata fields.
func (c *Cluster) SetName(n string) { c.Name = n }
func (c *Cluster) SetTree(t *Tree)  { c.Tree = t }
