package kubernetes_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/kubernetes/internal/admission"
)

const (
	modulePath     = "github.com/go-kure/kure/"
	exclusionsFile = "testdata/admission_exclusions.txt"
	// dumpEnv, when set, rewrites the exclusion list from the current findings
	// instead of checking against it. Use it once to re-bootstrap; never in CI.
	dumpEnv = "KURE_ADMISSION_DUMP"
)

// contractExempt is the fixed set of generic metadata helpers the contract
// admits by name (§5). It is not an exclusion list: it never grows.
var contractExempt = map[string]bool{
	"github.com/go-kure/kure/pkg/kubernetes.SetLabels":      true,
	"github.com/go-kure/kure/pkg/kubernetes.AddLabel":       true,
	"github.com/go-kure/kure/pkg/kubernetes.SetAnnotations": true,
	"github.com/go-kure/kure/pkg/kubernetes.AddAnnotation":  true,
}

// TestAdmission_SugarHelpersAreClassAdmissible classifies every exported
// Set*/Add* helper under pkg/kubernetes (internal packages excluded) and fails
// on any inadmissible helper that is not listed in the exclusion file, and on
// any listed helper that is no longer inadmissible (a stale entry). The
// exclusion file is temporary: the prune work item empties it, and once it is
// empty this test is the whole enforcement of ADR-038 §4.
func TestAdmission_SugarHelpersAreClassAdmissible(t *testing.T) {
	if testing.Short() {
		t.Skip("loads every package under pkg/kubernetes with type information")
	}
	findings, err := admission.Classify(admission.Options{
		Dir:      ".",
		Patterns: []string{"./..."},
		Exempt:   contractExempt,
	})
	if err != nil {
		t.Fatal(err)
	}

	counts := map[admission.Class]int{}
	inadmissible := map[string]admission.Finding{}
	for _, f := range findings {
		if strings.Contains(f.Package, "/internal/") {
			continue
		}
		counts[f.Class]++
		if f.Class == admission.Inadmissible {
			inadmissible[strings.TrimPrefix(f.Key(), modulePath)] = f
		}
	}
	t.Logf("helpers: append=%d pointer=%d composite=%d exempt=%d inadmissible=%d",
		counts[admission.Append], counts[admission.Pointer], counts[admission.Composite], counts[admission.Exempt], counts[admission.Inadmissible])
	if counts[admission.Exempt] != len(contractExempt) {
		t.Errorf("expected the %d contract-exempt helpers to exist, classified %d", len(contractExempt), counts[admission.Exempt])
	}

	if os.Getenv(dumpEnv) != "" {
		dumpExclusions(t, inadmissible)
		return
	}

	excluded, err := admission.ReadExclusions(exclusionsFile)
	if err != nil {
		t.Fatal(err)
	}
	excludedSet := map[string]bool{}
	for _, key := range excluded {
		excludedSet[key] = true
		if _, still := inadmissible[key]; !still {
			t.Errorf("%s: listed in %s but no longer inadmissible (or gone); remove the entry", key, exclusionsFile)
		}
	}
	for key, f := range inadmissible {
		if !excludedSet[key] {
			t.Errorf("%s (%s:%d): %s; make it class-admissible or delete it", key, filepath.Base(f.Pos.Filename), f.Pos.Line, f.Reason)
		}
	}
}

func dumpExclusions(t *testing.T, inadmissible map[string]admission.Finding) {
	t.Helper()
	keys := make([]string, 0, len(inadmissible))
	for k := range inadmissible {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("# Inadmissible Set*/Add* helpers tolerated by TestAdmission_SugarHelpersAreClassAdmissible.\n")
	b.WriteString("# The prune work item of the builder-contract epic emptied it; it stays empty.\n")
	b.WriteString("# Entries only ever leave; a new helper must be class-admissible (see pkg/kubernetes/README.md).\n")
	b.WriteString("# Regenerate with: KURE_ADMISSION_DUMP=1 go test ./pkg/kubernetes -run TestAdmission\n\n")
	for _, k := range keys {
		b.WriteString(k + "\n")
	}
	if err := os.WriteFile(exclusionsFile, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %d exclusions to %s", len(keys), exclusionsFile)
}
