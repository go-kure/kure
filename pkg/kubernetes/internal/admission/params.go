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
// that takes a parameter naming a struct or interface this tree defines
// itself, under prefix (an import-path prefix, e.g.
// "github.com/go-kure/kure/pkg/kubernetes"); ownSpecType lists every layer the
// walk follows. A named scalar declared under prefix (a string enum) is not a
// spec type and is not reported; a type parameter is not either.
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
				inTree := strings.HasPrefix(p.PkgPath, prefix)
				for i := 0; i < params.Len(); i++ {
					v := params.At(i)
					if !ownSpecType(v.Type(), prefix, inTree) {
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

// ownSpecType reports whether a parameter type t names, anywhere inside it, a
// struct or interface this tree defines itself. "Defines" is followed through
// every layer a type can hide behind:
//
//   - a named struct or interface declared in a package under prefix;
//   - pointer, slice, array, map and channel layers, named or not, and a
//     function type's parameters and results (a callback taking *Config is
//     the same vocabulary);
//   - an alias declared under prefix, whose target is owned even when it is
//     a container around an unnamed literal (type Config = *struct{...});
//   - a named container declared under prefix (type List []struct{...});
//   - an unnamed struct, or an unnamed interface with at least one method,
//     written in the signature itself — the parameter list belongs to this
//     tree when the function's own package is under prefix, so a literal
//     written there is defined here.
//
// A named type from outside prefix (the upstream API) is not entered, a type
// parameter is not reported, and an interface with no methods (any) is not a
// spec type. The seen set stops a self-referential name (type Tree []Tree)
// from recursing forever.
func ownSpecType(t types.Type, prefix string, inTree bool) bool {
	return ownSpecTypeIn(t, prefix, inTree, map[*types.Named]bool{})
}

// ownSpecTypeIn is ownSpecType with owned reporting whether the type being
// walked was written in, or reached through a name declared in, a package
// under prefix — which decides whether an unnamed literal found there counts.
func ownSpecTypeIn(t types.Type, prefix string, owned bool, seen map[*types.Named]bool) bool {
	under := func(pkg *types.Package) bool {
		return pkg != nil && strings.HasPrefix(pkg.Path(), prefix)
	}
	switch x := t.(type) {
	case *types.Alias:
		return ownSpecTypeIn(x.Rhs(), prefix, owned || under(x.Obj().Pkg()), seen)
	case *types.Pointer:
		return ownSpecTypeIn(x.Elem(), prefix, owned, seen)
	case *types.Slice:
		return ownSpecTypeIn(x.Elem(), prefix, owned, seen)
	case *types.Array:
		return ownSpecTypeIn(x.Elem(), prefix, owned, seen)
	case *types.Chan:
		return ownSpecTypeIn(x.Elem(), prefix, owned, seen)
	case *types.Map:
		return ownSpecTypeIn(x.Key(), prefix, owned, seen) || ownSpecTypeIn(x.Elem(), prefix, owned, seen)
	case *types.Signature:
		for _, tuple := range []*types.Tuple{x.Params(), x.Results()} {
			for i := 0; i < tuple.Len(); i++ {
				if ownSpecTypeIn(tuple.At(i).Type(), prefix, owned, seen) {
					return true
				}
			}
		}
		return false
	case *types.Named:
		if seen[x] {
			return false
		}
		seen[x] = true
		mine := under(x.Obj().Pkg())
		switch x.Underlying().(type) {
		case *types.Struct, *types.Interface:
			return mine
		case *types.Pointer, *types.Slice, *types.Array, *types.Map, *types.Chan, *types.Signature:
			return ownSpecTypeIn(x.Underlying(), prefix, mine, seen)
		}
		return false
	case *types.Struct:
		return owned
	case *types.Interface:
		return owned && x.NumMethods() > 0
	}
	return false
}
