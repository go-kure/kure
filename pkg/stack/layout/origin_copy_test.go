package layout_test

import (
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// TestIndexOrigins_DependencyCopy: the index resolves a DependsOn bundle by
// its Name, so a copy of a rendered bundle stands for that bundle. A copy that
// sets another KustomizationName than the rendered bundle's is refused, naming
// the dependant, the bundle and both names; a copy of a bundle whose
// Kustomization name is also in the dependant's NamedDependsOn is one
// dependency in both lists. Both are said in the words stack.ValidateCluster
// uses for the same cluster. A copy without a KustomizationName is the bundle.
// The tree is walked while the copy agrees and nothing is in both lists, so
// each refusal is the index's own.
func TestIndexOrigins_DependencyCopy(t *testing.T) {
	for _, tc := range []struct {
		name     string
		copyName string
		named    []string
		want     []string // substrings of the error; nil means indexed
	}{
		{name: "the bundle's KustomizationName", copyName: "db-cr"},
		{name: "no KustomizationName", copyName: ""},
		{name: "no KustomizationName, beside another named dependency", copyName: "", named: []string{"external"}},
		{name: "another KustomizationName", copyName: "x",
			want: []string{`bundle "web"`, `copy of bundle "db"`, `KustomizationName "x"`, `KustomizationName "db-cr"`}},
		{name: "another KustomizationName, also a named dependency", copyName: "x", named: []string{"x"},
			want: []string{`bundle "web"`, `copy of bundle "db"`, `KustomizationName "x"`, `KustomizationName "db-cr"`}},
		{name: "no KustomizationName, the bundle's is a named dependency", copyName: "", named: []string{"db-cr"},
			want: []string{`bundle "web"`, `dependency "db-cr" appears in both DependsOn and NamedDependsOn`, `DependsOn bundle "db"`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := &stack.Bundle{Name: "db", KustomizationName: "db-cr", Applications: []*stack.Application{configMapApp("db")}}
			copyOfDB := *db
			web := &stack.Bundle{Name: "web", Applications: []*stack.Application{configMapApp("web")},
				DependsOn: []*stack.Bundle{&copyOfDB}}
			dbn := &stack.Node{Name: "dbn", Bundle: db}
			webn := &stack.Node{Name: "webn", Bundle: web}
			r := &stack.Node{Name: "r", Children: []*stack.Node{dbn, webn}}
			dbn.SetParent(r)
			webn.SetParent(r)
			c := &stack.Cluster{Name: "demo", Node: r}
			ml := walk(t, c, nodeOnly)
			if _, err := layout.IndexOrigins(ml, c); err != nil {
				t.Fatalf("IndexOrigins with a true copy: %v", err)
			}

			copyOfDB.KustomizationName = tc.copyName
			web.NamedDependsOn = tc.named
			ix, err := layout.IndexOrigins(ml, c)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("IndexOrigins: %v", err)
				}
				if got := ix.UnitDependencies(layoutAt(t, ml, "r/webn")); len(got) != 1 || got[0] != "db-cr" {
					t.Errorf("UnitDependencies(web) = %v, want [db-cr]", got)
				}
				if verr := stack.ValidateCluster(c); verr != nil {
					t.Errorf("ValidateCluster refuses what IndexOrigins accepts: %v", verr)
				}
				return
			}
			if err == nil {
				t.Fatal("IndexOrigins accepted the dependency copy")
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("IndexOrigins error %q does not contain %s", err, want)
				}
			}
			verr := stack.ValidateCluster(c)
			if verr == nil || !strings.Contains(verr.Error(), err.Error()) {
				t.Errorf("ValidateCluster error %q does not say what IndexOrigins says: %q", verr, err)
			}
		})
	}
}

// TestIndexOrigins_DependencyCopyWithTheNameInEffect: the copy is compared
// with the rendered bundle on the name in effect, not on the field. A bundle
// without a KustomizationName is named by its Name, so a copy that sets that
// Name as its KustomizationName names the same Kustomization and is the
// bundle; any other value is refused.
func TestIndexOrigins_DependencyCopyWithTheNameInEffect(t *testing.T) {
	db := &stack.Bundle{Name: "db", Applications: []*stack.Application{configMapApp("db")}}
	copyOfDB := *db
	web := &stack.Bundle{Name: "web", Applications: []*stack.Application{configMapApp("web")},
		DependsOn: []*stack.Bundle{&copyOfDB}}
	dbn := &stack.Node{Name: "dbn", Bundle: db}
	webn := &stack.Node{Name: "webn", Bundle: web}
	r := &stack.Node{Name: "r", Children: []*stack.Node{dbn, webn}}
	dbn.SetParent(r)
	webn.SetParent(r)
	c := &stack.Cluster{Name: "demo", Node: r}
	ml := walk(t, c, nodeOnly)

	copyOfDB.KustomizationName = "db"
	ix, err := layout.IndexOrigins(ml, c)
	if err != nil {
		t.Fatalf("IndexOrigins refuses a copy that names the bundle's Kustomization: %v", err)
	}
	if got := ix.UnitDependencies(layoutAt(t, ml, "r/webn")); len(got) != 1 || got[0] != "db" {
		t.Errorf("UnitDependencies(web) = %v, want [db]", got)
	}
	if verr := stack.ValidateCluster(c); verr != nil {
		t.Errorf("ValidateCluster refuses what IndexOrigins accepts: %v", verr)
	}

	copyOfDB.KustomizationName = "x"
	assertIndexError(t, ml, c, `copy of bundle "db" with KustomizationName "x"`)
}

// TestIndexOrigins_DependencyCopyOfAnUmbrellaChild: the copy is compared with
// the rendered bundle wherever that sits, and the bundle is named by its path.
func TestIndexOrigins_DependencyCopyOfAnUmbrellaChild(t *testing.T) {
	db := &stack.Bundle{Name: "db", KustomizationName: "db-cr", Applications: []*stack.Application{configMapApp("db")}}
	platform := &stack.Bundle{Name: "platform", Applications: []*stack.Application{configMapApp("core")}, Children: []*stack.Bundle{db}}
	copyOfDB := *db
	web := &stack.Bundle{Name: "web", Applications: []*stack.Application{configMapApp("web")},
		DependsOn: []*stack.Bundle{&copyOfDB}}
	pn := &stack.Node{Name: "pn", Bundle: platform}
	webn := &stack.Node{Name: "webn", Bundle: web}
	r := &stack.Node{Name: "r", Children: []*stack.Node{pn, webn}}
	pn.SetParent(r)
	webn.SetParent(r)
	c := &stack.Cluster{Name: "demo", Node: r}
	ml := walk(t, c, nodeOnly)

	copyOfDB.KustomizationName = "x"
	assertIndexError(t, ml, c, `copy of bundle "platform/db"`)
}
