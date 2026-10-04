package fluxcd_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// typedItems is a typed object with an Items field, as a hand-written list
// type has one.
type typedItems struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Items             []unstructured.Unstructured `json:"items"`
}

func (l *typedItems) DeepCopyObject() runtime.Object {
	c := *l
	c.Items = slices.Clone(l.Items)
	return &c
}

// typedListOf returns a typed object of the given kind holding items.
func typedListOf(kind string, items ...*unstructured.Unstructured) *typedItems {
	l := &typedItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: kind}}
	l.Name, l.Namespace = "holder", "default"
	for _, it := range items {
		l.Items = append(l.Items, *it)
	}
	return l
}

// rawItems is a typed List whose items are runtime.RawExtension values, each
// either an object or the raw JSON of one, as the core v1 List holds them.
type rawItems struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Items             []runtime.RawExtension `json:"items"`
}

func (l *rawItems) DeepCopyObject() runtime.Object {
	c := *l
	c.Items = slices.Clone(l.Items)
	return &c
}

func rawListOf(items ...runtime.RawExtension) *rawItems {
	l := &rawItems{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "List"}, Items: items}
	l.Name, l.Namespace = "holder", "default"
	return l
}

// bareObject is a runtime.Object without object metadata methods: a typed List
// can hold one as an item, and it is not a client.Object.
type bareObject struct {
	metav1.TypeMeta `json:",inline"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	Spec            map[string]any `json:"spec,omitempty"`
}

func (b *bareObject) DeepCopyObject() runtime.Object {
	c := *b
	return &c
}

// bareOf returns obj as a bareObject: the same serialized object, without the
// metadata methods.
func bareOf(t *testing.T, obj client.Object) *bareObject {
	t.Helper()
	raw, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	b := &bareObject{}
	if err := json.Unmarshal(raw, b); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	return b
}

// jsonAs is a runtime.Object without object metadata methods that serializes
// as the JSON it holds, or not at all when it holds none.
type jsonAs struct {
	metav1.TypeMeta
	raw string
}

func (j *jsonAs) DeepCopyObject() runtime.Object {
	c := *j
	return &c
}

func (j *jsonAs) MarshalJSON() ([]byte, error) {
	if j.raw == "" {
		return nil, errors.New("not serializable")
	}
	return []byte(j.raw), nil
}

// TestIntegrateWithLayout_ListItemMustSerializeAsObject: an item of a typed
// List that has no object metadata is read as the object the writers serialize
// for it, so one that does not serialize, or not as an object, is refused in
// place of being left out.
func TestIntegrateWithLayout_ListItemMustSerializeAsObject(t *testing.T) {
	items := map[string]*jsonAs{
		"does not serialize": {},
		"not an object":      {raw: `"text"`},
	}
	for name, item := range items {
		for _, placement := range placements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				var obj client.Object = rawListOf(runtime.RawExtension{Object: item})
				app := stack.NewApplication("raw", "default", &fakeAppConfig{objs: []*client.Object{&obj}})
				c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", app)}}
				rules := recursiveRules("nodeOnly", placement)
				_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, rules)
				if err == nil || !strings.Contains(err.Error(), "read list items") {
					t.Errorf("got %v, want a list read error", err)
				}
			})
		}
	}
}

// TestIntegrateWithLayout_RawListItemMustBeJSON: a raw item of a typed List
// that is not JSON cannot be read as the object it stands for, and the
// integration says so in place of leaving it out.
func TestIntegrateWithLayout_RawListItemMustBeJSON(t *testing.T) {
	for _, placement := range placements {
		t.Run(string(placement), func(t *testing.T) {
			var obj client.Object = rawListOf(runtime.RawExtension{Raw: []byte("{")})
			app := stack.NewApplication("raw", "default", &fakeAppConfig{objs: []*client.Object{&obj}})
			c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", app)}}
			rules := recursiveRules("nodeOnly", placement)
			_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, rules)
			if err == nil || !strings.Contains(err.Error(), "read list items") {
				t.Errorf("got %v, want a list read error", err)
			}
		})
	}
}

// rawItemSlice is a named slice type of raw items, as a hand-written list type
// can declare its Items field.
type rawItemSlice []runtime.RawExtension

// namedRawItems is a typed List whose Items field has a named slice type.
type namedRawItems struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Items             rawItemSlice `json:"items"`
}

func (l *namedRawItems) DeepCopyObject() runtime.Object {
	c := *l
	c.Items = slices.Clone(l.Items)
	return &c
}

// pointerRawItems is a typed List whose Items field is a pointer to the slice.
type pointerRawItems struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Items             *[]runtime.RawExtension `json:"items"`
}

func (l *pointerRawItems) DeepCopyObject() runtime.Object {
	c := *l
	if l.Items != nil {
		items := slices.Clone(*l.Items)
		c.Items = &items
	}
	return &c
}

// TestIntegrateWithLayout_RawItemFirstWhateverTheSliceType: an item that
// carries raw JSON and an object is read from the raw JSON, which is what the
// writers serialize, whatever the Go type of the List's Items field: a named
// slice type, or a pointer to the slice. A nil pointer cannot be read as a
// list of items and is refused with a read error.
func TestIntegrateWithLayout_RawItemFirstWhateverTheSliceType(t *testing.T) {
	listMeta := metav1.TypeMeta{APIVersion: "v1", Kind: "List"}
	item := func(t *testing.T) runtime.RawExtension {
		t.Helper()
		raw, err := json.Marshal(fluxKustomization("web", "elsewhere"))
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		return runtime.RawExtension{Raw: raw, Object: fluxKustomization("other", "platform")}
	}
	cases := map[string]struct {
		emitted func(t *testing.T) client.Object
		want    string
	}{
		"named slice type": {
			emitted: func(t *testing.T) client.Object {
				l := &namedRawItems{TypeMeta: listMeta, Items: rawItemSlice{item(t)}}
				l.Name, l.Namespace = "holder", "default"
				return l
			},
			want: `already has Flux Kustomization "web"`,
		},
		"pointer to the slice": {
			emitted: func(t *testing.T) client.Object {
				l := &pointerRawItems{TypeMeta: listMeta, Items: &[]runtime.RawExtension{item(t)}}
				l.Name, l.Namespace = "holder", "default"
				return l
			},
			want: `already has Flux Kustomization "web"`,
		},
		"nil pointer to the slice": {
			emitted: func(t *testing.T) client.Object {
				l := &pointerRawItems{TypeMeta: listMeta}
				l.Name, l.Namespace = "holder", "default"
				return l
			},
			want: "read list items",
		},
	}
	for name, tc := range cases {
		for _, placement := range placements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				obj := tc.emitted(t)
				platformApp := stack.NewApplication("platform-ks", "default", &fakeAppConfig{objs: []*client.Object{&obj}})
				web := &stack.Node{Name: "web", Bundle: srBundle("web", cmApp("web-app"))}
				c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", platformApp), Children: []*stack.Node{web}}}
				rules := propertyGroupings["nodeOnly"]
				rules.FluxPlacement = placement
				_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, rules)
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Errorf("got %v, want an error containing %q", err, tc.want)
				}
			})
		}
	}
}

// umbrellaCluster: prod -> apps (node) -> platform (umbrella bundle) with the
// child bundles infra and services, infra holding the child bundle network.
func umbrellaCluster() *stack.Cluster {
	network := srBundle("network", cmApp("network-app"))
	infra := srBundle("infra", cmApp("infra-app"))
	infra.Children = []*stack.Bundle{network}
	platform := srBundle("platform", cmApp("platform-app"))
	platform.Children = []*stack.Bundle{infra, srBundle("services", cmApp("services-app"))}
	apps := &stack.Node{Name: "apps", Bundle: platform}
	root := &stack.Node{Name: "demo", Children: []*stack.Node{apps}}
	apps.SetParent(root)
	return &stack.Cluster{Name: "demo", Node: root}
}

// umbrellaChildren returns every umbrella child layout of ml's tree.
func umbrellaChildren(ml *layout.ManifestLayout) []*layout.ManifestLayout {
	var out []*layout.ManifestLayout
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		if l.UmbrellaChild {
			out = append(out, l)
		}
		for _, c := range l.Children {
			walk(c)
		}
	}
	walk(ml)
	return out
}

// TestIntegratedTree_KeptKustomizationAppliesItsDirectory: a Kustomization
// the caller placed where the integration would place its own, with the same
// name and spec.path, is kept in place of a generated one. Its directory is
// applied by it, so the tree writes whatever form the kept object has.
func TestIntegratedTree_KeptKustomizationAppliesItsDirectory(t *testing.T) {
	for _, placement := range placements {
		for _, form := range []string{"typed", "unstructured", "in a List"} {
			t.Run(string(placement)+"/"+form, func(t *testing.T) {
				rules := recursiveRules("nodeOnly", placement)
				if placement == layout.FluxSeparate {
					t.Skip("the flux-system layout is the integration's own; a caller's copy there is refused")
				}

				// Where the integration places the CR of the umbrella child infra.
				first := integrated(t, umbrellaCluster(), rules)
				var hostPath string
				var generated *kustv1.Kustomization
				var find func(l *layout.ManifestLayout)
				find = func(l *layout.ManifestLayout) {
					for _, r := range l.Resources {
						if k, ok := r.(*kustv1.Kustomization); ok && k.Name == "infra" {
							hostPath, generated = l.FullRepoPath(), k
						}
					}
					for _, c := range l.Children {
						find(c)
					}
				}
				find(first)
				if generated == nil {
					t.Fatal("no generated Kustomization named infra")
				}

				cluster := umbrellaCluster()
				ml, err := layout.WalkCluster(cluster, rules)
				if err != nil {
					t.Fatalf("WalkCluster: %v", err)
				}
				content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(generated.DeepCopy())
				if err != nil {
					t.Fatalf("ToUnstructured: %v", err)
				}
				u := &unstructured.Unstructured{Object: content}
				u.SetAPIVersion(kustv1.GroupVersion.String())
				u.SetKind("Kustomization")
				var own client.Object
				switch form {
				case "typed":
					own = generated.DeepCopy()
				case "unstructured":
					own = u
				case "in a List":
					own = wrapInList(u)
				}
				host := layoutAtPath(t, ml, hostPath)
				host.Resources = append(host.Resources, own)

				if err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, cluster, rules); err != nil {
					t.Fatalf("IntegrateWithLayout: %v", err)
				}
				for writer, err := range writeErrs(t, ml) {
					if err != nil {
						t.Errorf("%s: %v", writer, err)
					}
				}
			})
		}
	}
}

// TestIntegratedTree_UnlistedDirectoriesAreApplied: the pre-write check
// refuses a directory its parent does not list and no Flux Kustomization
// builds (go-kure/kure#977). The integrator marks every directory one of its
// Kustomizations names, so each umbrella child of an integrated tree is
// marked and every writer accepts the tree, in every placement. The check is
// live on such a tree: with one umbrella child's mark taken away, every
// writer refuses it and names the directory.
func TestIntegratedTree_UnlistedDirectoriesAreApplied(t *testing.T) {
	for _, placement := range placements {
		t.Run(string(placement), func(t *testing.T) {
			rules := recursiveRules("nodeOnly", placement)

			ml := integrated(t, umbrellaCluster(), rules)
			children := umbrellaChildren(ml)
			if len(children) != 3 {
				t.Fatalf("got %d umbrella child layouts, want 3", len(children))
			}
			for _, c := range children {
				if !c.FluxBuild() {
					t.Errorf("umbrella child %q is not marked as built by a Flux Kustomization", c.FullRepoPath())
				}
			}
			writeAll(t, ml)

			for i := range children {
				ml := integrated(t, umbrellaCluster(), rules)
				child := umbrellaChildren(ml)[i]
				child.SetFluxBuild(false)
				want := `layout "` + child.FullRepoPath() + `" is not listed by its parent layout`
				for writer, err := range writeErrs(t, ml) {
					if err == nil || !strings.Contains(err.Error(), want) {
						t.Errorf("%s: got %v, want it to contain %q", writer, err, want)
					}
				}
			}
		})
	}
}
