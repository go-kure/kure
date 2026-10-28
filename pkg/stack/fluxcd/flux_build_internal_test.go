package fluxcd

import (
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
)

// TestTreeBuilds_SpecPathStaysInTheTree: build writes the kustomization.yaml
// Flux's generator makes into spec.path's directory, so a spec.path that
// leaves the tree is refused before anything is written, whatever its caller
// checked.
func TestTreeBuilds_SpecPathStaysInTheTree(t *testing.T) {
	parent := t.TempDir()
	tree := filepath.Join(parent, "tree")
	if err := os.Mkdir(tree, 0o750); err != nil {
		t.Fatal(err)
	}
	b := &treeBuilds{dir: tree, written: true}
	for _, p := range []string{"..", "../outside", "./../outside", "demo/../../outside"} {
		t.Run(p, func(t *testing.T) {
			k := &kustv1.Kustomization{}
			k.Name, k.Namespace = "escape", "flux-system"
			k.Spec.Path = p
			_, err := b.build(k)
			if err == nil || !strings.Contains(err.Error(), "is outside the tree") {
				t.Fatalf("got %v, want spec.path %q refused as outside the tree", err, p)
			}
			for _, f := range []string{"kustomization.yaml", filepath.Join("outside", "kustomization.yaml")} {
				if _, err := os.Stat(filepath.Join(parent, f)); !os.IsNotExist(err) {
					t.Errorf("%s exists beside the tree (stat: %v)", f, err)
				}
			}
		})
	}
}

// buildIn writes files into a fresh tree, with TREE in their content replaced
// by the tree's path, and builds spec.path app as build does for a tree it
// wrote.
func buildIn(t *testing.T, files map[string]string) error {
	t.Helper()
	tree := t.TempDir()
	for name, content := range files {
		p := filepath.Join(tree, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(strings.ReplaceAll(content, "TREE", tree)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	b := &treeBuilds{dir: tree, written: true}
	k := &kustv1.Kustomization{}
	k.Name, k.Namespace = "app", "flux-system"
	k.Spec.Path = "app"
	_, err := b.build(k)
	return err
}

const (
	appKustomization = "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n- cm.yaml\n"
	appConfigMap     = "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm\n  namespace: default\n"
	remotePatch      = "apiVersion: builtin\nkind: PatchTransformer\nmetadata:\n  name: remote-patch\npath: URL/p.yaml\ntarget:\n  kind: ConfigMap\n"
)

// mustBeUnread fails unless err is a build kure does not read, naming every
// part.
func mustBeUnread(t *testing.T, err error, parts ...string) {
	t.Helper()
	var u unreadBuild
	if !stderrors.As(err, &u) {
		t.Fatalf("got %v, want a build kure does not read", err)
	}
	for _, p := range parts {
		if !strings.Contains(err.Error(), p) {
			t.Errorf("got %v, want it to name %s", err, p)
		}
	}
}

// TestBuildGuard_AbsolutePluginConfigPath: a transformers entry may name its
// config file by an absolute path, which kustomize reads unchanged; the
// config's remote patch path is found there as well, and nothing is fetched
// (go-kure/kure#1168).
func TestBuildGuard_AbsolutePluginConfigPath(t *testing.T) {
	w := NewNetworkWitness(t)
	err := buildIn(t, map[string]string{
		"app/kustomization.yaml": appKustomization + "transformers:\n- TREE/plugins/pt.yaml\n",
		"app/cm.yaml":            appConfigMap,
		"plugins/pt.yaml":        strings.ReplaceAll(remotePatch, "URL", w.URL),
	})
	w.Check(t)
	mustBeUnread(t, err, "plugins/pt.yaml", w.URL+"/p.yaml")
}

// TestBuildGuard_FailsClosed: a file the build reads that kure cannot check
// for remote references is not read, never handed to kustomize to read as it
// may: a component kustomization or a plugin config that does not parse, a
// builtin kind the check does not know, a plugin that is not builtin (Flux
// refuses those too), and plugin configs that a kustomization builds, whose
// fields its transforms may write. Nothing is fetched.
func TestBuildGuard_FailsClosed(t *testing.T) {
	for name, tc := range map[string]struct {
		files map[string]string
		names string
	}{
		"component that does not parse": {names: "comp/kustomization.yaml", files: map[string]string{
			"app/kustomization.yaml":  appKustomization + "components:\n- ../comp\n",
			"comp/kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\nresources: [unclosed\n",
		}},
		"plugin config that does not parse": {names: "app/pt.yaml", files: map[string]string{
			"app/kustomization.yaml": appKustomization + "transformers:\n- pt.yaml\n",
			"app/pt.yaml":            "apiVersion: builtin\nkind: [unclosed\n",
		}},
		"unknown builtin kind": {names: "NoSuchTransformer", files: map[string]string{
			"app/kustomization.yaml": appKustomization + "transformers:\n- pt.yaml\n",
			"app/pt.yaml":            "apiVersion: builtin\nkind: NoSuchTransformer\nmetadata:\n  name: x\npath: URL/p.yaml\n",
		}},
		"plugin that is not builtin": {names: "example.com/v1", files: map[string]string{
			"app/kustomization.yaml": appKustomization + "transformers:\n- pt.yaml\n",
			"app/pt.yaml":            "apiVersion: example.com/v1\nkind: MyTransformer\nmetadata:\n  name: x\npath: URL/p.yaml\n",
		}},
		"plugin configs a kustomization builds": {names: "app/plugins", files: map[string]string{
			"app/kustomization.yaml":         appKustomization + "transformers:\n- plugins\n",
			"app/plugins/kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n- pt.yaml\n",
			"app/plugins/pt.yaml":            remotePatch,
		}},
	} {
		t.Run(name, func(t *testing.T) {
			w := NewNetworkWitness(t)
			files := map[string]string{"app/cm.yaml": appConfigMap}
			for n, c := range tc.files {
				files[n] = strings.ReplaceAll(c, "URL", w.URL)
			}
			err := buildIn(t, files)
			w.Check(t)
			mustBeUnread(t, err, tc.names)
		})
	}
}
