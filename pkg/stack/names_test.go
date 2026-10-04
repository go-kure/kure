package stack

import (
	"strings"
	"testing"
)

func TestValidateKustomizationName(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr string // substring of the error; "" means valid
	}{
		{"simple", "web", ""},
		{"dotted subdomain", "my.app", ""},
		{"digits and dashes", "0-web-1", ""},
		{"at the limit", strings.Repeat("a", KustomizationNameMaxLength), ""},
		{"over the limit", strings.Repeat("a", KustomizationNameMaxLength+1), "at most 63 characters"},
		{"uppercase", "Y-infra", "RFC 1123 subdomain"},
		{"underscore", "my_app", "RFC 1123 subdomain"},
		{"slash", "a/b", "RFC 1123 subdomain"},
		{"leading dash", "-web", "RFC 1123 subdomain"},
		{"trailing dot", "web.", "RFC 1123 subdomain"},
		{"empty", "", "RFC 1123 subdomain"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKustomizationName(tt.in)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateKustomizationName(%q) = %v, want nil", tt.in, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateKustomizationName(%q) = %v, want an error containing %q", tt.in, err, tt.wantErr)
			}
		})
	}
}

func TestValidateDirectoryName(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantErr string
	}{
		{"simple", "web", ""},
		{"dotted", "my.app", ""},
		{"mixed case and underscore", "Group_A", ""},
		{"leading dot", ".hidden", ""},
		{"empty", "", "empty"},
		{"current directory", ".", `"."`},
		{"parent directory", "..", `".."`},
		{"slash", "a/b", "path separator"},
		{"absolute", "/abs", "path separator"},
		{"backslash", `a\b`, "path separator"},
		// On Windows filepath.Join reads the backslash as a separator, so
		// this name would leave the directory it is joined under.
		{"backslash traversal", `..\outside`, "path separator"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDirectoryName(tt.in)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateDirectoryName(%q) = %v, want nil", tt.in, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateDirectoryName(%q) = %v, want an error containing %q", tt.in, err, tt.wantErr)
			}
		})
	}
}

// TestBundleValidate_Names: a bundle's name becomes a Flux Kustomization name
// and a directory name, so Validate refuses one that is not valid as both,
// and names the bundle by its path from the bundle Validate was called on.
func TestBundleValidate_Names(t *testing.T) {
	long := strings.Repeat("a", KustomizationNameMaxLength+1)
	tests := []struct {
		name    string
		bundle  *Bundle
		wantErr []string // every substring must be in the error; nil means valid
	}{
		{"dotted subdomain", &Bundle{Name: "my.app"}, nil},
		{"at the limit", &Bundle{Name: strings.Repeat("a", KustomizationNameMaxLength)}, nil},
		{"over the limit", &Bundle{Name: long}, []string{long, "at most 63 characters"}},
		{"uppercase", &Bundle{Name: "Y-infra"}, []string{`'Y-infra'`, "RFC 1123 subdomain"}},
		{"slash", &Bundle{Name: "apps/web"}, []string{`'apps/web'`, "path separator"}},
		{"backslash", &Bundle{Name: `..\outside`}, []string{"path separator"}},
		{
			"umbrella child",
			&Bundle{Name: "platform", Children: []*Bundle{{Name: "infra"}, {Name: "Services"}}},
			[]string{`'platform/Services'`, "RFC 1123 subdomain"},
		},
		{
			"umbrella grandchild over the limit",
			&Bundle{Name: "platform", Children: []*Bundle{{Name: "infra", Children: []*Bundle{{Name: long}}}}},
			[]string{"'platform/infra/" + long + "'", "at most 63 characters"},
		},
		{
			// The older check on Children refuses this one first: it names
			// the parent and the child's index, not the path.
			"umbrella grandchild without a name",
			&Bundle{Name: "platform", Children: []*Bundle{{Name: "infra", Children: []*Bundle{{Name: ""}}}}},
			[]string{"'infra'", "child at index 0 has empty name"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.bundle.Validate()
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Validate() = nil, want an error")
			}
			for _, want := range tt.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("Validate() = %v, want it to contain %q", err, want)
				}
			}
		})
	}
}

// TestValidateCluster_Names: node names become directories, so ValidateCluster
// refuses one that is not a single path segment, with the node's path; a
// refused bundle name is reported with the path of the node that carries it.
func TestValidateCluster_Names(t *testing.T) {
	long := strings.Repeat("a", KustomizationNameMaxLength+1)
	tests := []struct {
		name    string
		root    *Node
		wantErr []string
	}{
		{
			"unnamed root",
			&Node{Name: "", Children: []*Node{{Name: "apps", Bundle: &Bundle{Name: "web"}}}},
			nil,
		},
		{
			"unnamed root with a bundle",
			&Node{Name: "", Bundle: &Bundle{Name: "web"}},
			nil,
		},
		{
			"mixed-case node name",
			&Node{Name: "platform", Children: []*Node{{Name: "groupA", Bundle: &Bundle{Name: "web"}}}},
			nil,
		},
		{
			"unnamed child node",
			&Node{Name: "platform", Children: []*Node{{Name: "", Bundle: &Bundle{Name: "web"}}}},
			[]string{`node "platform/"`, "empty"},
		},
		{
			"node name with a slash",
			&Node{Name: "platform", Children: []*Node{{Name: "apps/web"}}},
			[]string{`node "platform/apps/web"`, "path separator"},
		},
		{
			"node name with a backslash",
			&Node{Name: "platform", Children: []*Node{{Name: `..\outside`}}},
			[]string{"path separator"},
		},
		{
			"root named dot-dot",
			&Node{Name: ".."},
			[]string{`node ".."`, `".."`},
		},
		{
			"bundle name over the limit",
			&Node{Name: "platform", Children: []*Node{{Name: "apps", Bundle: &Bundle{Name: long}}}},
			[]string{`node "platform/apps"`, long, "at most 63 characters"},
		},
		{
			"uppercase umbrella child",
			&Node{Name: "platform", Bundle: &Bundle{Name: "y", Children: []*Bundle{{Name: "Y-infra"}}}},
			[]string{`node "platform"`, `'y/Y-infra'`, "RFC 1123 subdomain"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCluster(&Cluster{Name: "c", Node: tt.root})
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("ValidateCluster() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("ValidateCluster() = nil, want an error")
			}
			for _, want := range tt.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("ValidateCluster() = %v, want it to contain %q", err, want)
				}
			}
		})
	}
}
