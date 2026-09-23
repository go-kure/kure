package errors

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestExportedSentinelsHaveProducers fails when an exported Err* sentinel in
// this package is produced by no non-test code in the module. A sentinel no
// code returns is dead API: callers can compare against it, but no kure call
// ever produces it (go-kure/kure#758). Remove it rather than keep it exported.
func TestExportedSentinelsHaveProducers(t *testing.T) {
	sentinels := exportedSentinels(t, ".")
	if len(sentinels) == 0 {
		t.Fatal("found no exported Err* sentinels in pkg/errors; the parser no longer matches the package")
	}
	orphans := orphanSentinels(t, filepath.Join("..", ".."), "pkg/errors", sentinels)
	if len(orphans) > 0 {
		t.Errorf("%d exported sentinel(s) have no producer outside pkg/errors: %s", len(orphans), strings.Join(orphans, ", "))
	}
}

// exportedSentinels returns the exported package-level variables named Err*
// declared in the non-test Go files of dir.
func exportedSentinels(t *testing.T, dir string) map[string]bool {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, file := range files {
		if ok, err := compiledGoFile(file); err != nil {
			t.Fatal(err)
		} else if !ok {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			for _, spec := range gd.Specs {
				for _, name := range spec.(*ast.ValueSpec).Names {
					if strings.HasPrefix(name.Name, "Err") && name.IsExported() {
						out[name.Name] = true
					}
				}
			}
		}
	}
	return out
}

// orphanSentinels walks the non-test Go files under root, skipping pkgRel
// (the sentinels' own package, relative to root), and returns the sentinels
// that no file produces, sorted.
func orphanSentinels(t *testing.T, root, pkgRel string, sentinels map[string]bool) []string {
	t.Helper()
	used := map[string]bool{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip what the go tool does not build as part of this module: names
		// starting with "." or "_", testdata, vendor, and nested modules.
		name := d.Name()
		ignored := p != root && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_"))
		if d.IsDir() {
			if ignored || name == "testdata" || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			if p != root {
				if _, serr := os.Stat(filepath.Join(p, "go.mod")); serr == nil {
					return filepath.SkipDir
				}
			}
			if rel, _ := filepath.Rel(root, p); filepath.ToSlash(rel) == pkgRel {
				return filepath.SkipDir
			}
			return nil
		}
		if compiled, cerr := compiledGoFile(p); cerr != nil || !compiled {
			return cerr
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if perr != nil {
			return perr
		}
		if !importsKureErrors(f) {
			return nil
		}
		info := resolve(fset, f)
		compared := comparedExprs(f, info)
		// Comments are not parsed, so a mention in prose is not a producer.
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || !sentinels[sel.Sel.Name] || compared[sel] {
				return true
			}
			x, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			// The qualifier must resolve to the import itself: a local
			// variable or parameter spelled like the alias resolves to a Var.
			if pn, ok := info.Uses[x].(*types.PkgName); ok && pn.Imported().Path() == kureErrorsPath {
				used[sel.Sel.Name] = true
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	var orphans []string
	for name := range sentinels {
		if !used[name] {
			orphans = append(orphans, name)
		}
	}
	sort.Strings(orphans)
	return orphans
}

// kureErrorsPath is this package's import path; only selectors on an import of
// it can reference one of its sentinels.
const kureErrorsPath = "github.com/go-kure/kure/pkg/errors"

// importsKureErrors reports whether f imports kureErrorsPath; a file that
// does not cannot reference its sentinels, so it is not type-checked.
func importsKureErrors(f *ast.File) bool {
	for _, imp := range f.Imports {
		if strings.Trim(imp.Path.Value, `"`) == kureErrorsPath {
			return true
		}
	}
	return false
}

// stubImporter satisfies every import with an empty package, so a single file
// type-checks without loading its dependencies. Selectors into those packages
// then fail to resolve, but the package name on their left is still recorded.
type stubImporter struct{}

func (stubImporter) Import(path string) (*types.Package, error) {
	name := path[strings.LastIndex(path, "/")+1:]
	if path == kureErrorsPath {
		name = "errors"
	}
	pkg := types.NewPackage(path, name)
	pkg.MarkComplete()
	return pkg, nil
}

// compiledGoFile reports whether the go tool compiles path into its package's
// non-test build: a .go file, not _test.go, not named with a leading "." or
// "_", and admitted by its build constraints. Declarations and producers are
// both selected by this one rule.
func compiledGoFile(path string) (bool, error) {
	name := filepath.Base(path)
	if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
		return false, nil
	}
	return build.Default.MatchFile(filepath.Dir(path), name)
}

// resolve type-checks f alone and records what each identifier refers to and
// which expressions are types. Errors from the stubbed imports and the file's
// missing package siblings are expected and ignored; identifier resolution
// within the file is still exact. A dot or blank import records no
// package-name use, so such a file produces nothing and the guard errs towards
// reporting an orphan rather than hiding one.
func resolve(fset *token.FileSet, f *ast.File) *types.Info {
	info := &types.Info{
		Uses:  map[*ast.Ident]types.Object{},
		Types: map[ast.Expr]types.TypeAndValue{},
	}
	conf := types.Config{Importer: stubImporter{}, Error: func(error) {}}
	_, _ = conf.Check(f.Name.Name, fset, []*ast.File{f}, info)
	return info
}

// comparedExprs returns the expressions in f that reference a value without
// producing it: arguments to an Is or As call, operands of == and !=, a tagged
// switch's tag and case values, and values assigned to the blank identifier
// (the idiom for keeping a name referenced), each with any parentheses and
// type conversions removed.
//
// The check is syntactic: a sentinel stored in a variable and only then
// compared counts as produced. That needs data-flow analysis to tell apart
// and is a known limit, not a supported way to keep an orphan.
func comparedExprs(f *ast.File, info *types.Info) map[ast.Expr]bool {
	out := map[ast.Expr]bool{}
	mark := func(e ast.Expr) { out[unwrap(e, info)] = true }
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr:
			if sel, ok := ast.Unparen(n.Fun).(*ast.SelectorExpr); ok && (sel.Sel.Name == "Is" || sel.Sel.Name == "As") {
				for _, arg := range n.Args {
					mark(arg)
				}
			}
		case *ast.AssignStmt:
			if len(n.Lhs) == len(n.Rhs) {
				for i, lhs := range n.Lhs {
					if id, ok := lhs.(*ast.Ident); ok && id.Name == "_" {
						mark(n.Rhs[i])
					}
				}
			}
		case *ast.ValueSpec:
			if len(n.Names) == len(n.Values) {
				for i, name := range n.Names {
					if name.Name == "_" {
						mark(n.Values[i])
					}
				}
			}
		case *ast.BinaryExpr:
			if n.Op == token.EQL || n.Op == token.NEQ {
				mark(n.X)
				mark(n.Y)
			}
		case *ast.SwitchStmt:
			if n.Tag == nil {
				return true
			}
			mark(n.Tag)
			for _, stmt := range n.Body.List {
				for _, e := range stmt.(*ast.CaseClause).List {
					mark(e)
				}
			}
		}
		return true
	})
	return out
}

// unwrap strips parentheses and type conversions such as error(x) from e.
// A single-argument call is kept only when its callee resolves to a function
// declared in the file or a builtin. Anything else is unwrapped as a possible
// conversion: a type expression (including inline types such as
// interface{ Error() string }), and a callee from an import, which the stubbed
// imports cannot tell apart from a type such as kerrors.KureError. Unwrapping
// a real imported function errs towards reporting an orphan, never hiding one.
func unwrap(e ast.Expr, info *types.Info) ast.Expr {
	for {
		e = ast.Unparen(e)
		call, ok := e.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return e
		}
		var id *ast.Ident
		switch fun := ast.Unparen(call.Fun).(type) {
		case *ast.Ident:
			id = fun
		case *ast.SelectorExpr:
			id = fun.Sel
		}
		switch info.Uses[id].(type) {
		case *types.Func, *types.Builtin:
			return e
		}
		e = call.Args[0]
	}
}

// writeFixture writes files (path relative to a fresh temp dir -> content)
// and returns the temp dir.
func writeFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestOrphanSentinels_Fixtures(t *testing.T) {
	const decl = "package errors\n\nimport stderrors \"errors\"\n\nvar ErrProbe = stderrors.New(\"probe\")\n"
	// produced returns the sentinel: a producer wherever the build sees it.
	const produced = "package c\n\nimport kerrors \"github.com/go-kure/kure/pkg/errors\"\n\nfunc F() error { return kerrors.ErrProbe }\n"
	for _, tc := range []struct {
		desc       string
		consumer   string
		wantOrphan bool
		path       string            // consumer's path; default pkg/c/c.go
		extra      map[string]string // further fixture files
	}{
		{"a returned sentinel is produced", `package c

import kerrors "github.com/go-kure/kure/pkg/errors"

func F() error { return kerrors.ErrProbe }
`, false, "", nil},
		{"errors.Is comparison is not a producer", `package c

import (
	stderrors "errors"

	kerrors "github.com/go-kure/kure/pkg/errors"
)

func F(err error) bool { return stderrors.Is(err, kerrors.ErrProbe) }
`, true, "", nil},
		{"== comparison is not a producer", `package c

import kerrors "github.com/go-kure/kure/pkg/errors"

func F(err error) bool { return err == kerrors.ErrProbe }
`, true, "", nil},
		{"a switch case is not a producer", `package c

import kerrors "github.com/go-kure/kure/pkg/errors"

func F(err error) bool {
	switch err {
	case kerrors.ErrProbe:
		return true
	}
	return false
}
`, true, "", nil},
		{"a default-named import produces", `package c

import "github.com/go-kure/kure/pkg/errors"

func F() error { return errors.Wrap(errors.ErrProbe, "ctx") }
`, false, "", nil},
		{"an unrelated same-name field is not a producer", `package c

import kerrors "github.com/go-kure/kure/pkg/errors"

var _ = kerrors.New

type s struct{ ErrProbe error }

func F(v s) error { return v.ErrProbe }
`, true, "", nil},
		{"a parenthesized == operand is not a producer", `package c

import kerrors "github.com/go-kure/kure/pkg/errors"

func F(err error) bool { return err == (kerrors.ErrProbe) }
`, true, "", nil},
		{"a parenthesized errors.Is argument is not a producer", `package c

import (
	stderrors "errors"

	kerrors "github.com/go-kure/kure/pkg/errors"
)

func F(err error) bool { return stderrors.Is(err, (kerrors.ErrProbe)) }
`, true, "", nil},
		{"a parenthesized errors.Is callee is not a producer", `package c

import (
	stderrors "errors"

	kerrors "github.com/go-kure/kure/pkg/errors"
)

func F(err error) bool { return (stderrors.Is)(err, kerrors.ErrProbe) }
`, true, "", nil},
		{"a converted errors.Is argument is not a producer", `package c

import (
	stderrors "errors"

	kerrors "github.com/go-kure/kure/pkg/errors"
)

func F(err error) bool { return stderrors.Is(err, error(kerrors.ErrProbe)) }
`, true, "", nil},
		{"a converted return is a producer", `package c

import kerrors "github.com/go-kure/kure/pkg/errors"

func F() error { return error(kerrors.ErrProbe) }
`, false, "", nil},
		{"a file its build constraint excludes is not a producer", "//go:build ignore\n\n" + produced, true, "", nil},
		{"a switch tag is not a producer", `package c

import kerrors "github.com/go-kure/kure/pkg/errors"

func F(err error) bool {
	switch kerrors.ErrProbe {
	case err:
		return true
	}
	return false
}
`, true, "", nil},
		{"an inline interface conversion in a comparison is not a producer", `package c

import kerrors "github.com/go-kure/kure/pkg/errors"

func F(err error) bool { return err == interface{ Error() string }(kerrors.ErrProbe) }
`, true, "", nil},
		{"an imported-type conversion in a comparison is not a producer", `package c

import kerrors "github.com/go-kure/kure/pkg/errors"

func F(err error) bool { return err == kerrors.KureError(kerrors.ErrProbe) }
`, true, "", nil},
		{"a local function call in a comparison is a producer", `package c

import kerrors "github.com/go-kure/kure/pkg/errors"

func wrap(err error) error { return err }

func F(err error) bool { return err == wrap(kerrors.ErrProbe) }
`, false, "", nil},
		{"a package-level blank assignment is not a producer", `package c

import kerrors "github.com/go-kure/kure/pkg/errors"

var _ = kerrors.ErrProbe
`, true, "", nil},
		{"a blank assignment in a function is not a producer", `package c

import kerrors "github.com/go-kure/kure/pkg/errors"

func F() { _ = kerrors.ErrProbe }
`, true, "", nil},
		{"a named assignment is a producer", `package c

import kerrors "github.com/go-kure/kure/pkg/errors"

var errDefault = kerrors.ErrProbe

func F() error { return errDefault }
`, false, "", nil},
		{"a local shadowing the import alias is not a producer", `package c

import kerrors "github.com/go-kure/kure/pkg/errors"

type holder struct{ ErrProbe error }

var _ = kerrors.New

func F(kerrors holder) error { return kerrors.ErrProbe }
`, true, "", nil},
		{"a file the go tool ignores is not a producer", produced, true, "pkg/c/_ignored.go", nil},
		{"a file under a directory the go tool ignores is not a producer", produced, true, "pkg/_c/c.go", nil},
		{"a nested module is not a producer", produced, true, "nested/c.go",
			map[string]string{"nested/go.mod": "module example.com/nested\n"}},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			path := tc.path
			if path == "" {
				path = "pkg/c/c.go"
			}
			files := map[string]string{
				"go.mod":               "module github.com/go-kure/kure\n",
				"pkg/errors/errors.go": decl,
				path:                   tc.consumer,
			}
			for p, c := range tc.extra {
				files[p] = c
			}
			root := writeFixture(t, files)
			sentinels := exportedSentinels(t, filepath.Join(root, "pkg", "errors"))
			got := orphanSentinels(t, root, "pkg/errors", sentinels)
			if isOrphan := len(got) == 1 && got[0] == "ErrProbe"; isOrphan != tc.wantOrphan {
				t.Errorf("orphans = %v, want ErrProbe orphaned = %v", got, tc.wantOrphan)
			}
		})
	}
}

func TestExportedSentinels_EveryPackageFile(t *testing.T) {
	// A sentinel declared in a second file of the package is inventoried too.
	root := writeFixture(t, map[string]string{
		"errors.go": "package errors\n\nimport stderrors \"errors\"\n\nvar ErrA = stderrors.New(\"a\")\n",
		"extra.go":  "package errors\n\nimport stderrors \"errors\"\n\nvar ErrB = stderrors.New(\"b\")\n",
	})
	got := exportedSentinels(t, root)
	if !got["ErrA"] || !got["ErrB"] {
		t.Errorf("sentinels = %v, want ErrA and ErrB", got)
	}
}

func TestExportedSentinels_OnlyCompiledFiles(t *testing.T) {
	// Declarations follow the same compiled-file rule as producers: a file the
	// go tool ignores or a build constraint excludes declares nothing.
	root := writeFixture(t, map[string]string{
		"errors.go":   "package errors\n\nimport stderrors \"errors\"\n\nvar ErrA = stderrors.New(\"a\")\n",
		"_ignored.go": "package errors\n\nimport stderrors \"errors\"\n\nvar ErrB = stderrors.New(\"b\")\n",
		"tagged.go":   "//go:build ignore\n\npackage errors\n\nimport stderrors \"errors\"\n\nvar ErrC = stderrors.New(\"c\")\n",
	})
	got := exportedSentinels(t, root)
	if len(got) != 1 || !got["ErrA"] {
		t.Errorf("sentinels = %v, want only ErrA", got)
	}
}
