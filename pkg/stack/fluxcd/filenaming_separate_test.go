package fluxcd_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for the FileNaming of the separate flux-system layout
// (go-kure/kure#976): it is the rules', so its files are named like the rest
// of the tree.

// separateNamingCluster is a root node with one child node, each with a
// bundle: the separate placement generates Kustomizations platform and apps.
func separateNamingCluster() *stack.Cluster {
	child := &stack.Node{Name: "apps", Bundle: srBundle("apps", cmApp("web"))}
	root := &stack.Node{Name: "platform", Bundle: srBundle("platform"), Children: []*stack.Node{child}}
	child.SetParent(root)
	child.Bundle.SetParent(root.Bundle)
	return &stack.Cluster{Name: "platform", Node: root}
}

// fluxSystemLayout returns the flux-system child of ml.
func fluxSystemLayout(t *testing.T, ml *layout.ManifestLayout) *layout.ManifestLayout {
	t.Helper()
	for _, c := range ml.Children {
		if c.Name == fluxstack.DefaultFluxDirName {
			return c
		}
	}
	t.Fatalf("layout %q has no %s child", ml.FullRepoPath(), fluxstack.DefaultFluxDirName)
	return nil
}

// fluxSystemFiles returns the base names of the files in dir other than
// kustomization.yaml, sorted, from a list of slash-separated paths.
func fluxSystemFiles(paths []string, dir string) []string {
	var got []string
	for _, p := range paths {
		base, ok := strings.CutPrefix(p, dir+"/")
		if ok && base != "kustomization.yaml" && !strings.Contains(base, "/") {
			got = append(got, base)
		}
	}
	slices.Sort(got)
	return got
}

func TestFluxSeparate_FluxSystemFilesUseRulesFileNaming(t *testing.T) {
	for _, tc := range []struct {
		name   string
		naming layout.FileNamingMode
		want   []string
	}{
		{"kind-name", layout.FileNamingKindName,
			[]string{"kustomization-apps.yaml", "kustomization-platform.yaml"}},
		{"default", layout.FileNamingDefault,
			[]string{"flux-system-kustomization-apps.yaml", "flux-system-kustomization-platform.yaml"}},
		{"unset", layout.FileNamingUnset,
			[]string{"flux-system-kustomization-apps.yaml", "flux-system-kustomization-platform.yaml"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			create := func() *layout.ManifestLayout {
				ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(
					separateNamingCluster(), layout.LayoutRules{FluxPlacement: layout.FluxSeparate, FileNaming: tc.naming})
				if err != nil {
					t.Fatalf("CreateLayoutWithResources: %v", err)
				}
				return ml
			}

			ml := create()
			if got := fluxSystemLayout(t, ml).FileNaming; got != tc.naming {
				t.Errorf("flux-system FileNaming = %q, want the rules' %q", got, tc.naming)
			}

			dir := t.TempDir()
			if err := ml.WriteToDisk(dir); err != nil {
				t.Fatalf("WriteToDisk: %v", err)
			}
			fluxDir := filepath.Join(dir, "platform", fluxstack.DefaultFluxDirName)
			entries, err := os.ReadDir(fluxDir)
			if err != nil {
				t.Fatalf("read %s: %v", fluxDir, err)
			}
			var onDisk []string
			for _, e := range entries {
				onDisk = append(onDisk, "platform/flux-system/"+e.Name())
			}
			if got := fluxSystemFiles(onDisk, "platform/flux-system"); !slices.Equal(got, tc.want) {
				t.Errorf("WriteToDisk flux-system files:\n got %v\nwant %v", got, tc.want)
			}
			kust, err := os.ReadFile(filepath.Join(fluxDir, "kustomization.yaml"))
			if err != nil {
				t.Fatalf("read flux-system kustomization.yaml: %v", err)
			}
			for _, f := range tc.want {
				if !strings.Contains(string(kust), "- "+f+"\n") {
					t.Errorf("flux-system kustomization.yaml does not list %s:\n%s", f, kust)
				}
			}

			var buf bytes.Buffer
			if err := create().WriteToTar(&buf); err != nil {
				t.Fatalf("WriteToTar: %v", err)
			}
			inTar := fileNamesFromFluxcd(extractTarFilesFluxcd(t, &buf))
			if got := fluxSystemFiles(inTar, "platform/flux-system"); !slices.Equal(got, tc.want) {
				t.Errorf("WriteToTar flux-system files:\n got %v\nwant %v", got, tc.want)
			}
		})
	}
}

// TestIntegrateWithLayout_Separate_FluxSystemFileNaming pins which FileNaming
// the flux-system layout gets when a caller integrates a tree it walked
// itself: the rules' when they set one, else the root layout's, so the
// directory is named like the tree it sits in.
func TestIntegrateWithLayout_Separate_FluxSystemFileNaming(t *testing.T) {
	for _, tc := range []struct {
		name            string
		walked, rules   layout.FileNamingMode
		wantFluxSystem  layout.FileNamingMode
		wantFluxCRFiles []string
	}{
		{"rules set it", layout.FileNamingKindName, layout.FileNamingKindName, layout.FileNamingKindName,
			[]string{"kustomization-apps.yaml", "kustomization-platform.yaml"}},
		{"rules leave it unset: the root layout's", layout.FileNamingKindName, layout.FileNamingUnset, layout.FileNamingKindName,
			[]string{"kustomization-apps.yaml", "kustomization-platform.yaml"}},
		{"rules win over the root layout's", layout.FileNamingKindName, layout.FileNamingDefault, layout.FileNamingDefault,
			[]string{"flux-system-kustomization-apps.yaml", "flux-system-kustomization-platform.yaml"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cluster := separateNamingCluster()
			ml, err := layout.WalkCluster(cluster, layout.LayoutRules{FluxPlacement: layout.FluxSeparate, FileNaming: tc.walked})
			if err != nil {
				t.Fatalf("WalkCluster: %v", err)
			}
			integrator := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
			rules := layout.LayoutRules{FluxPlacement: layout.FluxSeparate, FileNaming: tc.rules}
			if err := integrator.IntegrateWithLayout(ml, cluster, rules); err != nil {
				t.Fatalf("IntegrateWithLayout: %v", err)
			}
			if got := fluxSystemLayout(t, ml).FileNaming; got != tc.wantFluxSystem {
				t.Errorf("flux-system FileNaming = %q, want %q", got, tc.wantFluxSystem)
			}

			var buf bytes.Buffer
			if err := ml.WriteToTar(&buf); err != nil {
				t.Fatalf("WriteToTar: %v", err)
			}
			inTar := fileNamesFromFluxcd(extractTarFilesFluxcd(t, &buf))
			if got := fluxSystemFiles(inTar, "platform/flux-system"); !slices.Equal(got, tc.wantFluxCRFiles) {
				t.Errorf("flux-system files:\n got %v\nwant %v", got, tc.wantFluxCRFiles)
			}

			// Integrating again keeps the directory as the first call left it.
			if err := integrator.IntegrateWithLayout(ml, cluster, rules); err != nil {
				t.Fatalf("second IntegrateWithLayout: %v", err)
			}
			if got := fluxSystemLayout(t, ml).FileNaming; got != tc.wantFluxSystem {
				t.Errorf("after a second integration flux-system FileNaming = %q, want %q", got, tc.wantFluxSystem)
			}
		})
	}
}
