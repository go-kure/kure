package kustomizeschema

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/kustomize/kyaml/openapi"
	"sigs.k8s.io/kustomize/kyaml/resid"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// freshProcessEnv marks the process inFreshProcess starts. Each test here
// needs kustomize's schema as only it has left it, and the race detector's
// history of no earlier test, so each runs in a process of its own.
const freshProcessEnv = "KURE_TEST_FRESH_PROCESS"

// inFreshProcess runs the test name alone in a new process of this test
// binary and fails t with the process's output when it fails. A data race the
// race detector finds there fails it. It reports whether t is that process.
func inFreshProcess(t *testing.T, name string) bool {
	t.Helper()
	if os.Getenv(freshProcessEnv) != "" {
		return true
	}
	cmd := exec.Command(os.Args[0], "-test.run=^"+name+"$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), freshProcessEnv+"=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s in a fresh process: %v\n%s", name, err, out)
	}
	if !strings.Contains(string(out), "--- PASS: "+name) {
		t.Fatalf("%s did not run in the fresh process:\n%s", name, out)
	}
	return false
}

// TestParse_OrdersAScopeLookupBeforeKustomizesParse: in a fresh process,
// where nothing has parsed kustomize's schema yet, a goroutine looks up the
// scope of a kind kustomize's precomputed table lacks, as kure's layout check
// does (resid.NewGvk), and then takes no schema lock. A second later the test
// asks for a type's schema, as a strategic-merge patch in a kustomize build
// does, and kustomize parses the schema, writing the scope table the lookup
// read after releasing its lock. Parse before the lookup parses it first, so
// the later call writes nothing and -race finds no data race; without Parse,
// -race finds that race on every run.
//
// The delay orders nothing: a channel from the goroutine would order its
// lookup before the parse and hide the race. A sleep creates no
// happens-before edge, so the race detector's verdict does not depend on how
// long it is, only on the lookup having run before the parse.
func TestParse_OrdersAScopeLookupBeforeKustomizesParse(t *testing.T) {
	if inFreshProcess(t, "TestParse_OrdersAScopeLookupBeforeKustomizesParse") {
		lookupThenParse(t)
	}
}

// TestParse_ParsesAgainAfterAReset: kustomize's schema is parsed, then reset
// as fluxcd/pkg/kustomize's Build leaves it once it completes, and the same
// lookup and parse as above follow. Parse must parse it again before the
// lookup; one that parsed only the first time leaves the lookup reading the
// scope table kustomize's parse then writes, and -race finds that race.
func TestParse_ParsesAgainAfterAReset(t *testing.T) {
	if inFreshProcess(t, "TestParse_ParsesAgainAfterAReset") {
		Parse()
		openapi.ResetOpenAPI()
		lookupThenParse(t)
	}
}

// lookupThenParse is the sequence both tests run: a goroutine calls Parse and
// looks up a scope, then a type's schema is asked for while it waits.
func lookupThenParse(t *testing.T) {
	t.Helper()
	looked, done := make(chan bool, 1), make(chan struct{})
	go func() {
		Parse()
		looked <- resid.NewGvk("kustomize.toolkit.fluxcd.io", "v1", "Kustomization").IsClusterScoped()
		<-done
	}()
	time.Sleep(time.Second)
	if openapi.SchemaForResourceType(yaml.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"}) == nil {
		t.Error("no schema for v1 ConfigMap")
	}
	close(done)
	if <-looked {
		t.Error("a Flux Kustomization looked up as cluster-scoped")
	}
	if v, ok := BuiltIn(); !ok {
		t.Errorf("schema %q after Parse, want the built-in one", v)
	}
}
