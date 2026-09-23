package admission

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fixture module has two packages: "up" stands in for an upstream API
// module (the object and its spec), "own" for a kure package that builds it.
// Only "own" is under the checked prefix, so the object type is never a
// finding, exactly as *appsv1.Deployment is not one in the real tree.
const paramsUpstreamSource = `package up

type Spec struct{ Name string }

type Obj struct{ Spec Spec }
`

const paramsOwnSource = `package own

import "fixture/up"

type Config struct{ Name string }

type Variant interface{ isVariant() }

type Level string

type Alias = Config

type Wrapped up.Spec

type ConfigPtr *Config

type ConfigList []Config

type ConfigArray [2]Config

type ConfigByName map[string]Config

type Names []string

type Tree []Tree

type LitConfig = struct{ Name string }

type LitVariant = interface{ isLit() }

type UpAlias = up.Spec

type PtrLit = *struct{ Name string }

type SliceLit = []struct{ Name string }

type ArrLit = [1]struct{ Name string }

type MapLit = map[string]interface{ isM() }

type NamedLitList []struct{ Name string }

func Build(cfg *Config) *up.Obj                       { return &up.Obj{Spec: up.Spec{Name: cfg.Name}} }
func Apply(o *up.Obj, v Variant)                      {}
func Many(cfgs []Config, byName map[string]*Config)   {}
func Aliased(a Alias)                                 {}
func Defined(w *Wrapped)                              {}
func NamedPtr(p ConfigPtr)                            {}
func NamedList(l ConfigList)                          {}
func NamedArray(a ConfigArray)                        {}
func NamedMap(m ConfigByName)                         {}
func NamedScalars(n Names, t Tree)                    {}
func LitStruct(c *LitConfig)                          {}
func LitIface(v []LitVariant)                         {}
func UpstreamAlias(u *UpAlias)                        {}
func AliasPtr(p PtrLit)                               {}
func AliasSlice(s SliceLit)                           {}
func AliasArray(a ArrLit)                             {}
func AliasMap(m MapLit)                               {}
func NamedLit(l NamedLitList)                         {}
func Anon(c struct{ Name string })                    {}
func Callback(f func(*Config))                        {}
func AnyParams(v any, m map[string]interface{})       {}
func UpCallback(f func(*up.Spec) error)               {}
func Label(o *up.Obj, l Level)                        {}
func Plain(o *up.Obj, s *up.Spec, n int)              {}
func Generic[T any](o *up.Obj, v T)                   {}
func unexported(cfg *Config)                          {}
func (o *Wrapped) Method(cfg *Config)                 {}
`

func writeParamsFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":     "module fixture\n\ngo 1.26\n",
		"up/up.go":   paramsUpstreamSource,
		"own/own.go": paramsOwnSource,
	}
	for name, src := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestOwnParameterTypes_Fixture(t *testing.T) {
	dir := writeParamsFixture(t)
	findings, err := OwnParameterTypes(Options{
		Dir:      dir,
		Patterns: []string{"./..."},
		Env:      append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod"),
	}, "fixture/own")
	if err != nil {
		t.Fatal(err)
	}
	want := []struct{ name, param, typ string }{
		{"AliasArray", "a", "fixture/own.ArrLit"},
		{"AliasMap", "m", "fixture/own.MapLit"},
		{"AliasPtr", "p", "fixture/own.PtrLit"},
		{"AliasSlice", "s", "fixture/own.SliceLit"},
		{"Aliased", "a", "fixture/own.Alias"},
		{"Anon", "c", "struct{Name string}"},
		{"Apply", "v", "fixture/own.Variant"},
		{"Build", "cfg", "*fixture/own.Config"},
		{"Callback", "f", "func(*fixture/own.Config)"},
		{"Defined", "w", "*fixture/own.Wrapped"},
		{"LitIface", "v", "[]fixture/own.LitVariant"},
		{"LitStruct", "c", "*fixture/own.LitConfig"},
		{"Many", "byName", "map[string]*fixture/own.Config"},
		{"Many", "cfgs", "[]fixture/own.Config"},
		{"NamedArray", "a", "fixture/own.ConfigArray"},
		{"NamedList", "l", "fixture/own.ConfigList"},
		{"NamedLit", "l", "fixture/own.NamedLitList"},
		{"NamedMap", "m", "fixture/own.ConfigByName"},
		{"NamedPtr", "p", "fixture/own.ConfigPtr"},
	}
	if len(findings) != len(want) {
		t.Fatalf("expected %d findings, got %d: %+v", len(want), len(findings), findings)
	}
	for i, w := range want {
		f := findings[i]
		if f.Package != "fixture/own" || f.Name != w.name || f.Param != w.param || f.Type != w.typ || f.Pos.Line == 0 {
			t.Errorf("finding %d: want %s(%s %s), got %+v", i, w.name, w.param, w.typ, f)
		}
		if f.Key() != "fixture/own."+w.name {
			t.Errorf("finding %d: key %q", i, f.Key())
		}
	}
}

// TestOwnParameterTypes_UpstreamLiteralAliases swaps the upstream package's
// named spec for an alias to a struct literal. Ownership is decided where an
// alias is declared, so an upstream alias to a literal is upstream: the
// parameters that take it directly, through a callback, or through an
// in-tree re-export are not findings, and the 19 findings are unchanged
// (Wrapped is still declared in-tree, whatever its upstream base).
func TestOwnParameterTypes_UpstreamLiteralAliases(t *testing.T) {
	dir := writeParamsFixture(t)
	upstream := strings.Replace(paramsUpstreamSource, "type Spec struct{ Name string }", "type Spec = struct{ Name string }", 1)
	if upstream == paramsUpstreamSource {
		t.Fatal("fixture upstream source no longer declares Spec as expected")
	}
	if err := os.WriteFile(filepath.Join(dir, "up", "up.go"), []byte(upstream), 0o600); err != nil {
		t.Fatal(err)
	}
	findings, err := OwnParameterTypes(Options{
		Dir:      dir,
		Patterns: []string{"./..."},
		Env:      append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod"),
	}, "fixture/own")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range findings {
		switch f.Name {
		case "Plain", "UpCallback", "UpstreamAlias":
			t.Errorf("%s(%s %s): an upstream alias to a literal is not a kure-defined type", f.Name, f.Param, f.Type)
		}
	}
	if len(findings) != 19 {
		t.Errorf("expected the same 19 findings as with a named upstream spec, got %d: %+v", len(findings), findings)
	}
}

func TestOwnParameterTypes_PrefixMismatchReportsNothing(t *testing.T) {
	dir := writeParamsFixture(t)
	findings, err := OwnParameterTypes(Options{
		Dir:      dir,
		Patterns: []string{"./..."},
		Env:      append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod"),
	}, "elsewhere")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings under a foreign prefix, got %+v", findings)
	}
}

func TestOwnParameterTypes_LoadErrorIsReported(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module broken\n\ngo 1.26\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.go"), []byte("package broken\n\nfunc Bad(x Missing) {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := OwnParameterTypes(Options{
		Dir:      dir,
		Patterns: []string{"."},
		Env:      append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod"),
	}, "broken")
	if err == nil {
		t.Fatal("expected a load error for an undefined type")
	}
}
