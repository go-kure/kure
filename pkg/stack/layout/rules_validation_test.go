package layout_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// invalidRules are rules LayoutRules.Validate refuses, each with the field
// and the value its error names.
func invalidRules() map[string]struct {
	rules        layout.LayoutRules
	field, value string
} {
	type tc = struct {
		rules        layout.LayoutRules
		field, value string
	}
	return map[string]tc{
		"node grouping":        {layout.LayoutRules{NodeGrouping: "nested"}, "NodeGrouping", "nested"},
		"bundle grouping":      {layout.LayoutRules{BundleGrouping: "nested"}, "BundleGrouping", "nested"},
		"application grouping": {layout.LayoutRules{ApplicationGrouping: "nested"}, "ApplicationGrouping", "nested"},
		"file per":             {layout.LayoutRules{FilePer: "namespace"}, "FilePer", "namespace"},
		"flux placement":       {layout.LayoutRules{FluxPlacement: "inline"}, "FluxPlacement", "inline"},
		"file naming":          {layout.LayoutRules{FileNaming: "name-kind"}, "FileNaming", "name-kind"},
		"cluster name parent":  {layout.LayoutRules{ClusterName: "../prod"}, "ClusterName", "../prod"},
		"cluster name inside":  {layout.LayoutRules{ClusterName: "clusters/../prod"}, "ClusterName", "clusters/../prod"},
		"cluster name dot-dot": {layout.LayoutRules{ClusterName: ".."}, "ClusterName", ".."},
		// The walk would clean this one to "platform" and elide the wrapper,
		// so the writers would accept the tree. It is refused all the same:
		// one rule, no ".." segment.
		"cluster name cleaned away": {layout.LayoutRules{ClusterName: "x/../platform"}, "ClusterName", "x/../platform"},
	}
}

// TestLayoutRules_Validate_ClusterName pins the one ClusterName the rules
// refuse: a ".." path segment, which is what the writers refuse in a layout's
// directory. Every other spelling is accepted.
func TestLayoutRules_Validate_ClusterName(t *testing.T) {
	for _, name := range []string{"", ".", "prod", "clusters/prod", "clusters/prod/", "./prod", "/prod", "a..b", "..prod", "prod.."} {
		if err := (layout.LayoutRules{ClusterName: name}).Validate(); err != nil {
			t.Errorf("ClusterName %q refused: %v", name, err)
		}
	}
	for name, tc := range invalidRules() {
		err := tc.rules.Validate()
		if err == nil || !strings.Contains(err.Error(), tc.field) || !strings.Contains(err.Error(), tc.value) {
			t.Errorf("%s: Validate = %v, want an error naming %s and %q", name, err, tc.field, tc.value)
		}
	}
}

// TestWalk_RefusesInvalidRules pins that both walks validate the rules they are
// given before they build anything (go-kure/kure#979): an unknown option value
// is not walked as if it were another one, and a ClusterName no writer would
// write is refused here instead of at write.
func TestWalk_RefusesInvalidRules(t *testing.T) {
	walks := map[string]func(*stack.Cluster, layout.LayoutRules) (bool, error){
		"WalkCluster": func(c *stack.Cluster, r layout.LayoutRules) (bool, error) {
			ml, err := layout.WalkCluster(c, r)
			return ml != nil, err
		},
		"WalkClusterByPackage": func(c *stack.Cluster, r layout.LayoutRules) (bool, error) {
			pkgs, err := layout.WalkClusterByPackage(c, r)
			return pkgs != nil, err
		},
	}
	for walkName, walk := range walks {
		for name, tc := range invalidRules() {
			t.Run(walkName+"/"+name, func(t *testing.T) {
				built, err := walk(twoTier("platform", "web-bundle"), tc.rules)
				if err == nil || !strings.Contains(err.Error(), tc.field) || !strings.Contains(err.Error(), tc.value) {
					t.Fatalf("err = %v, want one naming %s and %q", err, tc.field, tc.value)
				}
				if built {
					t.Error("a layout was returned with the error")
				}
			})
		}
		// The rules are wrong whatever the cluster is.
		t.Run(walkName+"/nil cluster", func(t *testing.T) {
			if _, err := walk(nil, layout.LayoutRules{NodeGrouping: "nested"}); err == nil || !strings.Contains(err.Error(), "NodeGrouping") {
				t.Errorf("err = %v, want the rules error", err)
			}
		})
	}
}

// TestWalk_AcceptedClusterNameIsWritten pins the other half: a ClusterName the
// rules accept is one every writer writes, to disk and to tar alike.
func TestWalk_AcceptedClusterNameIsWritten(t *testing.T) {
	for _, name := range []string{".", "prod", "clusters/prod", "clusters/prod/", "./prod", "/prod", "a..b"} {
		t.Run(name, func(t *testing.T) {
			build := func() *layout.ManifestLayout {
				ml, err := layout.WalkCluster(twoTier("platform", "web-bundle"), layout.LayoutRules{ClusterName: name})
				if err != nil {
					t.Fatalf("WalkCluster: %v", err)
				}
				return ml
			}
			dir := t.TempDir()
			if err := build().WriteToDisk(filepath.Join(dir, "disk")); err != nil {
				t.Errorf("WriteToDisk: %v", err)
			}
			if err := layout.WriteManifest(filepath.Join(dir, "manifest"), layout.DefaultLayoutConfig(), build()); err != nil {
				t.Errorf("WriteManifest: %v", err)
			}
			var tarball strings.Builder
			if err := build().WriteToTar(&tarball); err != nil {
				t.Fatalf("WriteToTar: %v", err)
			}
			onDisk := map[string]bool{}
			root := filepath.Join(dir, "disk")
			err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				rel, err := filepath.Rel(root, p)
				onDisk[filepath.ToSlash(rel)] = true
				return err
			})
			if err != nil {
				t.Fatalf("read disk tree: %v", err)
			}
			inTar := map[string]bool{}
			for _, n := range collectTarNames(t, strings.NewReader(tarball.String())) {
				if !strings.HasSuffix(n, "/") {
					inTar[strings.TrimPrefix(n, "./")] = true
				}
			}
			if len(onDisk) == 0 {
				t.Fatal("nothing written to disk")
			}
			for f := range onDisk {
				if !inTar[f] {
					t.Errorf("%s is on disk and not in the tar (%v)", f, inTar)
				}
			}
			for f := range inTar {
				if !onDisk[f] {
					t.Errorf("%s is in the tar and not on disk (%v)", f, onDisk)
				}
			}
		})
	}
}
