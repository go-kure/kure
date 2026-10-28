package fluxcd

import (
	"maps"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"sigs.k8s.io/kustomize/api/krusty"
	kusttypes "sigs.k8s.io/kustomize/api/types"
)

// TestRemoteReference_FieldTablesMatchKustomize pins the one thing
// buildGuard's property rests on, that kustomize reads files through no field
// its tables leave out (go-kure/kure#1168): kustomizationFields names every
// JSON field of kustomize's Kustomization and nothing else,
// builtinConfigFields every builtin plugin kustomize registers and nothing
// else, and the kustomize module the build links is the one whose builtin
// plugin configs the walk was audited against.
func TestRemoteReference_FieldTablesMatchKustomize(t *testing.T) {
	var fields []string
	var walk func(reflect.Type)
	walk = func(typ reflect.Type) {
		for f := range typ.Fields() {
			name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
			if f.Anonymous && name == "" {
				walk(f.Type)
				continue
			}
			fields = append(fields, name)
		}
	}
	walk(reflect.TypeFor[kusttypes.Kustomization]())
	if got, want := slices.Sorted(maps.Keys(kustomizationFields)), slices.Sorted(slices.Values(fields)); !slices.Equal(got, want) {
		t.Errorf("kustomizationFields names\n%v\nwant kustomize's Kustomization fields\n%v", got, want)
	}

	if got, want := slices.Sorted(maps.Keys(builtinConfigFields)), slices.Sorted(slices.Values(krusty.GetBuiltinPluginNames())); !slices.Equal(got, want) {
		t.Errorf("builtinConfigFields names\n%v\nwant kustomize's builtin plugins\n%v", got, want)
	}

	gomod, err := os.ReadFile("../../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	var linked string
	for line := range strings.Lines(string(gomod)) {
		if f := strings.Fields(line); len(f) == 4 && f[0] == "sigs.k8s.io/kustomize/api" && f[1] == "=>" {
			linked = f[3]
		}
	}
	if linked != kustomizeAPIAudited {
		t.Errorf("go.mod replaces sigs.k8s.io/kustomize/api with %q, want %q, the version buildGuard's builtin plugin configs were audited against: audit them again and move kustomizeAPIAudited", linked, kustomizeAPIAudited)
	}
}

// TestRemoteReference_Classify: an entry kustomize accumulates is remote when
// kustomize would clone it or request it, and local paths are not; a
// generator file source is split as kustomize splits it.
func TestRemoteReference_Classify(t *testing.T) {
	for s, want := range map[string]bool{
		"github.com/org/repo/base?ref=v1":    true,
		"git@github.com:org/repo":            true,
		"git::https://example.com/org/repo":  true,
		"ssh://git@example.com/org/repo":     true,
		"file:///srv/repo//base?ref=v1":      true,
		"https://example.com/org/repo//base": true,
		"https://example.com/raw/cm.yaml":    true,
		"http://example.com/cm.yaml":         true,
		"base":                               false,
		"./base":                             false,
		"../x":                               false,
		"apps/web":                           false,
		"a/b/c":                              false,
		"cm.yaml":                            false,
		"/abs/path/cm.yaml":                  false,
		"github.com.yaml":                    false,
	} {
		if got := isRemoteFile(s) || isRepoURL(s); got != want {
			t.Errorf("%q remote: got %v, want %v", s, got, want)
		}
	}
	for s, want := range map[string]bool{"http://example.com/p.yaml": true, "https://example.com/p.yaml": true, "github.com/org/repo": false, "p.yaml": false} {
		if got := isRemoteFile(s); got != want {
			t.Errorf("%q fetched by Load: got %v, want %v", s, got, want)
		}
	}
	got := fileSourcePaths([]string{"f.txt", "k=http://h/f", "http://h/f?a=b", "k=http://h/f?a=b", "=f", "k=", "a=b=c"})
	if want := []string{"f.txt", "http://h/f", "b"}; !slices.Equal(got, want) {
		t.Errorf("fileSourcePaths: got %q, want %q", got, want)
	}
}
