package fluxcd_test

import (
	"bytes"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"testing"

	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// The bootstrap mode comparison of go-kure/kure#1068: one config rendered in
// both modes, reduced to the root a cluster ends up with, the source and the
// Kustomization that applies the tree. Design:
// docs/history/20261025-DESIGN-bootstrap-contract.md.

// rootView is the root one mode makes Flux create, field by field, as the
// cluster sees it. In gotk mode that is the objects kure emits; in
// flux-operator mode it is what flux-operator v0.58.1 renders from the
// FluxInstance's sync (internal/builder/templates.go: source and
// Kustomization both named sync.name, or the FluxInstance's namespace when
// that is empty; OCI ref as ref.tag, Git ref as ref.name; Kustomization
// interval 10m0s and prune true, fixed).
type rootView map[string]string

// rootFields is the order the comparison lists the fields in. same marks the
// fields both modes must agree on: they decide what the cluster pulls and
// applies. The others are recorded, so a difference is visible and a change
// in either mode shows up in the stored file.
var rootFields = []struct {
	name string
	same bool
}{
	{"documents", false},
	{"source", false},
	{"source.kind", true},
	{"source.namespace", true},
	{"source.url", true},
	{"source.ref", false},
	{"source.revision", true},
	{"source.interval", true},
	{"source.secretRef", true},
	{"source.provider", true},
	{"source.insecure", true},
	{"kustomization", false},
	{"kustomization.namespace", true},
	{"kustomization.sourceRef", false},
	{"kustomization.sourceRef.namespace", true},
	{"kustomization.path", false},
	{"kustomization.directory", true},
	{"kustomization.prune", false},
	{"kustomization.interval", false},
	{"patches", false},
}

func gotkRoot(t *testing.T, objs []client.Object) rootView {
	t.Helper()
	v := rootView{}
	var sources, kusts int
	for _, obj := range objs {
		switch o := obj.(type) {
		case *sourcev1.OCIRepository:
			sources++
			v["source"] = "OCIRepository " + o.Namespace + "/" + o.Name
			v["source.kind"] = "OCIRepository"
			v["source.namespace"] = o.Namespace
			v["source.url"] = o.Spec.URL
			v["source.ref"], v["source.revision"] = "tag: "+o.Spec.Reference.Tag, "tag "+o.Spec.Reference.Tag
			if r := o.Spec.Reference.SemVer; r != "" {
				// source-controller takes ref.semver over ref.tag.
				v["source.ref"], v["source.revision"] = "semver: "+r, "semver "+r
			}
			v["source.interval"] = o.Spec.Interval.Duration.String()
			if o.Spec.SecretRef != nil {
				v["source.secretRef"] = o.Spec.SecretRef.Name
			}
			v["source.provider"] = o.Spec.Provider
			v["source.insecure"] = fmt.Sprint(o.Spec.Insecure)
		case *sourcev1.GitRepository:
			sources++
			v["source"] = "GitRepository " + o.Namespace + "/" + o.Name
			v["source.kind"] = "GitRepository"
			v["source.namespace"] = o.Namespace
			v["source.url"] = o.Spec.URL
			if r := o.Spec.Reference; r != nil {
				v["source.ref"], v["source.revision"] = "branch: "+r.Branch, "refs/heads/"+r.Branch
			}
			v["source.interval"] = o.Spec.Interval.Duration.String()
		case *kustv1.Kustomization:
			kusts++
			v["kustomization"] = "Kustomization " + o.Namespace + "/" + o.Name
			v["kustomization.namespace"] = o.Namespace
			v["kustomization.sourceRef"] = o.Spec.SourceRef.Kind + " " + o.Spec.SourceRef.Name
			v["kustomization.sourceRef.namespace"] = o.Spec.SourceRef.Namespace
			v[sourceRefTarget] = o.Spec.SourceRef.Kind + " " + orDefault(o.Spec.SourceRef.Namespace, o.Namespace) + "/" + o.Spec.SourceRef.Name
			v["kustomization.path"] = o.Spec.Path
			v["kustomization.directory"] = path.Clean(o.Spec.Path)
			v["kustomization.prune"] = fmt.Sprint(o.Spec.Prune)
			v["kustomization.interval"] = o.Spec.Interval.Duration.String()
		}
	}
	if sources > 1 || kusts != 1 {
		t.Fatalf("gotk mode emitted %d sources and %d Kustomizations, want at most one source and one Kustomization", sources, kusts)
	}
	v["documents"] = fmt.Sprintf("%d source, %d Kustomization", sources, kusts)
	return v
}

func operatorRoot(t *testing.T, objs []client.Object) rootView {
	t.Helper()
	var fis []*fluxv1.FluxInstance
	for _, obj := range objs {
		if fi, ok := obj.(*fluxv1.FluxInstance); ok {
			fis = append(fis, fi)
		}
		if k := obj.GetObjectKind().GroupVersionKind().Kind; k == "OCIRepository" || k == "GitRepository" || k == "Kustomization" {
			t.Fatalf("flux-operator mode emitted a %s; the operator creates the root itself", k)
		}
	}
	if len(fis) != 1 {
		t.Fatalf("flux-operator mode emitted %d FluxInstances, want 1", len(fis))
	}
	fi := fis[0]
	v := rootView{}
	if fi.Spec.Kustomize != nil {
		var ps []string
		for _, p := range fi.Spec.Kustomize.Patches {
			ps = append(ps, fmt.Sprintf("%s/%s: %s", p.Target.Kind, p.Target.Name, strings.Join(strings.Fields(p.Patch), " ")))
		}
		v["patches"] = strings.Join(ps, "; ")
	}
	s := fi.Spec.Sync
	if s == nil {
		v["documents"] = "FluxInstance without sync"
		return v
	}
	v["documents"] = "FluxInstance with sync: 1 source, 1 Kustomization"
	name := s.Name
	if name == "" {
		name = fi.Namespace
	}
	v["source"] = s.Kind + " " + fi.Namespace + "/" + name
	v["source.kind"] = s.Kind
	v["source.namespace"] = fi.Namespace
	v["source.url"] = s.URL
	switch s.Kind {
	case "OCIRepository":
		// The revision and insecure are read from the source the patches
		// produce, so a patch that does not apply does not count.
		spec := appliedSyncSource(t, fi, name)
		ref, _ := spec["ref"].(map[string]any)
		tag, _ := ref["tag"].(string)
		v["source.ref"], v["source.revision"] = "tag: "+s.Ref, "tag "+tag
		// source-controller takes ref.semver over ref.tag.
		if r, _ := ref["semver"].(string); r != "" {
			v["source.ref"] = "tag: " + s.Ref + ", patched to semver: " + r
			v["source.revision"] = "semver " + r
		}
		v["source.insecure"] = fmt.Sprint(spec["insecure"] == true)
	case "GitRepository":
		v["source.ref"], v["source.revision"] = "name: "+s.Ref, s.Ref
	}
	if s.Interval != nil {
		v["source.interval"] = s.Interval.Duration.String()
	}
	v["source.secretRef"] = s.PullSecret
	v["source.provider"] = s.Provider
	v["kustomization"] = "Kustomization " + fi.Namespace + "/" + name
	v["kustomization.namespace"] = fi.Namespace
	v["kustomization.sourceRef"] = s.Kind + " " + name
	// The operator's sync Kustomization names no sourceRef namespace
	// (templates.go: kind and name only), so it resolves in its own.
	v["kustomization.sourceRef.namespace"] = ""
	v[sourceRefTarget] = s.Kind + " " + fi.Namespace + "/" + name
	v["kustomization.path"] = s.Path
	v["kustomization.directory"] = path.Clean(s.Path)
	v["kustomization.prune"] = "true"
	v["kustomization.interval"] = "10m0s"
	return v
}

// appliedSyncSource is the spec of the OCIRepository, named name, that
// flux-operator creates for fi's sync: the source its sync template writes
// (templates.go: ref.tag from sync.ref, url) with fi's kustomize patches
// applied by a kustomize build, as the operator applies them to every object
// it builds. A patch whose target misses the source leaves it as written.
func appliedSyncSource(t *testing.T, fi *fluxv1.FluxInstance, name string) map[string]any {
	t.Helper()
	source, err := sigsyaml.Marshal(map[string]any{
		"apiVersion": sourcev1.GroupVersion.String(),
		"kind":       sourcev1.OCIRepositoryKind,
		"metadata":   map[string]any{"name": name, "namespace": fi.Namespace},
		"spec":       map[string]any{"ref": map[string]any{"tag": fi.Spec.Sync.Ref}, "url": fi.Spec.Sync.URL},
	})
	if err != nil {
		t.Fatalf("marshalling the sync source: %v", err)
	}
	kustomization := "resources:\n  - source.yaml\n"
	if fi.Spec.Kustomize != nil && len(fi.Spec.Kustomize.Patches) > 0 {
		patches, err := sigsyaml.Marshal(fi.Spec.Kustomize.Patches)
		if err != nil {
			t.Fatalf("marshalling the patches: %v", err)
		}
		kustomization += "patches:\n" + string(patches)
	}
	fs := filesys.MakeFsInMemory()
	for file, data := range map[string]string{"source.yaml": string(source), "kustomization.yaml": kustomization} {
		if err := fs.WriteFile(filepath.Join("/sync", file), []byte(data)); err != nil {
			t.Fatalf("writing %s: %v", file, err)
		}
	}
	res, err := krusty.MakeKustomizer(krusty.MakeDefaultOptions()).Run(fs, "/sync")
	if err != nil {
		t.Fatalf("kustomize build of the sync source: %v", err)
	}
	if n := len(res.Resources()); n != 1 {
		t.Fatalf("kustomize build of the sync source gave %d objects, want 1", n)
	}
	m, err := res.Resources()[0].Map()
	if err != nil {
		t.Fatalf("reading the built sync source: %v", err)
	}
	spec, _ := m["spec"].(map[string]any)
	return spec
}

// bootstrapModeConfigs is the shared matrix: each config is rendered in both
// modes.
func bootstrapModeConfigs() map[string]*stack.BootstrapConfig {
	const oci, git = "oci://registry.example.com/fleet", "https://git.example.com/fleet.git"
	config := func(kind, url, ref string) *stack.BootstrapConfig {
		return &stack.BootstrapConfig{
			Enabled:     true,
			FluxVersion: fluxstack.GotkVersion,
			Registry:    "ghcr.io/fluxcd",
			SourceKind:  kind,
			SourceURL:   url,
			SourceRef:   ref,
		}
	}
	named := config("OCIRepository", oci, "v1.2.3")
	named.SyncName = "fleet"
	pruneOff, pruneOn := config("OCIRepository", oci, "v1.2.3"), config("OCIRepository", oci, "v1.2.3")
	off, on := false, true
	pruneOff.Prune, pruneOn.Prune = &off, &on
	access := config("OCIRepository", oci, "v1.2.3")
	access.SourceSecretRef, access.SourceProvider, access.SourceInsecure = "registry-auth", "generic", true
	semver := config("OCIRepository", oci, "")
	semver.SourceSemVer = ">=1.0.0 <2.0.0"
	return map[string]*stack.BootstrapConfig{
		"oci-tag":         config("OCIRepository", oci, "v1.2.3"),
		"oci-default-ref": config("", oci, ""),
		"git-branch":      config("GitRepository", git, "main"),
		"sync-name":       named,
		"prune-unset":     config("OCIRepository", oci, "v1.2.3"),
		"prune-false":     pruneOff,
		"prune-true":      pruneOn,
		"source-access":   access,
		"oci-semver":      semver,
		"no-source":       config("", "", ""),
	}
}

// TestBootstrapModes_RootIdentities renders every config of the matrix in
// both modes and holds them to one root each: at most one source and exactly
// one Kustomization in gotk mode, one FluxInstance in flux-operator mode,
// whose sync is one source and one Kustomization. With a source, the
// Kustomization's sourceRef must resolve to it in each mode. The fields that
// decide what the cluster pulls and applies must agree; the comparison, differences
// included, is stored under testdata/bootstrap-modes.
func TestBootstrapModes_RootIdentities(t *testing.T) {
	for name, config := range bootstrapModeConfigs() {
		t.Run(name, func(t *testing.T) {
			render := func(mode string) []client.Object {
				c := *config
				c.FluxMode = mode
				objs, err := fluxstack.NewBootstrapGenerator().GenerateBootstrap(&c, &stack.Node{Name: "prod"}, layout.LayoutRules{ClusterName: "demo"})
				if err != nil {
					t.Fatalf("%s: GenerateBootstrap: %v", mode, err)
				}
				return objs
			}
			gotk, operator := gotkRoot(t, render(fluxstack.ModeGotk)), operatorRoot(t, render(fluxstack.DefaultFluxMode))
			if config.SourceURL != "" {
				for mode, v := range map[string]rootView{"gotk": gotk, "flux-operator": operator} {
					if v[sourceRefTarget] != v["source"] {
						t.Errorf("%s: the Kustomization's sourceRef resolves to %s, the source is %s", mode, v[sourceRefTarget], v["source"])
					}
				}
			}
			var out bytes.Buffer
			fmt.Fprintf(&out, "%-33s | %-44s | %s\n", "field", "gotk", "flux-operator")
			for _, f := range rootFields {
				g, o := gotk[f.name], operator[f.name]
				mark := ""
				if g != o {
					mark = "   <- differs"
					if f.same && config.SourceURL != "" {
						t.Errorf("%s: gotk %q, flux-operator %q; both modes must agree on it", f.name, g, o)
					}
				}
				fmt.Fprintf(&out, "%-33s | %-44s | %s%s\n", f.name, orNone(g), orNone(o), mark)
			}
			compareWithStored(t, filepath.Join("testdata", "bootstrap-modes", name+".txt"), out.Bytes())
		})
	}
}

// sourceRefTarget is the object the root Kustomization's sourceRef resolves
// to, "Kind namespace/name", an unset namespace being the Kustomization's own.
// It is checked against the emitted source, not stored.
const sourceRefTarget = "kustomization.sourceRef.target"

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func orNone(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
