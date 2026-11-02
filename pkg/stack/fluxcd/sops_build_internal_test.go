package fluxcd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
)

// buildDecrypting is buildIn with spec.decryption set to provider (none when
// ""), and with the files' modes kept as given by modes (a name absent from it
// is 0600).
func buildDecrypting(t *testing.T, provider string, files map[string]string, modes map[string]os.FileMode) error {
	t.Helper()
	tree := t.TempDir()
	for name, content := range files {
		p := filepath.Join(tree, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		mode, ok := modes[name]
		if !ok {
			mode = 0o600
		}
		if err := os.WriteFile(p, []byte(strings.ReplaceAll(content, "TREE", tree)), mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(p, mode); err != nil {
			t.Fatal(err)
		}
	}
	b := &treeBuilds{dir: tree, written: true}
	k := &kustv1.Kustomization{}
	k.Name, k.Namespace = "app", "flux-system"
	k.Spec.Path = "app"
	if provider != "" {
		k.Spec.Decryption = &kustv1.Decryption{Provider: provider}
	}
	_, err := b.build(k)
	return err
}

// Every SOPS marker kustomize-controller tests for (decryptor.go:113-122,
// v1.10.1), one per format.
const (
	markerYAML   = "mac: ENC[AES256_GCM,data:x,tag:y,type:str]"
	markerJSON   = `"mac": "ENC[AES256_GCM,data:x,tag:y,type:str]"`
	markerDotenv = "sops_mac=ENC[AES256_GCM,data:x,tag:y,type:str]"
	markerINI    = "[sops]"
)

const marked = "# " + markerYAML + "\n"

const (
	secretFiles = appKustomization + "secretGenerator:\n- name: s\n  files:\n  - s=s.txt\n"
	patchedCM   = "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm\n  namespace: default\ndata:\n  a: b\n"
)

// TestBuildGuard_SOPSMarkerIsNotRead: with spec.decryption set, a build that
// reads a file holding a SOPS marker is not read, whichever read reaches the
// file (go-kure/kure#1178, #1223): Flux decrypts the file, in place before
// the build or as a built object after it, and the hash of the name of a
// generated object, or a replacement into a name or a namespace, is over
// what it decrypts to. The refusal names the file. Each case reads the file
// through a different loader path, so a read that bypassed the guard would
// surface here.
func TestBuildGuard_SOPSMarkerIsNotRead(t *testing.T) {
	for name, tc := range map[string]struct {
		files map[string]string
		file  string
	}{
		"secretGenerator files": {map[string]string{
			"app/kustomization.yaml": secretFiles,
			"app/cm.yaml":            appConfigMap,
			"app/s.txt":              marked,
		}, "app/s.txt"},
		"secretGenerator envs": {map[string]string{
			"app/kustomization.yaml": appKustomization + "secretGenerator:\n- name: s\n  envs:\n  - s.env\n",
			"app/cm.yaml":            appConfigMap,
			"app/s.env":              "a=b\n" + marked,
		}, "app/s.env"},
		"secretGenerator legacy env": {map[string]string{
			"app/kustomization.yaml": appKustomization + "secretGenerator:\n- name: s\n  env: s.env\n",
			"app/cm.yaml":            appConfigMap,
			"app/s.env":              "a=b\n" + marked,
		}, "app/s.env"},
		"secretGenerator with needs-hash and no hash suffix": {map[string]string{
			"app/kustomization.yaml": appKustomization + "secretGenerator:\n- name: s\n  files:\n  - s.txt\n  options:\n    disableNameSuffixHash: true\n    annotations:\n      kustomize.config.k8s.io/needs-hash: \"true\"\n",
			"app/cm.yaml":            appConfigMap,
			"app/s.txt":              marked,
		}, "app/s.txt"},
		"replacement into a name": {map[string]string{
			"app/kustomization.yaml": secretFiles + "replacements:\n- source:\n    kind: Secret\n    name: s\n    fieldPath: data.s\n  targets:\n  - select:\n      kind: ConfigMap\n      name: cm\n    fieldPaths:\n    - metadata.name\n",
			"app/cm.yaml":            appConfigMap,
			"app/s.txt":              marked,
		}, "app/s.txt"},
		"replacement into a namespace": {map[string]string{
			"app/kustomization.yaml": secretFiles + "replacements:\n- source:\n    kind: Secret\n    name: s\n    fieldPath: data.s\n  targets:\n  - select:\n      kind: ConfigMap\n      name: cm\n    fieldPaths:\n    - metadata.namespace\n",
			"app/cm.yaml":            appConfigMap,
			"app/s.txt":              marked,
		}, "app/s.txt"},
		"patches path": {map[string]string{
			"app/kustomization.yaml": appKustomization + "patches:\n- path: p.yaml\n  target:\n    kind: ConfigMap\n",
			"app/cm.yaml":            appConfigMap,
			"app/p.yaml":             marked + patchedCM,
		}, "app/p.yaml"},
		"replacements path": {map[string]string{
			"app/kustomization.yaml": appKustomization + "replacements:\n- path: r.yaml\n",
			"app/cm.yaml":            appConfigMap,
			"app/r.yaml":             marked + "source:\n  kind: ConfigMap\n  name: cm\ntargets: []\n",
		}, "app/r.yaml"},
		"resources": {map[string]string{
			"app/kustomization.yaml": appKustomization,
			"app/cm.yaml":            appConfigMap + marked,
		}, "app/cm.yaml"},
		"whole-resource SOPS document": {map[string]string{
			"app/kustomization.yaml": appKustomization,
			"app/cm.yaml":            "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: ENC[AES256_GCM,data:x]\nsops:\n  " + markerYAML + "\n",
		}, "app/cm.yaml"},
		"legacy bases": {map[string]string{
			"app/kustomization.yaml":  appKustomization + "bases:\n- ../base\n",
			"app/cm.yaml":             appConfigMap,
			"base/kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nconfigMapGenerator:\n- name: g\n  files:\n  - g.txt\n",
			"base/g.txt":              marked,
		}, "base/g.txt"},
		"components": {map[string]string{
			"app/kustomization.yaml":  appKustomization + "components:\n- ../comp\n",
			"app/cm.yaml":             appConfigMap,
			"comp/kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\nsecretGenerator:\n- name: c\n  files:\n  - c.txt\n",
			"comp/c.txt":              marked,
		}, "comp/c.txt"},
		"configMapGenerator source, over-refused": {map[string]string{
			"app/kustomization.yaml": appKustomization + "configMapGenerator:\n- name: g\n  files:\n  - g.txt\n",
			"app/cm.yaml":            appConfigMap,
			"app/g.txt":              marked,
		}, "app/g.txt"},
		"yaml marker": {map[string]string{
			"app/kustomization.yaml": secretFiles,
			"app/cm.yaml":            appConfigMap,
			"app/s.txt":              markerYAML + "\n",
		}, "app/s.txt"},
		"json marker": {map[string]string{
			"app/kustomization.yaml": secretFiles,
			"app/cm.yaml":            appConfigMap,
			"app/s.txt":              "{" + markerJSON + "}\n",
		}, "app/s.txt"},
		"dotenv marker": {map[string]string{
			"app/kustomization.yaml": secretFiles,
			"app/cm.yaml":            appConfigMap,
			"app/s.txt":              markerDotenv + "\n",
		}, "app/s.txt"},
		"ini marker": {map[string]string{
			"app/kustomization.yaml": secretFiles,
			"app/cm.yaml":            appConfigMap,
			"app/s.txt":              markerINI + "\nmac = ENC[x]\n",
		}, "app/s.txt"},
	} {
		t.Run(name, func(t *testing.T) {
			err := buildDecrypting(t, "sops", tc.files, nil)
			mustBeUnread(t, err, tc.file, "SOPS")
		})
	}
}

// TestBuildGuard_SOPSMarkerAppliesToAnyProvider: spec.decryption with any
// provider is enough (as mergeUnread), so no provider's data path is tracked.
func TestBuildGuard_SOPSMarkerAppliesToAnyProvider(t *testing.T) {
	for _, provider := range []string{"sops", "other"} {
		t.Run(provider, func(t *testing.T) {
			err := buildDecrypting(t, provider, map[string]string{
				"app/kustomization.yaml": secretFiles,
				"app/cm.yaml":            appConfigMap,
				"app/s.txt":              marked,
			}, nil)
			mustBeUnread(t, err, "app/s.txt", "SOPS")
		})
	}
}

// TestBuildGuard_SOPSMarkerNeedsDecryption: without spec.decryption Flux
// decrypts nothing, so a marked file is read like any other; with it and no
// marked file nothing is refused either.
func TestBuildGuard_SOPSMarkerNeedsDecryption(t *testing.T) {
	files := func(content string) map[string]string {
		return map[string]string{
			"app/kustomization.yaml": secretFiles,
			"app/cm.yaml":            appConfigMap,
			"app/s.txt":              content,
		}
	}
	t.Run("marked file, no decryption", func(t *testing.T) {
		if err := buildDecrypting(t, "", files(marked), nil); err != nil {
			t.Fatalf("got %v, want the build read", err)
		}
	})
	t.Run("decryption, no marked file", func(t *testing.T) {
		if err := buildDecrypting(t, "sops", files("plain\n"), nil); err != nil {
			t.Fatalf("got %v, want the build read", err)
		}
	})
}

// TestBuildGuard_SOPSMarkerBeforeBuildError: Flux decrypts a file before it
// builds, so a ciphertext patch kustomize cannot parse is no failure there;
// the build is not read, rather than reported as failed.
func TestBuildGuard_SOPSMarkerBeforeBuildError(t *testing.T) {
	err := buildDecrypting(t, "sops", map[string]string{
		"app/kustomization.yaml": appKustomization + "patches:\n- path: p.yaml\n  target:\n    kind: ConfigMap\n",
		"app/cm.yaml":            appConfigMap,
		"app/p.yaml":             marked + "not: [a patch\n",
	}, nil)
	mustBeUnread(t, err, "app/p.yaml", "SOPS")
}

// TestBuildGuard_SOPSReadErrorUnderDecryption: a file the build cannot read
// may hold a marker, so with spec.decryption set the build is not read
// (kustomize otherwise reads it as absent).
func TestBuildGuard_SOPSReadErrorUnderDecryption(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode 0 file")
	}
	files := map[string]string{
		"app/kustomization.yaml": secretFiles,
		"app/cm.yaml":            appConfigMap,
		"app/s.txt":              "plain\n",
	}
	modes := map[string]os.FileMode{"app/s.txt": 0}
	mustBeUnread(t, buildDecrypting(t, "sops", files, modes), "app/s.txt")
}
