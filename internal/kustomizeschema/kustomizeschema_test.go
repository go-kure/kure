package kustomizeschema

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/kustomize/kyaml/openapi"
	"sigs.k8s.io/kustomize/kyaml/openapi/kubernetesapi"
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

// TestBuiltIn_TellsTheDefaultSchemaFromACustomOne runs in the test binary's
// own process, so that it counts for coverage: after Parse the schema is
// kustomize's default built-in one; a custom schema set after it is not, and a
// reset makes it the default again.
func TestBuiltIn_TellsTheDefaultSchemaFromACustomOne(t *testing.T) {
	Parse()
	if v, ok := BuiltIn(); !ok || v != kubernetesapi.DefaultOpenAPI {
		t.Errorf("BuiltIn() = %q, %v after Parse, want %q, true", v, ok, kubernetesapi.DefaultOpenAPI)
	}
	if err := openapi.SetSchema(nil, []byte(`{"definitions":{}}`), true); err != nil {
		t.Fatalf("set a custom schema: %v", err)
	}
	t.Cleanup(func() {
		openapi.ResetOpenAPI()
		Parse()
	})
	if v, ok := BuiltIn(); ok {
		t.Errorf("BuiltIn() = %q, true under a custom schema, want false", v)
	}
	openapi.ResetOpenAPI()
	Parse()
	if v, ok := BuiltIn(); !ok {
		t.Errorf("BuiltIn() = %q, false after a reset, want true", v)
	}
}

// TestDefinitions_BuiltInMatchesThePin: in a fresh process, with nothing yet
// parsed, the fingerprint of kustomize's untouched schema is builtInDefinitions.
// A kyaml bump that changes the definitions, or the layout of the types the
// walk reads, fails it with the value to set; without this test the stale pin
// would refuse every build. A call before any Parse also shows that
// Definitions parses the schema itself.
func TestDefinitions_BuiltInMatchesThePin(t *testing.T) {
	if inFreshProcess(t, "TestDefinitions_BuiltInMatchesThePin") {
		got, ok := Definitions()
		if !ok {
			t.Fatalf("Definitions() = %s, want the pin %s: set builtInDefinitions to it if kyaml, or the types of its schema, changed", got, builtInDefinitions)
		}
		again, _ := Definitions()
		if again != got {
			t.Errorf("Definitions() = %s, then %s: it is not stable", got, again)
		}
	}
}

// changedDefinitions runs mutate in a fresh process on the parsed built-in
// schema and wants Definitions to tell it, with a fingerprint other than the
// pin's; the control is the pin test above, which finds the schema unchanged.
func changedDefinitions(t *testing.T, name string, mutate func(t *testing.T)) {
	t.Helper()
	if !inFreshProcess(t, name) {
		return
	}
	Parse()
	if got, ok := Definitions(); !ok {
		t.Fatalf("before the change Definitions() = %s, want the pin", got)
	}
	mutate(t)
	if got, ok := Definitions(); ok || got == builtInDefinitions {
		t.Errorf("after the change Definitions() = %s, %v, want a fingerprint other than the pin", got, ok)
	}
	if v, ok := BuiltIn(); !ok {
		t.Errorf("BuiltIn() = %q, false: the change was meant to leave the version alone", v)
	}
}

// TestDefinitions_AnAddedDefinitionIsDetected: a definition under a new key,
// as openapi.AddSchema adds a CRD's.
func TestDefinitions_AnAddedDefinitionIsDetected(t *testing.T) {
	changedDefinitions(t, "TestDefinitions_AnAddedDefinitionIsDetected", func(t *testing.T) {
		add := `{"swagger":"2.0","info":{"title":"x","version":"v1"},"paths":{},"definitions":{"io.example.v1.Added":{"type":"object"}}}`
		if err := openapi.AddSchema([]byte(add)); err != nil {
			t.Fatalf("add a schema: %v", err)
		}
	})
}

// TestDefinitions_AReplacedDefinitionIsDetected: a definition under a key the
// built-in schema has, with the same number of definitions: a check of the
// keys alone does not see it.
func TestDefinitions_AReplacedDefinitionIsDetected(t *testing.T) {
	changedDefinitions(t, "TestDefinitions_AReplacedDefinitionIsDetected", func(t *testing.T) {
		replace := `{"swagger":"2.0","info":{"title":"x","version":"v1"},"paths":{},"definitions":{"io.k8s.api.core.v1.ConfigMap":{"type":"object","description":"replaced"}}}`
		if err := openapi.AddSchema([]byte(replace)); err != nil {
			t.Fatalf("replace a definition: %v", err)
		}
	})
}

// TestDefinitions_AnEditInPlaceIsDetected: a write through the schema
// openapi.Schema() returns, which adds no definition and replaces none.
func TestDefinitions_AnEditInPlaceIsDetected(t *testing.T) {
	changedDefinitions(t, "TestDefinitions_AnEditInPlaceIsDetected", func(t *testing.T) {
		defs := openapi.Schema().Definitions
		d, ok := defs["io.k8s.api.core.v1.ConfigMap"]
		if !ok {
			t.Fatal("no ConfigMap definition")
		}
		d.Description += " edited"
		defs["io.k8s.api.core.v1.ConfigMap"] = d
	})
}

// TestDefinitions_AMoveToExtraPropsIsDetected: the merge strategy of a pod's
// containers, "x-kubernetes-patch-strategy", is moved from the property's
// Extensions, which kyaml reads, to its ExtraProps, which it does not. The
// schema marshals to the same JSON before and after (the control: a hash of
// the JSON would not see it), while a patch to one container now replaces the
// list, and Definitions sees it.
func TestDefinitions_AMoveToExtraPropsIsDetected(t *testing.T) {
	changedDefinitions(t, "TestDefinitions_AMoveToExtraPropsIsDetected", func(t *testing.T) {
		const strategy = "x-kubernetes-patch-strategy"
		defs := openapi.Schema().Definitions
		before, err := json.Marshal(defs)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		props := defs["io.k8s.api.core.v1.PodSpec"].Properties
		containers := props["containers"]
		value, ok := containers.Extensions[strategy]
		if !ok {
			t.Fatalf("PodSpec containers has no %s extension", strategy)
		}
		delete(containers.Extensions, strategy)
		containers.ExtraProps = map[string]any{strategy: value}
		props["containers"] = containers
		after, err := json.Marshal(defs)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if string(before) != string(after) {
			t.Fatal("the move changed the JSON: the test no longer shows what a JSON hash misses")
		}
	})
}

// TestDefinitions_ACopyInExtraPropsIsDetected: the same strategy is also
// written to ExtraProps, with the Extensions entry left in place: the JSON
// merges both into one key of one value, so it is unchanged again, and kyaml
// reads the same Extensions. Nothing of what kyaml reads changed, but a
// fingerprint that is exact for every field must still see the write, which
// is what makes the ExtraProps half of the previous case count on its own.
func TestDefinitions_ACopyInExtraPropsIsDetected(t *testing.T) {
	changedDefinitions(t, "TestDefinitions_ACopyInExtraPropsIsDetected", func(t *testing.T) {
		const strategy = "x-kubernetes-patch-strategy"
		defs := openapi.Schema().Definitions
		before, err := json.Marshal(defs)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		props := defs["io.k8s.api.core.v1.PodSpec"].Properties
		containers := props["containers"]
		value, ok := containers.Extensions[strategy]
		if !ok {
			t.Fatalf("PodSpec containers has no %s extension", strategy)
		}
		containers.ExtraProps = map[string]any{strategy: value}
		props["containers"] = containers
		after, err := json.Marshal(defs)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if string(before) != string(after) {
			t.Fatal("the copy changed the JSON: the test no longer shows what a JSON hash misses")
		}
	})
}

// TestDefinitions_AChangedExtensionIsDetected: the merge strategy of a pod's
// containers is changed to "replace" in the property's Extensions, the field
// kyaml reads, and nothing else is touched: it is the half of the walk that
// no other case isolates.
func TestDefinitions_AChangedExtensionIsDetected(t *testing.T) {
	changedDefinitions(t, "TestDefinitions_AChangedExtensionIsDetected", func(t *testing.T) {
		const strategy = "x-kubernetes-patch-strategy"
		props := openapi.Schema().Definitions["io.k8s.api.core.v1.PodSpec"].Properties
		containers := props["containers"]
		if _, ok := containers.Extensions[strategy]; !ok {
			t.Fatalf("PodSpec containers has no %s extension", strategy)
		}
		containers.Extensions[strategy] = "replace"
		props["containers"] = containers
	})
}

// namedStrategy is a string by kind only: kyaml's own type assertion on a
// patch strategy (openapi.go PatchStrategyAndKey, ps.(string)) rejects it.
type namedStrategy string

// TestDefinitions_ANamedTypeValueIsDetected: the containers strategy is
// replaced by namedStrategy("merge"), which reads as "merge" to a walk that
// tags kinds only and makes kyaml's assertion panic on a container patch.
func TestDefinitions_ANamedTypeValueIsDetected(t *testing.T) {
	changedDefinitions(t, "TestDefinitions_ANamedTypeValueIsDetected", func(t *testing.T) {
		const strategy = "x-kubernetes-patch-strategy"
		props := openapi.Schema().Definitions["io.k8s.api.core.v1.PodSpec"].Properties
		containers := props["containers"]
		if v, ok := containers.Extensions[strategy]; !ok || v != "merge" {
			t.Fatalf("PodSpec containers %s = %v, want merge", strategy, v)
		}
		containers.Extensions[strategy] = namedStrategy("merge")
		props["containers"] = containers
	})
}

// TestDefinitions_ARemovedDefinitionIsDetected: a definition deleted from the
// schema.
func TestDefinitions_ARemovedDefinitionIsDetected(t *testing.T) {
	changedDefinitions(t, "TestDefinitions_ARemovedDefinitionIsDetected", func(t *testing.T) {
		delete(openapi.Schema().Definitions, "io.k8s.api.core.v1.ConfigMap")
	})
}

// TestDefinitions_AResetReturnsToTheBuiltIn: after definitions were added, a
// reset and the next parse leave the built-in schema, whose fingerprint is
// the pin's again.
func TestDefinitions_AResetReturnsToTheBuiltIn(t *testing.T) {
	if !inFreshProcess(t, "TestDefinitions_AResetReturnsToTheBuiltIn") {
		return
	}
	add := `{"swagger":"2.0","info":{"title":"x","version":"v1"},"paths":{},"definitions":{"io.example.v1.Added":{"type":"object"}}}`
	Parse()
	if err := openapi.AddSchema([]byte(add)); err != nil {
		t.Fatalf("add a schema: %v", err)
	}
	if got, ok := Definitions(); ok {
		t.Fatalf("Definitions() = %s after an added definition, want a fingerprint other than the pin", got)
	}
	openapi.ResetOpenAPI()
	Parse()
	if got, ok := Definitions(); !ok {
		t.Errorf("Definitions() = %s after a reset, want the pin", got)
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
