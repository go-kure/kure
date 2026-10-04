package layout_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for Bundle.KustomizationName in the origin index: a unit is named
// after its first bundle's Kustomization name, references to a Kustomization
// by name resolve through those names, and a bundle is still found by its own
// name.

// TestIndexOrigins_KustomizationName: with every bundle in its own directory,
// a unit takes its bundle's KustomizationName; a dependency on the bundle
// resolves to it, whether the dependency is the bundle, a copy of it or a
// bundle that only shares its name; and a reference by name finds the unit
// under its Kustomization name, not under the bundle's.
func TestIndexOrigins_KustomizationName(t *testing.T) {
	db := &stack.Bundle{Name: "db", KustomizationName: "db-cr", Applications: []*stack.Application{configMapApp("db")}}
	copyOfDB := *db
	web := &stack.Bundle{Name: "web", Applications: []*stack.Application{configMapApp("web")},
		DependsOn: []*stack.Bundle{&copyOfDB}}
	api := &stack.Bundle{Name: "api", Applications: []*stack.Application{configMapApp("api")},
		NamedDependsOn: []string{"db-cr", "db", "external"}}
	dbn := &stack.Node{Name: "dbn", Bundle: db}
	webn := &stack.Node{Name: "webn", Bundle: web}
	apin := &stack.Node{Name: "apin", Bundle: api}
	r := &stack.Node{Name: "r", Children: []*stack.Node{dbn, webn, apin}}
	for _, n := range r.Children {
		n.SetParent(r)
	}
	c := &stack.Cluster{Name: "demo", Node: r}
	ml := walk(t, c, nodeOnly)
	ix, err := layout.IndexOrigins(ml, c)
	if err != nil {
		t.Fatalf("IndexOrigins: %v", err)
	}

	for _, tc := range []struct {
		what string
		b    *stack.Bundle
		want string
	}{
		{"the bundle", db, "db-cr"},
		{"a copy of the bundle", &copyOfDB, "db-cr"},
		{"a bundle that only shares its name", &stack.Bundle{Name: "db"}, "db-cr"},
		{"a bundle without the field", web, "web"},
		{"a bundle outside the index", &stack.Bundle{Name: "outside", KustomizationName: "outside-cr"}, "outside-cr"},
	} {
		if got := ix.UnitName(tc.b); got != tc.want {
			t.Errorf("UnitName(%s) = %q, want %q", tc.what, got, tc.want)
		}
	}
	if got := ix.UnitOfName("db-cr"); got != "db-cr" {
		t.Errorf(`UnitOfName("db-cr") = %q, want db-cr`, got)
	}
	if got := ix.UnitOfName("db"); got != "db" {
		t.Errorf(`UnitOfName("db") = %q, want it unchanged: no Kustomization has the bundle's name`, got)
	}
	if got := ix.UnitDependencies(layoutAt(t, ml, "r/webn")); !slices.Equal(got, []string{"db-cr"}) {
		t.Errorf("UnitDependencies(web) = %v, want [db-cr]", got)
	}
	if got := ix.UnitNamedDependencies(layoutAt(t, ml, "r/apin")); !slices.Equal(got, []string{"db-cr", "db", "external"}) {
		t.Errorf("UnitNamedDependencies(api) = %v, want [db-cr db external]", got)
	}
}

// TestIndexOrigins_KustomizationName_MergedUnit: bundles merged into one
// directory share one unit, named after the first bundle's KustomizationName.
// A reference to a merged bundle's own Kustomization name follows the merge,
// and one from inside the unit is dropped.
func TestIndexOrigins_KustomizationName_MergedUnit(t *testing.T) {
	rb := &stack.Bundle{Name: "rb", KustomizationName: "root-cr", Applications: []*stack.Application{configMapApp("core")},
		NamedDependsOn: []string{"b1-cr", "external"}}
	b1 := &stack.Bundle{Name: "b1", KustomizationName: "b1-cr", Applications: []*stack.Application{configMapApp("one")}}
	c1 := &stack.Node{Name: "c1", Bundle: b1}
	r := &stack.Node{Name: "r", Bundle: rb, Children: []*stack.Node{c1}}
	c1.SetParent(r)
	c := &stack.Cluster{Name: "demo", Node: r}
	ml := walk(t, c, nodeFlat)
	ix, err := layout.IndexOrigins(ml, c)
	if err != nil {
		t.Fatalf("IndexOrigins: %v", err)
	}
	if got := ix.UnitName(b1); got != "root-cr" {
		t.Errorf("UnitName(b1) = %q, want root-cr, the unit it is merged into", got)
	}
	if got := ix.UnitOfName("b1-cr"); got != "root-cr" {
		t.Errorf(`UnitOfName("b1-cr") = %q, want root-cr`, got)
	}
	if got := ix.UnitOfName("b1"); got != "b1" {
		t.Errorf(`UnitOfName("b1") = %q, want it unchanged`, got)
	}
	if got := ix.UnitNamedDependencies(ix.Units()[0]); !slices.Equal(got, []string{"external"}) {
		t.Errorf("UnitNamedDependencies = %v, want [external] (b1-cr is applied by this unit)", got)
	}
}

// TestIndexOrigins_KustomizationNameOfAnotherBundlesName: a bundle may take as
// KustomizationName the Name of a bundle that has a KustomizationName of its
// own: no two Kustomizations share a name. A reference to that name then
// reaches the bundle whose Kustomization has it, not the bundle called so.
func TestIndexOrigins_KustomizationNameOfAnotherBundlesName(t *testing.T) {
	db := &stack.Bundle{Name: "db", KustomizationName: "db-cr", Applications: []*stack.Application{configMapApp("db")}}
	worker := &stack.Bundle{Name: "worker", KustomizationName: "db", Applications: []*stack.Application{configMapApp("worker")}}
	dbn := &stack.Node{Name: "dbn", Bundle: db}
	workern := &stack.Node{Name: "workern", Bundle: worker}
	r := &stack.Node{Name: "r", Children: []*stack.Node{dbn, workern}}
	dbn.SetParent(r)
	workern.SetParent(r)
	c := &stack.Cluster{Name: "demo", Node: r}
	ix, err := layout.IndexOrigins(walk(t, c, nodeOnly), c)
	if err != nil {
		t.Fatalf("IndexOrigins: %v", err)
	}
	if got := ix.UnitName(db); got != "db-cr" {
		t.Errorf("UnitName(db) = %q, want db-cr", got)
	}
	if got := ix.UnitName(worker); got != "db" {
		t.Errorf("UnitName(worker) = %q, want db", got)
	}
	if got := ix.UnitName(&stack.Bundle{Name: "db"}); got != "db-cr" {
		t.Errorf("UnitName(a bundle named db) = %q, want db-cr: a bundle is found by its Name", got)
	}
}

// TestIndexOrigins_RejectsDuplicateKustomizationName: two bundles whose
// Kustomizations would get one name are refused, and the error names both
// bundles by their paths and the name they share.
func TestIndexOrigins_RejectsDuplicateKustomizationName(t *testing.T) {
	app := func(name string) []*stack.Application { return []*stack.Application{configMapApp(name)} }
	cluster := func(bundles ...*stack.Bundle) *stack.Cluster {
		r := &stack.Node{Name: "r"}
		for i, b := range bundles {
			n := &stack.Node{Name: string(rune('a' + i)), Bundle: b}
			n.SetParent(r)
			r.Children = append(r.Children, n)
		}
		return &stack.Cluster{Name: "demo", Node: r}
	}
	tests := []struct {
		name    string
		cluster *stack.Cluster
		want    []string
	}{
		{
			name: "two bundles set one KustomizationName",
			cluster: cluster(
				&stack.Bundle{Name: "shop-a", KustomizationName: "shop", Applications: app("one")},
				&stack.Bundle{Name: "shop-b", KustomizationName: "shop", Applications: app("two")}),
			want: []string{`"shop-a"`, `"shop-b"`, `"shop"`},
		},
		{
			name: "a KustomizationName equal to another bundle's name",
			cluster: cluster(
				&stack.Bundle{Name: "shop", Applications: app("one")},
				&stack.Bundle{Name: "store", KustomizationName: "shop", Applications: app("two")}),
			want: []string{`"shop"`, `"store"`},
		},
		{
			name: "an umbrella child with its umbrella's Kustomization name",
			cluster: cluster(
				&stack.Bundle{Name: "platform", KustomizationName: "platform-cr", Applications: app("one"),
					Children: []*stack.Bundle{{Name: "infra", KustomizationName: "platform-cr", Applications: app("two")}}}),
			want: []string{`"platform"`, `"platform/infra"`, `"platform-cr"`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := layout.IndexOrigins(walk(t, tt.cluster, nodeOnly), tt.cluster)
			if err == nil {
				t.Fatal("IndexOrigins accepted two bundles with one Kustomization name")
			}
			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("IndexOrigins error %q does not contain %s", err, want)
				}
			}
		})
	}

	// Two bundles with one Bundle.Name stay refused, whatever their
	// Kustomization names: a bundle is resolved by its name.
	sameName := cluster(
		&stack.Bundle{Name: "shop", KustomizationName: "shop-a", Applications: app("one")},
		&stack.Bundle{Name: "shop", KustomizationName: "shop-b", Applications: app("two")})
	assertIndexError(t, walk(t, sameName, nodeOnly), sameName, `two bundles are named "shop"`)
}
