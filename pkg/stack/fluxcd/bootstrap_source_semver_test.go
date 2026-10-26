package fluxcd_test

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
)

// semverRanges are ranges whose text is YAML syntax when unquoted (">", "*",
// "||") next to plain ones, so the flux-operator patch is shown to carry each
// verbatim.
var semverRanges = map[string]string{
	"bounded":     ">=1.0.0 <2.0.0",
	"caret":       "^1.2",
	"tilde":       "~1.2.3",
	"any":         "*",
	"alternative": "1.x || >=3.0.0",
}

// semverConfig is accessConfig in mode with no tag and the range rng.
func semverConfig(mode, rng string) *stack.BootstrapConfig {
	config := accessConfig(mode, sourceAccess{})
	config.SourceRef = ""
	config.SourceSemVer = rng
	return config
}

// TestBootstrapSourceSemVer_Gotk: the range is the gotk OCIRepository's whole
// ref, and no other object changes from the same config without it.
func TestBootstrapSourceSemVer_Gotk(t *testing.T) {
	for name, rng := range semverRanges {
		t.Run(name, func(t *testing.T) {
			bg := fluxstack.NewBootstrapGenerator()
			root := &stack.Node{Name: "prod"}
			plain, err := bg.GenerateBootstrap(semverConfig(fluxstack.ModeGotk, ""), root, accessRules)
			if err != nil {
				t.Fatalf("GenerateBootstrap without a range: %v", err)
			}
			got, err := bg.GenerateBootstrap(semverConfig(fluxstack.ModeGotk, rng), root, accessRules)
			if err != nil {
				t.Fatalf("GenerateBootstrap: %v", err)
			}
			if len(got) != len(plain) {
				t.Fatalf("%d objects, want %d as without a range", len(got), len(plain))
			}

			want := findObject[*sourcev1.OCIRepository](t, plain).DeepCopy()
			want.Spec.Reference = &sourcev1.OCIRepositoryRef{SemVer: rng}
			if src := findObject[*sourcev1.OCIRepository](t, got); !reflect.DeepEqual(src, want) {
				t.Errorf("OCIRepository:\n got %#v\nwant %#v", src.Spec.Reference, want.Spec.Reference)
			}
			for i := range got {
				if _, ok := got[i].(*sourcev1.OCIRepository); ok {
					continue
				}
				if !reflect.DeepEqual(got[i], plain[i]) {
					t.Errorf("object %d (%s %s) changed", i, got[i].GetObjectKind().GroupVersionKind().Kind, got[i].GetName())
				}
			}
		})
	}
}

// TestBootstrapSourceSemVer_FluxOperator: through both entry points the range
// adds one kustomize patch, next to the insecure one when that is set too, and
// changes nothing else: the sync keeps the default tag its required ref takes.
func TestBootstrapSourceSemVer_FluxOperator(t *testing.T) {
	for entry, generate := range fluxInstanceEntryPoints {
		for name, insecure := range map[string]bool{"alone": false, "with insecure": true} {
			t.Run(entry+"/"+name, func(t *testing.T) {
				without := semverConfig(fluxstack.DefaultFluxMode, "")
				without.SourceInsecure = insecure
				plain, err := generate(without)
				if err != nil {
					t.Fatalf("without a range: %v", err)
				}
				with := semverConfig(fluxstack.DefaultFluxMode, semverRanges["bounded"])
				with.SourceInsecure = insecure
				got, err := generate(with)
				if err != nil {
					t.Fatalf("with a range: %v", err)
				}
				if got.Spec.Sync.Ref != fluxstack.DefaultSourceRef {
					t.Errorf("sync.ref = %q, want the default tag %q", got.Spec.Sync.Ref, fluxstack.DefaultSourceRef)
				}
				var before int
				if plain.Spec.Kustomize != nil {
					before = len(plain.Spec.Kustomize.Patches)
				}
				if got.Spec.Kustomize == nil || len(got.Spec.Kustomize.Patches) != before+1 {
					t.Fatalf("spec.kustomize = %#v, want %d patches", got.Spec.Kustomize, before+1)
				}
				patches := got.Spec.Kustomize.Patches
				if !reflect.DeepEqual(patches[len(patches)-1].Target, patches[0].Target) {
					t.Errorf("semver patch target %#v, want the insecure patch's %#v", patches[len(patches)-1].Target, patches[0].Target)
				}
				want := plain.DeepCopy()
				want.Spec.Kustomize = got.Spec.Kustomize // the patch is checked by TestBootstrapSourceSemVer_Patch
				if before > 0 && !reflect.DeepEqual(patches[:before], plain.Spec.Kustomize.Patches) {
					t.Errorf("patches before the semver one = %#v, want %#v", patches[:before], plain.Spec.Kustomize.Patches)
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("FluxInstance:\n got %#v\nwant %#v", got.Spec, want.Spec)
				}
			})
		}
	}
}

// TestBootstrapSourceSemVer_Patch builds the FluxInstance's patches over the
// sync objects flux-operator generates, as TestBootstrapSourceAccess_InsecurePatch
// does: the sync OCIRepository's ref is the range and nothing else, the sync
// Kustomization and another OCIRepository are untouched, for each range, an
// explicit and the default sync name, and with the insecure patch beside it.
func TestBootstrapSourceSemVer_Patch(t *testing.T) {
	for rngName, rng := range semverRanges {
		for syncCase, syncName := range map[string]string{"named sync": "fleet", "default sync name": ""} {
			for _, insecure := range []bool{false, true} {
				name := rngName + "/" + syncCase
				if insecure {
					name += "/with insecure"
				}
				t.Run(name, func(t *testing.T) {
					config := semverConfig(fluxstack.DefaultFluxMode, rng)
					config.SyncName = syncName
					config.SourceInsecure = insecure
					fi, err := fluxstack.NewBootstrapGenerator().GenerateFluxInstance(config, &stack.Node{Name: "prod"}, accessRules)
					if err != nil {
						t.Fatalf("GenerateFluxInstance: %v", err)
					}
					applied := syncName
					if applied == "" {
						applied = fi.GetNamespace()
					}
					patches, err := sigsyaml.Marshal(fi.Spec.Kustomize.Patches)
					if err != nil {
						t.Fatalf("marshalling the patches: %v", err)
					}
					// The two sync objects as flux-operator's sync template
					// writes them for this FluxInstance, and a second
					// OCIRepository the patch must not reach.
					sync := strings.NewReplacer("NAME", applied, "NS", fi.GetNamespace(), "REF", fi.Spec.Sync.Ref).Replace(`apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata:
  name: NAME
  namespace: NS
spec:
  interval: 1h0m0s
  ref:
    tag: REF
  url: oci://registry.example.com/fleet
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: NAME
  namespace: NS
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
  ref:
    tag: REF
  url: oci://registry.example.com/other
`)
					fs := filesys.MakeFsInMemory()
					dir := "/sync"
					for file, data := range map[string]string{
						"sync.yaml":          sync,
						"kustomization.yaml": "resources:\n  - sync.yaml\npatches:\n" + string(patches),
					} {
						if err := fs.WriteFile(filepath.Join(dir, file), []byte(data)); err != nil {
							t.Fatalf("writing %s: %v", file, err)
						}
					}
					res, err := krusty.MakeKustomizer(krusty.MakeDefaultOptions()).Run(fs, dir)
					if err != nil {
						t.Fatalf("kustomize build: %v", err)
					}
					refs, insecures := map[string]any{}, map[string]any{}
					for _, r := range res.Resources() {
						m, err := r.Map()
						if err != nil {
							t.Fatalf("reading %s: %v", r.CurId(), err)
						}
						spec, _ := m["spec"].(map[string]any)
						key := r.GetKind() + "/" + r.GetName()
						if v, found := spec["ref"]; found {
							refs[key] = v
						}
						if v, found := spec["insecure"]; found {
							insecures[key] = v
						}
					}
					wantRefs := map[string]any{
						"OCIRepository/" + applied: map[string]any{"semver": rng},
						"OCIRepository/other":      map[string]any{"tag": fi.Spec.Sync.Ref},
					}
					if !reflect.DeepEqual(refs, wantRefs) {
						t.Errorf("spec.ref after the build = %v, want %v", refs, wantRefs)
					}
					wantInsecure := map[string]any{}
					if insecure {
						wantInsecure["OCIRepository/"+applied] = true
					}
					if !reflect.DeepEqual(insecures, wantInsecure) {
						t.Errorf("spec.insecure after the build = %v, want %v", insecures, wantInsecure)
					}
				})
			}
		}
	}
}

// TestBootstrapSourceSemVer_StoredOutput keeps the whole generated output with
// a range set, one file per mode, so a review sees the YAML.
func TestBootstrapSourceSemVer_StoredOutput(t *testing.T) {
	for _, mode := range []string{fluxstack.ModeGotk, fluxstack.DefaultFluxMode} {
		t.Run(mode, func(t *testing.T) {
			objs, err := fluxstack.NewBootstrapGenerator().GenerateBootstrap(
				semverConfig(mode, semverRanges["bounded"]), &stack.Node{Name: "prod"}, accessRules)
			if err != nil {
				t.Fatalf("GenerateBootstrap: %v", err)
			}
			compareWithStored(t, filepath.Join("testdata", "bootstrap-output", mode+"-oci-semver.txt"), bootstrapBytes(t, objs))
		})
	}
}

// TestBootstrapSourceSemVer_Refused: a range that would reach no source, sit
// on a Git source, compete with an explicit tag or not parse is refused in
// both modes and through both FluxInstance entry points, with nothing returned
// and an error naming sourceSemVer.
func TestBootstrapSourceSemVer_Refused(t *testing.T) {
	cases := map[string]func(*stack.BootstrapConfig){
		"without a source": func(c *stack.BootstrapConfig) { c.SourceURL = "" },
		"on a GitRepository": func(c *stack.BootstrapConfig) {
			c.SourceKind, c.SourceURL = "GitRepository", "https://git.example.com/fleet.git"
		},
		"with an explicit tag":   func(c *stack.BootstrapConfig) { c.SourceRef = "v1.2.3" },
		"with the default's tag": func(c *stack.BootstrapConfig) { c.SourceRef = fluxstack.DefaultSourceRef },
		"not a range":            func(c *stack.BootstrapConfig) { c.SourceSemVer = "not-a-range" },
		"an unfinished range":    func(c *stack.BootstrapConfig) { c.SourceSemVer = ">=1.0.0 <" },
	}
	check := func(t *testing.T, got any, err error) {
		t.Helper()
		if err == nil {
			t.Fatal("error = nil, want a refusal")
		}
		if !reflect.ValueOf(got).IsNil() {
			t.Errorf("returned %v next to the error", got)
		}
		if !errors.IsType(err, errors.ErrorTypeResource) && !errors.IsType(err, errors.ErrorTypeValidation) {
			t.Errorf("error type = %T (%v), want a pkg/errors validation error", err, err)
		}
		if !strings.Contains(err.Error(), "sourceSemVer") {
			t.Errorf("error %q does not name sourceSemVer", err)
		}
	}
	for name, edit := range cases {
		t.Run("gotk/"+name, func(t *testing.T) {
			config := semverConfig(fluxstack.ModeGotk, semverRanges["bounded"])
			edit(config)
			objs, err := fluxstack.NewBootstrapGenerator().GenerateBootstrap(config, &stack.Node{Name: "prod"}, accessRules)
			check(t, objs, err)
		})
		for entry, generate := range fluxInstanceEntryPoints {
			t.Run(entry+"/"+name, func(t *testing.T) {
				config := semverConfig(fluxstack.DefaultFluxMode, semverRanges["bounded"])
				edit(config)
				fi, err := generate(config)
				check(t, fi, err)
			})
		}
	}
}
