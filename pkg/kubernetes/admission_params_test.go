package kubernetes_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/kubernetes/internal/admission"
)

// TestAdmission_NoOwnParameterTypes walks every exported top-level function
// under pkg/kubernetes (internal packages excluded) and fails on any
// parameter whose type is a struct or interface this tree declares itself.
// The upstream Go struct is the construction API (README §1); a kure-defined
// struct or sum type taken as an argument is a second vocabulary for the same
// object — the Kind(&Config) layer the builder contract's release 2 retired —
// and there is no exclusion list: a scalar enum such as PSALevel is not a spec
// type and is not reported, everything else must take the upstream type.
func TestAdmission_NoOwnParameterTypes(t *testing.T) {
	if testing.Short() {
		t.Skip("loads every package under pkg/kubernetes with type information")
	}
	findings, err := admission.OwnParameterTypes(admission.Options{
		Dir:      ".",
		Patterns: []string{"./..."},
	}, strings.TrimSuffix(modulePath, "/")+"/pkg/kubernetes")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, f := range findings {
		if strings.Contains(f.Package, "/internal/") {
			continue
		}
		n++
		t.Errorf("%s (%s:%d): parameter %s takes %s, a type this tree defines; take the upstream type instead (README §1)",
			strings.TrimPrefix(f.Key(), modulePath), filepath.Base(f.Pos.Filename), f.Pos.Line, f.Param, f.Type)
	}
	t.Logf("exported functions taking a kure-defined struct or interface: %d", n)
}
