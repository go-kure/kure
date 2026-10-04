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
		// A directory with a NUL byte in its name cannot be created: the
		// write would fail after validation had passed.
		{"NUL byte", "a\x00b", "NUL byte"},
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

// TestBundleValidate_Names: a bundle's name becomes the name of the object a
// delivery engine applies it with and a directory name, so Validate refuses
// one that is not a DNS-1123 subdomain and a single path segment, and names
// the bundle by its path from the bundle Validate was called on. The
// 63-character limit of a Flux Kustomization name is not checked here: it is
// the Flux workflow's, and a longer name is valid for another engine.
func TestBundleValidate_Names(t *testing.T) {
	overFlux := strings.Repeat("a", KustomizationNameMaxLength+1)
	longest := strings.Repeat("a", 253)
	long := strings.Repeat("a", 254)
	tests := []struct {
		name    string
		bundle  *Bundle
		wantErr []string // every substring must be in the error; nil means valid
	}{
		{"dotted subdomain", &Bundle{Name: "my.app"}, nil},
		{"at the Flux limit", &Bundle{Name: strings.Repeat("a", KustomizationNameMaxLength)}, nil},
		{"over the Flux limit", &Bundle{Name: overFlux}, nil},
		{"at the subdomain limit", &Bundle{Name: longest}, nil},
		{"over the subdomain limit", &Bundle{Name: long}, []string{long, "no more than 253 characters"}},
		{
			"umbrella child over the Flux limit",
			&Bundle{Name: "platform", Children: []*Bundle{{Name: overFlux}}},
			nil,
		},
		{"uppercase", &Bundle{Name: "Y-infra"}, []string{`'Y-infra'`, "RFC 1123 subdomain"}},
		{"slash", &Bundle{Name: "apps/web"}, []string{`'apps/web'`, "path separator"}},
		{"backslash", &Bundle{Name: `..\outside`}, []string{"path separator"}},
		{
			"umbrella child",
			&Bundle{Name: "platform", Children: []*Bundle{{Name: "infra"}, {Name: "Services"}}},
			[]string{`'platform/Services'`, "RFC 1123 subdomain"},
		},
		{
			"umbrella grandchild over the subdomain limit",
			&Bundle{Name: "platform", Children: []*Bundle{{Name: "infra", Children: []*Bundle{{Name: long}}}}},
			[]string{"'platform/infra/" + long + "'", "no more than 253 characters"},
		},
		{
			// The older check on Children refuses this one first: it names
			// the parent and the child's index, not the path.
			"umbrella grandchild without a name",
			&Bundle{Name: "platform", Children: []*Bundle{{Name: "infra", Children: []*Bundle{{Name: ""}}}}},
			[]string{"'infra'", "child at index 0 has empty name"},
		},
		// KustomizationName is the name of the same object when it is set, so
		// it meets the same rule; the bundle is still named by its path, and
		// the field says which of its two names is refused.
		{"KustomizationName dotted subdomain", &Bundle{Name: "web", KustomizationName: "my.app"}, nil},
		{"KustomizationName over the Flux limit", &Bundle{Name: "web", KustomizationName: overFlux}, nil},
		{"KustomizationName at the subdomain limit", &Bundle{Name: "web", KustomizationName: longest}, nil},
		{
			"KustomizationName over the subdomain limit",
			&Bundle{Name: "web", KustomizationName: long},
			[]string{"'web'", "field 'kustomizationName'", long, "no more than 253 characters"},
		},
		{
			"KustomizationName uppercase",
			&Bundle{Name: "web", KustomizationName: "Web-CR"},
			[]string{"'web'", "field 'kustomizationName'", `"Web-CR"`, "RFC 1123 subdomain"},
		},
		{
			"KustomizationName with a slash",
			&Bundle{Name: "web", KustomizationName: "apps/web"},
			[]string{"'web'", "field 'kustomizationName'", `"apps/web"`, "RFC 1123 subdomain"},
		},
		{
			"umbrella child KustomizationName",
			&Bundle{Name: "platform", Children: []*Bundle{{Name: "infra", KustomizationName: "Infra"}}},
			[]string{"'platform/infra'", "field 'kustomizationName'", `"Infra"`, "RFC 1123 subdomain"},
		},
		{
			// With a valid KustomizationName the Name is still the bundle's
			// identity and its directory, and is checked as before.
			"uppercase Name next to a valid KustomizationName",
			&Bundle{Name: "Y-infra", KustomizationName: "y-infra"},
			[]string{`'Y-infra'`, "field 'name'", "RFC 1123 subdomain"},
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
	overFlux := strings.Repeat("a", KustomizationNameMaxLength+1)
	long := strings.Repeat("a", 254)
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
			// Flux's limit, checked by the Flux workflow and not here.
			"bundle name over the Flux limit",
			&Node{Name: "platform", Children: []*Node{{Name: "apps", Bundle: &Bundle{Name: overFlux}}}},
			nil,
		},
		{
			"bundle name over the subdomain limit",
			&Node{Name: "platform", Children: []*Node{{Name: "apps", Bundle: &Bundle{Name: long}}}},
			[]string{`node "platform/apps"`, long, "no more than 253 characters"},
		},
		{
			"uppercase umbrella child",
			&Node{Name: "platform", Bundle: &Bundle{Name: "y", Children: []*Bundle{{Name: "Y-infra"}}}},
			[]string{`node "platform"`, `'y/Y-infra'`, "RFC 1123 subdomain"},
		},
		{
			// Flux's limit applies to the name in effect, and is not checked here.
			"KustomizationName over the Flux limit",
			&Node{Name: "platform", Children: []*Node{{Name: "apps", Bundle: &Bundle{Name: "web", KustomizationName: overFlux}}}},
			nil,
		},
		{
			"uppercase KustomizationName",
			&Node{Name: "platform", Children: []*Node{{Name: "apps", Bundle: &Bundle{Name: "web", KustomizationName: "Web-CR"}}}},
			[]string{`node "platform/apps"`, "'web'", "field 'kustomizationName'", `"Web-CR"`, "RFC 1123 subdomain"},
		},
		{
			"uppercase KustomizationName of an umbrella child",
			&Node{Name: "platform", Bundle: &Bundle{Name: "y", Children: []*Bundle{{Name: "infra", KustomizationName: "Y-infra"}}}},
			[]string{`node "platform"`, `'y/infra'`, "field 'kustomizationName'", "RFC 1123 subdomain"},
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

// TestValidateCluster_NodeNameIsPathSegment pins why a node name is checked
// whatever the layout rules are, flat node grouping included: the model itself
// joins node names with "/" into the node's path and the path-map key
// (Node.GetPath, Node.InitializePathMap). A name holding "/" gives two
// different nodes one key, and so does an unnamed child, long before any
// directory is made.
func TestValidateCluster_NodeNameIsPathSegment(t *testing.T) {
	tree := func(children ...*Node) *Node {
		root := &Node{Name: "p", Children: children}
		var link func(parent *Node)
		link = func(parent *Node) {
			for _, c := range parent.Children {
				c.SetParent(parent)
				link(c)
			}
		}
		link(root)
		return root
	}
	count := func(n *Node) int {
		total := 0
		var walk func(*Node)
		walk = func(n *Node) {
			total++
			for _, c := range n.Children {
				walk(c)
			}
		}
		walk(n)
		return total
	}

	t.Run("separator in a name", func(t *testing.T) {
		slashed := &Node{Name: "a/b"}
		nested := &Node{Name: "b"}
		root := tree(slashed, &Node{Name: "a", Children: []*Node{nested}})

		if slashed.GetPath() != nested.GetPath() {
			t.Fatalf("paths %q and %q: want the same path for two nodes", slashed.GetPath(), nested.GetPath())
		}
		root.InitializePathMap()
		if got, nodes := len(root.pathMap), count(root); got >= nodes {
			t.Fatalf("path map holds %d keys for %d nodes: want fewer keys than nodes", got, nodes)
		}
		err := ValidateCluster(&Cluster{Name: "c", Node: root})
		if err == nil || !strings.Contains(err.Error(), `node "p/a/b"`) || !strings.Contains(err.Error(), "path separator") {
			t.Fatalf("ValidateCluster() = %v, want the node refused for its separator", err)
		}
	})

	t.Run("unnamed children", func(t *testing.T) {
		first, second := &Node{}, &Node{}
		root := tree(first, second)

		if first.GetPath() != "p/" || second.GetPath() != "p/" {
			t.Fatalf("paths %q and %q: want both unnamed children at %q", first.GetPath(), second.GetPath(), "p/")
		}
		root.InitializePathMap()
		if got, nodes := len(root.pathMap), count(root); got >= nodes {
			t.Fatalf("path map holds %d keys for %d nodes: want fewer keys than nodes", got, nodes)
		}
		err := ValidateCluster(&Cluster{Name: "c", Node: root})
		if err == nil || !strings.Contains(err.Error(), `node "p/"`) || !strings.Contains(err.Error(), "empty") {
			t.Fatalf("ValidateCluster() = %v, want the unnamed child refused", err)
		}
	})
}
