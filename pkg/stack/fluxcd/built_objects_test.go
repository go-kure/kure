package fluxcd_test

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
	fluxkustomize "github.com/fluxcd/pkg/kustomize"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// A health check's writer is read from what each Kustomization's build
// applies (go-kure/kure#1094 E): an object's identity as substituted, and a
// generated object under its hashed name.

// builtNames returns the names of the v1 objects of kind that k's build
// makes on the tree ml writes, built as kustomize-controller builds it.
func builtNames(t *testing.T, ml *layout.ManifestLayout, k *kustv1.Kustomization, kind string) []string {
	t.Helper()
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(k)
	if err != nil {
		t.Fatal(err)
	}
	tree := filepath.Join(t.TempDir(), "repo")
	if err := os.CopyFS(tree, os.DirFS(writeAll(t, ml)["WriteToDisk"].root)); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(tree, filepath.FromSlash(k.Spec.Path))
	if _, err := fluxkustomize.NewGenerator(tree, unstructured.Unstructured{Object: obj}).WriteFile(dir); err != nil {
		t.Fatalf("Flux generator on %q: %v", k.Spec.Path, err)
	}
	built, err := buildLikeFlux(t, tree, dir)
	if err != nil {
		t.Fatalf("build of %q: %v", k.Spec.Path, err)
	}
	var out []string
	for _, r := range built.Resources() {
		if r.GetApiVersion() == "v1" && r.GetKind() == kind {
			out = append(out, r.GetName())
		}
	}
	return out
}

// builtName returns the one name builtNames lists that starts with prefix.
func builtName(t *testing.T, names []string, prefix string) string {
	t.Helper()
	var out []string
	for _, n := range names {
		if strings.HasPrefix(n, prefix) {
			out = append(out, n)
		}
	}
	if len(out) != 1 {
		t.Fatalf("got %v built with prefix %q, want one", out, prefix)
	}
	return out[0]
}

// go-kure/kure#1145: b-longer writes the ConfigMap ${NS}/ready and depends on
// a, which health-checks red/ready. With NS=red set inline, b-longer's apply
// writes red/ready, the edge is certain and the cycle is refused. With NS
// read from the cluster, or set nowhere (so the namespace is empty), the
// writer stays uncertain. Written ${NS:=red}, the namespace is red where NS is
// set nowhere, and refused; read from the cluster, NS may override the
// default, and the writer stays uncertain.
func TestBuiltObjects_SubstitutedIdentityIsRead(t *testing.T) {
	fromCluster := []stack.SubstituteRef{{Kind: "ConfigMap", Name: "vars"}}
	cases := map[string]struct {
		namespace string
		postBuild *stack.PostBuild
		refused   bool
	}{
		"NS set inline":                       {postBuild: &stack.PostBuild{Substitute: map[string]string{"NS": "red"}}, refused: true},
		"NS read from the cluster":            {postBuild: &stack.PostBuild{SubstituteFrom: fromCluster}},
		"NS set nowhere":                      {postBuild: &stack.PostBuild{Substitute: map[string]string{"OTHER": "red"}}},
		"NS defaulted":                        {namespace: "${NS:=red}", postBuild: &stack.PostBuild{Substitute: map[string]string{"OTHER": "red"}}, refused: true},
		"NS defaulted, read from the cluster": {namespace: "${NS:=red}", postBuild: &stack.PostBuild{Substitute: map[string]string{"OTHER": "red"}, SubstituteFrom: fromCluster}},
	}
	for name, tc := range cases {
		for _, placement := range append(slices.Clone(integratedPlacements), layout.FluxSeparate) {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				namespace := tc.namespace
				if namespace == "" {
					namespace = "${NS}"
				}
				c := pairCluster(stack.HealthCheck{APIVersion: "v1", Kind: "ConfigMap", Namespace: "red", Name: "ready"},
					[]client.Object{unstructuredObj("v1", "ConfigMap", "default", "a-cm")},
					[]client.Object{unstructuredObj("v1", "ConfigMap", namespace, "ready")})
				c.Node.Children[1].Bundle.PostBuild = tc.postBuild
				err := integrateWalked(t, c, placement, nil)
				if tc.refused && (err == nil || !strings.Contains(err.Error(), "can never all become Ready")) {
					t.Fatalf("got %v, want the cycle through a's health check refused", err)
				}
				if !tc.refused && err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
			})
		}
	}
}

// b-longer writes the ConfigMap red/ready and depends on a, which
// health-checks it. The ConfigMap's kustomize.toolkit.fluxcd.io/ssa
// annotation or kustomize.toolkit.fluxcd.io/reconcile label is written with a
// defaulted variable, ${SKIP:=Ignore} or ${SKIP:=disabled}. Where SKIP is set
// nowhere the default applies, the apply skips the ConfigMap and the writer
// stays uncertain. Under a substituteFrom the cluster may set SKIP, to
// Override or enabled, so whether the apply skips it is not known: the skip
// is read as written, as main reads it, the edge is certain and the cycle is
// refused. A value set inline is not read either, nor one whose default
// nests a variable: only a literal skip holds, and only where the
// substitution leaves it as written; an inline quote that swallows it, or an
// inline anchor and alias that would move a value the cluster sets into it,
// leave no skip.
//
// The same holds for the key: Flux substitutes the whole object, keys
// included, so a key written ${KEY:=kustomize.toolkit.fluxcd.io/ssa}, or set
// inline, quoted or not, is not read as the skip key, the edge stays and the
// cycle is refused. A key holding a $, escaped, set inline or one the
// cluster may set, may become a skip key at apply and meet a written one:
// beside one no skip is read, even a literal skip.
//
// An object referencing any variable inline leaves unset reads no skip at
// all, even a literal one beside a value the cluster may set elsewhere: an
// expression the cluster controls may change the text around it at apply,
// as a substring of an inline quote whose length the cluster sets, which
// swallows the skip line. The same quote cut by a length set inline leaves
// it, and so does an inline anchor that nothing aliases: an object every
// variable of which is set inline applies as substituted offline.
//
// What may move an entry out of its map is not read either: an inline value
// with a line break YAML reads but Flux keeps (CR, U+0085, U+2028, U+2029),
// referenced by the ConfigMap, refuses it, and an unreferenced one does not;
// a key at its top level the cluster may set refuses it, since it may become
// its kind; a metadata key the cluster may set, which may become its
// annotations, reads no skip beside it, a written one does. An inline key <<
// merging a skip is a key holding a $, and reads none.
//
// Without a substituteFrom the object is read as Flux substitutes it, from
// the file kure writes: an inline carriage return in an annotation moves a
// skip into the annotations, and the writer stays uncertain, but not from the
// kubectl.kubernetes.io/last-applied-configuration annotation, which the
// writer strips, so Flux never sees it and the cycle is refused.
func TestBuiltObjects_SubstitutedSkipIsRead(t *testing.T) {
	const ssa, reconcile = "kustomize.toolkit.fluxcd.io/ssa", "kustomize.toolkit.fluxcd.io/reconcile"
	fromCluster := []stack.SubstituteRef{{Kind: "ConfigMap", Name: "vars"}}
	cases := map[string]struct {
		label bool
		key   string
		value string
		extra map[string]string
		// other, if set, is the other map: the annotations where label is
		// set, the labels otherwise.
		other     map[string]string
		postBuild *stack.PostBuild
		refused   bool
		// topKey and metaKey, if set, are a key at the ConfigMap's top level
		// and one in its metadata; wantErr, if set, is the refusal expected
		// in place of the cycle's.
		topKey, metaKey, wantErr string
	}{
		"ssa certain beside a key the cluster may set, read from the cluster": {value: "Ignore", extra: map[string]string{"example.com/tag${SUFFIX}": "x"},
			postBuild: &stack.PostBuild{Substitute: map[string]string{"OTHER": "red"}, SubstituteFrom: fromCluster}, refused: true},
		"ssa certain beside an inline key, read from the cluster": {value: "Ignore", extra: map[string]string{"example.com/tag${TAIL}": "x"},
			postBuild: &stack.PostBuild{Substitute: map[string]string{"TAIL": "x"}, SubstituteFrom: fromCluster}, refused: true},
		"ssa certain beside an escaped key, read from the cluster": {value: "Ignore", extra: map[string]string{"example.com/tag$${TAIL}": "x"},
			postBuild: &stack.PostBuild{Substitute: map[string]string{"OTHER": "red"}, SubstituteFrom: fromCluster}, refused: true},
		"ssa certain swallowed by an inline quote, read from the cluster": {value: "Ignore", extra: map[string]string{"a": "${QUOTE}", "z": "${QUOTE}"},
			postBuild: &stack.PostBuild{Substitute: map[string]string{"QUOTE": `"`}, SubstituteFrom: fromCluster}, refused: true},
		"ssa aliased to an inline anchor, read from the cluster": {label: true, key: ssa, value: "${ALIAS}",
			other:     map[string]string{"example.com/value": "${ANCHOR}${SKIP:=Ignore}"},
			postBuild: &stack.PostBuild{Substitute: map[string]string{"ANCHOR": "&skip ", "ALIAS": "*skip"}, SubstituteFrom: fromCluster}, refused: true},
		"ssa certain beside an inline anchor, read from the cluster": {label: true, key: ssa, value: "Ignore",
			other:     map[string]string{"example.com/value": "${ANCHOR}x"},
			postBuild: &stack.PostBuild{Substitute: map[string]string{"ANCHOR": "&skip "}, SubstituteFrom: fromCluster}},
		"ssa certain beside a value the cluster may set, read from the cluster": {label: true, key: ssa, value: "Ignore",
			other:     map[string]string{"example.com/value": "${VALUE:=x}"},
			postBuild: &stack.PostBuild{Substitute: map[string]string{"OTHER": "red"}, SubstituteFrom: fromCluster}, refused: true},
		"ssa certain swallowed by a quote the cluster exposes, read from the cluster": {value: "Ignore",
			extra:     map[string]string{"a": "${QUOTE:0:${LEN:=0}}x", "z": "x${QUOTE:0:${LEN:=0}}"},
			postBuild: &stack.PostBuild{Substitute: map[string]string{"QUOTE": `"`}, SubstituteFrom: fromCluster}, refused: true},
		"ssa certain beside a quote cut inline, read from the cluster": {value: "Ignore",
			extra:     map[string]string{"a": "${QUOTE:0:${LEN:=0}}x", "z": "x${QUOTE:0:${LEN:=0}}"},
			postBuild: &stack.PostBuild{Substitute: map[string]string{"QUOTE": `"`, "LEN": "0"}, SubstituteFrom: fromCluster}},
		"ssa key the cluster may turn into a written one, read from the cluster": {value: "Ignore", extra: map[string]string{ssa + "${TAIL/${CUT:=y}/}": "Override"},
			postBuild: &stack.PostBuild{Substitute: map[string]string{"TAIL": "x"}, SubstituteFrom: fromCluster}, refused: true},
		"ssa written beside an inline key with a cluster value, read from the cluster": {value: "Ignore", extra: map[string]string{"${KEY}": "${SKIP:=Override}"},
			postBuild: &stack.PostBuild{Substitute: map[string]string{"KEY": ssa}, SubstituteFrom: fromCluster}, refused: true},
		"ssa key nesting an unset default, read from the cluster": {key: "${KEY:=${FALLBACK}}", value: "Ignore",
			postBuild: &stack.PostBuild{Substitute: map[string]string{"KEY": ssa}, SubstituteFrom: fromCluster}, refused: true},
		"ssa key meeting a written one, read from the cluster": {value: "Override", extra: map[string]string{ssa + "${SUFFIX:=}": "Ignore"},
			postBuild: &stack.PostBuild{Substitute: map[string]string{"OTHER": "red"}, SubstituteFrom: fromCluster}, refused: true},
		"ssa defaulted":                              {value: "${SKIP:=Ignore}", postBuild: &stack.PostBuild{Substitute: map[string]string{"OTHER": "red"}}},
		"ssa defaulted, read from the cluster":       {value: "${SKIP:=Ignore}", postBuild: &stack.PostBuild{Substitute: map[string]string{"OTHER": "red"}, SubstituteFrom: fromCluster}, refused: true},
		"ssa set inline, read from the cluster":      {value: "${SKIP:=Override}", postBuild: &stack.PostBuild{Substitute: map[string]string{"SKIP": "Ignore"}, SubstituteFrom: fromCluster}, refused: true},
		"reconcile defaulted":                        {label: true, value: "${SKIP:=disabled}", postBuild: &stack.PostBuild{Substitute: map[string]string{"OTHER": "red"}}},
		"reconcile defaulted, read from the cluster": {label: true, value: "${SKIP:=disabled}", postBuild: &stack.PostBuild{Substitute: map[string]string{"OTHER": "red"}, SubstituteFrom: fromCluster}, refused: true},
		"ssa key defaulted":                          {key: "${KEY:=" + ssa + "}", value: "Ignore", postBuild: &stack.PostBuild{Substitute: map[string]string{"OTHER": "red"}}},
		"ssa key defaulted, read from the cluster":   {key: "${KEY:=" + ssa + "}", value: "Ignore", postBuild: &stack.PostBuild{Substitute: map[string]string{"OTHER": "red"}, SubstituteFrom: fromCluster}, refused: true},
		"reconcile key defaulted, read from the cluster": {label: true, key: "${KEY:=" + reconcile + "}", value: "disabled",
			postBuild: &stack.PostBuild{Substitute: map[string]string{"OTHER": "red"}, SubstituteFrom: fromCluster}, refused: true},
		"ssa key set inline, value defaulted, read from the cluster": {key: "${KEY}", value: "${SKIP:=Ignore}",
			postBuild: &stack.PostBuild{Substitute: map[string]string{"KEY": ssa}, SubstituteFrom: fromCluster}, refused: true},
		"ssa key and value set inline, read from the cluster": {key: "${KEY}", value: "${SKIP}",
			postBuild: &stack.PostBuild{Substitute: map[string]string{"KEY": ssa, "SKIP": "Ignore"}, SubstituteFrom: fromCluster}, refused: true},
		"ssa key set inline single-quoted, value defaulted, read from the cluster": {key: "${KEY}", value: "${SKIP:=Ignore}",
			postBuild: &stack.PostBuild{Substitute: map[string]string{"KEY": "'" + ssa + "'"}, SubstituteFrom: fromCluster}, refused: true},
		"ssa key set inline double-quoted, value defaulted, read from the cluster": {key: "${KEY}", value: "${SKIP:=Ignore}",
			postBuild: &stack.PostBuild{Substitute: map[string]string{"KEY": `"` + ssa + `"`}, SubstituteFrom: fromCluster}, refused: true},
		"ssa key set inline quoted, value set inline, read from the cluster": {key: "${KEY}", value: "${SKIP}",
			postBuild: &stack.PostBuild{Substitute: map[string]string{"KEY": "'" + ssa + "'", "SKIP": "Ignore"}, SubstituteFrom: fromCluster}, refused: true},
		"reconcile disabled beside an ssa the cluster may set, read from the cluster": {label: true, value: "disabled", extra: map[string]string{ssa: "${SKIP:=Override}"},
			postBuild: &stack.PostBuild{Substitute: map[string]string{"OTHER": "red"}, SubstituteFrom: fromCluster}, refused: true},
		"ssa key set inline across a carriage return, read from the cluster": {key: "${KEY}", value: "${SKIP:=Ignore}",
			postBuild: &stack.PostBuild{Substitute: map[string]string{"KEY": "note: x\r  labels:\r    " + ssa}, SubstituteFrom: fromCluster}, wantErr: "holding a line break Flux keeps"},
		"ssa key set inline across a U+0085, read from the cluster": {key: "${KEY}", value: "${SKIP:=Ignore}",
			postBuild: &stack.PostBuild{Substitute: map[string]string{"KEY": "note: x\u0085  labels:\u0085    " + ssa}, SubstituteFrom: fromCluster}, wantErr: "holding a line break Flux keeps"},
		"ssa key set inline across a U+2028, read from the cluster": {key: "${KEY}", value: "${SKIP:=Ignore}",
			postBuild: &stack.PostBuild{Substitute: map[string]string{"KEY": "note: x\u2028  labels:\u2028    " + ssa}, SubstituteFrom: fromCluster}, wantErr: "holding a line break Flux keeps"},
		"ssa key set inline across a U+2029, read from the cluster": {key: "${KEY}", value: "${SKIP:=Ignore}",
			postBuild: &stack.PostBuild{Substitute: map[string]string{"KEY": "note: x\u2029  labels:\u2029    " + ssa}, SubstituteFrom: fromCluster}, wantErr: "holding a line break Flux keeps"},
		"ssa certain beside an unreferenced inline line break, read from the cluster": {value: "Ignore",
			postBuild: &stack.PostBuild{Substitute: map[string]string{"UNUSED": "a\rb"}, SubstituteFrom: fromCluster}},
		"type key the cluster may set": {value: "Ignore", topKey: "kind${SUFFIX:=Not}",
			postBuild: &stack.PostBuild{Substitute: map[string]string{"OTHER": "red"}, SubstituteFrom: fromCluster}, wantErr: "has a key at its top level"},
		"type key the cluster may not set": {value: "Ignore", topKey: "kindNot",
			postBuild: &stack.PostBuild{Substitute: map[string]string{"OTHER": "red"}, SubstituteFrom: fromCluster}},
		"ssa certain beside a metadata key the cluster may set": {value: "Ignore", metaKey: "x${S:=y}",
			postBuild: &stack.PostBuild{Substitute: map[string]string{"OTHER": "red"}, SubstituteFrom: fromCluster}, refused: true},
		"ssa certain beside a written metadata key": {value: "Ignore", metaKey: "xy",
			postBuild: &stack.PostBuild{Substitute: map[string]string{"OTHER": "red"}, SubstituteFrom: fromCluster}},
		"ssa merged under an inline key, read from the cluster": {key: "${KEY}", value: "${VALUE:=${UNSET}}",
			postBuild: &stack.PostBuild{Substitute: map[string]string{"KEY": "<<", "VALUE": "{" + ssa + ": Ignore}"}, SubstituteFrom: fromCluster}, refused: true},
		"ssa nested default unused, read from the cluster": {value: "${SKIP:=${FALLBACK:=Override}}",
			postBuild: &stack.PostBuild{Substitute: map[string]string{"SKIP": "Ignore"}, SubstituteFrom: fromCluster}, refused: true},
		"ssa set inline across a carriage return in an annotation the writer keeps": {key: "example.com/note", value: "${PAYLOAD}",
			postBuild: &stack.PostBuild{Substitute: map[string]string{"PAYLOAD": "x\r    " + ssa + ": Ignore"}}},
		"ssa set inline across a carriage return in an annotation the writer strips": {key: "kubectl.kubernetes.io/last-applied-configuration", value: "${PAYLOAD}",
			postBuild: &stack.PostBuild{Substitute: map[string]string{"PAYLOAD": "x\r    " + ssa + ": Ignore"}}, refused: true},
	}
	for name, tc := range cases {
		for _, placement := range append(slices.Clone(integratedPlacements), layout.FluxSeparate) {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				ready := unstructuredObj("v1", "ConfigMap", "red", "ready")
				key := tc.key
				switch {
				case key != "":
				case tc.label:
					key = reconcile
				default:
					key = ssa
				}
				metadata := map[string]string{key: tc.value}
				maps.Copy(metadata, tc.extra)
				if tc.label {
					ready.SetLabels(metadata)
					ready.SetAnnotations(tc.other)
				} else {
					ready.SetAnnotations(metadata)
					ready.SetLabels(tc.other)
				}
				if tc.topKey != "" {
					ready.Object[tc.topKey] = "ignored"
				}
				if tc.metaKey != "" {
					ready.Object["metadata"].(map[string]any)[tc.metaKey] = "x"
				}
				c := pairCluster(stack.HealthCheck{APIVersion: "v1", Kind: "ConfigMap", Namespace: "red", Name: "ready"},
					[]client.Object{unstructuredObj("v1", "ConfigMap", "default", "a-cm")},
					[]client.Object{ready})
				c.Node.Children[1].Bundle.PostBuild = tc.postBuild
				err := integrateWalked(t, c, placement, nil)
				if tc.wantErr != "" {
					if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
						t.Fatalf("got %v, want the ConfigMap refused: %s", err, tc.wantErr)
					}
					return
				}
				if tc.refused && (err == nil || !strings.Contains(err.Error(), "can never all become Ready")) {
					t.Fatalf("got %v, want the cycle through a's health check refused", err)
				}
				if !tc.refused && err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
			})
		}
	}
}

// A Kustomization that is no member, but whose build is the tree's, may be
// the certain writer: a's application emits h, which builds a's directory
// with a component adding default/h-cm, which nothing else applies, and
// b-longer health-checks h-cm. Under UmbrellasDoNotWait, which follows the
// other Kustomizations of the tree, b-longer waits for h's apply; when h
// depends on b-longer the cycle is refused, naming it, and without that
// dependsOn the tree is admitted.
func TestBuiltObjects_NonMemberIsTheCertainWriter(t *testing.T) {
	for _, dependsOnB := range []bool{true, false} {
		for _, placement := range integratedPlacements {
			name := "h independent/"
			if dependsOnB {
				name = "h depends on b-longer/"
			}
			t.Run(name+string(placement), func(t *testing.T) {
				ml, err := integrateNonMemberWriter(t, placement, dependsOnB, nil)
				if dependsOnB && (err == nil || !strings.Contains(err.Error(), "can never all become Ready") || !strings.Contains(err.Error(), "apply flux-system/h")) {
					t.Fatalf("got %v, want the cycle through b-longer's health check on what h writes refused; tree %s", err, describeKustomizations(ml))
				}
				if !dependsOnB && err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
			})
		}
	}
}

// integrateNonMemberWriter integrates, under UmbrellasDoNotWait, the tree of
// TestBuiltObjects_NonMemberIsTheCertainWriter: a emits h, whose build of a's
// directory adds h-cm through a component, and b-longer, without wait,
// health-checks h-cm; with dependsOnB h depends on b-longer. a also emits
// each of others.
func integrateNonMemberWriter(t *testing.T, placement layout.FluxPlacement, dependsOnB bool, others func(dir string) []*kustv1.Kustomization) (*layout.ManifestLayout, error) {
	t.Helper()
	c := pairCluster(stack.HealthCheck{},
		[]client.Object{unstructuredObj("v1", "ConfigMap", "default", "a-cm")},
		[]client.Object{unstructuredObj("v1", "ConfigMap", "default", "b-cm")})
	c.Node.Children[0].Bundle.HealthChecks = nil
	// No dependsOn on a, which the setting would write as one on h:
	// only the existence edge onto h's apply can close the cycle.
	c.Node.Children[1].Bundle.DependsOn = nil
	// A leaf waits under the setting, and Flux ignores health checks
	// under wait.
	c.Node.Children[1].Bundle.Wait = boolPtr(false)
	c.Node.Children[1].Bundle.HealthChecks = []stack.HealthCheck{{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "h-cm"}}
	rules := withPlacement(nodeOnly, placement)
	ml, err := layout.WalkCluster(c, rules)
	if err != nil {
		t.Fatalf("WalkCluster: %v", err)
	}
	a := findLayout(ml, "a-cm")
	if a == nil {
		t.Fatal("no layout holds a-cm")
	}
	var h *kustv1.Kustomization
	if dependsOnB {
		h = handWritten("h", "b-longer")
	} else {
		h = handWritten("h")
	}
	h.Spec.Path = "./" + a.FullRepoPath()
	h.Spec.Components = []string{"component"}
	a.Resources = append(a.Resources, h)
	if others != nil {
		for _, o := range others("./" + a.FullRepoPath()) {
			a.Resources = append(a.Resources, o)
		}
	}
	// a's kustomization.yaml lists neither file: only h's build,
	// through its component, holds h-cm.
	a.ExtraFiles = append(a.ExtraFiles,
		layout.ExtraFile{Name: "component/kustomization.yaml", Content: []byte("apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\nresources:\n- h-cm.yaml\n")},
		layout.ExtraFile{Name: "component/h-cm.yaml", Content: []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: h-cm\n  namespace: default\n")})
	return ml, fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules)
}

// A Kustomization on another Source that builds a's directory is no writer:
// its build is not the tree's, so it is read as the directory holds it, which
// shows no h-cm. A transform may still make one (spec.components; spec.patches
// over a directory that holds objects; a name prefix over one that holds a
// ConfigMap), so h stops being the certain writer, the existence edge goes
// and the tree is admitted. Without a transform h stays certain and the cycle
// is refused.
func TestBuiltObjects_AnotherSourcesTransformLeavesTheWriterUncertain(t *testing.T) {
	for name, tc := range map[string]struct {
		edit    func(k *kustv1.Kustomization)
		refused bool
	}{
		"components": {edit: func(k *kustv1.Kustomization) { k.Spec.Components = []string{"component"} }},
		"patches": {edit: func(k *kustv1.Kustomization) {
			k.Spec.Patches = []kustomize.Patch{{Patch: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: a-cm\n", Target: &kustomize.Selector{Kind: "ConfigMap"}}}
		}},
		"name prefix":  {edit: func(k *kustv1.Kustomization) { k.Spec.NamePrefix = "h-" }},
		"no transform": {edit: func(*kustv1.Kustomization) {}, refused: true},
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				ml, err := integrateNonMemberWriter(t, placement, true, func(dir string) []*kustv1.Kustomization {
					o := handWritten("other")
					o.Spec.Path = dir
					o.Spec.SourceRef = kustv1.CrossNamespaceSourceReference{Kind: "GitRepository", Name: "other-repo"}
					tc.edit(o)
					return []*kustv1.Kustomization{o}
				})
				if tc.refused && (err == nil || !strings.Contains(err.Error(), "apply flux-system/h")) {
					t.Fatalf("got %v, want the cycle through h refused; tree %s", err, describeKustomizations(ml))
				}
				if !tc.refused && err != nil {
					t.Fatalf("got %v, want the tree admitted: other may write h-cm", err)
				}
			})
		}
	}
}

// A Kustomization on another Source that builds a's directory with a
// component, other, is read as its first copy an apply does not skip: a's
// copy carries kustomize.toolkit.fluxcd.io/ssa Ignore, so a's apply skips it.
// Where p, which also builds a's directory, removes the mark, other applies
// with its component and h stops being the certain writer: the tree is
// admitted. Where p also gives other a spec.kubeConfig, other applies on
// another cluster, h stays certain and the cycle is refused; so it is where no
// p applies other at all.
func TestBuiltObjects_AnotherSourceIsReadAsItsAppliedCopy(t *testing.T) {
	const unskip = "- op: remove\n  path: /metadata/annotations/kustomize.toolkit.fluxcd.io~1ssa\n"
	for name, c := range map[string]struct {
		patch   string
		refused bool
	}{
		"applied here":           {patch: unskip},
		"applied on another one": {patch: unskip + "- op: add\n  path: /spec/kubeConfig\n  value:\n    secretRef:\n      name: remote-kubeconfig\n", refused: true},
		"never applied":          {refused: true},
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				ml, err := integrateNonMemberWriter(t, placement, true, func(dir string) []*kustv1.Kustomization {
					o := handWritten("other")
					o.Spec.Path, o.Spec.Components = dir, []string{"component"}
					o.Spec.SourceRef = kustv1.CrossNamespaceSourceReference{Kind: "GitRepository", Name: "other-repo"}
					o.Annotations = map[string]string{"kustomize.toolkit.fluxcd.io/ssa": "Ignore"}
					out := []*kustv1.Kustomization{o}
					if c.patch != "" {
						p := handWritten("p")
						p.Spec.Path = dir
						p.Spec.Patches = []kustomize.Patch{{Patch: c.patch, Target: &kustomize.Selector{Kind: "Kustomization", Name: "other"}}}
						out = append(out, p)
					}
					return out
				})
				if c.refused && (err == nil || !strings.Contains(err.Error(), "apply flux-system/h")) {
					t.Fatalf("got %v, want the cycle through h refused; tree %s", err, describeKustomizations(ml))
				}
				if !c.refused && err != nil {
					t.Fatalf("got %v, want the tree admitted: other may write h-cm", err)
				}
			})
		}
	}
}

// A Kustomization that is no writer and names a single-file (AppFileSingle)
// layout builds nothing (go-kure/kure#1197): Flux finds no directory there.
// helper, an AppFileSingle child of a's directory, holds h-cm, which a's
// build deletes and h's build of a's directory keeps, so h is its certain
// writer; b-longer health-checks h-cm and h depends on b-longer, and the cycle
// is refused. other, on another Source, names helper's path: it applies
// nothing, so h stays certain and the cycle is still refused.
func TestBuiltObjects_ANonWriterOnAFileWritesNothing(t *testing.T) {
	for name, withOther := range map[string]bool{"without other": false, "other names the file": true} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				c := pairCluster(stack.HealthCheck{},
					[]client.Object{unstructuredObj("v1", "ConfigMap", "default", "a-cm")},
					[]client.Object{unstructuredObj("v1", "ConfigMap", "default", "b-cm")})
				a := c.Node.Children[0].Bundle
				a.HealthChecks = nil
				a.Patches = []stack.Patch{{
					Patch:  "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: h-cm\n  namespace: default\n$patch: delete\n",
					Target: &stack.PatchSelector{Kind: "ConfigMap", Name: "h-cm"},
				}}
				b := c.Node.Children[1].Bundle
				b.DependsOn = nil
				b.Wait = boolPtr(false)
				b.HealthChecks = []stack.HealthCheck{{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "h-cm"}}
				rules := withPlacement(nodeOnly, placement)
				ml, err := layout.WalkCluster(c, rules)
				if err != nil {
					t.Fatalf("WalkCluster: %v", err)
				}
				dir := findLayout(ml, "a-cm")
				if dir == nil {
					t.Fatal("no layout holds a-cm")
				}
				dir.Children = append(dir.Children, &layout.ManifestLayout{
					Name: "helper", Namespace: dir.FullRepoPath(), ApplicationFileMode: layout.AppFileSingle,
					Resources: []client.Object{unstructuredObj("v1", "ConfigMap", "default", "h-cm")},
				})
				h := handWritten("h", "b-longer")
				h.Spec.Path = "./" + dir.FullRepoPath()
				dir.Resources = append(dir.Resources, h)
				if withOther {
					o := handWritten("other")
					o.Spec.Path = "./" + dir.FullRepoPath() + "/helper"
					o.Spec.SourceRef = kustv1.CrossNamespaceSourceReference{Kind: "GitRepository", Name: "other-repo"}
					dir.Resources = append(dir.Resources, o)
				}
				err = fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules)
				if err == nil || !strings.Contains(err.Error(), "apply flux-system/h") {
					t.Fatalf("got %v, want the cycle through h refused; tree %s", err, describeKustomizations(ml))
				}
			})
		}
	}
}

// keptPair integrates pairCluster with no health check, lets edit change the
// layout that holds b-cm and b-longer's kept Kustomization, makes a's kept
// Kustomization health-check the v1 object of kind in default that
// b-longer's build makes under a name starting with prefix, and re-integrates.
func keptPair(t *testing.T, placement layout.FluxPlacement, bObjs []client.Object, kind, prefix string, edit func(l *layout.ManifestLayout, b *kustv1.Kustomization)) error {
	t.Helper()
	return keptPairChecking(t, placement, bObjs, kind, func(ml *layout.ManifestLayout, b *kustv1.Kustomization) string {
		return builtName(t, builtNames(t, ml, b, kind), prefix)
	}, edit)
}

// keptPairChecking is keptPair with the checked object's name given by name,
// from the edited tree and b-longer's kept Kustomization.
func keptPairChecking(t *testing.T, placement layout.FluxPlacement, bObjs []client.Object, kind string, name func(ml *layout.ManifestLayout, b *kustv1.Kustomization) string, edit func(l *layout.ManifestLayout, b *kustv1.Kustomization)) error {
	t.Helper()
	c := pairCluster(stack.HealthCheck{}, []client.Object{unstructuredObj("v1", "ConfigMap", "default", "a-cm")}, bObjs)
	c.Node.Children[0].Bundle.HealthChecks = nil
	rules := withPlacement(nodeOnly, placement)
	ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, rules)
	if err != nil {
		t.Fatalf("CreateLayoutWithResources: %v", err)
	}
	byName := kustomizationsByName(ml)
	a, b := byName["a"], byName["b-longer"]
	if a == nil || b == nil {
		t.Fatalf("no a or b-longer in %s", describeKustomizations(ml))
	}
	l := findLayout(ml, "b-cm")
	if l == nil {
		t.Fatal("no layout holds b-cm")
	}
	edit(l, b)
	checks("v1", kind, "default", name(ml, b))(a)
	return fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
}

// b-longer generates the ConfigMap settings, which its apply writes as
// settings plus a content hash, and a health-checks that name. Built as
// kustomize-controller builds it (here, for its target namespace) b-longer's
// result holds the hashed name, the edge is certain and the cycle refused. A
// b-longer whose build transforms nothing is read as the tree holds it, which
// computes no hash: a ConfigMap whose name starts with settings- stays
// uncertain.
func TestBuiltObjects_GeneratedNameIsRead(t *testing.T) {
	for _, transformed := range []bool{true, false} {
		for _, placement := range integratedPlacements {
			name := "transforms nothing/"
			if transformed {
				name = "target namespace/"
			}
			t.Run(name+string(placement), func(t *testing.T) {
				err := keptPair(t, placement, []client.Object{unstructuredObj("v1", "ConfigMap", "default", "b-cm")}, "ConfigMap", "settings-",
					func(l *layout.ManifestLayout, b *kustv1.Kustomization) {
						l.ConfigMapGenerators = append(l.ConfigMapGenerators, layout.ConfigMapGeneratorSpec{Name: "settings", Files: []string{"values.yaml"}})
						l.ExtraFiles = append(l.ExtraFiles, layout.ExtraFile{Name: "values.yaml", Content: []byte("k: v\n")})
						if transformed {
							b.Spec.TargetNamespace = "default"
						}
					})
				if transformed && (err == nil || !strings.Contains(err.Error(), "can never all become Ready")) {
					t.Fatalf("got %v, want the cycle through a's health check refused", err)
				}
				if !transformed && err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
			})
		}
	}
}

// b-longer writes the Secret default/creds, and a component of its build
// generates a Secret over it. a health-checks the Secret b-longer's build
// makes. Which Secret a merge (behavior: merge) changes, and under which
// name, kure does not read (go-kure/kure#1181): b-longer's build is not read,
// so the tree is refused naming it, however the merge is spelled (a key in
// another case) and whatever else the component sets (annotations over the
// Secret), whichever namespace or name suffix the merge lands under, and
// also where a configMapGenerator reads the loaded component as data too. A
// generator creating another Secret, extra, merges nothing: the edge is
// certain and the cycle is refused. A component no build loads merges
// nothing either: with b-longer's spec.components empty, read as the tree
// holds it or built for its target namespace, creds is the literal Secret
// b-longer writes, and the cycle is refused. So it is when a
// configMapGenerator only reads the component's file as data, also when it
// reads every kustomization file name of that directory as data (each one a
// kustomization kure parses: a file of such a name that does not parse makes
// the build not read, go-kure/kure#1168).
func TestBuiltObjects_MergingBuildIsNotRead(t *testing.T) {
	creds := unstructuredObj("v1", "Secret", "default", "creds")
	creds.Object["data"] = map[string]any{"base": "dg=="}
	const merge = "- name: creds\n  namespace: default\n  behavior: merge\n  literals:\n  - k=v\n"
	cases := map[string]struct {
		generator, prefix, suffix, targetNamespace   string
		unused, dataFile, everyName, refused, unread bool
		// blue, if set, has b-longer also write blue/creds; checked, if
		// set, is the exact name a health-checks in default.
		blue    bool
		checked string
	}{
		"merge":                          {generator: merge, prefix: "creds", unread: true},
		"merge, a key in another case":   {generator: strings.Replace(merge, "behavior:", "Behavior:", 1), prefix: "creds", unread: true},
		"merge, annotations over it":     {generator: merge + "commonAnnotations:\n  kure.go-kure.github.io/merge-probe: fixed\n", prefix: "creds", unread: true},
		"merge in another namespace":     {generator: strings.Replace(merge, "namespace: default", "namespace: blue", 1), blue: true, checked: "creds", unread: true},
		"merge, name suffix x":           {generator: merge, prefix: "credsx", suffix: "x", unread: true},
		"merge, component also data":     {generator: merge, prefix: "creds", dataFile: true, unread: true},
		"create":                         {generator: "- name: extra\n  namespace: default\n  literals:\n  - k=v\n", prefix: "extra-", refused: true},
		"merge, component unused":        {generator: merge, prefix: "creds", unused: true, refused: true},
		"merge, component unused, built": {generator: merge, prefix: "creds", unused: true, targetNamespace: "default", refused: true},
		"merge, read only as data":       {generator: merge, prefix: "creds", unused: true, dataFile: true, targetNamespace: "default", refused: true},
		"merge, every name read as data": {generator: merge, prefix: "creds", unused: true, dataFile: true, everyName: true, targetNamespace: "default", refused: true},
	}
	for name, tc := range cases {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				objs := []client.Object{unstructuredObj("v1", "ConfigMap", "default", "b-cm"), creds.DeepCopy()}
				name := func(ml *layout.ManifestLayout, b *kustv1.Kustomization) string {
					return builtName(t, builtNames(t, ml, b, "Secret"), tc.prefix)
				}
				if tc.blue {
					blue := creds.DeepCopy()
					blue.SetNamespace("blue")
					objs = append(objs, blue)
				}
				if tc.checked != "" {
					name = func(*layout.ManifestLayout, *kustv1.Kustomization) string { return tc.checked }
				}
				err := keptPairChecking(t, placement, objs, "Secret", name,
					func(l *layout.ManifestLayout, b *kustv1.Kustomization) {
						l.ExtraFiles = append(l.ExtraFiles, layout.ExtraFile{
							Name:    "component/kustomization.yaml",
							Content: []byte("apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\nsecretGenerator:\n" + tc.generator),
						})
						if !tc.unused {
							b.Spec.Components = []string{"component"}
						}
						if tc.dataFile {
							files := []string{"component/kustomization.yaml"}
							if tc.everyName {
								for _, n := range []string{"component/kustomization.yml", "component/Kustomization"} {
									l.ExtraFiles = append(l.ExtraFiles, layout.ExtraFile{Name: n, Content: []byte("apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n")})
									files = append(files, n)
								}
							}
							l.ConfigMapGenerators = append(l.ConfigMapGenerators, layout.ConfigMapGeneratorSpec{Name: "data", Files: files})
						}
						b.Spec.NameSuffix = tc.suffix
						b.Spec.TargetNamespace = tc.targetNamespace
					})
				if tc.unread {
					if err == nil || !strings.Contains(err.Error(), "is not read") || !strings.Contains(err.Error(), "component/kustomization.yaml, which may merge into a Secret") {
						t.Fatalf("got %v, want b-longer's build not read", err)
					}
					return
				}
				if tc.refused && (err == nil || !strings.Contains(err.Error(), "can never all become Ready")) {
					t.Fatalf("got %v, want the cycle through a's health check refused", err)
				}
				if !tc.refused && err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
			})
		}
	}
}

// b-longer applies child, written with kind ${ALIAS} and a field a:
// ${ANCHOR}${TYPE:=ConfigMap}, under a substituteFrom. ANCHOR set inline to
// "&t " and ALIAS to "*t" make the kind an alias of a's value, which the
// cluster may set to Kustomization: offline child is a ConfigMap, at apply it
// may be a Flux Kustomization, so its type cannot be read from the tree, and
// the tree is refused, naming the kind; the same through its apiVersion. The
// alias may come from the text the inline values make rather than from one
// of them: a kind ${EMPTY:=*t}, ${EMPTY}*t or ${V/a/*t}, with EMPTY "" and V
// "a", is *t as Flux substitutes it, and is refused too. With TYPE set inline
// too, or a kind written literally, or a kind that references
// no variable left to the cluster, child is what kure substitutes, and the
// tree is admitted.
func TestBuiltObjects_AliasedTypeIsRefused(t *testing.T) {
	fromCluster := []stack.SubstituteRef{{Kind: "ConfigMap", Name: "vars"}}
	anchor := map[string]string{"ANCHOR": "&t ", "ALIAS": "*t"}
	cases := map[string]struct {
		apiVersion, kind, a string
		inline              map[string]string
		wantErr             string
	}{
		"kind aliased to a cluster value": {apiVersion: "kustomize.toolkit.fluxcd.io/v1", kind: "${ALIAS}", a: "${ANCHOR}${TYPE:=ConfigMap}",
			inline: anchor, wantErr: "in its kind"},
		"apiVersion aliased to a cluster value": {apiVersion: "${ALIAS}", kind: "Kustomization", a: "${ANCHOR}${API:=v1}",
			inline: anchor, wantErr: "in its apiVersion"},
		"kind aliased by a variable's default": {apiVersion: "kustomize.toolkit.fluxcd.io/v1", kind: "${EMPTY:=*t}", a: "${ANCHOR}${TYPE:=ConfigMap}",
			inline: map[string]string{"ANCHOR": "&t ", "EMPTY": ""}, wantErr: "in its kind"},
		"kind aliased by the text around a variable": {apiVersion: "kustomize.toolkit.fluxcd.io/v1", kind: "${EMPTY}*t", a: "${ANCHOR}${TYPE:=ConfigMap}",
			inline: map[string]string{"ANCHOR": "&t ", "EMPTY": ""}, wantErr: "in its kind"},
		"kind aliased by a replacement": {apiVersion: "kustomize.toolkit.fluxcd.io/v1", kind: "${V/a/*t}", a: "${ANCHOR}${TYPE:=ConfigMap}",
			inline: map[string]string{"ANCHOR": "&t ", "V": "a"}, wantErr: "in its kind"},
		"kind aliased to an inline value": {apiVersion: "kustomize.toolkit.fluxcd.io/v1", kind: "${ALIAS}", a: "${ANCHOR}${TYPE:=ConfigMap}",
			inline: map[string]string{"ANCHOR": "&t ", "ALIAS": "*t", "TYPE": "ConfigMap"}},
		"kind written beside a cluster value": {apiVersion: "kustomize.toolkit.fluxcd.io/v1", kind: "ConfigMap", a: "${ANCHOR}${TYPE:=ConfigMap}",
			inline: anchor},
	}
	for name, tc := range cases {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				child := unstructuredObj(tc.apiVersion, tc.kind, "default", "child")
				child.Object["a"] = tc.a
				c := pairCluster(stack.HealthCheck{APIVersion: "v1", Kind: "ConfigMap", Namespace: "red", Name: "ready"},
					[]client.Object{unstructuredObj("v1", "ConfigMap", "default", "a-cm")},
					[]client.Object{unstructuredObj("v1", "ConfigMap", "default", "b-cm"), child})
				c.Node.Children[1].Bundle.PostBuild = &stack.PostBuild{Substitute: tc.inline, SubstituteFrom: fromCluster}
				err := integrateWalked(t, c, placement, nil)
				if tc.wantErr == "" {
					if err != nil {
						t.Fatalf("got %v, want the tree admitted", err)
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), `Flux Kustomization "b-longer"`) || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("got %v, want child refused %s", err, tc.wantErr)
				}
			})
		}
	}
}

// b-longer substitutes a variable whose name Flux refuses
// (kustomize_varsub.go:230-233), so its substitution fails as Flux's does.
// The refusal names b-longer, whose spec needs the fix, not only the object
// it was substituting.
func TestBuiltObjects_FailedSubstitutionNamesTheKustomization(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			c := pairCluster(stack.HealthCheck{APIVersion: "v1", Kind: "ConfigMap", Namespace: "red", Name: "ready"},
				[]client.Object{unstructuredObj("v1", "ConfigMap", "default", "a-cm")},
				[]client.Object{unstructuredObj("v1", "ConfigMap", "red", "ready")})
			c.Node.Children[1].Bundle.PostBuild = &stack.PostBuild{Substitute: map[string]string{"BAD-NAME": "x"}}
			err := integrateWalked(t, c, placement, nil)
			if err == nil || !strings.Contains(err.Error(), `Flux Kustomization "b-longer"`) || !strings.Contains(err.Error(), "var name is invalid") {
				t.Fatalf("got %v, want the invalid variable refused, naming b-longer", err)
			}
		})
	}
}
