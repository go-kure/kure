package layout_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for layout.TopDirectory (go-kure/kure#979): the directory at the top
// of the tree WalkCluster builds, which is the one a Flux bootstrap is pointed
// at.

// TestTopDirectory pins the directory for each kind of root node, with and
// without a ClusterName.
func TestTopDirectory(t *testing.T) {
	named, unnamed := &stack.Node{Name: "prod"}, &stack.Node{}
	for name, tc := range map[string]struct {
		root        *stack.Node
		clusterName string
		want        string
	}{
		"named root":   {named, "", "prod"},
		"unnamed root": {unnamed, "", "cluster"},
		"no root node": {nil, "", "."},

		"named root under a ClusterName":   {named, "clusters/eu", "clusters/eu"},
		"unnamed root under a ClusterName": {unnamed, "clusters/eu", "clusters/eu"},
		"no root node under a ClusterName": {nil, "clusters/eu", "clusters/eu"},

		// The root node is the cluster directory when the directory's last
		// segment is its name: the top is that directory, not one below it.
		"ClusterName is the root's name":      {named, "prod", "prod"},
		"ClusterName ends in the root's name": {named, "clusters/prod", "clusters/prod"},

		"ClusterName .": {named, ".", "."},

		// A rooted ClusterName is the directory the writers resolve it to,
		// with no leading slash; the walk's own FullRepoPath keeps the slash.
		"rooted ClusterName":                {named, "/rooted", "rooted"},
		"rooted ClusterName, unnamed root":  {unnamed, "/rooted", "rooted"},
		"rooted ClusterName of two":         {named, "/clusters/eu", "clusters/eu"},
		"ClusterName /":                     {named, "/", "."},
		"ClusterName /, no root node":       {nil, "/", "."},
		"rooted ClusterName is root's name": {named, "/prod", "prod"},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := layout.TopDirectory(tc.root, layout.LayoutRules{ClusterName: tc.clusterName})
			if err != nil {
				t.Fatalf("TopDirectory: %v", err)
			}
			if got != tc.want {
				t.Errorf("TopDirectory = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestTopDirectory_RootNodeName: where the root node's name is the directory,
// a name that is no directory name is refused, as WalkCluster refuses the
// cluster, by an error that names the node and the field; nothing is returned
// next to it. Under a ClusterName the name is no part of the directory, and
// the cluster directory is returned for the same root nodes.
func TestTopDirectory_RootNodeName(t *testing.T) {
	for name, reason := range map[string]string{
		"../prod": "path separator",
		"a/b":     "path separator",
		`a\b`:     "path separator",
		".":       "not a directory name of its own",
		"..":      "not a directory name of its own",
	} {
		root := &stack.Node{Name: name}
		t.Run(name+"/no ClusterName", func(t *testing.T) {
			dir, err := layout.TopDirectory(root, layout.LayoutRules{})
			if err == nil {
				t.Fatalf("TopDirectory = %q, want the root node's name refused", dir)
			}
			for _, want := range []string{"Node '" + name + "'", "field 'name'", reason} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("err = %v, want it to contain %q", err, want)
				}
			}
			if dir != "" {
				t.Errorf("TopDirectory returned %q with the error", dir)
			}
			if _, werr := layout.WalkCluster(&stack.Cluster{Name: "demo", Node: root}, layout.LayoutRules{}); werr == nil {
				t.Error("WalkCluster builds a tree for a root node TopDirectory refuses")
			}
		})
		t.Run(name+"/under a ClusterName", func(t *testing.T) {
			dir, err := layout.TopDirectory(root, layout.LayoutRules{ClusterName: "clusters/eu"})
			if err != nil {
				t.Fatalf("TopDirectory: %v", err)
			}
			if dir != "clusters/eu" {
				t.Errorf("TopDirectory = %q, want %q", dir, "clusters/eu")
			}
		})
	}
}

// topDirectoryShapes are clusters whose walked tree has a different top, or a
// top FlattenSingleTier could move: a root node with a bundle and a child node,
// the same without a name, a root node that renders nothing, and one over a
// single child node that renders nothing.
func topDirectoryShapes() map[string]func() *stack.Cluster {
	return map[string]func() *stack.Cluster{
		"named root": func() *stack.Cluster { return twoTier("platform", "web-bundle") },
		"unnamed root": func() *stack.Cluster {
			c := twoTier("platform", "web-bundle")
			c.Node.Name = ""
			return c
		},
		"empty named root": func() *stack.Cluster {
			return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "apps"}}
		},
		"empty unnamed root": func() *stack.Cluster {
			return &stack.Cluster{Name: "demo", Node: &stack.Node{}}
		},
		"root over one empty child": func() *stack.Cluster {
			return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "apps", Children: []*stack.Node{{Name: "only"}}}}
		},
	}
}

// TestTopDirectory_IsTheWalksTop holds TopDirectory to the walk over every
// grouping, every spelling of a ClusterName the rules accept and both values of
// FlattenSingleTier: the tree WalkCluster returns is at the directory
// TopDirectory names. A FlattenSingleTier collapse moves no top: the layout
// that absorbs a collapsed one keeps its own directory.
//
// The walk's FullRepoPath keeps the leading slash of a rooted ClusterName and
// TopDirectory does not, so the slash is dropped from the walk's side here;
// TestTopDirectory pins the rooted values themselves.
func TestTopDirectory_IsTheWalksTop(t *testing.T) {
	groupings := map[string]layout.LayoutRules{"groupByName": groupByName, "nodeOnly": nodeOnly, "nodeFlat": nodeFlat}
	clusterNames := []string{"", ".", "prod", "apps", "platform", "clusters/prod", "clusters/prod/", "clusters/apps", "./prod", "/prod", "a..b"}
	for shape, build := range topDirectoryShapes() {
		for grouping, base := range groupings {
			for _, clusterName := range clusterNames {
				for _, flatten := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%s/ClusterName=%q/flatten=%t", shape, grouping, clusterName, flatten), func(t *testing.T) {
						rules := withClusterName(base, clusterName)
						rules.FlattenSingleTier = flatten
						c := build()

						top, err := layout.TopDirectory(c.Node, rules)
						if err != nil {
							t.Fatalf("TopDirectory: %v", err)
						}
						if got := strings.TrimLeft(walk(t, c, rules).FullRepoPath(), "/"); got != top {
							t.Errorf("the walk's top is %q, TopDirectory says %q", got, top)
						}
					})
				}
			}
		}
	}
}

// TestTopDirectory_RefusesInvalidRules: the rules are validated as the walk
// validates them, whatever the root node is, so a caller cannot be given a
// directory for rules no tree is walked with.
func TestTopDirectory_RefusesInvalidRules(t *testing.T) {
	for name, tc := range invalidRules() {
		for rootName, root := range map[string]*stack.Node{"named root": {Name: "prod"}, "unnamed root": {}, "no root node": nil} {
			t.Run(name+"/"+rootName, func(t *testing.T) {
				dir, err := layout.TopDirectory(root, tc.rules)
				if err == nil || !strings.Contains(err.Error(), tc.field) || !strings.Contains(err.Error(), tc.value) {
					t.Fatalf("err = %v, want one naming %s and %q", err, tc.field, tc.value)
				}
				if dir != "" {
					t.Errorf("TopDirectory returned %q with the error", dir)
				}
			})
		}
	}
}
