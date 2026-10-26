package fluxcd_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	fluxkustomize "github.com/fluxcd/pkg/kustomize"
	securefs "github.com/fluxcd/pkg/kustomize/filesys"
	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	kusttypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/yaml"

	kerrors "github.com/go-kure/kure/pkg/errors"
)

// The tests build a tree as kustomize-controller does through buildLikeFlux,
// not through Flux's own SecureBuild (go-kure/kure#1122). Flux's Build resets
// kustomize's global OpenAPI schema around every build, under a lock only its
// own builds take: in the parallel tests that reset writes the schema every
// other kustomize build reads, and it parses the built-in schema again each
// time. buildLikeFlux makes the build without the reset, and TestMain holds it
// to Flux's: once every test has run, it builds each tree buildLikeFlux built
// again with SecureBuild, one at a time, and fails the package on any build
// that gave something else.

func TestMain(m *testing.M) {
	code := m.Run()
	if !buildsAsFlux() && code == 0 {
		code = 1
	}
	os.Exit(code)
}

// fluxInput is one build buildLikeFlux made: every directory (nil) and file
// of the tree by its path in the tree, the directory built, also by its path
// in the tree, and what the build gave (buildResult).
type fluxInput struct {
	files  map[string][]byte
	dir    string
	result string
}

// fluxInputs holds every distinct build buildLikeFlux made, by fluxInputKey,
// for buildsAsFlux.
var fluxInputs sync.Map

// buildLikeFlux builds path in tree as Flux's SecureBuild does, with its
// secure file system, its kustomize options and its recovery from a kustomize
// panic, but without its reset of kustomize's global OpenAPI schema. That
// reset keeps one build's openapi schema out of the next: no kustomization
// these tests build sets one, and a tree that does is refused rather than
// built otherwise than Flux builds it. That it builds as Flux does is not
// assumed: TestMain's replay of every build through SecureBuild is the proof,
// and fails the package when a Flux release changes its build.
func buildLikeFlux(t *testing.T, tree, path string) (resmap.ResMap, error) {
	t.Helper()
	files, err := fluxTree(tree)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := filepath.Rel(tree, path)
	if err != nil {
		t.Fatal(err)
	}
	fsys, err := securefs.MakeFsOnDiskSecure(tree)
	if err != nil {
		t.Fatal(err)
	}
	rm, err := krustyLikeFlux(fsys, path)
	fluxInputs.LoadOrStore(fluxInputKey(files, dir), fluxInput{files: files, dir: dir, result: buildResult(rm, err, tree)})
	return rm, err
}

// krustyLikeFlux is the build of Flux's Build after its schema reset.
func krustyLikeFlux(fsys filesys.FileSystem, path string) (rm resmap.ResMap, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = kerrors.Errorf("recovered from kustomize build panic: %v", r)
		}
	}()
	return krusty.MakeKustomizer(&krusty.Options{
		LoadRestrictions: kusttypes.LoadRestrictionsNone,
		PluginConfig:     kusttypes.DisabledPluginConfig(),
	}).Run(fsys, path)
}

// fluxTree reads every directory and file below tree, and refuses a
// kustomization that sets openapi, the one field whose build depends on the
// schema reset buildLikeFlux leaves out. A kustomization that does not parse
// is left to the build to report.
func fluxTree(tree string) (map[string][]byte, error) {
	names := konfig.RecognizedKustomizationFileNames()
	files := map[string][]byte{}
	err := filepath.WalkDir(tree, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == tree {
			return err
		}
		rel, err := filepath.Rel(tree, p)
		if err != nil {
			return err
		}
		if d.IsDir() {
			files[rel] = nil
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files[rel] = b
		var k kusttypes.Kustomization
		if slices.Contains(names, d.Name()) && yaml.Unmarshal(b, &k) == nil && len(k.OpenAPI) > 0 {
			return kerrors.Errorf("%s sets openapi: Flux builds it with a reset of the OpenAPI schema buildLikeFlux does not make, so its build there is not Flux's", rel)
		}
		return nil
	})
	return files, err
}

func fluxInputKey(files map[string][]byte, dir string) string {
	h := sha256.New()
	for _, p := range slices.Sorted(maps.Keys(files)) {
		if files[p] == nil {
			fmt.Fprintf(h, "dir %q\n", p)
			continue
		}
		fmt.Fprintf(h, "file %q %d\n", p, len(files[p]))
		h.Write(files[p])
	}
	fmt.Fprintf(h, "build %q\n", dir)
	return hex.EncodeToString(h.Sum(nil))
}

// buildResult is what a build of a tree at root gave, without root: the YAML
// of what it built, or its error.
func buildResult(rm resmap.ResMap, err error, root string) string {
	if err == nil {
		var y []byte
		if y, err = rm.AsYaml(); err == nil {
			return string(y)
		}
	}
	return "error: " + strings.ReplaceAll(err.Error(), root, "<tree>")
}

// buildsAsFlux builds every tree in fluxInputs again with Flux's SecureBuild
// and reports, on stderr, each build that gave something else than
// buildLikeFlux did. It runs once no test runs, so the schema reset of
// SecureBuild races with nothing.
func buildsAsFlux() bool {
	ok, n := true, 0
	fluxInputs.Range(func(_, v any) bool {
		in := v.(fluxInput)
		n++
		got, err := secureBuildOf(in)
		switch {
		case err != nil:
			fmt.Fprintf(os.Stderr, "buildsAsFlux: %s: %v\n", in.dir, err)
			ok = false
		case got != in.result:
			fmt.Fprintf(os.Stderr, "buildsAsFlux: buildLikeFlux built %s otherwise than Flux's SecureBuild\n--- buildLikeFlux\n%s\n--- SecureBuild\n%s\n", in.dir, in.result, got)
			ok = false
		}
		return true
	})
	fmt.Printf("buildsAsFlux: %d builds compared with Flux's SecureBuild\n", n)
	return ok
}

// secureBuildOf writes the tree of in and builds it with SecureBuild.
func secureBuildOf(in fluxInput) (string, error) {
	tmp, err := os.MkdirTemp("", "buildsasflux-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	root := filepath.Join(tmp, "repo")
	if err := os.Mkdir(root, 0o755); err != nil {
		return "", err
	}
	for p, b := range in.files {
		full := filepath.Join(root, p)
		if b == nil {
			err = os.MkdirAll(full, 0o755)
		} else if err = os.MkdirAll(filepath.Dir(full), 0o755); err == nil {
			err = os.WriteFile(full, b, 0o644)
		}
		if err != nil {
			return "", err
		}
	}
	rm, err := fluxkustomize.SecureBuild(root, filepath.Join(root, in.dir), false)
	return buildResult(rm, err, root), nil
}
