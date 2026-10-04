package layout_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for the FileNaming of the layouts an augmenter adds
// (go-kure/kure#976): one left unset takes its parent's, one that sets its own
// keeps it and passes it on.

// namingAugmenter adds four layouts below its application's: hooks and, below
// it, post, both with FileNaming unset; own, with FileNaming set to ownNaming,
// and below it own-sub, unset. appNaming, when set, replaces the application
// layout's own FileNaming.
type namingAugmenter struct {
	appNaming layout.FileNamingMode
	ownNaming layout.FileNamingMode
}

func (namingAugmenter) Generate(*stack.Application) ([]*client.Object, error) {
	var o client.Object = cmLayout("chart", "").Resources[0]
	return []*client.Object{&o}, nil
}

func (a namingAugmenter) AugmentLayout(ml *layout.ManifestLayout) error {
	if a.appNaming != layout.FileNamingUnset {
		ml.FileNaming = a.appNaming
	}
	hooks := cmLayout("hooks", ml.FullRepoPath())
	hooks.Children = append(hooks.Children, cmLayout("post", hooks.FullRepoPath()))
	own := cmLayout("own", ml.FullRepoPath())
	own.FileNaming = a.ownNaming
	own.Children = append(own.Children, cmLayout("own-sub", own.FullRepoPath()))
	ml.Children = append(ml.Children, hooks, own)
	return nil
}

// walkNamingAugmenter walks a one-node cluster whose bundle holds one
// application configured with aug, and returns the tree's root.
func walkNamingAugmenter(t *testing.T, rules layout.FileNamingMode, aug namingAugmenter) *layout.ManifestLayout {
	t.Helper()
	bundle := &stack.Bundle{Name: "platform", Applications: []*stack.Application{stack.NewApplication("chart", "default", aug)}}
	cluster := &stack.Cluster{Name: "platform", Node: &stack.Node{Name: "platform", Bundle: bundle}}
	ml, err := layout.WalkCluster(cluster, layout.LayoutRules{ClusterName: ".", FileNaming: rules})
	if err != nil {
		t.Fatalf("WalkCluster: %v", err)
	}
	return ml
}

// fileNamingByPath maps every layout's directory to its FileNaming.
func fileNamingByPath(ml *layout.ManifestLayout, out map[string]layout.FileNamingMode) map[string]layout.FileNamingMode {
	out[ml.FullRepoPath()] = ml.FileNaming
	for _, c := range ml.Children {
		fileNamingByPath(c, out)
	}
	return out
}

func TestWalkCluster_AugmenterLayoutsInheritFileNaming(t *testing.T) {
	const (
		unset    = layout.FileNamingUnset
		def      = layout.FileNamingDefault
		kindName = layout.FileNamingKindName
	)
	for _, tc := range []struct {
		name  string
		rules layout.FileNamingMode
		aug   namingAugmenter
		// want is the FileNaming of chart, hooks, post, own and own-sub.
		want [5]layout.FileNamingMode
	}{
		{"rules kind-name reach the unset layouts", kindName, namingAugmenter{},
			[5]layout.FileNamingMode{kindName, kindName, kindName, kindName, kindName}},
		{"a layout that sets its own keeps it and passes it on", kindName, namingAugmenter{ownNaming: def},
			[5]layout.FileNamingMode{kindName, kindName, kindName, def, def}},
		{"its own naming under unset rules", unset, namingAugmenter{ownNaming: kindName},
			[5]layout.FileNamingMode{unset, unset, unset, kindName, kindName}},
		{"the application layout's own naming is the parent's", kindName, namingAugmenter{appNaming: def},
			[5]layout.FileNamingMode{def, def, def, def, def}},
		{"rules default", def, namingAugmenter{ownNaming: kindName},
			[5]layout.FileNamingMode{def, def, def, kindName, kindName}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := fileNamingByPath(walkNamingAugmenter(t, tc.rules, tc.aug), map[string]layout.FileNamingMode{})
			for i, dir := range []string{
				"platform/chart", "platform/chart/hooks", "platform/chart/hooks/post",
				"platform/chart/own", "platform/chart/own/own-sub",
			} {
				naming, ok := got[dir]
				if !ok {
					t.Fatalf("no layout at %q in %v", dir, got)
				}
				if naming != tc.want[i] {
					t.Errorf("layout %q FileNaming = %q, want %q", dir, naming, tc.want[i])
				}
			}
		})
	}
}

// TestWriters_AugmenterLayoutsUseInheritedFileNaming pins the file names
// WriteToDisk and WriteToTar give the layouts an augmenter adds: the tree's
// pattern, except below a layout that set its own.
func TestWriters_AugmenterLayoutsUseInheritedFileNaming(t *testing.T) {
	want := []string{
		"platform/chart/configmap-chart.yaml",
		"platform/chart/hooks/configmap-hooks.yaml",
		"platform/chart/hooks/post/configmap-post.yaml",
		"platform/chart/own/default-configmap-own.yaml",
		"platform/chart/own/own-sub/default-configmap-own-sub.yaml",
	}
	walk := func() *layout.ManifestLayout {
		return walkNamingAugmenter(t, layout.FileNamingKindName, namingAugmenter{ownNaming: layout.FileNamingDefault})
	}

	t.Run("WriteToDisk", func(t *testing.T) {
		dir := writeTreeToDisk(t, walk())
		var got []string
		err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, err := filepath.Rel(dir, path)
			got = append(got, filepath.ToSlash(rel))
			return err
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
		assertResourceFiles(t, got, want)
		if bad := unresolvedKustomizeRefs(t, dir); len(bad) > 0 {
			t.Errorf("unresolved kustomize references: %v", bad)
		}
	})

	t.Run("WriteToTar", func(t *testing.T) {
		var buf bytes.Buffer
		if err := walk().WriteToTar(&buf); err != nil {
			t.Fatalf("WriteToTar: %v", err)
		}
		assertResourceFiles(t, collectTarNames(t, &buf), want)
	})
}

// assertResourceFiles compares the ConfigMap files among written with want.
func assertResourceFiles(t *testing.T, written, want []string) {
	t.Helper()
	var got []string
	for _, name := range written {
		if strings.Contains(filepath.Base(name), "configmap-") {
			got = append(got, name)
		}
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("resource files:\n got %v\nwant %v", got, want)
	}
}
