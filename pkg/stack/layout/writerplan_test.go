package layout

import "testing"

// TestRelPath: relPath matches path segments exactly, so a segment that
// differs only in case is not shared, and a path rooted on one side only is
// never at or below the other.
func TestRelPath(t *testing.T) {
	cases := []struct {
		base, target string
		rel          string
		inside       bool
	}{
		{"p", "p", ".", true},
		{"p", "p/sub/svc.yaml", "sub/svc.yaml", true},
		{"p", "p/Sub/svc.yaml", "Sub/svc.yaml", true},
		{"p", "./p/sub/", "sub", true},
		{".", "x/svc.yaml", "x/svc.yaml", true},
		{"/out/p", "/out/p/sub", "sub", true},
		{"p", "q/svc.yaml", "../q/svc.yaml", false},
		{"p", "p2/svc.yaml", "../p2/svc.yaml", false},
		{"p", "svc.yaml", "../svc.yaml", false},
		{"p", "P/Sub/svc.yaml", "../P/Sub/svc.yaml", false},
		// A base with no segments still climbs out of a target above it.
		{".", "../svc.yaml", "../svc.yaml", false},
		{".", "../q/svc.yaml", "../q/svc.yaml", false},
		{"", "..", "..", false},
		{"../a", "../a/svc.yaml", "svc.yaml", true},
		{"../a", "../svc.yaml", "../svc.yaml", false},
		// Rooted on one side only.
		{".", "/x/svc.yaml", "/x/svc.yaml", false},
		{"/out/p", "p/svc.yaml", "p/svc.yaml", false},
	}
	for _, tc := range cases {
		rel, inside := relPath(tc.base, tc.target)
		if rel != tc.rel || inside != tc.inside {
			t.Errorf("relPath(%q, %q) = %q, %v; want %q, %v", tc.base, tc.target, rel, inside, tc.rel, tc.inside)
		}
	}
}
