package fluxcd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"sigs.k8s.io/kustomize/kyaml/openapi"
)

// definitionsFreshEnv marks the process inDefinitionsProcess starts: the tests
// here change the definitions of kustomize's process-global schema, which
// every later test of the binary would read.
const definitionsFreshEnv = "KURE_TEST_FRESH_PROCESS"

// inDefinitionsProcess runs the test name alone in a new process of this test
// binary and fails t with the process's output when it fails. It reports
// whether t is that process.
func inDefinitionsProcess(t *testing.T, name string) bool {
	t.Helper()
	if os.Getenv(definitionsFreshEnv) != "" {
		return true
	}
	cmd := exec.Command(os.Args[0], "-test.run=^"+name+"$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), definitionsFreshEnv+"=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s in a fresh process: %v\n%s", name, err, out)
	}
	if !strings.Contains(string(out), "--- PASS: "+name) {
		t.Fatalf("%s did not run in the fresh process:\n%s", name, out)
	}
	return false
}

// addDefinition adds a definition under a key the built-in schema lacks, as
// openapi.AddSchema adds a CRD's.
func addDefinition(t *testing.T) {
	t.Helper()
	added := `{"swagger":"2.0","info":{"title":"added","version":"v1"},"paths":{},"definitions":{"io.example.v1.Added":{"type":"object"}}}`
	if err := openapi.AddSchema([]byte(added)); err != nil {
		t.Fatalf("add a schema: %v", err)
	}
}

// definitionTree writes a tree of one directory app, which builds a
// ConfigMap, and returns the treeBuilds over it and a Kustomization of app.
func definitionTree(t *testing.T) (*treeBuilds, *kustv1.Kustomization) {
	t.Helper()
	tree := t.TempDir()
	for name, content := range map[string]string{"app/kustomization.yaml": appKustomization, "app/cm.yaml": appConfigMap} {
		p := filepath.Join(tree, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	k := &kustv1.Kustomization{}
	k.Name, k.Namespace = "app", "flux-system"
	k.Spec.Path = "app"
	return &treeBuilds{dir: tree, written: true}, k
}

// TestTreeBuilds_AnAddedDefinitionIsNotRead: a definition added to the
// built-in schema before the first build refuses that build and every later
// one of the tree, naming the definitions (go-kure/kure#1166). The control is
// the unchanged schema, whose builds are read and whose finish finds nothing.
func TestTreeBuilds_AnAddedDefinitionIsNotRead(t *testing.T) {
	if !inDefinitionsProcess(t, "TestTreeBuilds_AnAddedDefinitionIsNotRead") {
		return
	}
	b, k := definitionTree(t)
	if _, err := b.build(k); err != nil {
		t.Fatalf("with the built-in schema: got %v, want the build read", err)
	}
	if err := b.finish(); err != nil {
		t.Fatalf("with the built-in schema: finish got %v, want none", err)
	}

	addDefinition(t)
	changed, k := definitionTree(t)
	for i := range 2 {
		_, err := changed.build(k)
		mustBeUnread(t, err, "definitions of kustomize's OpenAPI schema in this process are not its built-in ones", "before its first build")
		if i == 0 && !changed.definitionsChecked {
			t.Error("the first build did not check the definitions")
		}
	}
	if err := changed.finish(); err != nil {
		t.Errorf("finish got %v after a refusal at the first build, want none: the builds refused each", err)
	}
}

// TestTreeBuilds_ADefinitionAddedAfterTheFirstBuildRefusesTheTree: the first
// build is read; another use of kustomize then adds a definition, which the
// later builds of the tree would be built over. The definitions are checked
// per tree, so those builds are not refused one by one, and finish, after the
// last build, refuses the tree instead: newBuiltViews calls it before judging
// any view.
func TestTreeBuilds_ADefinitionAddedAfterTheFirstBuildRefusesTheTree(t *testing.T) {
	if !inDefinitionsProcess(t, "TestTreeBuilds_ADefinitionAddedAfterTheFirstBuildRefusesTheTree") {
		return
	}
	b, k := definitionTree(t)
	if _, err := b.build(k); err != nil {
		t.Fatalf("first build: got %v, want it read", err)
	}
	addDefinition(t)
	if _, err := b.build(k); err != nil {
		t.Fatalf("second build: got %v, want the once-per-tree check not to repeat", err)
	}
	err := b.finish()
	if err == nil || !strings.Contains(err.Error(), "not its built-in ones") || !strings.Contains(err.Error(), "while its builds ran") || !strings.Contains(err.Error(), "none of what the tree's builds make is read") {
		t.Fatalf("finish got %v, want the tree refused for a definition added while its builds ran", err)
	}
}

// TestTreeBuilds_AVersionChangedAfterTheFirstBuildRefusesTheTree: the version
// check runs per build, but not after the last one, so finish repeats it.
func TestTreeBuilds_AVersionChangedAfterTheFirstBuildRefusesTheTree(t *testing.T) {
	if !inDefinitionsProcess(t, "TestTreeBuilds_AVersionChangedAfterTheFirstBuildRefusesTheTree") {
		return
	}
	b, k := definitionTree(t)
	if _, err := b.build(k); err != nil {
		t.Fatalf("first build: got %v, want it read", err)
	}
	if err := openapi.SetSchema(nil, []byte(`{"swagger":"2.0","info":{"title":"custom","version":"v1"},"paths":{},"definitions":{}}`), true); err != nil {
		t.Fatalf("set a custom schema: %v", err)
	}
	err := b.finish()
	if err == nil || !strings.Contains(err.Error(), "not its built-in one") || !strings.Contains(err.Error(), "the schema changed while the tree's builds ran") {
		t.Fatalf("finish got %v, want the tree refused for a schema set while its builds ran", err)
	}
}

// TestTreeBuilds_FinishAsksNothingOfATreeThatBuiltNothing: a tree no build ran
// over has nothing to refuse, whatever the schema holds, and costs no hash.
func TestTreeBuilds_FinishAsksNothingOfATreeThatBuiltNothing(t *testing.T) {
	if !inDefinitionsProcess(t, "TestTreeBuilds_FinishAsksNothingOfATreeThatBuiltNothing") {
		return
	}
	addDefinition(t)
	b := &treeBuilds{}
	if err := b.finish(); err != nil {
		t.Errorf("finish got %v on a tree that built nothing, want none", err)
	}
	if b.definitionsChecked {
		t.Error("finish checked the definitions of a tree that built nothing")
	}
}
