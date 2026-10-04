package stack

import (
	"strings"
	"testing"
)

// Tests for a DependsOn bundle that is a copy of one of the cluster's bundles:
// the layout resolves it by its Name to that bundle, so a copy that names
// another Kustomization than the bundle contradicts it.

// dependencyCluster is r -> {dbn, webn}: dbn holds db, and webn holds the
// bundle web, which depends on dep and names the Kustomizations in named.
func dependencyCluster(db, dep *Bundle, named ...string) *Cluster {
	web := &Bundle{Name: "web", DependsOn: []*Bundle{dep}, NamedDependsOn: named}
	dbn := &Node{Name: "dbn", Bundle: db}
	webn := &Node{Name: "webn", Bundle: web}
	r := &Node{Name: "r", Children: []*Node{dbn, webn}}
	dbn.SetParent(r)
	webn.SetParent(r)
	return &Cluster{Name: "demo", Node: r}
}

// TestValidateCluster_DependencyCopy: a DependsOn bundle that is not one of
// the cluster's bundles but has the Name of one is a copy of it and stands for
// it. A copy that leaves KustomizationName empty is that bundle. One that sets
// another KustomizationName than the bundle's is refused, naming the
// dependant, the bundle and both names. One that stands for a bundle whose
// Kustomization name is also in NamedDependsOn is one dependency in both
// lists. Both are reported before the bundles are validated one by one:
// Bundle.Validate reads the copy's own name, so its comparison with
// NamedDependsOn would name the wrong cause, or miss the pair.
func TestValidateCluster_DependencyCopy(t *testing.T) {
	const inBoth = "appears in both DependsOn and NamedDependsOn"
	tests := []struct {
		name    string
		cluster func() *Cluster
		want    []string // substrings of the error; nil means valid
	}{
		{
			name: "the bundle itself",
			cluster: func() *Cluster {
				db := &Bundle{Name: "db", KustomizationName: "db-cr"}
				return dependencyCluster(db, db)
			},
		},
		{
			name: "a copy with the bundle's KustomizationName",
			cluster: func() *Cluster {
				return dependencyCluster(&Bundle{Name: "db", KustomizationName: "db-cr"}, &Bundle{Name: "db", KustomizationName: "db-cr"})
			},
		},
		{
			name: "a copy of a bundle without KustomizationName",
			cluster: func() *Cluster {
				return dependencyCluster(&Bundle{Name: "db"}, &Bundle{Name: "db"})
			},
		},
		{
			name: "a bundle outside the cluster",
			cluster: func() *Cluster {
				return dependencyCluster(&Bundle{Name: "db", KustomizationName: "db-cr"}, &Bundle{Name: "cache", KustomizationName: "x"})
			},
		},
		{
			name: "a name-only copy of a bundle that has a KustomizationName",
			cluster: func() *Cluster {
				return dependencyCluster(&Bundle{Name: "db", KustomizationName: "db-cr"}, &Bundle{Name: "db"})
			},
		},
		{
			name: "a name-only copy beside a named dependency on another Kustomization",
			cluster: func() *Cluster {
				return dependencyCluster(&Bundle{Name: "db", KustomizationName: "db-cr"}, &Bundle{Name: "db"}, "external")
			},
		},
		{
			name: "a copy with another KustomizationName",
			cluster: func() *Cluster {
				return dependencyCluster(&Bundle{Name: "db", KustomizationName: "db-cr"}, &Bundle{Name: "db", KustomizationName: "x"})
			},
			want: []string{`bundle "web"`, `copy of bundle "db"`, `KustomizationName "x"`, `KustomizationName "db-cr"`},
		},
		{
			name: "a copy with a KustomizationName of a bundle that has none",
			cluster: func() *Cluster {
				return dependencyCluster(&Bundle{Name: "db"}, &Bundle{Name: "db", KustomizationName: "x"})
			},
			want: []string{`bundle "web"`, `copy of bundle "db"`, `KustomizationName "x"`, `KustomizationName ""`},
		},
		{
			// Bundle.Validate alone reports "x" in both lists: the copy's name,
			// which no Kustomization of this cluster gets.
			name: "the copy's KustomizationName is also a named dependency",
			cluster: func() *Cluster {
				return dependencyCluster(&Bundle{Name: "db", KustomizationName: "db-cr"}, &Bundle{Name: "db", KustomizationName: "x"}, "x")
			},
			want: []string{`bundle "web"`, `copy of bundle "db"`, `KustomizationName "x"`, `KustomizationName "db-cr"`},
		},
		{
			// Bundle.Validate alone accepts this pair, though both entries are
			// the Kustomization db-cr.
			name: "a name-only copy, the bundle's KustomizationName is also a named dependency",
			cluster: func() *Cluster {
				return dependencyCluster(&Bundle{Name: "db", KustomizationName: "db-cr"}, &Bundle{Name: "db"}, "db-cr")
			},
			want: []string{`bundle "web"`, `dependency "db-cr" ` + inBoth, `DependsOn bundle "db"`},
		},
		{
			name: "a copy with the bundle's KustomizationName, which is also a named dependency",
			cluster: func() *Cluster {
				return dependencyCluster(&Bundle{Name: "db", KustomizationName: "db-cr"}, &Bundle{Name: "db", KustomizationName: "db-cr"}, "db-cr")
			},
			want: []string{`dependency "db-cr" ` + inBoth},
		},
		{
			name: "a copy of a bundle without KustomizationName, whose name is also a named dependency",
			cluster: func() *Cluster {
				return dependencyCluster(&Bundle{Name: "db"}, &Bundle{Name: "db"}, "db")
			},
			want: []string{`dependency "db" ` + inBoth},
		},
		{
			name: "a copy of an umbrella child",
			cluster: func() *Cluster {
				db := &Bundle{Name: "db", KustomizationName: "db-cr"}
				platform := &Bundle{Name: "platform", Children: []*Bundle{db}}
				return dependencyCluster(platform, &Bundle{Name: "db", KustomizationName: "x"})
			},
			want: []string{`bundle "web"`, `copy of bundle "db"`, `KustomizationName "x"`, `KustomizationName "db-cr"`},
		},
		{
			name: "an umbrella child depends on the copy",
			cluster: func() *Cluster {
				api := &Bundle{Name: "api", DependsOn: []*Bundle{{Name: "db", KustomizationName: "x"}}}
				shop := &Bundle{Name: "shop", Children: []*Bundle{api}}
				c := dependencyCluster(&Bundle{Name: "db", KustomizationName: "db-cr"}, &Bundle{Name: "cache"})
				shopn := &Node{Name: "shopn", Bundle: shop}
				shopn.SetParent(c.Node)
				c.Node.Children = append(c.Node.Children, shopn)
				return c
			},
			want: []string{`bundle "api"`, `copy of bundle "db"`, `KustomizationName "x"`, `KustomizationName "db-cr"`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCluster(tt.cluster())
			if tt.want == nil {
				if err != nil {
					t.Fatalf("ValidateCluster: got %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("ValidateCluster accepted a dependency copy that names another Kustomization than the bundle")
			}
			wantsBoth := false
			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %s", err, want)
				}
				wantsBoth = wantsBoth || strings.Contains(want, inBoth)
			}
			if !wantsBoth && strings.Contains(err.Error(), inBoth) {
				t.Errorf("error %q reports the lists, not the contradictory copy", err)
			}
		})
	}
}

// TestValidateCluster_DependencyCopyLeavesOtherRefusalsAlone: the copy check
// runs before the bundles are validated, so it must not get in the way of what
// they refuse. An umbrella cycle is still reported as one (the check walks the
// umbrella children itself, and must end), and where two of the cluster's
// bundles share a Name a copy of that name is nobody's copy yet: that pair is
// the layout's to refuse.
func TestValidateCluster_DependencyCopyLeavesOtherRefusalsAlone(t *testing.T) {
	t.Run("umbrella cycle", func(t *testing.T) {
		a := &Bundle{Name: "a"}
		b := &Bundle{Name: "b", Children: []*Bundle{a}}
		a.Children = []*Bundle{b}
		err := ValidateCluster(&Cluster{Name: "demo", Node: &Node{Name: "r", Bundle: a}})
		if err == nil || !strings.Contains(err.Error(), "umbrella cycle") {
			t.Fatalf("got %v, want the umbrella cycle", err)
		}
	})
	t.Run("two bundles with the copy's name", func(t *testing.T) {
		c := dependencyCluster(&Bundle{Name: "db", KustomizationName: "db-a"}, &Bundle{Name: "db", KustomizationName: "x"})
		other := &Node{Name: "othern", Bundle: &Bundle{Name: "db", KustomizationName: "db-b"}}
		other.SetParent(c.Node)
		c.Node.Children = append(c.Node.Children, other)
		if err := ValidateCluster(c); err != nil && strings.Contains(err.Error(), "copy of bundle") {
			t.Fatalf("got %v, want no copy refusal while the name is not one bundle's", err)
		}
	})
	t.Run("nil dependency", func(t *testing.T) {
		if err := ValidateCluster(dependencyCluster(&Bundle{Name: "db"}, nil)); err != nil {
			t.Fatalf("got %v, want nil as before", err)
		}
	})
}

// TestValidateCluster_NameOnlyCopyBesideItsOwnName pins a known cost: a
// name-only copy of a bundle that has a KustomizationName, beside a
// NamedDependsOn entry equal to the copy's Name. The two are different
// Kustomizations (db-cr, and whichever is called db), yet Bundle.Validate reads
// the copy's own name and refuses the pair. Setting the bundle's
// KustomizationName on the copy says which one is meant and is accepted.
func TestValidateCluster_NameOnlyCopyBesideItsOwnName(t *testing.T) {
	err := ValidateCluster(dependencyCluster(&Bundle{Name: "db", KustomizationName: "db-cr"}, &Bundle{Name: "db"}, "db"))
	if err == nil || !strings.Contains(err.Error(), `dependency "db" appears in both DependsOn and NamedDependsOn`) {
		t.Fatalf("got %v, want the cross-list refusal of Bundle.Validate", err)
	}
	err = ValidateCluster(dependencyCluster(&Bundle{Name: "db", KustomizationName: "db-cr"}, &Bundle{Name: "db", KustomizationName: "db-cr"}, "db"))
	if err != nil {
		t.Fatalf("with the bundle's KustomizationName on the copy: got %v, want nil", err)
	}
}

// TestBundleValidate_KeepsItsOwnComparison: Bundle.Validate sees one bundle
// and no cluster, so a DependsOn bundle is what it says it is there: its own
// KustomizationName is the name compared with NamedDependsOn.
func TestBundleValidate_KeepsItsOwnComparison(t *testing.T) {
	web := &Bundle{Name: "web", DependsOn: []*Bundle{{Name: "db", KustomizationName: "x"}}, NamedDependsOn: []string{"x"}}
	err := web.Validate()
	if err == nil || !strings.Contains(err.Error(), `dependency "x" appears in both DependsOn and NamedDependsOn`) {
		t.Fatalf("got %v, want the cross-list refusal", err)
	}
}
