package fluxcd_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// umbrellaTerminals is UmbrellaTerminals(roots...), failing the test on an
// error.
func umbrellaTerminals(t *testing.T, roots ...*layout.ManifestLayout) map[string][]string {
	t.Helper()
	got, err := fluxstack.UmbrellaTerminals(roots...)
	if err != nil {
		t.Fatalf("UmbrellaTerminals: %v", err)
	}
	return got
}

// A tree no integration returned records no Flux Kustomization, so none of
// its Kustomizations is an umbrella: A, whose spec.path names the directory
// holding B and C, is a leaf, and B and C depending on each other refuse
// nothing, whatever Source A reads (go-kure/kure#1077).
func TestUmbrellaTerminals_HandBuiltTreeHasNoUmbrella(t *testing.T) {
	for name, source := range map[string]string{"foreign source": "other", "the bootstrap's source": "flux-system"} {
		t.Run(name, func(t *testing.T) {
			root := &layout.ManifestLayout{Name: "demo", Namespace: "clusters"}
			apps := &layout.ManifestLayout{Name: "apps", Namespace: root.FullRepoPath()}
			root.Children = []*layout.ManifestLayout{apps}
			a := handWritten("a")
			a.Spec.Path = apps.FullRepoPath()
			a.Spec.SourceRef.Name = source
			root.Resources = []client.Object{a}
			apps.Resources = []client.Object{handWritten("b", "c"), handWritten("c", "b")}
			got := umbrellaTerminals(t, root)
			if len(got) != 0 {
				t.Errorf("UmbrellaTerminals = %v, want no umbrella", got)
			}
		})
	}
}

// A Kustomization a caller adds to an integrated tree after the integration
// is no umbrella, even with the spec.path and Source of a generated one: the
// integration did not record it, and its own wait is its author's. The
// generated umbrella's terminals stay as they were.
func TestUmbrellaTerminals_CallerKustomizationIsLeaf(t *testing.T) {
	for _, placement := range integratedPlacements {
		for name, foreignSource := range map[string]bool{"foreign source": true, "the umbrella's source": false} {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				c := noWaitCluster(umbrellaOf("platform", srBundle("a", cmApp("a-app")), srBundle("b", cmApp("b-app"))), false)
				ml, err := integrateNoWait(c, nodeOnly, placement)
				if err != nil {
					t.Fatalf("CreateLayoutWithResources: %v", err)
				}
				want := umbrellaTerminals(t, ml)
				if len(want["flux-system/platform"]) == 0 {
					t.Fatalf("UmbrellaTerminals = %v: the case needs platform to be an umbrella", want)
				}
				platform := findKept(ml, "platform")
				foreign := handWritten("foreign")
				foreign.Spec.Path = platform.Spec.Path
				foreign.Spec.SourceRef = platform.Spec.SourceRef
				if foreignSource {
					foreign.Spec.SourceRef.Name = "other"
				}
				ml.Resources = append(ml.Resources, foreign)
				if got := umbrellaTerminals(t, ml); !reflect.DeepEqual(got, want) {
					t.Errorf("UmbrellaTerminals with a caller's Kustomization = %v, want %v", got, want)
				}
			})
		}
	}
}

// A record is read as kure reads identities: an omitted namespace is
// "default", an omitted Source namespace the Kustomization's own, and a path
// is cleaned. A caller's record of its own Kustomization in another form of
// the same values makes it an umbrella all the same.
func TestUmbrellaTerminals_RecordReadsAsIdentities(t *testing.T) {
	forms := map[string]struct {
		objNamespace, objSourceNamespace string
		recNamespace, recSourceNamespace string
		recPath                          func(string) string
	}{
		"object omits, record states": {"", "default", "default", "", func(p string) string { return "./" + p + "/" }},
		"object states, record omits": {"default", "", "", "default", func(p string) string { return p + "/." }},
	}
	for name, f := range forms {
		t.Run(name, func(t *testing.T) {
			root := &layout.ManifestLayout{Name: "demo", Namespace: "clusters"}
			apps := &layout.ManifestLayout{Name: "apps", Namespace: root.FullRepoPath()}
			root.Children = []*layout.ManifestLayout{apps}
			a := handWritten("a")
			a.Namespace = f.objNamespace
			a.Spec.Path = apps.FullRepoPath()
			a.Spec.SourceRef.Namespace = f.objSourceNamespace
			root.Resources = []client.Object{a}
			apps.Resources = []client.Object{handWritten("b"), handWritten("c")}
			root.SetFluxKustomizations([]layout.FluxKustomizationRecord{{
				Namespace: f.recNamespace, Name: "a", Path: f.recPath(apps.FullRepoPath()),
				SourceKind: "GitRepository", SourceNamespace: f.recSourceNamespace, SourceName: "flux-system",
			}})
			want := map[string][]string{"default/a": {"flux-system/b", "flux-system/c"}}
			if got := umbrellaTerminals(t, root); !reflect.DeepEqual(got, want) {
				t.Errorf("UmbrellaTerminals = %v, want %v", got, want)
			}
		})
	}
}

// twoUmbrellas is demo -> {apps (umbrella platform: a, b), web (umbrella
// web: c, d)}. Neither umbrella is in the other's build.
func twoUmbrellas() *stack.Cluster {
	apps := &stack.Node{Name: "apps", Bundle: umbrellaOf("platform", srBundle("a", cmApp("a-app")), srBundle("b", cmApp("b-app")))}
	web := &stack.Node{Name: "web", Bundle: umbrellaOf("web", srBundle("c", cmApp("c-app")), srBundle("d", cmApp("d-app")))}
	root := &stack.Node{Name: "demo", Children: []*stack.Node{apps, web}}
	apps.SetParent(root)
	web.SetParent(root)
	return &stack.Cluster{Name: "demo", Node: root}
}

// A generated umbrella edited after the integration (its Source, its cluster
// or its spec.path) is no longer what the integration recorded: it is a leaf,
// and the other umbrella's terminals stay as they were.
func TestUmbrellaTerminals_EditedUmbrellaIsLeaf(t *testing.T) {
	edits := map[string]func(ml *layout.ManifestLayout, k *kustv1.Kustomization){
		"source renamed":    func(_ *layout.ManifestLayout, k *kustv1.Kustomization) { k.Spec.SourceRef.Name = "other" },
		"kubeConfig set":    func(_ *layout.ManifestLayout, k *kustv1.Kustomization) { k.Spec.KubeConfig = remoteCluster() },
		"path moved to web": func(ml *layout.ManifestLayout, k *kustv1.Kustomization) { k.Spec.Path = findKept(ml, "web").Spec.Path },
	}
	for _, placement := range integratedPlacements {
		for name, edit := range edits {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				ml, err := integrateNoWait(twoUmbrellas(), nodeOnly, placement)
				if err != nil {
					t.Fatalf("CreateLayoutWithResources: %v", err)
				}
				unedited := umbrellaTerminals(t, ml)
				if len(unedited["flux-system/platform"]) == 0 || len(unedited["flux-system/web"]) == 0 {
					t.Fatalf("UmbrellaTerminals = %v: the case needs platform and web to be umbrellas", unedited)
				}
				edit(ml, findKept(ml, "platform"))
				want := map[string][]string{"flux-system/web": unedited["flux-system/web"]}
				for k, v := range unedited {
					if k != "flux-system/platform" && k != "flux-system/web" {
						want[k] = v
					}
				}
				if got := umbrellaTerminals(t, ml); !reflect.DeepEqual(got, want) {
					t.Errorf("UmbrellaTerminals after the edit = %v, want %v", got, want)
				}
			})
		}
	}
}

// replaceWithList replaces obj, wherever ml holds it, with an unstructured List
// holding it as its one item.
func replaceWithList(t *testing.T, ml *layout.ManifestLayout, obj client.Object) {
	t.Helper()
	raw, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	list := &metav1.List{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "List"}, Items: []runtime.RawExtension{{Raw: raw}}}
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(list)
	if err != nil {
		t.Fatal(err)
	}
	u := &unstructured.Unstructured{Object: m}
	var walk func(l *layout.ManifestLayout) bool
	walk = func(l *layout.ManifestLayout) bool {
		for i, r := range l.Resources {
			if r == obj {
				l.Resources[i] = u
				return true
			}
		}
		for _, c := range l.Children {
			if walk(c) {
				return true
			}
		}
		return false
	}
	if !walk(ml) {
		t.Fatalf("%s is not in the layout", obj.GetName())
	}
}

// The record names a Kustomization by what it reads, not by its Go form: a
// generated umbrella replaced by an unstructured copy, or by a List holding
// it, is still the umbrella it was.
func TestUmbrellaTerminals_RecordedUmbrellaInAnyForm(t *testing.T) {
	forms := map[string]func(t *testing.T, ml *layout.ManifestLayout, k *kustv1.Kustomization){
		"unstructured": func(t *testing.T, ml *layout.ManifestLayout, k *kustv1.Kustomization) {
			replaceUnstructured(t, ml, k, func(*unstructured.Unstructured) {})
		},
		"list item": func(t *testing.T, ml *layout.ManifestLayout, k *kustv1.Kustomization) { replaceWithList(t, ml, k) },
	}
	for _, placement := range integratedPlacements {
		for name, form := range forms {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				ml, err := integrateNoWait(twoUmbrellas(), nodeOnly, placement)
				if err != nil {
					t.Fatalf("CreateLayoutWithResources: %v", err)
				}
				want := umbrellaTerminals(t, ml)
				form(t, ml, findKept(ml, "platform"))
				if got := umbrellaTerminals(t, ml); !reflect.DeepEqual(got, want) {
					t.Errorf("UmbrellaTerminals with platform as %s = %v, want %v", name, got, want)
				}
			})
		}
	}
}

// A generated umbrella on one Source applying a child umbrella on another is
// still expanded through the child to its leaves: the integration chose both
// Sources, and recorded them.
func TestUmbrellaTerminals_MultiSourceNestedUmbrella(t *testing.T) {
	parentSR := &stack.SourceRef{Kind: "GitRepository", Name: "parent-source", Namespace: "flux-system"}
	childSR := &stack.SourceRef{Kind: "GitRepository", Name: "child-source", Namespace: "flux-system"}
	leaf := func(name string) *stack.Bundle {
		return &stack.Bundle{Name: name, SourceRef: childSR, Applications: []*stack.Application{cmApp(name + "-app")}}
	}
	mid := &stack.Bundle{Name: "mid", SourceRef: childSR, Children: []*stack.Bundle{leaf("g1"), leaf("g2")}}
	platform := &stack.Bundle{Name: "platform", SourceRef: parentSR, Children: []*stack.Bundle{mid}}
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			ml, err := integrateNoWait(noWaitCluster(platform, false), nodeOnly, placement)
			if err != nil {
				t.Fatalf("CreateLayoutWithResources: %v", err)
			}
			if got := findKept(ml, "mid").Spec.SourceRef.Name; got != "child-source" {
				t.Fatalf("mid reads %q, want child-source: the case needs two Sources", got)
			}
			want := map[string][]string{
				"flux-system/platform": {"flux-system/g1", "flux-system/g2"},
				"flux-system/mid":      {"flux-system/g1", "flux-system/g2"},
			}
			if got := umbrellaTerminals(t, ml); !reflect.DeepEqual(got, want) {
				t.Errorf("UmbrellaTerminals = %v, want %v", got, want)
			}
		})
	}
}

// emittingAugmenter is an application that emits objs and adds a child
// layout "hooks" holding one hand-written Flux Kustomization, hook-k.
type emittingAugmenter struct{ objs []client.Object }

func (a emittingAugmenter) Generate(*stack.Application) ([]*client.Object, error) {
	var out []*client.Object
	for i := range a.objs {
		out = append(out, &a.objs[i])
	}
	return out, nil
}

func (emittingAugmenter) AugmentLayout(ml *layout.ManifestLayout) error {
	ml.Children = append(ml.Children, &layout.ManifestLayout{Name: "hooks", Namespace: ml.FullRepoPath(), Resources: []client.Object{handWritten("hook-k")}})
	return nil
}

// A Kustomization an application emits that the integration keeps in place
// of the one it generates (same name, host, spec.path and Source) is the
// integration's: UmbrellaTerminals reads it as an umbrella, as the
// integration's own UmbrellasDoNotWait pass does, with the terminals the
// generated one has.
func TestUmbrellaTerminals_KeptApplicationKustomizationIsUmbrella(t *testing.T) {
	cluster := func(objs ...client.Object) *stack.Cluster {
		return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "root", Bundle: srBundle("root",
			stack.NewApplication("chart", "default", emittingAugmenter{objs: objs}))}}
	}
	rules := allFlat
	rules.FluxPlacement = layout.FluxIntegratedPerLayout
	probe := integrated(t, cluster(), rules)
	want := umbrellaTerminals(t, probe)
	var hooks *kustv1.Kustomization
	for _, k := range kustomizations(probe) {
		if k.Name != "hook-k" && reflect.DeepEqual(want[crKeyOf(k)], []string{"flux-system/hook-k"}) {
			hooks = k
		}
	}
	if hooks == nil {
		t.Fatalf("UmbrellaTerminals = %v: the case needs the hooks layout's Kustomization to be an umbrella over hook-k", want)
	}
	emitted := hooks.DeepCopy()
	ml := integrated(t, cluster(emitted), rules)
	if n := len(kustomizations(ml)); n != len(kustomizations(probe)) {
		t.Fatalf("%d Kustomizations, want %d: the integration must keep the emitted %s, not add one", n, len(kustomizations(probe)), emitted.Name)
	}
	if got := umbrellaTerminals(t, ml); !reflect.DeepEqual(got, want) {
		t.Errorf("UmbrellaTerminals with the kept application Kustomization = %v, want %v", got, want)
	}
}

// crKeyOf is k's namespace/name.
func crKeyOf(k *kustv1.Kustomization) string { return k.Namespace + "/" + k.Name }

// fluxRecords maps the directory of every layout of ml to what it records
// (FluxKustomizations), an empty record left out.
func fluxRecords(ml *layout.ManifestLayout) map[string][]layout.FluxKustomizationRecord {
	out := map[string][]layout.FluxKustomizationRecord{}
	walkLayouts(ml, func(l *layout.ManifestLayout) {
		if recs := l.FluxKustomizations(); len(recs) > 0 {
			out[l.FullRepoPath()] = recs
		}
	})
	return out
}

// staleRecord is a record no integration wrote.
var staleRecord = []layout.FluxKustomizationRecord{{Namespace: "flux-system", Name: "stale", Path: "elsewhere", SourceKind: "GitRepository", SourceNamespace: "flux-system", SourceName: "flux-system"}}

// An integration records, on every layout, the Kustomizations it placed or
// kept there, replacing what the layout recorded before: integrating the
// tree again records the same, and a record no integration wrote is gone.
func TestIntegrateWithLayout_RecordsFluxKustomizations(t *testing.T) {
	for _, placement := range append(slices.Clone(integratedPlacements), layout.FluxSeparate) {
		t.Run(string(placement), func(t *testing.T) {
			c := twoUmbrellas()
			rules := withPlacement(nodeOnly, placement)
			li := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
			ml, err := li.CreateLayoutWithResources(c, rules)
			if err != nil {
				t.Fatalf("CreateLayoutWithResources: %v", err)
			}
			first := fluxRecords(ml)
			var recorded []string
			for _, recs := range first {
				for _, r := range recs {
					recorded = append(recorded, r.Name)
				}
			}
			slices.Sort(recorded)
			var placed []string
			for _, k := range kustomizations(ml) {
				placed = append(placed, k.Name)
			}
			slices.Sort(placed)
			if !slices.Equal(recorded, placed) {
				t.Fatalf("recorded %v, want every Kustomization placed: %v", recorded, placed)
			}
			terms := umbrellaTerminals(t, ml)

			var bare *layout.ManifestLayout
			walkLayouts(ml, func(l *layout.ManifestLayout) {
				if bare == nil && len(l.FluxKustomizations()) == 0 {
					bare = l
				}
			})
			if bare == nil {
				t.Fatal("every layout records a Kustomization: the case needs one that records none")
			}
			bare.SetFluxKustomizations(staleRecord)
			if err := li.IntegrateWithLayout(ml, c, rules); err != nil {
				t.Fatalf("IntegrateWithLayout again: %v", err)
			}
			if got := fluxRecords(ml); !reflect.DeepEqual(got, first) {
				t.Errorf("records after integrating again = %v, want %v", got, first)
			}
			if got := umbrellaTerminals(t, ml); !reflect.DeepEqual(got, terms) {
				t.Errorf("UmbrellaTerminals after integrating again = %v, want %v", got, terms)
			}
		})
	}
}

// A separate integration that generates nothing (a tree without bundles)
// still succeeds as an integration: it leaves no earlier record behind.
func TestIntegrateWithLayout_EmptySeparateClearsRecords(t *testing.T) {
	c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "demo"}}
	rules := withPlacement(nodeOnly, layout.FluxSeparate)
	ml, err := layout.WalkCluster(c, rules)
	if err != nil {
		t.Fatalf("WalkCluster: %v", err)
	}
	walkLayouts(ml, func(l *layout.ManifestLayout) { l.SetFluxKustomizations(staleRecord) })
	if err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules); err != nil {
		t.Fatalf("IntegrateWithLayout: %v", err)
	}
	if n := len(kustomizations(ml)); n != 0 {
		t.Fatalf("%d Kustomizations generated: the case needs an integration that generates none", n)
	}
	if got := fluxRecords(ml); len(got) != 0 {
		t.Errorf("records after the integration = %v, want none", got)
	}
}

// A refused integration leaves every layout's record as the caller gave it,
// whether the refusal comes while placing (a namesake already in the tree)
// or after (the reconcile-order check).
func TestIntegrateWithLayout_RefusalKeepsRecords(t *testing.T) {
	cases := map[string]struct {
		cluster func() *stack.Cluster
		rules   layout.LayoutRules
		edit    func(ml *layout.ManifestLayout)
	}{
		"namesake": {
			cluster: twoUmbrellas,
			rules:   nodeOnly,
			edit: func(ml *layout.ManifestLayout) {
				ml.Resources = append(ml.Resources, fluxKustomization("platform", "elsewhere"))
			},
		},
		"reconcile order": {
			cluster: createdLeafCluster,
			rules:   propertyGroupings["nodeOnly"],
			edit:    func(*layout.ManifestLayout) {},
		},
	}
	for name, tc := range cases {
		for _, placement := range append(slices.Clone(integratedPlacements), layout.FluxSeparate) {
			if name == "reconcile order" && placement == layout.FluxSeparate {
				continue // separate placement applies every CR from flux-system: no creation order to refuse
			}
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				c := tc.cluster()
				rules := withPlacement(tc.rules, placement)
				ml, err := layout.WalkCluster(c, rules)
				if err != nil {
					t.Fatalf("WalkCluster: %v", err)
				}
				tc.edit(ml)
				walkLayouts(ml, func(l *layout.ManifestLayout) { l.SetFluxKustomizations(staleRecord) })
				want := fluxRecords(ml)
				if err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules); err == nil {
					t.Fatal("IntegrateWithLayout succeeded: the case needs a refusal")
				}
				if got := fluxRecords(ml); !reflect.DeepEqual(got, want) {
					t.Errorf("records after the refusal = %v, want %v", got, want)
				}
			})
		}
	}
}
