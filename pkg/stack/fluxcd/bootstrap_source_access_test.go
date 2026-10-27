package fluxcd_test

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	sigsyaml "sigs.k8s.io/yaml"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// sourceAccess is one combination of the three source access options.
type sourceAccess struct {
	secretRef string
	provider  string
	insecure  bool
}

// sourceAccessCases are each option alone and all three together.
var sourceAccessCases = map[string]sourceAccess{
	"secret ref": {secretRef: "registry-creds"},
	"provider":   {provider: "aws"},
	"insecure":   {insecure: true},
	"all three":  {secretRef: "registry-creds", provider: "azure", insecure: true},
}

// accessConfig is an OCIRepository bootstrap config in mode with access set.
func accessConfig(mode string, access sourceAccess) *stack.BootstrapConfig {
	return &stack.BootstrapConfig{
		Enabled:         true,
		FluxMode:        mode,
		FluxVersion:     fluxstack.GotkVersion,
		Registry:        "ghcr.io/fluxcd",
		SourceURL:       "oci://registry.example.com/fleet",
		SourceRef:       "v1.2.3",
		SourceSecretRef: access.secretRef,
		SourceProvider:  access.provider,
		SourceInsecure:  access.insecure,
	}
}

var accessRules = layout.LayoutRules{ClusterName: "demo"}

// findObject returns the one object of kind in objs.
func findObject[T client.Object](t *testing.T, objs []client.Object) T {
	t.Helper()
	var found []T
	for _, obj := range objs {
		if o, ok := obj.(T); ok {
			found = append(found, o)
		}
	}
	if len(found) != 1 {
		t.Fatalf("found %d objects of type %T, want 1", len(found), *new(T))
	}
	return found[0]
}

// TestBootstrapSourceAccess_Gotk: each option lands on the gotk OCIRepository
// and nowhere else changes; the source with every option unset is the stored
// one (TestBootstrap_OutputUnchanged).
func TestBootstrapSourceAccess_Gotk(t *testing.T) {
	for name, access := range sourceAccessCases {
		t.Run(name, func(t *testing.T) {
			bg := fluxstack.NewBootstrapGenerator()
			root := &stack.Node{Name: "prod"}
			plain, err := bg.GenerateBootstrap(accessConfig(fluxstack.ModeGotk, sourceAccess{}), root, accessRules)
			if err != nil {
				t.Fatalf("GenerateBootstrap without options: %v", err)
			}
			got, err := bg.GenerateBootstrap(accessConfig(fluxstack.ModeGotk, access), root, accessRules)
			if err != nil {
				t.Fatalf("GenerateBootstrap: %v", err)
			}
			if len(got) != len(plain) {
				t.Fatalf("%d objects, want %d as without options", len(got), len(plain))
			}

			want := findObject[*sourcev1.OCIRepository](t, plain).DeepCopy()
			if access.secretRef != "" {
				want.Spec.SecretRef = &meta.LocalObjectReference{Name: access.secretRef}
			}
			want.Spec.Provider = access.provider
			want.Spec.Insecure = access.insecure
			if src := findObject[*sourcev1.OCIRepository](t, got); !reflect.DeepEqual(src, want) {
				t.Errorf("OCIRepository:\n got %#v\nwant %#v", src.Spec, want.Spec)
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

// fluxInstanceEntryPoints are the two public ways to a FluxInstance; both
// build it in one place, and both must carry the options and refuse what the
// other refuses.
var fluxInstanceEntryPoints = map[string]func(*stack.BootstrapConfig) (*fluxv1.FluxInstance, error){
	"GenerateBootstrap": func(config *stack.BootstrapConfig) (*fluxv1.FluxInstance, error) {
		objs, err := fluxstack.NewBootstrapGenerator().GenerateBootstrap(config, &stack.Node{Name: "prod"}, accessRules)
		if err != nil || objs == nil {
			return nil, err
		}
		var fi *fluxv1.FluxInstance
		for _, obj := range objs {
			if o, ok := obj.(*fluxv1.FluxInstance); ok {
				fi = o
			}
		}
		return fi, nil
	},
	"GenerateFluxInstance": func(config *stack.BootstrapConfig) (*fluxv1.FluxInstance, error) {
		return fluxstack.NewBootstrapGenerator().GenerateFluxInstance(config, &stack.Node{Name: "prod"}, accessRules)
	},
}

// TestBootstrapSourceAccess_FluxOperator: the secret and provider land on the
// sync, and insecure is a kustomize patch present exactly when it is set,
// through both entry points.
func TestBootstrapSourceAccess_FluxOperator(t *testing.T) {
	for entry, generate := range fluxInstanceEntryPoints {
		for name, access := range sourceAccessCases {
			t.Run(entry+"/"+name, func(t *testing.T) {
				plain, err := generate(accessConfig(fluxstack.DefaultFluxMode, sourceAccess{}))
				if err != nil {
					t.Fatalf("without options: %v", err)
				}
				got, err := generate(accessConfig(fluxstack.DefaultFluxMode, access))
				if err != nil {
					t.Fatalf("with options: %v", err)
				}
				want := plain.DeepCopy()
				want.Spec.Sync.PullSecret = access.secretRef
				want.Spec.Sync.Provider = access.provider
				if access.insecure {
					want.Spec.Kustomize = got.Spec.Kustomize // checked by TestBootstrapSourceAccess_InsecurePatch
					if got.Spec.Kustomize == nil || len(got.Spec.Kustomize.Patches) != 1 {
						t.Fatalf("spec.kustomize = %#v, want one patch", got.Spec.Kustomize)
					}
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("FluxInstance:\n got %#v\nwant %#v", got.Spec, want.Spec)
				}
			})
		}
	}
}

// TestBootstrapSourceAccess_InsecurePatch builds the FluxInstance's patches
// over the sync objects flux-operator generates, the way the operator does: the
// patches are marshalled into the kustomization's patches list and kustomize
// builds it. The sync OCIRepository comes out with spec.insecure true and the
// sync Kustomization is untouched, for an explicit sync name and for the
// default one, the FluxInstance's namespace.
func TestBootstrapSourceAccess_InsecurePatch(t *testing.T) {
	for name, syncName := range map[string]string{"named sync": "fleet", "default sync name": ""} {
		t.Run(name, func(t *testing.T) {
			config := accessConfig(fluxstack.DefaultFluxMode, sourceAccess{insecure: true})
			config.SyncName = syncName
			fi, err := fluxstack.NewBootstrapGenerator().GenerateFluxInstance(config, &stack.Node{Name: "prod"}, accessRules)
			if err != nil {
				t.Fatalf("GenerateFluxInstance: %v", err)
			}
			applied := syncName
			if applied == "" {
				applied = fi.GetNamespace()
			}
			insecure := map[string]any{}
			for id, spec := range buildOperatorSync(t, fi, applied) {
				if v, found := spec["insecure"]; found {
					insecure[id] = v
				}
			}
			want := map[string]any{"OCIRepository/" + applied: true}
			if !reflect.DeepEqual(insecure, want) {
				t.Errorf("spec.insecure after the build = %v, want %v", insecure, want)
			}
		})
	}
}

// buildOperatorSync builds fi's kustomize patches over the sync objects
// flux-operator generates, named applied, the way the operator does: the
// patches are marshalled into the kustomization's patches list and kustomize
// builds it. It returns each built object's spec by "Kind/name".
func buildOperatorSync(t *testing.T, fi *fluxv1.FluxInstance, applied string) map[string]map[string]any {
	t.Helper()
	var patches []byte
	if fi.Spec.Kustomize != nil {
		var err error
		if patches, err = sigsyaml.Marshal(fi.Spec.Kustomize.Patches); err != nil {
			t.Fatalf("marshalling the patches: %v", err)
		}
	}
	// The two sync objects as flux-operator's sync template writes them
	// (internal/builder/templates.go, v0.62.0), and a second OCIRepository a
	// patch must not reach.
	sync := strings.NewReplacer("NAME", applied, "NS", fi.GetNamespace()).Replace(`apiVersion: source.toolkit.fluxcd.io/v1
kind: OCIRepository
metadata:
  name: NAME
  namespace: NS
spec:
  interval: 1h0m0s
  ref:
    tag: v1.2.3
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
  url: oci://registry.example.com/other
`)
	kustomization := "resources:\n  - sync.yaml\n"
	if len(patches) > 0 {
		kustomization += "patches:\n" + string(patches)
	}
	fs := filesys.MakeFsInMemory()
	dir := "/sync"
	for file, data := range map[string]string{"sync.yaml": sync, "kustomization.yaml": kustomization} {
		if err := fs.WriteFile(filepath.Join(dir, file), []byte(data)); err != nil {
			t.Fatalf("writing %s: %v", file, err)
		}
	}
	res, err := krusty.MakeKustomizer(krusty.MakeDefaultOptions()).Run(fs, dir)
	if err != nil {
		t.Fatalf("kustomize build: %v", err)
	}
	specs := map[string]map[string]any{}
	for _, r := range res.Resources() {
		m, err := r.Map()
		if err != nil {
			t.Fatalf("reading %s: %v", r.CurId(), err)
		}
		spec, _ := m["spec"].(map[string]any)
		specs[r.GetKind()+"/"+r.GetName()] = spec
	}
	return specs
}

// TestBootstrapPrune_FluxOperator: an explicit Prune reaches the sync
// Kustomization flux-operator generates, through a FluxInstance patch built
// the way the operator builds it, for an explicit and the default sync name,
// alone and next to the insecure patch, through both entry points. Unset
// emits no patch and keeps the operator's prune: true; nothing else in the
// sync changes (go-kure/kure#1088).
func TestBootstrapPrune_FluxOperator(t *testing.T) {
	off, on := false, true
	for entry, generate := range fluxInstanceEntryPoints {
		for _, syncName := range []string{"fleet", ""} {
			for _, insecure := range []bool{false, true} {
				for name, tc := range map[string]struct {
					prune *bool
					want  bool
				}{"unset": {nil, true}, "false": {&off, false}, "true": {&on, true}} {
					t.Run(fmt.Sprintf("%s/sync %q/insecure %t/prune %s", entry, syncName, insecure, name), func(t *testing.T) {
						config := accessConfig(fluxstack.DefaultFluxMode, sourceAccess{insecure: insecure})
						config.SyncName, config.Prune = syncName, tc.prune
						fi, err := generate(config)
						if err != nil {
							t.Fatalf("generate: %v", err)
						}
						applied := syncName
						if applied == "" {
							applied = fi.GetNamespace()
						}
						wantPatches := 0
						if insecure {
							wantPatches++
						}
						if tc.prune != nil {
							wantPatches++
						}
						if got := 0; fi.Spec.Kustomize != nil {
							if got = len(fi.Spec.Kustomize.Patches); got != wantPatches {
								t.Errorf("%d patches, want %d", got, wantPatches)
							}
						} else if wantPatches != 0 {
							t.Errorf("no spec.kustomize, want %d patches", wantPatches)
						}
						specs := buildOperatorSync(t, fi, applied)
						kust := specs["Kustomization/"+applied]
						if kust["prune"] != tc.want {
							t.Errorf("sync Kustomization spec.prune after the build = %v, want %v", kust["prune"], tc.want)
						}
						for id, spec := range specs {
							if _, found := spec["prune"]; found && id != "Kustomization/"+applied {
								t.Errorf("%s has spec.prune; the patch must reach the sync Kustomization only", id)
							}
						}
						if v, found := specs["OCIRepository/"+applied]["insecure"]; found != insecure || found && v != true {
							t.Errorf("sync OCIRepository spec.insecure = %v (set %t), want set %t", v, found, insecure)
						}
					})
				}
			}
		}
	}
}

// TestBootstrapPrune_NoSync: without a SourceURL the FluxInstance has no sync,
// and an explicit Prune adds no patch, through both entry points.
func TestBootstrapPrune_NoSync(t *testing.T) {
	off := false
	for entry, generate := range fluxInstanceEntryPoints {
		t.Run(entry, func(t *testing.T) {
			config := accessConfig(fluxstack.DefaultFluxMode, sourceAccess{})
			config.SourceURL, config.Prune = "", &off
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

// TestBootstrapSourceAccess_StoredOutput keeps the whole generated output with
// every option set, one file per mode, so a review sees the YAML.
func TestBootstrapSourceAccess_StoredOutput(t *testing.T) {
	for _, mode := range []string{fluxstack.ModeGotk, fluxstack.DefaultFluxMode} {
		t.Run(mode, func(t *testing.T) {
			objs, err := fluxstack.NewBootstrapGenerator().GenerateBootstrap(
				accessConfig(mode, sourceAccessCases["all three"]), &stack.Node{Name: "prod"}, accessRules)
			if err != nil {
				t.Fatalf("GenerateBootstrap: %v", err)
			}
			compareWithStored(t, filepath.Join("testdata", "bootstrap-output", mode+"-oci-source-access.txt"), bootstrapBytes(t, objs))
		})
	}
}

// TestBootstrapSourceAccess_Refused: options that would reach no source, or a
// provider an OCIRepository does not take, are refused in both modes and
// through both FluxInstance entry points, with nothing returned and an error
// naming the option.
func TestBootstrapSourceAccess_Refused(t *testing.T) {
	cases := map[string]struct {
		edit  func(*stack.BootstrapConfig)
		names string
	}{
		"secret ref without a source": {
			edit:  func(c *stack.BootstrapConfig) { c.SourceURL, c.SourceSecretRef = "", "creds" },
			names: "sourceSecretRef",
		},
		"insecure without a source": {
			edit:  func(c *stack.BootstrapConfig) { c.SourceURL, c.SourceInsecure = "", true },
			names: "sourceInsecure",
		},
		"provider on a GitRepository": {
			edit: func(c *stack.BootstrapConfig) {
				c.SourceKind, c.SourceURL, c.SourceProvider = "GitRepository", "https://git.example.com/fleet.git", "azure"
			},
			names: "sourceProvider",
		},
		"secret ref on a GitRepository": {
			edit: func(c *stack.BootstrapConfig) {
				c.SourceKind, c.SourceURL, c.SourceSecretRef = "GitRepository", "https://git.example.com/fleet.git", "creds"
			},
			names: "sourceSecretRef",
		},
		"github provider on an OCIRepository": {
			edit:  func(c *stack.BootstrapConfig) { c.SourceProvider = "github" },
			names: "sourceProvider",
		},
		"unknown provider": {
			edit:  func(c *stack.BootstrapConfig) { c.SourceProvider = "AWS" },
			names: "sourceProvider",
		},
		"verify without a source": {
			edit: func(c *stack.BootstrapConfig) {
				c.SourceURL, c.SourceVerify = "", &stack.SourceVerification{SecretRef: "cosign-pub"}
			},
			names: "sourceVerify",
		},
		"verify on a GitRepository": {
			edit: func(c *stack.BootstrapConfig) {
				c.SourceKind, c.SourceURL = "GitRepository", "https://git.example.com/fleet.git"
				c.SourceVerify = &stack.SourceVerification{SecretRef: "cosign-pub"}
			},
			names: "sourceVerify",
		},
		"unknown verify provider": {
			edit:  func(c *stack.BootstrapConfig) { c.SourceVerify = &stack.SourceVerification{Provider: "sigstore"} },
			names: "sourceVerify.provider",
		},
		// Keyless cosign with no identity match accepts every signer, the
		// default provider and a self-hosted Sigstore root included.
		"keyless cosign without an identity": {
			edit:  func(c *stack.BootstrapConfig) { c.SourceVerify = &stack.SourceVerification{} },
			names: "sourceVerify.matchOIDCIdentity",
		},
		"keyless cosign on a trusted root without an identity": {
			edit: func(c *stack.BootstrapConfig) {
				c.SourceVerify = &stack.SourceVerification{Provider: "cosign", TrustedRootSecretRef: "sigstore-root"}
			},
			names: "sourceVerify.matchOIDCIdentity",
		},
		"identity with an empty issuer": {
			edit: func(c *stack.BootstrapConfig) {
				c.SourceVerify = &stack.SourceVerification{MatchOIDCIdentity: []stack.OIDCIdentityMatch{{Subject: "^ci$"}}}
			},
			names: "sourceVerify.matchOIDCIdentity[0].issuer",
		},
		"identity with a subject that is not a regular expression": {
			edit: func(c *stack.BootstrapConfig) {
				c.SourceVerify = &stack.SourceVerification{MatchOIDCIdentity: []stack.OIDCIdentityMatch{
					{Issuer: "^https://issuer$", Subject: "^ci$"},
					{Issuer: "^https://issuer$", Subject: "("},
				}}
			},
			names: "sourceVerify.matchOIDCIdentity[1].subject",
		},
		// A character the YAML round trip folds (NEL) or refuses (DEL, a
		// noncharacter) in a pattern that compiles.
		"identity with a NEL in its subject": {
			edit: func(c *stack.BootstrapConfig) {
				c.SourceVerify = &stack.SourceVerification{MatchOIDCIdentity: []stack.OIDCIdentityMatch{
					{Issuer: "^https://issuer$", Subject: "^[^\u0085]+$"},
				}}
			},
			names: "sourceVerify.matchOIDCIdentity[0].subject",
		},
		"identity with a DEL in its issuer": {
			edit: func(c *stack.BootstrapConfig) {
				c.SourceVerify = &stack.SourceVerification{MatchOIDCIdentity: []stack.OIDCIdentityMatch{
					{Issuer: "^https://issuer\x7f$", Subject: "^ci$"},
				}}
			},
			names: "sourceVerify.matchOIDCIdentity[0].issuer",
		},
		"identity with a noncharacter in its subject": {
			edit: func(c *stack.BootstrapConfig) {
				c.SourceVerify = &stack.SourceVerification{MatchOIDCIdentity: []stack.OIDCIdentityMatch{
					{Issuer: "^https://issuer$", Subject: "^ci$"},
					{Issuer: "^https://issuer$", Subject: "^ci\xef\xbf\xbe$"},
				}}
			},
			names: "sourceVerify.matchOIDCIdentity[1].subject",
		},
	}
	check := func(t *testing.T, got any, err error, names string) {
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
		if !strings.Contains(err.Error(), names) {
			t.Errorf("error %q does not name %s", err, names)
		}
	}
	for name, tc := range cases {
		t.Run("gotk/"+name, func(t *testing.T) {
			config := accessConfig(fluxstack.ModeGotk, sourceAccess{})
			tc.edit(config)
			objs, err := fluxstack.NewBootstrapGenerator().GenerateBootstrap(config, &stack.Node{Name: "prod"}, accessRules)
			check(t, objs, err, tc.names)
		})
		for entry, generate := range fluxInstanceEntryPoints {
			t.Run(entry+"/"+name, func(t *testing.T) {
				config := accessConfig(fluxstack.DefaultFluxMode, sourceAccess{})
				tc.edit(config)
				fi, err := generate(config)
				check(t, fi, err, tc.names)
			})
		}
	}
}

// TestBootstrapSourceVerify_RefusalNamesSource: in gotk mode every
// verification refusal names the source it is about, SyncName when it names
// the source and the root node otherwise, never the other one.
func TestBootstrapSourceVerify_RefusalNamesSource(t *testing.T) {
	refused := map[string]stack.SourceVerification{
		"unknown provider": {Provider: "sigstore"},
		"no identity":      {},
		"empty issuer":     {MatchOIDCIdentity: []stack.OIDCIdentityMatch{{Subject: "^ci$"}}},
		"bad subject":      {MatchOIDCIdentity: []stack.OIDCIdentityMatch{{Issuer: "^https://issuer$", Subject: "("}}},
		"NEL in a subject": {MatchOIDCIdentity: []stack.OIDCIdentityMatch{{Issuer: "^https://issuer$", Subject: "^[^\u0085]+$"}}},
	}
	for name, verify := range refused {
		for _, syncName := range []string{"", "fleet"} {
			want, other := "'prod'", "'fleet'"
			if syncName != "" {
				want, other = other, want
			}
			t.Run(name+"/syncName="+syncName, func(t *testing.T) {
				config := accessConfig(fluxstack.ModeGotk, sourceAccess{})
				config.SyncName = syncName
				config.SourceVerify = &verify
				_, err := fluxstack.NewBootstrapGenerator().GenerateBootstrap(config, &stack.Node{Name: "prod"}, accessRules)
				if err == nil {
					t.Fatal("error = nil, want a refusal")
				}
				if msg := err.Error(); !strings.Contains(msg, want) || strings.Contains(msg, other) {
					t.Errorf("error %q names the wrong source: want %s, not %s", msg, want, other)
				}
			})
		}
	}
}
