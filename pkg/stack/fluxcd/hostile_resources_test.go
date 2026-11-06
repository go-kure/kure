package fluxcd_test

import (
	"encoding"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"runtime/debug"
	"slices"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// A caller may hand kure a resource that is hostile to being read: one whose
// method panics, one that formats without end, one that holds itself. Every
// entry point refuses such a resource, or answers as it did before it, and
// none panics or overflows the stack (go-kure/kure#1201). The cases are one
// matrix, each run through every entry point, so that a fix for one shape is
// checked against all of them. Each case runs in a child process, as a stack
// overflow is fatal and ends the test binary.

// kindPanics is a resource whose GetObjectKind panics with panicWith.
type kindPanics struct {
	corev1.ConfigMap
	panicWith any
}

func (k *kindPanics) GetObjectKind() schema.ObjectKind { panic(k.panicWith) }

func (k *kindPanics) DeepCopyObject() runtime.Object { c := *k; return &c }

// privateCycleList is a typed List with an item no object, and a private map
// that holds itself: formatting the list has no end.
type privateCycleList struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Items             []int `json:"items"`
	cycle             map[string]any
}

func (l *privateCycleList) DeepCopyObject() runtime.Object { c := *l; return &c }

// zeroPanics is a field whose IsZero panics, which encoding/json calls for a
// field tagged omitzero.
type zeroPanics struct{ V int }

func (zeroPanics) IsZero() bool { panic("IsZero") }

// zeroItem is an object whose serialization panics, held by a List that does
// not write it.
type zeroItem struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	X                 zeroPanics `json:"x,omitzero"`
}

func (o *zeroItem) DeepCopyObject() runtime.Object { c := *o; return &c }

// probeZeroHolder is a field whose IsZero formats a private map that holds
// itself, which has no end: the stack overflows, and nothing recovers that.
type probeZeroHolder struct{ held *unstructured.Unstructured }

func (p probeZeroHolder) IsZero() bool { return fmt.Sprint(p.held.Object) == "" }

// probeZeroResource holds that field with the omitzero tag, so encoding/json
// calls its IsZero when it writes the resource.
type probeZeroResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Probe             probeZeroHolder `json:"probe,omitzero"`
}

func (o *probeZeroResource) DeepCopyObject() runtime.Object { c := *o; return &c }

// unwrittenItems is a typed List whose Items are not written, and which
// writes another array of the same length under items.
type unwrittenItems struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Items             []*zeroItem `json:"-"`
	Written           []any       `json:"items"`
}

func (l *unwrittenItems) DeepCopyObject() runtime.Object { c := *l; return &c }

// hiddenProbeItems is a typed List like unwrittenItems, whose hidden items
// hold the field whose IsZero overflows the stack.
type hiddenProbeItems struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Items             []*probeZeroResource `json:"-"`
	Written           []any                `json:"items"`
}

func (l *hiddenProbeItems) DeepCopyObject() runtime.Object { c := *l; return &c }

// recursiveEmbed embeds itself and a trusted marshaler: checking whether its
// marshaler is promoted from a trusted type must not recurse without end.
type recursiveEmbed struct {
	*recursiveEmbed //nolint:unused // the embedding is the case under test
	*unstructured.Unstructured
}

// embedsUnstructured is a wrapper that takes its methods, DeepCopyObject
// included, from the *Unstructured it embeds.
type embedsUnstructured struct{ *unstructured.Unstructured }

// embedsValue embeds the Unstructured itself.
type embedsValue struct{ unstructured.Unstructured }

func hostileCases() map[string]func() client.Object {
	list := func() *objectItems {
		l := &objectItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}}
		l.Name, l.Namespace = "holder", "default"
		return l
	}
	badCM := func() *objectItems {
		cm := unstructuredObj("v1", "ConfigMap", "default", "cm")
		cm.Object["spec"] = map[string]any{"m": panicMarshaler{}}
		l := list()
		l.Items = []runtime.Object{nil, cm}
		return l
	}
	return map[string]func() client.Object{
		"typed nil resource":              func() client.Object { return (*corev1.ConfigMap)(nil) },
		"typed nil unstructured resource": func() client.Object { return (*unstructured.Unstructured)(nil) },
		"type that embeds itself and a trusted marshaler": func() client.Object {
			return &recursiveEmbed{Unstructured: unstructuredObj("v1", "ConfigMap", "default", "cm")}
		},
		"omitzero IsZero formats a map that holds itself": func() client.Object {
			u := unstructuredObj("v1", "ConfigMap", "default", "cm")
			u.Object["self"] = u.Object
			o := &probeZeroResource{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Thing"}, Probe: probeZeroHolder{held: u}}
			o.Name, o.Namespace = "holder", "default"
			return o
		},
		"wrapper embedding Unstructured holds a plain int": func() client.Object {
			u := unstructuredObj("v1", "ConfigMap", "default", "cm")
			u.Object["spec"] = map[string]any{"n": 7}
			return embedsUnstructured{u}
		},
		"wrapper embedding Unstructured by value holds a plain int": func() client.Object {
			u := unstructuredObj("v1", "ConfigMap", "default", "cm")
			u.Object["spec"] = map[string]any{"n": 7}
			return &embedsValue{*u}
		},
		"wrapper among the items of a typed List holds a plain int": func() client.Object {
			u := unstructuredObj("v1", "ConfigMap", "default", "cm")
			u.Object["spec"] = map[string]any{"n": 7}
			l := list()
			l.Items = []runtime.Object{embedsUnstructured{u}}
			return l
		},
		"typed nil wrapper embedding Unstructured": func() client.Object { return (*embedsUnstructured)(nil) },
		// A typed object that is no List holds an Unstructured with a plain int in
		// a RawExtension: its copy follows Object whatever Raw holds, and the
		// copy panics on the int (go-kure/kure#1252).
		"unstructured with a plain int held in a raw extension": func() client.Object {
			u := unstructuredObj("v1", "ConfigMap", "default", "cm")
			u.Object["spec"] = map[string]any{"n": 7}
			cr := &appsv1.ControllerRevision{Data: runtime.RawExtension{Raw: []byte("{}"), Object: u}}
			cr.Name, cr.Namespace = "held", "default"
			return cr
		},
		"embedded nil pointer": func() client.Object {
			return &struct{ *corev1.ConfigMap }{}
		},
		"GetObjectKind panics with a nil error": func() client.Object {
			return &kindPanics{panicWith: (*fs.PathError)(nil)}
		},
		"GetObjectKind panics with a string": func() client.Object {
			return &kindPanics{panicWith: "boom"}
		},
		"GetObjectKind panics with a map that holds itself": func() client.Object {
			m := map[string]any{}
			m["self"] = m
			return &kindPanics{panicWith: m}
		},
		"items of no object kind and a private cycle": func() client.Object {
			l := &privateCycleList{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}, Items: []int{1}}
			l.Name, l.Namespace = "holder", "default"
			l.cycle = map[string]any{}
			l.cycle["self"] = l.cycle
			return l
		},
		"list with a nil item before a marshaler": func() client.Object { return badCM() },
		"pointer items with a nil item": func() client.Object {
			items := badCM().Items
			l := &ptrItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}, Items: &items}
			l.Name, l.Namespace = "holder", "default"
			return l
		},
		"list that holds itself under a nil item": func() client.Object {
			l := list()
			l.Items = []runtime.Object{nil, l}
			return l
		},
		"list cycle through a raw extension": func() client.Object {
			l := list()
			l.Items = []runtime.Object{nil, &metav1.List{Items: []runtime.RawExtension{{Object: l}}}}
			return l
		},
		"very deep chain": func() client.Object {
			head := &chainNode{}
			head.Name, head.Namespace = "head", "default"
			for tail, i := head, 0; i < 3000; i++ {
				tail.Next = &chainNode{}
				tail = tail.Next
			}
			return head
		},
		"lists held twice": func() client.Object {
			var next runtime.Object = &metav1.List{}
			for i := 0; i < 60; i++ {
				l := list()
				l.Items = []runtime.Object{next, next}
				next = l
			}
			return next.(client.Object)
		},
		"marshaler of another package in a private field": func() client.Object {
			cm := unstructuredObj("v1", "ConfigMap", "default", "cm")
			o := &hidingHolder{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Thing"}}
			o.Hidden.inner.u = cm
			return o
		},
		"map key that marshals itself": func() client.Object {
			cm := unstructuredObj("v1", "ConfigMap", "default", "cm")
			return &keyedMarshaler{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Thing"}, Keyed: map[key]any{{U: cm}: nil}}
		},
		"interface map key that marshals itself": func() client.Object {
			cm := unstructuredObj("v1", "ConfigMap", "default", "cm")
			return &ifaceKeyed{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Thing"}, Keyed: map[encoding.TextMarshaler]string{key{U: cm}: "v"}}
		},
		"list whose unwritten item panics when serialized": func() client.Object {
			l := &unwrittenItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}, Items: []*zeroItem{{}}, Written: []any{map[string]any{"apiVersion": "v1", "kind": "ConfigMap"}}}
			l.Name, l.Namespace = "holder", "default"
			return l
		},
		"list whose hidden item formats a map that holds itself": func() client.Object {
			u := unstructuredObj("v1", "ConfigMap", "default", "cm")
			u.Object["self"] = u.Object
			item := &probeZeroResource{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Thing"}, Probe: probeZeroHolder{held: u}}
			item.Name, item.Namespace = "item", "default"
			l := &hiddenProbeItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}, Items: []*probeZeroResource{item}, Written: []any{map[string]any{"apiVersion": "v1", "kind": "ConfigMap"}}}
			l.Name, l.Namespace = "holder", "default"
			return l
		},
		"map key that holds itself": func() client.Object {
			m := map[string]any{}
			m["self"] = m
			return &keyedField{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Thing"}, Keyed: map[*map[string]any]any{&m: nil}}
		},
	}
}

// hostileCluster is a cluster whose application emits r, so that a layout
// walked from it carries the origins the entry points check.
func hostileCluster(r client.Object) *stack.Cluster {
	obj := r
	app := stack.NewApplication("hostile", "default", &fakeAppConfig{objs: []*client.Object{&obj}})
	return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "root", Bundle: srBundle("root", app)}}
}

// runHostile runs every entry point on a fresh layout walked from a cluster
// whose application emits r, for each Flux placement, and on a layout built by
// hand that holds r, and returns what each entry point answered, named. A walk
// that fails is reported as it came. The walker skips some objects (a typed
// nil), so only the hand-built layout reaches the entry points with those.
//
// A heavy case (heavyCases) runs on the first placement only, on the three
// entry points that call normalizeCallerObjects, and not on the hand-built
// layouts and CreateLayoutWithResources: see heavyCases.
func runHostile(r func() client.Object, heavy bool) (answers []string) {
	answer := func(entry string, err error) {
		if err != nil {
			answers = append(answers, entry+": "+err.Error())
		}
	}
	placements := allPlacements
	if heavy {
		placements = allPlacements[:1]
	}
	for _, placement := range placements {
		rules := withPlacement(nodeOnly, placement)
		walk := func() (*stack.Cluster, *layout.ManifestLayout, bool) {
			c := hostileCluster(r())
			ml, err := layout.WalkCluster(c, rules)
			answer(string(placement)+" walk", err)
			return c, ml, err == nil
		}
		if c, ml, ok := walk(); ok {
			_, err := fluxstack.NewResourceGenerator().GenerateFromLayout(ml, c)
			answer(string(placement)+" GenerateFromLayout", err)
		}
		if _, ml, ok := walk(); ok {
			_, err := fluxstack.UmbrellaTerminals(ml)
			answer(string(placement)+" UmbrellaTerminals", err)
		}
		if c, ml, ok := walk(); ok {
			answer(string(placement)+" IntegrateWithLayout", fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules))
		}
		if heavy {
			continue
		}
		_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(hostileCluster(r()), rules)
		answer(string(placement)+" CreateLayoutWithResources", err)
	}
	if heavy {
		return answers
	}
	// A layout built by hand holds r as it is, with no origins: the entry
	// points that need none answer for r itself.
	cluster := &stack.Cluster{Node: &stack.Node{Name: "root"}}
	rules := withPlacement(nodeOnly, layout.FluxSeparate)
	hand := func() *layout.ManifestLayout {
		return &layout.ManifestLayout{Name: "root", Resources: []client.Object{r()}}
	}
	_, err := fluxstack.NewResourceGenerator().GenerateFromLayout(hand(), cluster)
	answer("hand-built GenerateFromLayout", err)
	_, err = fluxstack.UmbrellaTerminals(hand())
	answer("hand-built UmbrellaTerminals", err)
	answer("hand-built IntegrateWithLayout", fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(hand(), cluster, rules))
	return answers
}

// refusalNames are the cases that every entry point must refuse, and what the
// refusal names.
var refusalNames = map[string]string{
	"typed nil resource":                                        "nil *v1.ConfigMap",
	"typed nil unstructured resource":                           "nil *unstructured.Unstructured",
	"typed nil wrapper embedding Unstructured":                  "nil *fluxcd_test.embedsUnstructured",
	"wrapper embedding Unstructured holds a plain int":          "fluxcd_test.embedsUnstructured",
	"wrapper embedding Unstructured by value holds a plain int": "fluxcd_test.embedsValue",
	"wrapper among the items of a typed List holds a plain int": "fluxcd_test.embedsUnstructured",
	"embedded nil pointer":                                      "struct { *v1.ConfigMap }",
	"GetObjectKind panics with a string":                        "fluxcd_test.kindPanics",
	"GetObjectKind panics with a nil error":                     "fluxcd_test.kindPanics",
	"marshaler of another package in a private field":           "hidingItem",
	"omitzero IsZero formats a map that holds itself":           "fluxcd_test.probeZeroHolder, whose IsZero method is called for its omitzero tag",
	"list whose hidden item formats a map that holds itself":    "fluxcd_test.probeZeroHolder, whose IsZero method is called for its omitzero tag",
}

// heavyCases are the cases that run to a bound of the walk (maxJSONNodes): each
// entry point reads that many values before it refuses, and at the real bound
// (four million) under -race that was most of the matrix's run time. The child
// lowers the bound to heavyBound, so the case still ends at the bound on every
// entry point; TestCallerObject_ListsHeldTwiceAreRefusedAtOnce keeps one walk
// at the real bound. The bound is enforced by one code path,
// walkHeld, which the three entry points below reach through
// normalizeCallerObjects, each by its own call, so each is still checked:
//   - GenerateFromLayout (resource_generator.go),
//   - UmbrellaTerminals (umbrella_terminals.go),
//   - IntegrateWithLayout (layout_integrator.go).
//
// What a heavy case skips repeats these: the other placements (the walk of the
// resource does not depend on the placement), the hand-built layouts (the same
// three entry points, on a layout without origins) and
// CreateLayoutWithResources (a WalkCluster followed by IntegrateWithLayout).
// The depth stays: the walk to the bound is what the case proves.
var heavyCases = map[string]bool{"lists held twice": true}

// heavyBound is the bound a heavy case runs at in its child process: far below
// the 2^60 values the case holds, so it still ends at the bound.
const heavyBound = 1 << 14

const hostileChildEnv = "KURE_HOSTILE_CASE"

// TestHostileResources_Child is the child process of TestHostileResources.
func TestHostileResources_Child(t *testing.T) {
	name := os.Getenv(hostileChildEnv)
	if name == "" {
		t.Skip("runs only as the child of TestHostileResources")
	}
	r, ok := hostileCases()[name]
	if !ok {
		t.Fatalf("no case %q", name)
	}
	if heavyCases[name] {
		defer fluxstack.SetMaxJSONNodesForTest(heavyBound)()
	}
	// A case that overflows the stack must end soon, not after the default 1 GB
	// of stack is used: lower the limit in this child only.
	debug.SetMaxStack(64 << 20)
	for _, a := range runHostile(r, heavyCases[name]) {
		fmt.Println("ANSWER " + a)
	}
}

func TestHostileResources_NoEntryPointPanics(t *testing.T) {
	cases := hostileCases()
	names := make([]string, 0, len(cases))
	for name := range cases {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			// Each case runs in a process of its own and reads nothing the
			// others write, so they run side by side.
			t.Parallel()
			cmd := exec.Command(os.Args[0], "-test.run=^TestHostileResources_Child$")
			cmd.Env = append(os.Environ(), hostileChildEnv+"="+name)
			out, err := cmd.CombinedOutput()
			if err != nil {
				lines := strings.Split(strings.TrimSpace(string(out)), "\n")
				t.Errorf("the child failed: %v\n%s", err, strings.Join(lines[:min(len(lines), 12)], "\n"))
				return
			}
			var answers []string
			for _, line := range strings.Split(string(out), "\n") {
				if a, ok := strings.CutPrefix(line, "ANSWER "); ok {
					answers = append(answers, a)
				}
			}
			for _, a := range answers {
				if !strings.HasPrefix(a, "hand-built") && strings.Contains(a, "build it with layout.WalkCluster") {
					t.Errorf("an entry point stopped at a missing origin, not at the resource: %s", a)
				}
			}
			// A refusal concerns the resource: the answers of these cases name it.
			if want, ok := refusalNames[name]; ok && !slices.ContainsFunc(answers, func(a string) bool { return strings.Contains(a, want) }) {
				t.Errorf("no entry point refused the resource naming %q; answers:\n%s", want, strings.Join(answers, "\n"))
			}
			// A layout built by hand keeps the resource whatever the walker skips,
			// so each of the three entry points must refuse it by name: one that
			// accepts it, or refuses it for another reason, is not hidden by the
			// others that do.
			if want, ok := refusalNames[name]; ok {
				for _, entry := range []string{"hand-built GenerateFromLayout", "hand-built UmbrellaTerminals", "hand-built IntegrateWithLayout"} {
					if !slices.ContainsFunc(answers, func(a string) bool { return strings.HasPrefix(a, entry+": ") && strings.Contains(a, want) }) {
						t.Errorf("%s did not refuse the resource naming %q; answers:\n%s", entry, want, strings.Join(answers, "\n"))
					}
				}
			}
		})
	}
}
