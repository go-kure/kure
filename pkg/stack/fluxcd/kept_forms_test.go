package fluxcd_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
	metaapi "github.com/fluxcd/pkg/apis/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for the checks the integrated placements run over the Kustomizations
// an integration placed or kept (go-kure/kure#979, item 6): one it keeps in
// place of its own is read in typed form whatever form the caller gave it.

// keptForms are the forms a Kustomization already in the tree can have. Only
// the first is a typed object among a layout's resources; the typed one in a
// List is an item of a List that holds it as an object.
var keptForms = []string{"typed", "typed in a List", "unstructured", "unstructured in a List"}

// untypedKeptForms are the keptForms that carry unstructured content.
var untypedKeptForms = []string{"unstructured", "unstructured in a List"}

var integratedPlacements = []layout.FluxPlacement{layout.FluxIntegratedPerBundle, layout.FluxIntegratedPerLayout}

// keptIn walks build() with rules and puts, into the layout that hosts the
// Kustomization the integration generates under name, a copy of it in the
// given form: the integration then keeps that copy in place of its own. change
// edits the copy; mangle, if set, edits its unstructured content, which only
// the untypedKeptForms carry. The returned tree is not integrated.
func keptIn(t *testing.T, build func() *stack.Cluster, rules layout.LayoutRules, name, form string, change func(k *kustv1.Kustomization), mangle func(content map[string]any)) (*layout.ManifestLayout, *stack.Cluster) {
	t.Helper()
	var hostPath string
	var generated *kustv1.Kustomization
	var find func(l *layout.ManifestLayout)
	find = func(l *layout.ManifestLayout) {
		for _, r := range l.Resources {
			if k, ok := r.(*kustv1.Kustomization); ok && k.Name == name {
				hostPath, generated = l.FullRepoPath(), k
			}
		}
		for _, c := range l.Children {
			find(c)
		}
	}
	find(integrated(t, build(), rules))
	if generated == nil {
		t.Fatalf("no generated Kustomization named %q", name)
	}

	c := build()
	ml, err := layout.WalkCluster(c, rules)
	if err != nil {
		t.Fatalf("WalkCluster: %v", err)
	}
	kept := generated.DeepCopy()
	if change != nil {
		change(kept)
	}
	var own client.Object = kept
	switch form {
	case "typed":
	case "typed in a List":
		own = rawListOf(runtime.RawExtension{Object: kept})
	case "unstructured", "unstructured in a List":
		content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(kept)
		if err != nil {
			t.Fatalf("ToUnstructured: %v", err)
		}
		if mangle != nil {
			mangle(content)
		}
		u := &unstructured.Unstructured{Object: content}
		u.SetAPIVersion(kustv1.GroupVersion.String())
		u.SetKind("Kustomization")
		own = u
		if form == "unstructured in a List" {
			own = wrapInList(u)
		}
	default:
		t.Fatalf("unknown form %q", form)
	}
	host := layoutAtPath(t, ml, hostPath)
	host.Resources = append(host.Resources, own)
	return ml, c
}

// integrate runs the integration over a tree keptIn returned.
func integrate(ml *layout.ManifestLayout, c *stack.Cluster, rules layout.LayoutRules) error {
	return fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
}

// checkKeptTreeWrites integrates a tree whose kept Kustomization is unchanged
// and writes it: the control of a refusal, with the disk and tar writers
// producing the same files.
func checkKeptTreeWrites(t *testing.T, build func() *stack.Cluster, rules layout.LayoutRules, name, form string) {
	t.Helper()
	ml, c := keptIn(t, build, rules, name, form, nil, nil)
	if err := integrate(ml, c, rules); err != nil {
		t.Fatalf("the unchanged kept Kustomization is refused: %v", err)
	}
	trees := writeAll(t, ml)
	disk, tar := treeFiles(t, trees["WriteToDisk"].root), treeFiles(t, trees["WriteToTar"].root)
	if len(disk) == 0 {
		t.Fatal("WriteToDisk wrote no file")
	}
	if len(disk) != len(tar) {
		t.Errorf("WriteToDisk wrote %d files, WriteToTar %d", len(disk), len(tar))
	}
	for p, content := range disk {
		if other, ok := tar[p]; !ok || !bytes.Equal(content, other) {
			t.Errorf("%s differs between WriteToDisk and WriteToTar", p)
		}
	}
}

// treeFiles maps every file below root, by its path relative to root, to its
// content.
func treeFiles(t *testing.T, root string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		out[rel], err = os.ReadFile(p)
		return err
	})
	if err != nil {
		t.Fatalf("read %s: %v", root, err)
	}
	return out
}

// TestKeptKustomization_ReconcileOrder: the dependsOn, wait and health checks
// of a Kustomization the integration keeps enter the reconcile-order check in
// every form, as those of a generated one do. Each case closes a cycle only
// through a field of the kept Kustomization; without that field the tree
// integrates and writes.
func TestKeptKustomization_ReconcileOrder(t *testing.T) {
	// nested: r -> a (bundle) -> b (bundle). b's CR sits in a's directory,
	// which only a's Kustomization applies.
	nested := func() *stack.Cluster {
		b := &stack.Node{Name: "b", Bundle: srBundle("b", cmApp("b-app"))}
		a := &stack.Node{Name: "a", Bundle: srBundle("a", cmApp("a-app")), Children: []*stack.Node{b}}
		r := &stack.Node{Name: "r", Children: []*stack.Node{a}}
		b.SetParent(a)
		a.SetParent(r)
		return &stack.Cluster{Name: "demo", Node: r}
	}
	// siblings: r -> [a (bundle), b (bundle)], b depending on a.
	siblings := func() *stack.Cluster {
		a := &stack.Node{Name: "a", Bundle: srBundle("a", cmApp("a-app"))}
		b := &stack.Node{Name: "b", Bundle: srBundle("b", cmApp("b-app"))}
		b.Bundle.DependsOn = []*stack.Bundle{a.Bundle}
		r := &stack.Node{Name: "r", Children: []*stack.Node{a, b}}
		a.SetParent(r)
		b.SetParent(r)
		return &stack.Cluster{Name: "demo", Node: r}
	}
	// dependent: nested, with b depending on a. a's Kustomization applies a's
	// directory, which holds b's CR.
	dependent := func() *stack.Cluster {
		c := nested()
		a := c.Node.Children[0]
		a.Children[0].Bundle.DependsOn = []*stack.Bundle{a.Bundle}
		return c
	}
	for _, tc := range []struct {
		name   string
		build  func() *stack.Cluster
		change func(k *kustv1.Kustomization)
	}{
		{
			name:  "dependsOn a Kustomization only it creates",
			build: nested,
			change: func(k *kustv1.Kustomization) {
				k.Spec.DependsOn = []kustv1.DependencyReference{{Name: "b"}}
			},
		},
		{
			name:  "health check on a Kustomization that depends on it",
			build: siblings,
			change: func(k *kustv1.Kustomization) {
				k.Spec.Wait = false
				k.Spec.HealthChecks = []metaapi.NamespacedObjectKindReference{{APIVersion: kustv1.GroupVersion.String(), Kind: "Kustomization", Name: "b", Namespace: k.Namespace}}
			},
		},
		{
			name:  "wait for a Kustomization that depends on it",
			build: dependent,
			change: func(k *kustv1.Kustomization) {
				k.Spec.Wait = true
			},
		},
	} {
		for _, placement := range integratedPlacements {
			for _, form := range keptForms {
				t.Run(tc.name+"/"+string(placement)+"/"+form, func(t *testing.T) {
					rules := propertyGroupings["nodeOnly"]
					rules.FluxPlacement = placement
					checkKeptTreeWrites(t, tc.build, rules, "a", form)

					ml, c := keptIn(t, tc.build, rules, "a", form, tc.change, nil)
					if err := integrate(ml, c, rules); err == nil || !strings.Contains(err.Error(), "never") {
						t.Errorf("got %v, want a reconcile-order refusal", err)
					}
				})
			}
		}
	}
}

// TestKeptKustomization_RootBuildChangesHostedSource: a kept Kustomization
// whose patch selects a Source the integration hosts, or whose postBuild
// substitutes into it, is accepted in every form: no build the Flux bootstrap
// applies holds that Source beside it (go-kure/kure#908 refuses the one that
// would). The layout Kustomization of the root node's layout, under
// FluxIntegratedPerLayout below a ClusterName wrapper, builds the directory
// that hosts the Source, and the wrapper the bootstrap applies lists that
// Kustomization, not the directory: it is the only one to apply the Source
// (go-kure/kure#979). The root bundle's builds the bundle's own directory,
// which holds no Source, in every placement.
func TestKeptKustomization_RootBuildChangesHostedSource(t *testing.T) {
	plain := func(_, _ *stack.Bundle) {}
	wrapped := propertyGroupings["nodeOnly"]
	wrapped.FluxPlacement = layout.FluxIntegratedPerLayout
	wrapped.ClusterName = "prod"
	for _, tc := range []struct {
		name   string
		build  func() *stack.Cluster
		change func(k *kustv1.Kustomization)
	}{
		{
			name:  "patch selecting the Source",
			build: rootBundleTree("", plain),
			change: func(k *kustv1.Kustomization) {
				k.Spec.Patches = []kustomize.Patch{{Patch: sourcePatch, Target: &kustomize.Selector{Kind: "GitRepository"}}}
			},
		},
		{
			name:  "postBuild substituting into the Source",
			build: rootBundleTree("https://${GIT_HOST}/shared.git", plain),
			change: func(k *kustv1.Kustomization) {
				k.Spec.PostBuild = &kustv1.PostBuild{Substitute: map[string]string{"GIT_HOST": "git.example.com"}}
			},
		},
	} {
		const rootDir = "prod/platform"
		rootCR := rootLayoutCRName(t, tc.build, wrapped, rootDir)
		for _, form := range keptForms {
			t.Run(tc.name+"/root node's layout/"+form, func(t *testing.T) {
				checkKeptTreeWrites(t, tc.build, wrapped, rootCR, form)

				ml, c := keptIn(t, tc.build, wrapped, rootCR, form, tc.change, nil)
				if err := integrate(ml, c, wrapped); err != nil {
					t.Fatalf("the kept Kustomization of the root node's layout is the only one to apply what the layout hosts: %v", err)
				}
				if got, want := sourceCopies(ml, "shared"), map[string]int{rootDir: 1}; !intMapsEqual(got, want) {
					t.Errorf("GitRepository shared hosted %v, want %v", got, want)
				}
				// The integration placed no Kustomization of its own beside
				// the caller's, which carries the patch or postBuild.
				var typed []*kustv1.Kustomization
				for _, k := range kustomizations(ml) {
					if k.Name == rootCR {
						typed = append(typed, k)
					}
				}
				if form != "typed" {
					if len(typed) != 0 {
						t.Errorf("the integration placed its own Kustomization %q beside the kept one", rootCR)
					}
					checkBootstrapBuildHoldsNoSource(t, ml)
					return
				}
				if len(typed) != 1 || (len(typed[0].Spec.Patches) == 0 && typed[0].Spec.PostBuild == nil) {
					t.Fatalf("the tree holds %d typed Kustomizations %q, want the kept one alone, with its patch or postBuild", len(typed), rootCR)
				}
				checkOnlyTheCRAppliesTheSource(t, ml, rootDir)
			})
			for _, placement := range integratedPlacements {
				t.Run(tc.name+"/root bundle/"+string(placement)+"/"+form, func(t *testing.T) {
					rules := propertyGroupings["nodeOnly"]
					rules.FluxPlacement = placement
					ml, c := keptIn(t, tc.build, rules, "platform", form, tc.change, nil)
					if err := integrate(ml, c, rules); err != nil {
						t.Fatalf("the root bundle's Kustomization builds its own directory, not the Source's: %v", err)
					}
				})
			}
		}
	}
}

// TestKeptKustomization_SourceTwiceInItsBuild: the directory a kept
// Kustomization builds is a build the integration answers for, in every form.
// Two copies of a Source the pass derives, which the caller put in two layouts
// that build includes, are refused: kustomize refuses one object twice. The
// case needs a build that lists a child directory, which a
// FluxIntegratedPerLayout parent does not (it lists the child's CR).
func TestKeptKustomization_SourceTwiceInItsBuild(t *testing.T) {
	// platform -> a (bundle) -> group (no bundle) -> web (bundle, shared
	// Source). a's kustomization.yaml lists group, so a's build holds both.
	build := func() *stack.Cluster {
		web := &stack.Node{Name: "web", Bundle: sharedBundle("web")}
		group := &stack.Node{Name: "group", Children: []*stack.Node{web}}
		a := &stack.Node{Name: "a", Bundle: srBundle("a", cmApp("a-app")), Children: []*stack.Node{group}}
		return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Children: []*stack.Node{a}}}
	}
	rules := propertyGroupings["nodeOnly"]
	rules.FluxPlacement = layout.FluxIntegratedPerBundle
	for _, form := range keptForms {
		t.Run(form, func(t *testing.T) {
			checkKeptTreeWrites(t, build, rules, "a", form)

			ml, c := keptIn(t, build, rules, "a", form, nil, nil)
			for _, p := range []string{"platform/a", "platform/a/group"} {
				l := layoutAtPath(t, ml, p)
				l.Resources = append(l.Resources, sharedSource(t))
			}
			err := integrate(ml, c, rules)
			if err == nil {
				t.Fatal("got no error, want a refusal of two copies in the build of platform/a")
			}
			for _, want := range []string{`"platform/a"`, `"platform/a/group"`, `GitRepository "shared"`, `the kustomize build of "platform/a" includes both`} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal %q does not name %q", err, want)
				}
			}
		})
	}
}

// TestKeptKustomization_UnreadableIsRefused: a kept Kustomization that cannot
// be read in typed form is an error naming the layout that holds it and the
// object, in place of a Kustomization the checks leave out. The refusal comes
// after the integration placed b's Kustomization, and leaves the caller's tree
// as it was.
func TestKeptKustomization_UnreadableIsRefused(t *testing.T) {
	build := func() *stack.Cluster {
		a := &stack.Node{Name: "a", Bundle: srBundle("a", cmApp("a-app"))}
		b := &stack.Node{Name: "b", Bundle: srBundle("b", cmApp("b-app"))}
		r := &stack.Node{Name: "r", Children: []*stack.Node{a, b}}
		a.SetParent(r)
		b.SetParent(r)
		return &stack.Cluster{Name: "demo", Node: r}
	}
	mangle := func(content map[string]any) {
		content["spec"].(map[string]any)["wait"] = "yes"
	}
	for _, placement := range integratedPlacements {
		for _, form := range untypedKeptForms {
			t.Run(string(placement)+"/"+form, func(t *testing.T) {
				rules := propertyGroupings["nodeOnly"]
				rules.FluxPlacement = placement
				ml, c := keptIn(t, build, rules, "a", form, nil, mangle)
				before := countResources(ml)
				err := integrate(ml, c, rules)
				if err == nil {
					t.Fatal("got no error, want a refusal of the Kustomization that cannot be read")
				}
				for _, want := range []string{`layout "r"`, `Flux Kustomization "flux-system/a"`, "typed form"} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("refusal %q does not name %q", err, want)
					}
				}
				if after := countResources(ml); after != before {
					t.Errorf("refused integration left %d resources, want the caller's %d", after, before)
				}
			})
		}
	}
}
