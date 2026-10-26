package fluxcd_test

import (
	"encoding/json"
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

	kureio "github.com/go-kure/kure/pkg/io"
	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
)

// sourceVerifyCases are each way a source verifies a signature, with the
// spec.verify each must produce.
var sourceVerifyCases = map[string]struct {
	in   stack.SourceVerification
	want sourcev1.OCIRepositoryVerification
}{
	"cosign key": {
		in:   stack.SourceVerification{Provider: "cosign", SecretRef: "cosign-pub"},
		want: sourcev1.OCIRepositoryVerification{Provider: "cosign", SecretRef: &meta.LocalObjectReference{Name: "cosign-pub"}},
	},
	"keyless cosign": {
		in: stack.SourceVerification{
			Provider: "cosign",
			MatchOIDCIdentity: []stack.OIDCIdentityMatch{
				{Issuer: "^https://token.actions.githubusercontent.com$", Subject: "^https://github.com/example/fleet/.*$"},
				{Issuer: "^https://accounts.google.com$", Subject: "^release@example.com$"},
			},
			TrustedRootSecretRef: "sigstore-root",
		},
		want: sourcev1.OCIRepositoryVerification{
			Provider: "cosign",
			MatchOIDCIdentity: []sourcev1.OIDCIdentityMatch{
				{Issuer: "^https://token.actions.githubusercontent.com$", Subject: "^https://github.com/example/fleet/.*$"},
				{Issuer: "^https://accounts.google.com$", Subject: "^release@example.com$"},
			},
			TrustedRootSecretRef: &meta.LocalObjectReference{Name: "sigstore-root"},
		},
	},
	"notation": {
		in:   stack.SourceVerification{Provider: "notation", SecretRef: "notation-trust"},
		want: sourcev1.OCIRepositoryVerification{Provider: "notation", SecretRef: &meta.LocalObjectReference{Name: "notation-trust"}},
	},
	// Characters outside ASCII and a C0 control, which the YAML round trip keeps
	// as written, and a NEL written as the regular-expression escape
	// validateSourceVerify asks for in place of the character.
	"non-ASCII and a tab in patterns": {
		in: stack.SourceVerification{MatchOIDCIdentity: []stack.OIDCIdentityMatch{
			{Issuer: "^https://issuer.example/caf\u00e9\xe2\x80\xa8\t$", Subject: "^[^\\x{85}\U0001F512]+: #*|$"},
		}},
		want: sourcev1.OCIRepositoryVerification{
			Provider:          "cosign",
			MatchOIDCIdentity: []sourcev1.OIDCIdentityMatch{{Issuer: "^https://issuer.example/caf\u00e9\xe2\x80\xa8\t$", Subject: "^[^\\x{85}\U0001F512]+: #*|$"}},
		},
	},
	"default provider": {
		in:   stack.SourceVerification{SecretRef: "cosign-pub"},
		want: sourcev1.OCIRepositoryVerification{Provider: "cosign", SecretRef: &meta.LocalObjectReference{Name: "cosign-pub"}},
	},
}

// verifyConfig is accessConfig in mode with SourceVerify v and insecure.
func verifyConfig(mode string, v *stack.SourceVerification, insecure bool) *stack.BootstrapConfig {
	config := accessConfig(mode, sourceAccess{insecure: insecure})
	config.SourceVerify = v
	return config
}

// asGeneric is v as a kustomize build reads it back: JSON's maps, slices and
// strings.
func asGeneric(t *testing.T, v any) any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshalling %T: %v", v, err)
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("reading back %T: %v", v, err)
	}
	return out
}

// TestBootstrapSourceVerify_Gotk: the verification is the gotk
// OCIRepository's spec.verify, an empty provider the CRD's default, and no
// other object changes from the same config without it.
func TestBootstrapSourceVerify_Gotk(t *testing.T) {
	for name, tc := range sourceVerifyCases {
		t.Run(name, func(t *testing.T) {
			bg := fluxstack.NewBootstrapGenerator()
			root := &stack.Node{Name: "prod"}
			plain, err := bg.GenerateBootstrap(verifyConfig(fluxstack.ModeGotk, nil, false), root, accessRules)
			if err != nil {
				t.Fatalf("GenerateBootstrap without verification: %v", err)
			}
			in := tc.in
			got, err := bg.GenerateBootstrap(verifyConfig(fluxstack.ModeGotk, &in, false), root, accessRules)
			if err != nil {
				t.Fatalf("GenerateBootstrap: %v", err)
			}
			if len(got) != len(plain) {
				t.Fatalf("%d objects, want %d as without verification", len(got), len(plain))
			}

			want := findObject[*sourcev1.OCIRepository](t, plain).DeepCopy()
			want.Spec.Verify = tc.want.DeepCopy()
			if src := findObject[*sourcev1.OCIRepository](t, got); !reflect.DeepEqual(src, want) {
				t.Errorf("OCIRepository:\n got %#v\nwant %#v", src.Spec.Verify, want.Spec.Verify)
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

// TestBootstrapSourceVerify_FluxOperator: through both entry points the
// FluxInstance gains exactly one patch, after any insecure one and on the same
// target, that adds the verification to the sync source; nothing else of the
// FluxInstance changes.
func TestBootstrapSourceVerify_FluxOperator(t *testing.T) {
	for entry, generate := range fluxInstanceEntryPoints {
		for name, tc := range sourceVerifyCases {
			for _, insecure := range []bool{false, true} {
				caseName := entry + "/" + name
				if insecure {
					caseName += "/with insecure"
				}
				t.Run(caseName, func(t *testing.T) {
					plain, err := generate(verifyConfig(fluxstack.DefaultFluxMode, nil, insecure))
					if err != nil {
						t.Fatalf("without verification: %v", err)
					}
					in := tc.in
					got, err := generate(verifyConfig(fluxstack.DefaultFluxMode, &in, insecure))
					if err != nil {
						t.Fatalf("with verification: %v", err)
					}
					var before int
					if plain.Spec.Kustomize != nil {
						before = len(plain.Spec.Kustomize.Patches)
					}
					if got.Spec.Kustomize == nil || len(got.Spec.Kustomize.Patches) != before+1 {
						t.Fatalf("spec.kustomize = %#v, want %d patches", got.Spec.Kustomize, before+1)
					}
					patches := got.Spec.Kustomize.Patches
					if before > 0 && !reflect.DeepEqual(patches[:before], plain.Spec.Kustomize.Patches) {
						t.Errorf("patches before the verify one = %#v, want %#v", patches[:before], plain.Spec.Kustomize.Patches)
					}
					wantTarget := map[string]string{"group": sourcev1.GroupVersion.Group, "kind": sourcev1.OCIRepositoryKind, "name": got.Namespace}
					if target := patches[before].Target; target == nil ||
						!reflect.DeepEqual(map[string]string{"group": target.Group, "kind": target.Kind, "name": target.Name}, wantTarget) {
						t.Errorf("verify patch target %#v, want %v", target, wantTarget)
					}
					if spec := appliedSyncSource(t, got, got.Namespace); !reflect.DeepEqual(spec["verify"], asGeneric(t, tc.want)) {
						t.Errorf("sync source spec.verify after the patches = %#v, want %#v", spec["verify"], asGeneric(t, tc.want))
					}
					want := plain.DeepCopy()
					want.Spec.Kustomize = got.Spec.Kustomize // the patches are checked above and by TestBootstrapSourceVerify_Patch
					if !reflect.DeepEqual(got, want) {
						t.Errorf("FluxInstance:\n got %#v\nwant %#v", got.Spec, want.Spec)
					}
				})
			}
		}
	}
}

// TestBootstrapSourceVerify_Patch builds the FluxInstance's patches over the
// sync objects flux-operator generates, as TestBootstrapSourceAccess_InsecurePatch
// does: the sync OCIRepository gains spec.verify and the sync Kustomization and
// another OCIRepository do not, for an explicit and the default sync name, with
// and without the insecure patch beside it.
func TestBootstrapSourceVerify_Patch(t *testing.T) {
	tc := sourceVerifyCases["keyless cosign"]
	for syncCase, syncName := range map[string]string{"named sync": "fleet", "default sync name": ""} {
		for _, insecure := range []bool{false, true} {
			name := syncCase
			if insecure {
				name += "/with insecure"
			}
			t.Run(name, func(t *testing.T) {
				in := tc.in
				config := verifyConfig(fluxstack.DefaultFluxMode, &in, insecure)
				config.SyncName = syncName
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
				// The two sync objects as flux-operator's sync template writes
				// them, and a second OCIRepository the patch must not reach.
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
				verifies, insecures := map[string]any{}, map[string]any{}
				for _, r := range res.Resources() {
					m, err := r.Map()
					if err != nil {
						t.Fatalf("reading %s: %v", r.CurId(), err)
					}
					spec, _ := m["spec"].(map[string]any)
					key := r.GetKind() + "/" + r.GetName()
					if v, found := spec["verify"]; found {
						verifies[key] = v
					}
					if v, found := spec["insecure"]; found {
						insecures[key] = v
					}
				}
				wantVerify := map[string]any{"OCIRepository/" + applied: asGeneric(t, tc.want)}
				if !reflect.DeepEqual(verifies, wantVerify) {
					t.Errorf("spec.verify after the build = %v, want %v", verifies, wantVerify)
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

// TestBootstrapSourceVerify_StoredOutput keeps the whole generated output with
// keyless verification and insecure set, one file per mode, so a review sees
// the YAML.
func TestBootstrapSourceVerify_StoredOutput(t *testing.T) {
	for _, mode := range []string{fluxstack.ModeGotk, fluxstack.DefaultFluxMode} {
		t.Run(mode, func(t *testing.T) {
			in := sourceVerifyCases["keyless cosign"].in
			objs, err := fluxstack.NewBootstrapGenerator().GenerateBootstrap(
				verifyConfig(mode, &in, true), &stack.Node{Name: "prod"}, accessRules)
			if err != nil {
				t.Fatalf("GenerateBootstrap: %v", err)
			}
			compareWithStored(t, filepath.Join("testdata", "bootstrap-output", mode+"-oci-source-verify.txt"), bootstrapBytes(t, objs))
		})
	}
}

// TestBootstrapSourceVerify_Encoded writes the generated objects as YAML with
// both of pkg/io's encoders, reads them back, and finds the same spec.verify
// on the root source in both modes: gotk's OCIRepository as read, and the
// flux-operator sync source after the read FluxInstance's patches.
func TestBootstrapSourceVerify_Encoded(t *testing.T) {
	encoders := map[string]kureio.EncodeOptions{"default": {}, "kubernetes field order": {KubernetesFieldOrder: true}}
	for name, tc := range sourceVerifyCases {
		for _, mode := range []string{fluxstack.ModeGotk, fluxstack.DefaultFluxMode} {
			for encoder, opts := range encoders {
				t.Run(name+"/"+mode+"/"+encoder, func(t *testing.T) {
					in := tc.in
					objs, err := fluxstack.NewBootstrapGenerator().GenerateBootstrap(
						verifyConfig(mode, &in, false), &stack.Node{Name: "prod"}, accessRules)
					if err != nil {
						t.Fatalf("GenerateBootstrap: %v", err)
					}
					ptrs := make([]*client.Object, len(objs))
					for i := range objs {
						ptrs[i] = &objs[i]
					}
					data, err := kureio.EncodeObjectsToYAMLWithOptions(ptrs, opts)
					if err != nil {
						t.Fatalf("encoding: %v", err)
					}
					var got any
					for _, doc := range strings.Split(string(data), "\n---\n") {
						var head struct{ Kind string }
						if err := sigsyaml.Unmarshal([]byte(doc), &head); err != nil {
							t.Fatalf("reading a document back: %v", err)
						}
						switch head.Kind {
						case sourcev1.OCIRepositoryKind:
							var src map[string]any
							if err := sigsyaml.Unmarshal([]byte(doc), &src); err != nil {
								t.Fatalf("reading the OCIRepository back: %v", err)
							}
							spec, _ := src["spec"].(map[string]any)
							got = spec["verify"]
						case fluxv1.FluxInstanceKind:
							fi := &fluxv1.FluxInstance{}
							if err := sigsyaml.Unmarshal([]byte(doc), fi); err != nil {
								t.Fatalf("reading the FluxInstance back: %v", err)
							}
							syncName := fi.Spec.Sync.Name
							if syncName == "" {
								syncName = fi.Namespace
							}
							got = appliedSyncSource(t, fi, syncName)["verify"]
						}
					}
					if want := asGeneric(t, tc.want); !reflect.DeepEqual(got, want) {
						t.Errorf("spec.verify read back = %#v, want %#v", got, want)
					}
				})
			}
		}
	}
}
