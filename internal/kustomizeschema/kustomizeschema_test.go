package kustomizeschema

import (
	"testing"
	"time"

	"sigs.k8s.io/kustomize/kyaml/openapi"
	"sigs.k8s.io/kustomize/kyaml/resid"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// TestParse_OrdersAScopeLookupBeforeKustomizesParse: in this package's own
// process, where nothing has parsed kustomize's schema yet, a goroutine looks
// up the scope of a kind kustomize's precomputed table lacks, as kure's layout
// check does (resid.NewGvk), and then takes no schema lock. A second later
// this goroutine asks for a type's schema, as a strategic-merge patch in a
// kustomize build does, and kustomize parses the schema, writing the scope
// table the lookup read after releasing its lock. Parse before the lookup
// parses it first, so the later call writes nothing and -race finds no data
// race; without Parse, -race finds that race on every run.
//
// The delay orders nothing: a channel from the goroutine would order its
// lookup before the parse and hide the race. A sleep creates no
// happens-before edge, so the race detector's verdict does not depend on how
// long it is, only on the lookup having run before the parse.
func TestParse_OrdersAScopeLookupBeforeKustomizesParse(t *testing.T) {
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
