package admission

import (
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/go-kure/kure/pkg/errors"
)

// ParamFinding is one parameter of an exported top-level function whose type
// the module under test defines itself where the upstream struct is the API.
type ParamFinding struct {
	// Package is the import path of the function's package.
	Package string
	// Name is the function name.
	Name string
	// Pos is the position of the declaration.
	Pos token.Position
	// Param is the parameter name ("_" when unnamed).
	Param string
	// Type is the parameter's type as written by the type checker.
	Type string
}

// Key returns "<package>.<Name>", the same form Finding uses.
func (f ParamFinding) Key() string { return f.Package + "." + f.Name }

// OwnParameterTypes loads the packages matching opts.Patterns and reports
// every exported top-level function in their non-test, non-generated files
// that takes a parameter whose type is declared under prefix (an import-path
// prefix, e.g. "github.com/go-kure/kure/pkg/kubernetes") and is a struct or
// an interface, reached directly or through any number of pointer, slice,
// array or map layers, named or not. A named scalar declared under prefix (a string enum)
// is not a spec type and is not reported; a type parameter is not either.
//
// This is the contract's rule that the upstream struct is the construction
// API: a function that takes a struct or a sum type kure invented is a
// second vocabulary for the same object, and the one this check exists to
// keep out once the Kind(&Config) layer was retired.
func OwnParameterTypes(opts Options, prefix string) ([]ParamFinding, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports,
		Dir:   opts.Dir,
		Env:   opts.Env,
		Tests: false,
	}
	pkgs, err := packages.Load(cfg, opts.Patterns...)
	if err != nil {
		return nil, errors.Wrap(err, "load packages")
	}
	var loadErrs []string
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			loadErrs = append(loadErrs, e.Error())
		}
	})
	if len(loadErrs) > 0 {
		return nil, errors.Errorf("load packages: %s", strings.Join(loadErrs, "; "))
	}

	var findings []ParamFinding
	for _, p := range pkgs {
		for _, file := range p.Syntax {
			name := filepath.Base(p.Fset.Position(file.Pos()).Filename)
			if strings.HasPrefix(name, "zz_generated") {
				continue
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv != nil || !fn.Name.IsExported() {
					continue
				}
				obj, ok := p.TypesInfo.Defs[fn.Name].(*types.Func)
				if !ok {
					continue
				}
				sig, ok := obj.Type().(*types.Signature)
				if !ok {
					continue
				}
				params := sig.Params()
				for i := 0; i < params.Len(); i++ {
					v := params.At(i)
					if !ownSpecType(v.Type(), prefix) {
						continue
					}
					pname := v.Name()
					if pname == "" {
						pname = "_"
					}
					findings = append(findings, ParamFinding{
						Package: p.PkgPath,
						Name:    fn.Name.Name,
						Pos:     p.Fset.Position(fn.Pos()),
						Param:   pname,
						Type:    types.TypeString(v.Type(), nil),
					})
				}
			}
		}
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Package != findings[j].Package {
			return findings[i].Package < findings[j].Package
		}
		if findings[i].Name != findings[j].Name {
			return findings[i].Name < findings[j].Name
		}
		return findings[i].Param < findings[j].Param
	})
	return findings, nil
}

// ownSpecType reports whether t is, or contains through pointer, slice,
// array or map layers, a named struct or interface type declared in a
// package whose import path starts with prefix. A named type whose
// underlying type is itself such a layer (type ConfigList []Config) is
// unwrapped too, so a container given a name does not hide its element; the
// seen set stops a self-referential name (type Tree []Tree) from recursing
// forever.
func ownSpecType(t types.Type, prefix string) bool {
	return ownSpecTypeSeen(t, prefix, map[*types.Named]bool{})
}

func ownSpecTypeSeen(t types.Type, prefix string, seen map[*types.Named]bool) bool {
	switch x := types.Unalias(t).(type) {
	case *types.Pointer:
		return ownSpecTypeSeen(x.Elem(), prefix, seen)
	case *types.Slice:
		return ownSpecTypeSeen(x.Elem(), prefix, seen)
	case *types.Array:
		return ownSpecTypeSeen(x.Elem(), prefix, seen)
	case *types.Map:
		return ownSpecTypeSeen(x.Key(), prefix, seen) || ownSpecTypeSeen(x.Elem(), prefix, seen)
	case *types.Named:
		if seen[x] {
			return false
		}
		seen[x] = true
		pkg := x.Obj().Pkg()
		switch x.Underlying().(type) {
		case *types.Struct, *types.Interface:
			return pkg != nil && strings.HasPrefix(pkg.Path(), prefix)
		case *types.Pointer, *types.Slice, *types.Array, *types.Map:
			return ownSpecTypeSeen(x.Underlying(), prefix, seen)
		}
		return false
	}
	return false
}
