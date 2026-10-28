package fluxcd_test

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/fluxcd/pkg/apis/kustomize"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/kyaml/openapi"

	"github.com/go-kure/kure/pkg/errors"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// kustomize parses its OpenAPI schema once per process, lazily, and keeps
// any schema a use of kustomize sets: the tests here that depend on that run
// in a process of their own, which inFreshProcess starts.
const (
	freshProcessEnv = "KURE_TEST_FRESH_PROCESS"
	webDirEnv       = "KURE_TEST_WEB_DIR"
)

// inFreshProcess runs the test name alone in a new process of this test
// binary, with webDir in its environment, and fails t with the process's
// output when it fails. A data race the race detector finds there fails it.
func inFreshProcess(t *testing.T, name, webDir string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^"+name+"$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), freshProcessEnv+"=1", webDirEnv+"="+webDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s in a fresh process: %v\n%s", name, err, out)
	}
	if !strings.Contains(string(out), "--- PASS: "+name) {
		t.Fatalf("%s did not run in the fresh process:\n%s", name, out)
	}
}

// schemaTree is the tree of TestBuiltView_ABuildThatReadsOpenAPIIsRefused
// without openapi: hand-written builds web's directory, dir, with the
// component extra, which a layout extra file writes and which adds the
// ConfigMap added; patch, if set, is a strategic-merge patch of added. It
// needs no testing.T, so goroutines can build it.
func schemaTree(dir string, patch bool) (*layout.ManifestLayout, error) {
	k := handWritten("hand-written")
	k.Spec.Path, k.Spec.Components = dir, []string{"extra"}
	if patch {
		k.Spec.Patches = []kustomize.Patch{{Patch: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: added\n  namespace: default\ndata:\n  patched: \"true\"\n"}}
	}
	cluster := noWaitCluster(srBundle("platform", objApp("platform-app", k)), false)
	rules := withPlacement(nodeOnly, layout.FluxIntegratedPerBundle)
	ml, err := layout.WalkCluster(cluster, rules)
	if err != nil {
		return nil, err
	}
	var at *layout.ManifestLayout
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		if at == nil && l.FullRepoPath() == dir {
			at = l
		}
		for _, c := range l.Children {
			walk(c)
		}
	}
	walk(ml)
	if at == nil {
		return nil, errors.Errorf("no layout at %s", dir)
	}
	at.ExtraFiles = append(at.ExtraFiles,
		layout.ExtraFile{Name: "extra/kustomization.yaml", Content: []byte("apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\nresources:\n- added.yaml\n")},
		layout.ExtraFile{Name: "extra/added.yaml", Content: []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: added\n  namespace: default\n")})
	return ml, fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, cluster, rules)
}

// TestKustomizeSchema_ColdIntegrationsDoNotRace: in a process where
// kustomize has not yet parsed its schema, layout walks and integrations
// start together. The layout check of each walk's write looks up the scope of
// a kind kustomize's precomputed table lacks (resid.NewGvk, here a Flux
// Kustomization), and the strategic-merge patch of each hand-written build
// makes kustomize parse the schema. kure parses it once before either, so
// under -race no data race is found.
//
// It is a check that cold, concurrent uses of kure pass under -race, not a
// proof that kure's parse is needed: a walk takes kustomize's schema lock
// again after its lookups, which orders them before any later parse, so a
// parse races with a lookup only when it starts between the two, a window
// this test does not hit, and it passes without kure's parse too. That proof
// is internal/kustomizeschema's test, whose lookup takes no lock after it.
func TestKustomizeSchema_ColdIntegrationsDoNotRace(t *testing.T) {
	if os.Getenv(freshProcessEnv) == "" {
		inFreshProcess(t, "TestKustomizeSchema_ColdIntegrationsDoNotRace", webDirectory(t, layout.FluxIntegratedPerBundle))
		return
	}
	const integrations, walks = 8, 8
	errs := make([]error, integrations+walks)
	var wg sync.WaitGroup
	for i := range walks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ks := make([]client.Object, 0, 50)
			for j := range 50 {
				ks = append(ks, handWritten(fmt.Sprintf("walked-%d", j)))
			}
			ml, err := layout.WalkCluster(noWaitCluster(srBundle("platform", objApp("platform-app", ks...)), false), withPlacement(nodeOnly, layout.FluxIntegratedPerBundle))
			if err == nil {
				err = ml.WriteToTar(io.Discard)
			}
			errs[integrations+i] = err
		}()
	}
	for i := range integrations {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = schemaTree(os.Getenv(webDirEnv), true)
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", i, err)
		}
	}
}

// TestKustomizeSchema_ACustomSchemaIsNotRead: another use of kustomize in
// the process set a custom schema, which kustomize keeps for every later
// build and kustomize-controller never builds with. hand-written's build is
// not read, naming it and the schema, never read under that schema. The
// control is the same tree in a process whose schema is the built-in one.
func TestKustomizeSchema_ACustomSchemaIsNotRead(t *testing.T) {
	if os.Getenv(freshProcessEnv) == "" {
		dir := webDirectory(t, layout.FluxIntegratedPerBundle)
		inFreshProcess(t, "TestKustomizeSchema_ACustomSchemaIsNotRead", dir)
		if _, err := schemaTree(dir, false); err != nil {
			t.Fatalf("with the built-in schema: got %v, want the tree admitted", err)
		}
		return
	}
	custom := []byte(`{"swagger":"2.0","info":{"title":"custom","version":"v1"},"paths":{},"definitions":{}}`)
	if err := openapi.SetSchema(nil, custom, true); err != nil {
		t.Fatalf("set a custom schema: %v", err)
	}
	_, err := schemaTree(os.Getenv(webDirEnv), false)
	const want = `Flux Kustomization "flux-system/hand-written" (spec.path`
	if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "is not read") || !strings.Contains(err.Error(), `schema in this process is "using custom schema from file provided", not its built-in one`) {
		t.Fatalf("got %v, want %s ...) not read under a custom schema", err, want)
	}
}
