package stack

import (
	"strings"
	"testing"
)

// TestBundle_UnitName: the name of the object a bundle is applied with is
// KustomizationName when it is set and the bundle's own name otherwise.
func TestBundle_UnitName(t *testing.T) {
	if got := (&Bundle{Name: "shop"}).UnitName(); got != "shop" {
		t.Errorf("UnitName() without KustomizationName = %q, want the bundle name", got)
	}
	if got := (&Bundle{Name: "shop", KustomizationName: "apps-shop"}).UnitName(); got != "apps-shop" {
		t.Errorf("UnitName() = %q, want KustomizationName", got)
	}
}

// TestBundleValidate_KustomizationNameReferences: wherever Validate compares a
// bundle with a Kustomization reference (a NamedDependsOn entry, or another
// bundle standing for its Kustomization) it compares the names the
// Kustomizations get, so a KustomizationName changes which pairs are one
// object. The checks on a bundle's own identity stay on Bundle.Name.
func TestBundleValidate_KustomizationNameReferences(t *testing.T) {
	db := func() *Bundle { return &Bundle{Name: "db", KustomizationName: "db-cr"} }
	tests := []struct {
		name    string
		bundle  *Bundle
		wantErr string
	}{
		{
			name:   "dependency and a named dependency on its bundle name are two Kustomizations",
			bundle: &Bundle{Name: "web", DependsOn: []*Bundle{db()}, NamedDependsOn: []string{"db"}},
		},
		{
			name:    "dependency and a named dependency on its Kustomization name are one",
			bundle:  &Bundle{Name: "web", DependsOn: []*Bundle{db()}, NamedDependsOn: []string{"db-cr"}},
			wantErr: `dependency "db-cr" appears in both DependsOn and NamedDependsOn`,
		},
		{
			name:   "child and a named dependency on its bundle name are two Kustomizations",
			bundle: &Bundle{Name: "platform", Children: []*Bundle{db()}, NamedDependsOn: []string{"db"}},
		},
		{
			name:    "child and a named dependency on its Kustomization name are one",
			bundle:  &Bundle{Name: "platform", Children: []*Bundle{db()}, NamedDependsOn: []string{"db-cr"}},
			wantErr: `child "db" also appears in namedDependsOn`,
		},
		{
			name: "child naming its parent's bundle name depends on another Kustomization",
			bundle: &Bundle{Name: "platform", KustomizationName: "platform-cr",
				Children: []*Bundle{{Name: "infra", NamedDependsOn: []string{"platform"}}}},
		},
		{
			name: "child naming its parent's Kustomization name depends on its parent",
			bundle: &Bundle{Name: "platform", KustomizationName: "platform-cr",
				Children: []*Bundle{{Name: "infra", NamedDependsOn: []string{"platform-cr"}}}},
			wantErr: `child "infra" has parent "platform" in namedDependsOn`,
		},
		{
			name: "child and dependency with one bundle name and two Kustomization names",
			bundle: &Bundle{Name: "platform",
				DependsOn: []*Bundle{{Name: "db", KustomizationName: "db-external"}},
				Children:  []*Bundle{db()}},
		},
		{
			name: "child whose Kustomization name is a dependency's",
			bundle: &Bundle{Name: "platform",
				DependsOn: []*Bundle{{Name: "database", KustomizationName: "db-cr"}},
				Children:  []*Bundle{db()}},
			wantErr: `child "db" also appears in dependsOn`,
		},
		{
			name: "two children with one bundle name stay refused",
			bundle: &Bundle{Name: "platform", Children: []*Bundle{
				{Name: "db", KustomizationName: "db-a"}, {Name: "db", KustomizationName: "db-b"}}},
			wantErr: `duplicate child name "db"`,
		},
		{
			name: "child with its parent's bundle name stays refused",
			bundle: &Bundle{Name: "platform", KustomizationName: "platform-cr",
				Children: []*Bundle{{Name: "platform", KustomizationName: "platform-child"}}},
			wantErr: `child name "platform" equals parent name`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.bundle.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() = %v, want an error containing %q", err, tt.wantErr)
			}
		})
	}
}

// TestDeepCopyBundle_KustomizationName: the fluent builder copies bundles, and
// a copy must name the same Kustomization as the bundle it was copied from.
func TestDeepCopyBundle_KustomizationName(t *testing.T) {
	got := deepCopyBundle(&Bundle{Name: "shop", KustomizationName: "apps-shop"})
	if got.KustomizationName != "apps-shop" || got.UnitName() != "apps-shop" {
		t.Errorf("copy has KustomizationName %q, UnitName() %q; want apps-shop", got.KustomizationName, got.UnitName())
	}
}
