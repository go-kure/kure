package fluxcd_test

import (
	"fmt"
	"maps"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
)

// bootstrapLabelsInput is the BootstrapConfig.Labels the tests set: a plain
// key, a prefixed key, a value YAML would read as a boolean unquoted, and an
// empty value, which Kubernetes admits.
func bootstrapLabelsInput() map[string]string {
	return map[string]string{
		"team":              "platform",
		"example.com/tier":  "root",
		"example.com/flag":  "true",
		"example.com/empty": "",
	}
}

// operatorLabels are the labels flux-operator v0.62.1's LabelTransformer
// stamps (internal/builder/templates.go:371-383, labelsTmpl), for the
// namespace and version the tests build with.
func operatorLabels(namespace string) map[string]string {
	return map[string]string{
		"app.kubernetes.io/managed-by": "flux-operator",
		"app.kubernetes.io/instance":   namespace,
		"app.kubernetes.io/version":    fluxstack.GotkVersion,
		"app.kubernetes.io/part-of":    "flux",
	}
}

// labelsSources are the two source kinds, with a URL and ref each.
var labelsSources = map[string]struct{ kind, url, ref string }{
	"OCIRepository": {"OCIRepository", "oci://registry.example.com/fleet", "v1.2.3"},
	"GitRepository": {"GitRepository", "https://git.example.com/fleet.git", "main"},
}

// labelsConfig is a bootstrap config in mode with a source of kind and labels.
func labelsConfig(mode, kind string, labels map[string]string) *stack.BootstrapConfig {
	s := labelsSources[kind]
	return &stack.BootstrapConfig{
		Enabled:     true,
		FluxMode:    mode,
		FluxVersion: fluxstack.GotkVersion,
		Registry:    "ghcr.io/fluxcd",
		SourceKind:  s.kind,
		SourceURL:   s.url,
		SourceRef:   s.ref,
		Labels:      labels,
	}
}

// TestBootstrapLabels_Gotk: the labels are metadata.labels on the gotk
// source, of either kind, and on the bootstrap Kustomization, and nothing else
// changes from the same config without them.
func TestBootstrapLabels_Gotk(t *testing.T) {
	for kind := range labelsSources {
		for _, syncName := range []string{"", "fleet"} {
			t.Run(fmt.Sprintf("%s/sync %q", kind, syncName), func(t *testing.T) {
				bg := fluxstack.NewBootstrapGenerator()
				root := &stack.Node{Name: "prod"}
				plainConfig := labelsConfig(fluxstack.ModeGotk, kind, nil)
				plainConfig.SyncName = syncName
				plain, err := bg.GenerateBootstrap(plainConfig, root, accessRules)
				if err != nil {
					t.Fatalf("GenerateBootstrap without labels: %v", err)
				}
				config := labelsConfig(fluxstack.ModeGotk, kind, bootstrapLabelsInput())
				config.SyncName = syncName
				got, err := bg.GenerateBootstrap(config, root, accessRules)
				if err != nil {
					t.Fatalf("GenerateBootstrap: %v", err)
				}
				if len(got) != len(plain) {
					t.Fatalf("%d objects, want %d as without labels", len(got), len(plain))
				}
				labelled := 0
				for i := range got {
					want := plain[i].DeepCopyObject().(client.Object)
					switch got[i].(type) {
					case *sourcev1.OCIRepository, *sourcev1.GitRepository, *kustv1.Kustomization:
						labelled++
						want.SetLabels(bootstrapLabelsInput())
					}
					if !reflect.DeepEqual(got[i], want) {
						t.Errorf("object %d (%s %s):\n got %#v\nwant %#v", i,
							got[i].GetObjectKind().GroupVersionKind().Kind, got[i].GetName(), got[i], want)
					}
				}
				if labelled != 2 {
					t.Errorf("%d labelled objects, want the source and the Kustomization", labelled)
				}
			})
		}
	}
}

// TestBootstrapLabels_GotkNotShared: the emitted objects share no map with
// BootstrapConfig.Labels or with each other.
func TestBootstrapLabels_GotkNotShared(t *testing.T) {
	config := labelsConfig(fluxstack.ModeGotk, "OCIRepository", bootstrapLabelsInput())
	objs, err := fluxstack.NewBootstrapGenerator().GenerateBootstrap(config, &stack.Node{Name: "prod"}, accessRules)
	if err != nil {
		t.Fatalf("GenerateBootstrap: %v", err)
	}
	src := findObject[*sourcev1.OCIRepository](t, objs)
	kust := findObject[*kustv1.Kustomization](t, objs)
	config.Labels["team"] = "changed in the config"
	src.Labels["team"] = "changed on the source"
	if got := kust.Labels["team"]; got != "platform" {
		t.Errorf("Kustomization label team = %q after editing the config and the source, want platform", got)
	}
	if got := config.Labels["team"]; got != "changed in the config" {
		t.Errorf("config label team = %q after editing the source, want the config's own edit", got)
	}
}

// TestBootstrapLabels_FluxOperator: through both entry points, for either
// source kind and an explicit or the default sync name, the FluxInstance gains
// exactly two patches after any other, one on the sync source and one on the
// sync Kustomization, and both objects come out of the operator's patching
// with the labels; nothing else of the FluxInstance changes.
func TestBootstrapLabels_FluxOperator(t *testing.T) {
	on := true
	for entry, generate := range fluxInstanceEntryPoints {
		for kind := range labelsSources {
			for _, syncName := range []string{"", "fleet"} {
				for _, prune := range []*bool{nil, &on} {
					t.Run(fmt.Sprintf("%s/%s/sync %q/prune set %t", entry, kind, syncName, prune != nil), func(t *testing.T) {
						plainConfig := labelsConfig(fluxstack.DefaultFluxMode, kind, nil)
						plainConfig.SyncName, plainConfig.Prune = syncName, prune
						plain, err := generate(plainConfig)
						if err != nil {
							t.Fatalf("without labels: %v", err)
						}
						config := labelsConfig(fluxstack.DefaultFluxMode, kind, bootstrapLabelsInput())
						config.SyncName, config.Prune = syncName, prune
						got, err := generate(config)
						if err != nil {
							t.Fatalf("with labels: %v", err)
						}
						var before []kustomize.Patch
						if plain.Spec.Kustomize != nil {
							before = plain.Spec.Kustomize.Patches
						}
						if got.Spec.Kustomize == nil || len(got.Spec.Kustomize.Patches) != len(before)+2 {
							t.Fatalf("spec.kustomize = %#v, want %d patches", got.Spec.Kustomize, len(before)+2)
						}
						patches := got.Spec.Kustomize.Patches
						if len(before) > 0 && !reflect.DeepEqual(patches[:len(before)], before) {
							t.Errorf("patches before the labels ones = %#v, want %#v", patches[:len(before)], before)
						}
						applied := syncName
						if applied == "" {
							applied = got.Namespace
						}
						for i, want := range []kustomize.Selector{
							{Group: sourcev1.GroupVersion.Group, Kind: kind, Name: applied},
							{Group: kustv1.GroupVersion.Group, Kind: kustv1.KustomizationKind, Name: applied},
						} {
							if target := patches[len(before)+i].Target; target == nil || *target != want {
								t.Errorf("labels patch %d target %#v, want %#v", i, target, want)
							}
						}
						source, kust := appliedSync(t, got, applied)
						for name, obj := range map[string]map[string]any{"source": source, "Kustomization": kust} {
							if labels := metadataLabels(obj); !reflect.DeepEqual(labels, asGeneric(t, bootstrapLabelsInput())) {
								t.Errorf("sync %s labels after the patches = %#v, want %#v", name, labels, bootstrapLabelsInput())
							}
						}
						want := plain.DeepCopy()
						want.Spec.Kustomize = got.Spec.Kustomize // the patches are checked above
						if !reflect.DeepEqual(got, want) {
							t.Errorf("FluxInstance:\n got %#v\nwant %#v", got.Spec, want.Spec)
						}
					})
				}
			}
		}
	}
}

// TestBootstrapLabels_NoSourceURL: without a SourceURL the labels have no
// effect, as SyncName has none: gotk emits the Kustomization alone and without
// labels, and the FluxInstance has no sync and no patch, through both entry
// points.
func TestBootstrapLabels_NoSourceURL(t *testing.T) {
	render := func(t *testing.T, mode string, labels map[string]string) []client.Object {
		t.Helper()
		config := labelsConfig(mode, "OCIRepository", labels)
		config.SourceURL, config.SourceRef = "", ""
		objs, err := fluxstack.NewBootstrapGenerator().GenerateBootstrap(config, &stack.Node{Name: "prod"}, accessRules)
		if err != nil {
			t.Fatalf("GenerateBootstrap: %v", err)
		}
		return objs
	}
	for _, mode := range []string{fluxstack.ModeGotk, fluxstack.DefaultFluxMode} {
		t.Run(mode, func(t *testing.T) {
			if got, plain := render(t, mode, bootstrapLabelsInput()), render(t, mode, nil); !reflect.DeepEqual(got, plain) {
				t.Errorf("output with labels and no SourceURL differs from the output without labels")
			}
		})
	}
	for entry, generate := range fluxInstanceEntryPoints {
		t.Run(entry, func(t *testing.T) {
			config := labelsConfig(fluxstack.DefaultFluxMode, "OCIRepository", bootstrapLabelsInput())
			config.SourceURL = ""
			fi, err := generate(config)
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			if fi.Spec.Sync != nil || fi.Spec.Kustomize != nil {
				t.Errorf("sync %#v, kustomize %#v; want neither without a SourceURL", fi.Spec.Sync, fi.Spec.Kustomize)
			}
		})
	}
}

// TestBootstrapLabels_Refused: a key flux-operator owns, a key that is not a
// qualified name and a value that is not a label value are refused in both
// modes and through both FluxInstance entry points, with or without a
// SourceURL, with nothing returned and an error naming the field and the
// label. A refusal of an operator-owned key names all four.
func TestBootstrapLabels_Refused(t *testing.T) {
	reserved := []string{
		"app.kubernetes.io/managed-by",
		"app.kubernetes.io/instance",
		"app.kubernetes.io/version",
		"app.kubernetes.io/part-of",
	}
	type refusal struct {
		labels map[string]string
		names  []string
	}
	cases := map[string]refusal{
		"key with a space":           {map[string]string{"bad key": "x"}, []string{"labels", `"bad key"`}},
		"key with an empty prefix":   {map[string]string{"/team": "x"}, []string{"labels", `"/team"`}},
		"key name too long":          {map[string]string{strings.Repeat("k", 64): "x"}, []string{"labels", strings.Repeat("k", 64)}},
		"value with a space":         {map[string]string{"team": "platform team"}, []string{"labels", `"team"`, `"platform team"`}},
		"value too long":             {map[string]string{"team": strings.Repeat("v", 64)}, []string{"labels", `"team"`}},
		"value starting with a dash": {map[string]string{"team": "-platform"}, []string{"labels", `"team"`}},
	}
	for _, key := range reserved {
		cases["operator key "+key] = refusal{
			labels: map[string]string{"team": "platform", key: "consumer"},
			names:  append([]string{"labels", `"` + key + `"`}, reserved...),
		}
	}
	check := func(t *testing.T, got any, err error, names []string) {
		t.Helper()
		if err == nil {
			t.Fatal("error = nil, want a refusal")
		}
		if !reflect.ValueOf(got).IsNil() {
			t.Errorf("returned %v next to the error", got)
		}
		if !errors.IsType(err, errors.ErrorTypeResource) {
			t.Errorf("error type = %T (%v), want a pkg/errors resource validation error", err, err)
		}
		for _, name := range names {
			if !strings.Contains(err.Error(), name) {
				t.Errorf("error %q does not name %s", err, name)
			}
		}
	}
	for name, tc := range cases {
		for _, withURL := range []bool{true, false} {
			edit := func(c *stack.BootstrapConfig) {
				if !withURL {
					c.SourceURL, c.SourceRef = "", ""
				}
			}
			suffix := fmt.Sprintf("/with SourceURL %t", withURL)
			t.Run("gotk/"+name+suffix, func(t *testing.T) {
				config := labelsConfig(fluxstack.ModeGotk, "OCIRepository", maps.Clone(tc.labels))
				edit(config)
				objs, err := fluxstack.NewBootstrapGenerator().GenerateBootstrap(config, &stack.Node{Name: "prod"}, accessRules)
				check(t, objs, err, tc.names)
			})
			for entry, generate := range fluxInstanceEntryPoints {
				t.Run(entry+"/"+name+suffix, func(t *testing.T) {
					config := labelsConfig(fluxstack.DefaultFluxMode, "OCIRepository", maps.Clone(tc.labels))
					edit(config)
					fi, err := generate(config)
					check(t, fi, err, tc.names)
				})
			}
		}
	}
}

// TestBootstrapLabels_RefusalIsDeterministic: with several refused labels the
// error is about the first key in sorted order, on every run and in both modes.
func TestBootstrapLabels_RefusalIsDeterministic(t *testing.T) {
	labels := map[string]string{
		"zz bad key":                   "x",
		"app.kubernetes.io/version":    "v1",
		"app.kubernetes.io/managed-by": "consumer",
		"team":                         "bad value",
	}
	for _, mode := range []string{fluxstack.ModeGotk, fluxstack.DefaultFluxMode} {
		t.Run(mode, func(t *testing.T) {
			var first string
			for range 20 {
				_, err := fluxstack.NewBootstrapGenerator().GenerateBootstrap(
					labelsConfig(mode, "OCIRepository", labels), &stack.Node{Name: "prod"}, accessRules)
				if err == nil {
					t.Fatal("error = nil, want a refusal")
				}
				if first == "" {
					first = err.Error()
					if !strings.Contains(first, `label "app.kubernetes.io/managed-by"`) {
						t.Errorf("error %q is not about the first key in sorted order", first)
					}
				} else if err.Error() != first {
					t.Fatalf("error changed between runs:\n%s\n%s", first, err)
				}
			}
		})
	}
}

// operatorLabelsTransformer is flux-operator v0.62.1's labelsTmpl
// (internal/builder/templates.go:371-383) rendered for namespace and the
// vendored Flux version.
func operatorLabelsTransformer(namespace string) string {
	return strings.NewReplacer("NS", namespace, "VERSION", fluxstack.GotkVersion).Replace(`apiVersion: builtin
kind: LabelTransformer
metadata:
  name: labels
labels:
  app.kubernetes.io/managed-by: flux-operator
  app.kubernetes.io/instance: NS
  app.kubernetes.io/version: "VERSION"
  app.kubernetes.io/part-of: flux
fieldSpecs:
  - path: metadata/labels
    create: true
`)
}

// buildOperatorSyncLabels builds patches over sync as flux-operator's builder
// lays its kustomization out (templates.go:12-56, :164): the sync objects as a
// resource, the LabelTransformer under transformers:, the FluxInstance's
// patches under patches:. It returns each built object's labels by
// "Kind/name". With withTransformer false the transformers: entry is left out.
func buildOperatorSyncLabels(t *testing.T, patches []kustomize.Patch, sync, namespace string, withTransformer bool) map[string]map[string]string {
	t.Helper()
	files := map[string]string{"sync.yaml": sync, "labels.yaml": operatorLabelsTransformer(namespace)}
	kustomization := "resources:\n  - sync.yaml\n"
	if withTransformer {
		kustomization = "transformers:\n  - labels.yaml\n" + kustomization
	}
	if len(patches) > 0 {
		written, err := sigsyaml.Marshal(patches)
		if err != nil {
			t.Fatalf("marshalling the patches: %v", err)
		}
		kustomization += "patches:\n" + string(written)
	}
	files["kustomization.yaml"] = kustomization
	fs := filesys.MakeFsInMemory()
	for file, data := range files {
		if err := fs.WriteFile(filepath.Join("/sync", file), []byte(data)); err != nil {
			t.Fatalf("writing %s: %v", file, err)
		}
	}
	res, err := krusty.MakeKustomizer(krusty.MakeDefaultOptions()).Run(fs, "/sync")
	if err != nil {
		t.Fatalf("kustomize build: %v", err)
	}
	out := map[string]map[string]string{}
	for _, r := range res.Resources() {
		out[r.GetKind()+"/"+r.GetName()] = r.GetLabels()
	}
	return out
}

// labelsSync is the sync source and Kustomization as flux-operator's sync
// template writes them (templates.go:425-480), named name in namespace, with
// the labels pre on both when it is not empty, and a second OCIRepository a
// patch must not reach.
func labelsSync(name, namespace, pre string) string {
	meta := "  name: " + name + "\n  namespace: " + namespace + "\n"
	if pre != "" {
		meta += "  labels:\n    " + pre + "\n"
	}
	return strings.NewReplacer("META\n", meta, "NAME", name, "NS", namespace).Replace(`apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata:
META
spec:
  interval: 1h0m0s
  ref:
    tag: v1.2.3
  url: oci://registry.example.com/fleet
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
META
spec:
  interval: 10m0s
  path: ./demo
  prune: true
  sourceRef:
    kind: OCIRepository
    name: NAME
---
apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata:
  name: other
  namespace: NS
spec:
  interval: 1h0m0s
  url: oci://registry.example.com/other
`)
}

// TestBootstrapLabels_OperatorTransformerRunsLast pins the order kure's labels
// patches rely on, in a kustomize build laid out as flux-operator's builder
// lays out its own: kustomize runs the transformers: entries, the operator's
// LabelTransformer, after the builtin patches, so
//   - the sync source and Kustomization carry the consumer's labels and the
//     operator's four together, although each patch adds the whole
//     metadata.labels map;
//   - a patch setting one of the operator's keys loses to the transformer,
//     which is why BootstrapConfig.Labels refuses those keys.
//
// The first holds only because the patch runs first: an add on
// /metadata/labels replaces a map already there, which the last case shows.
// If this fails after a kustomize bump, the order changed and the labels
// patches would drop the operator's labels or keep a consumer's value.
func TestBootstrapLabels_OperatorTransformerRunsLast(t *testing.T) {
	bg := fluxstack.NewBootstrapGenerator()
	ns := bg.DefaultNamespace
	fi, err := bg.GenerateFluxInstance(
		labelsConfig(fluxstack.DefaultFluxMode, "OCIRepository", bootstrapLabelsInput()), &stack.Node{Name: "prod"}, accessRules)
	if err != nil {
		t.Fatalf("GenerateFluxInstance: %v", err)
	}
	patches := fi.Spec.Kustomize.Patches

	t.Run("consumer and operator labels together", func(t *testing.T) {
		got := buildOperatorSyncLabels(t, patches, labelsSync(ns, ns, ""), ns, true)
		want := maps.Clone(bootstrapLabelsInput())
		maps.Copy(want, operatorLabels(ns))
		for _, id := range []string{"OCIRepository/" + ns, "Kustomization/" + ns} {
			if !reflect.DeepEqual(got[id], want) {
				t.Errorf("%s labels = %v, want %v", id, got[id], want)
			}
		}
		if other := got["OCIRepository/other"]; !reflect.DeepEqual(other, operatorLabels(ns)) {
			t.Errorf("OCIRepository/other labels = %v, want the operator's alone", other)
		}
	})

	t.Run("the transformer overwrites an operator key a patch sets", func(t *testing.T) {
		consumer := kustomize.Patch{
			Patch:  "- op: add\n  path: /metadata/labels\n  value:\n    app.kubernetes.io/managed-by: consumer\n    team: platform\n",
			Target: patches[0].Target,
		}
		got := buildOperatorSyncLabels(t, []kustomize.Patch{consumer}, labelsSync(ns, ns, ""), ns, true)
		labels := got["OCIRepository/"+ns]
		if labels["app.kubernetes.io/managed-by"] != "flux-operator" || labels["team"] != "platform" {
			t.Errorf("sync source labels = %v, want managed-by flux-operator and team platform", labels)
		}
	})

	t.Run("an add on metadata.labels replaces the map", func(t *testing.T) {
		got := buildOperatorSyncLabels(t, patches, labelsSync(ns, ns, "pre: existing"), ns, false)
		for _, id := range []string{"OCIRepository/" + ns, "Kustomization/" + ns} {
			if !reflect.DeepEqual(got[id], bootstrapLabelsInput()) {
				t.Errorf("%s labels = %v, want the patch's alone, the existing label gone", id, got[id])
			}
		}
	})
}

// TestBootstrapLabels_FluxInstanceEncoded: the labels patches survive the
// FluxInstance's own YAML round trip with every value a string, "true" and ""
// included.
func TestBootstrapLabels_FluxInstanceEncoded(t *testing.T) {
	fi, err := fluxstack.NewBootstrapGenerator().GenerateFluxInstance(
		labelsConfig(fluxstack.DefaultFluxMode, "OCIRepository", bootstrapLabelsInput()), &stack.Node{Name: "prod"}, accessRules)
	if err != nil {
		t.Fatalf("GenerateFluxInstance: %v", err)
	}
	data, err := sigsyaml.Marshal(fi)
	if err != nil {
		t.Fatalf("marshalling the FluxInstance: %v", err)
	}
	read := &fluxv1.FluxInstance{}
	if err := sigsyaml.Unmarshal(data, read); err != nil {
		t.Fatalf("reading the FluxInstance back: %v", err)
	}
	for _, p := range read.Spec.Kustomize.Patches {
		if p.Target == nil || !strings.Contains(p.Patch, "/metadata/labels") {
			continue
		}
		var body []struct {
			Value map[string]string `json:"value"`
		}
		if err := sigsyaml.Unmarshal([]byte(p.Patch), &body); err != nil {
			t.Fatalf("reading the %s labels patch: %v", p.Target.Kind, err)
		}
		if len(body) != 1 || !reflect.DeepEqual(body[0].Value, bootstrapLabelsInput()) {
			t.Errorf("%s labels patch reads back as %+v, want %v", p.Target.Kind, body, bootstrapLabelsInput())
		}
	}
}

// TestBootstrapLabels_StoredOutputUnchanged: a config without labels renders
// the stored output of the same config, so Labels left unset changes nothing
// (TestBootstrap_OutputUnchanged keeps those files).
func TestBootstrapLabels_StoredOutputUnchanged(t *testing.T) {
	for _, name := range []string{"gotk-oci", "gotk-git", "flux-operator-oci", "flux-operator-git"} {
		t.Run(name, func(t *testing.T) {
			config := bootstrapOutputConfigs()[name+"-labels"]
			config.Labels = nil
			objs, err := fluxstack.NewBootstrapGenerator().GenerateBootstrap(config, &stack.Node{Name: "prod"}, accessRules)
			if err != nil {
				t.Fatalf("GenerateBootstrap: %v", err)
			}
			compareWithStored(t, filepath.Join("testdata", "bootstrap-output", name+".txt"), bootstrapBytes(t, objs))
		})
	}
}
