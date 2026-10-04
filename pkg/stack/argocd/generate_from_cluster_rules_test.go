package argocd_test

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kerrors "github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/argocd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for GenerateFromCluster walking with the caller's layout rules
// (go-kure/kure#979): every source.path it returns is a directory the same
// rules write, whatever the writer.

// rulesCluster builds a root node (named rootName, which may be empty) with
// one child node, each with a bundle that renders one application.
func rulesCluster(rootName string) *stack.Cluster {
	bundle := func(name string) *stack.Bundle {
		return &stack.Bundle{Name: name, Applications: []*stack.Application{stack.NewApplication(name+"-app", "default", configMapApp{})}}
	}
	web := &stack.Node{Name: "web", Bundle: bundle("web")}
	root := &stack.Node{Name: rootName, Bundle: bundle("platform"), Children: []*stack.Node{web}}
	return &stack.Cluster{Name: "demo", Node: root}
}

// sourcePaths maps every Application in objs to its spec.source.path.
func sourcePaths(t *testing.T, objs []client.Object) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, o := range objs {
		p, _, err := unstructured.NestedString(o.(*unstructured.Unstructured).Object, "spec", "source", "path")
		if err != nil {
			t.Fatal(err)
		}
		out[o.GetName()] = p
	}
	return out
}

// tarFiles returns the name of every file entry of a tar archive.
func tarFiles(t *testing.T, r io.Reader) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return out
		}
		if err != nil {
			t.Fatalf("tar: %v", err)
		}
		if hdr.Typeflag != tar.TypeDir {
			out[path.Clean(hdr.Name)] = true
		}
	}
}

// TestGenerateFromCluster_UsesCallerRules: the source.path of every
// Application is the directory a walk with the same rules writes its bundle
// to, on disk and in a tar archive.
func TestGenerateFromCluster_UsesCallerRules(t *testing.T) {
	withClusterName := func(name string) layout.LayoutRules {
		r := layout.DefaultLayoutRules()
		r.ClusterName = name
		return r
	}
	cases := map[string]struct {
		root  string
		rules layout.LayoutRules
		want  map[string]string
	}{
		// The root node's bundle "platform" has a directory of its own inside
		// the root node's, beside the root's child node.
		"default rules":               {root: "platform", rules: layout.DefaultLayoutRules(), want: map[string]string{"platform": "platform/platform", "web": "platform/web"}},
		"unnamed root, ClusterName .": {root: "", rules: withClusterName("."), want: map[string]string{"platform": "platform", "web": "web"}},
		"ClusterName clusters/prod":   {root: "platform", rules: withClusterName("clusters/prod"), want: map[string]string{"platform": "clusters/prod/platform/platform", "web": "clusters/prod/platform/web"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			objs, err := argocd.Engine().GenerateFromCluster(rulesCluster(tc.root), tc.rules)
			if err != nil {
				t.Fatalf("GenerateFromCluster: %v", err)
			}
			got := sourcePaths(t, objs)
			if len(got) != len(tc.want) {
				t.Fatalf("Application paths = %v, want %v", got, tc.want)
			}
			for app, p := range tc.want {
				if got[app] != p {
					t.Errorf("Application %s has source.path %q, want %q", app, got[app], p)
				}
			}

			// The tree the caller writes with the same rules holds every path.
			ml, err := layout.WalkCluster(rulesCluster(tc.root), tc.rules)
			if err != nil {
				t.Fatalf("WalkCluster: %v", err)
			}
			disk := t.TempDir()
			if err := ml.WriteToDisk(disk); err != nil {
				t.Fatalf("WriteToDisk: %v", err)
			}
			var buf bytes.Buffer
			if err := ml.WriteToTar(&buf); err != nil {
				t.Fatalf("WriteToTar: %v", err)
			}
			archive := tarFiles(t, &buf)
			for app, p := range got {
				if _, err := os.Stat(filepath.Join(disk, p, "kustomization.yaml")); err != nil {
					t.Errorf("WriteToDisk: Application %s has source.path %q, which has no kustomization.yaml", app, p)
				}
				if !archive[path.Join(p, "kustomization.yaml")] {
					t.Errorf("WriteToTar: Application %s has source.path %q, which has no kustomization.yaml", app, p)
				}
			}
			onDisk := 0
			err = filepath.WalkDir(disk, func(p string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				onDisk++
				rel, err := filepath.Rel(disk, p)
				if err != nil {
					return err
				}
				if !archive[filepath.ToSlash(rel)] {
					t.Errorf("%s is on disk and not in the tar archive", rel)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if onDisk == 0 || onDisk != len(archive) {
				t.Errorf("WriteToDisk wrote %d files, WriteToTar %d", onDisk, len(archive))
			}
		})
	}
}

// notLayoutRules satisfies stack.LayoutRulesProvider without being
// layout.LayoutRules.
type notLayoutRules struct{}

func (notLayoutRules) Validate() error { return nil }

// TestGenerateFromCluster_RefusedRules: rules that are not layout.LayoutRules
// (nil included) are refused instead of replaced by defaults, and so is an
// integrated Flux placement, which CreateLayoutWithResources refuses too.
func TestGenerateFromCluster_RefusedRules(t *testing.T) {
	for name, provider := range map[string]stack.LayoutRulesProvider{"nil": nil, "another type": notLayoutRules{}} {
		objs, err := argocd.Engine().GenerateFromCluster(rulesCluster("platform"), provider)
		if err == nil || !strings.Contains(err.Error(), "rules must be of type layout.LayoutRules") || objs != nil {
			t.Errorf("%s rules: got %v, %v; want the rules-type refusal", name, objs, err)
		}
	}
	for _, p := range []layout.FluxPlacement{layout.FluxIntegratedPerLayout, layout.FluxIntegratedPerBundle} {
		rules := layout.DefaultLayoutRules()
		rules.FluxPlacement = p
		objs, err := argocd.Engine().GenerateFromCluster(rulesCluster("platform"), rules)
		if err == nil || !strings.Contains(err.Error(), "FluxPlacement") || objs != nil {
			t.Errorf("FluxPlacement %s: got %v, %v; want a refusal", p, objs, err)
		}
	}
}

// TestGenerateFromCluster_InvalidRules: rules the walk refuses (an unknown
// option value, a ClusterName with a ".." segment) are an error whatever the
// cluster is, an absent or empty one included, as they are for
// CreateLayoutWithResources; the error is the rules' validation error, which
// names the field. With valid rules such a cluster yields nothing.
func TestGenerateFromCluster_InvalidRules(t *testing.T) {
	clusters := map[string]func() *stack.Cluster{
		"cluster with bundles":        func() *stack.Cluster { return rulesCluster("platform") },
		"nil cluster":                 func() *stack.Cluster { return nil },
		"cluster without a root node": func() *stack.Cluster { return &stack.Cluster{Name: "empty"} },
	}
	for field, set := range map[string]func(*layout.LayoutRules){
		"NodeGrouping":        func(r *layout.LayoutRules) { r.NodeGrouping = "sideways" },
		"BundleGrouping":      func(r *layout.LayoutRules) { r.BundleGrouping = "sideways" },
		"ApplicationGrouping": func(r *layout.LayoutRules) { r.ApplicationGrouping = "sideways" },
		"FilePer":             func(r *layout.LayoutRules) { r.FilePer = "sideways" },
		"FluxPlacement":       func(r *layout.LayoutRules) { r.FluxPlacement = "sideways" },
		"FileNaming":          func(r *layout.LayoutRules) { r.FileNaming = "sideways" },
		"ClusterName":         func(r *layout.LayoutRules) { r.ClusterName = "a/../b" },
	} {
		rules := layout.DefaultLayoutRules()
		set(&rules)
		if err := rules.Validate(); err == nil {
			t.Fatalf("%s: the rules are valid, so the case tests nothing", field)
		}
		for clusterName, build := range clusters {
			objs, err := argocd.Engine().GenerateFromCluster(build(), rules)
			var verr *kerrors.ValidationError
			if !errors.As(err, &verr) || verr.Field != field {
				t.Errorf("%s, invalid %s: got %v, want the rules' validation error for that field", clusterName, field, err)
			}
			if objs != nil {
				t.Errorf("%s, invalid %s: got %d objects with the refusal", clusterName, field, len(objs))
			}
			if _, createErr := argocd.Engine().CreateLayoutWithResources(build(), rules); !errors.As(createErr, &verr) || verr.Field != field {
				t.Errorf("%s, invalid %s: CreateLayoutWithResources got %v, want the same refusal", clusterName, field, createErr)
			}
		}
	}

	for clusterName, build := range clusters {
		if clusterName == "cluster with bundles" {
			continue
		}
		objs, err := argocd.Engine().GenerateFromCluster(build(), layout.DefaultLayoutRules())
		if err != nil || objs != nil {
			t.Errorf("%s, valid rules: got %v, %v; want nothing", clusterName, objs, err)
		}
	}
}
