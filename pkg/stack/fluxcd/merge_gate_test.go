package fluxcd

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"sigs.k8s.io/kustomize/api/hasher"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/kustomize/kyaml/yaml"

	"github.com/go-kure/kure/internal/kustomizeschema"
)

// Invariant: kustomize's own build names the Secret a component merges into
// as the name it carries after the merge, so kure can read the merged Secret
// as built. A merge into a Secret the base generates with a hash takes the
// hash of the merged data; a merge into a literal Secret, or into one whose
// hash is disabled, keeps its name. The expected hash is computed by the
// kustomize hasher over the merged Secret written out by hand, never by the
// merge path.
func TestMergeIntoSecret_NameIsWhatKustomizeBuilds(t *testing.T) {
	const (
		head      = "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n"
		component = "apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\nsecretGenerator:\n" +
			"- name: creds\n  namespace: default\n  behavior: merge\n  literals:\n  - k=v\n"
		literalSecret = "apiVersion: v1\nkind: Secret\nmetadata:\n  name: creds\n  namespace: default\ndata:\n  base: dg==\n"
		generator     = "secretGenerator:\n- name: creds\n  namespace: default\n  literals:\n  - base=v\n"
		// the Secret the generated fixture holds after the merge
		merged = "apiVersion: v1\nkind: Secret\nmetadata:\n  name: creds\n  namespace: default\ntype: Opaque\ndata:\n  base: dg==\n  k: dg==\n"
	)
	hash := func(secret string) string {
		node, err := yaml.Parse(secret)
		if err != nil {
			t.Fatal(err)
		}
		h, err := (&hasher.Hasher{}).Hash(node)
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	cases := map[string]struct {
		base  string
		files map[string]string
		want  string
	}{
		"a literal Secret keeps its name": {
			base:  head + "resources:\n- secrets.yaml\ncomponents:\n- ../c\n",
			files: map[string]string{"base/secrets.yaml": literalSecret},
			want:  "creds",
		},
		"a literal Secret with stringData keeps its name": {
			base: head + "resources:\n- secrets.yaml\ncomponents:\n- ../c\n",
			files: map[string]string{"base/secrets.yaml": "apiVersion: v1\nkind: Secret\nmetadata:\n  name: creds\n" +
				"  namespace: default\nstringData:\n  s: x\n"},
			want: "creds",
		},
		"a generated Secret takes the hash of the merged data": {
			base: head + generator + "components:\n- ../c\n",
			want: "creds-" + hash(merged),
		},
		"a generated Secret with the hash disabled keeps its name": {
			base: head + "generatorOptions:\n  disableNameSuffixHash: true\n" + generator + "components:\n- ../c\n",
			want: "creds",
		},
	}
	kustomizeschema.Parse()
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			files := map[string]string{"base/kustomization.yaml": tc.base, "c/kustomization.yaml": component}
			for f, content := range tc.files {
				files[f] = content
			}
			for f, content := range files {
				p := filepath.Join(dir, filepath.FromSlash(f))
				if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			rm, err := kustomizeBuild(filesys.MakeFsOnDisk(), filepath.Join(dir, "base"))
			if err != nil {
				t.Fatalf("build: %v", err)
			}
			if rm.Size() != 1 {
				t.Fatalf("the build holds %d resources, want the one merged Secret", rm.Size())
			}
			got := rm.Resources()[0]
			if got.GetName() != tc.want {
				t.Errorf("the merged Secret is named %q, want %q", got.GetName(), tc.want)
			}
			if got.GetDataMap()["k"] != "dg==" {
				t.Errorf("the Secret holds %v: the merge did not happen, so the case tests nothing", got.GetDataMap())
			}
			out, err := got.AsYAML()
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("built:\n%s", out)
		})
	}
}

// Invariant: the kustomize api a binary links is the audited one only by its
// effective module, path and version: Go keeps the version a module requires
// in Dep.Version and the replacement apart, so a binary that requires v0.21.2
// and replaces it with v0.21.1 (kure's own) is audited, one that requires
// v0.21.1 and replaces it with v0.21.2 is not, and neither is another module
// carrying the audited version, a local replacement, or a binary without
// build information.
func TestUnauditedKustomize(t *testing.T) {
	mod := func(path, version string, replace *debug.Module) *debug.Module {
		return &debug.Module{Path: path, Version: version, Replace: replace}
	}
	cases := map[string]struct {
		info *debug.BuildInfo
		ok   bool
		// want is what the reason must contain; "" means audited.
		want string
	}{
		"required and replaced to the audited version": {
			info: &debug.BuildInfo{Deps: []*debug.Module{mod(kustomizeAPIModule, "v0.21.2", mod(kustomizeAPIModule, kustomizeAPIAudited, nil))}}, ok: true},
		"required at the audited version, not replaced": {
			info: &debug.BuildInfo{Deps: []*debug.Module{mod(kustomizeAPIModule, kustomizeAPIAudited, nil)}}, ok: true},
		"required at the audited version, replaced by another": {
			info: &debug.BuildInfo{Deps: []*debug.Module{mod(kustomizeAPIModule, kustomizeAPIAudited, mod(kustomizeAPIModule, "v0.21.2", nil))}}, ok: true,
			want: kustomizeAPIModule + " v0.21.2"},
		"not replaced, another version": {
			info: &debug.BuildInfo{Deps: []*debug.Module{mod(kustomizeAPIModule, "v0.21.2", nil)}}, ok: true,
			want: kustomizeAPIModule + " v0.21.2"},
		"replaced by another module at the audited version": {
			info: &debug.BuildInfo{Deps: []*debug.Module{mod(kustomizeAPIModule, "v0.21.2", mod("example.com/fork/api", kustomizeAPIAudited, nil))}}, ok: true,
			want: "example.com/fork/api " + kustomizeAPIAudited},
		"replaced by a local directory": {
			info: &debug.BuildInfo{Deps: []*debug.Module{mod(kustomizeAPIModule, "v0.21.2", mod("../api", "", nil))}}, ok: true,
			want: "../api"},
		"not among the modules": {
			info: &debug.BuildInfo{Deps: []*debug.Module{mod("example.com/other", "v1.0.0", nil)}}, ok: true,
			want: "does not list " + kustomizeAPIModule},
		"no build information": {ok: false, want: "no build information"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := unauditedKustomize(func() (*debug.BuildInfo, bool) { return tc.info, tc.ok })
			if tc.want == "" && got != "" {
				t.Errorf("got %q, want the audited kustomize", got)
			}
			if tc.want != "" && !strings.Contains(got, tc.want) {
				t.Errorf("got %q, want a reason naming %q", got, tc.want)
			}
		})
	}
}

// Invariant: the kustomize api this package's own test binary links is the
// audited one, as the binary reports it, so the build a test runs is read as
// kustomize-controller builds it. A go.mod that stops replacing the module, or
// replaces it with another version, fails here as well as in
// TestRemoteReference_FieldTablesMatchKustomize.
func TestUnauditedKustomize_OwnBinaryIsAudited(t *testing.T) {
	if why := unauditedKustomize(debug.ReadBuildInfo); why != "" {
		t.Errorf("the test binary's kustomize is not the audited one: %s", why)
	}
}
