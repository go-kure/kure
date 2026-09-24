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
// every exported top-level function in the non-test, non-generated files the
// current build context selects (pkg/kubernetes has no file behind a build
// constraint; one added later is checked only on the platforms that build it)
// that takes a parameter naming a struct or interface this tree defines
// itself, under prefix (an import-path prefix, e.g.
// "github.com/go-kure/kure/pkg/kubernetes"); ownSpecType lists every layer the
// walk follows. A named scalar declared under prefix (a string enum) is not a
// spec type and is not reported; a type parameter only when its constraint
// is.
//
// Like Classify, this is a guard against the Config layer drifting back in by
// ordinary authorship, not against a signature written to evade it: it
// follows the type-system layers listed on ownSpecType, and anything outside
// them is caught by review and by this package's own fixture, not here.
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
	// A pattern that matches no package loads nothing and reports no error;
	// passing on that would be a check that inspected nothing.
	if len(pkgs) == 0 {
		return nil, errors.Errorf("load packages: %s matched no package under %s", strings.Join(opts.Patterns, " "), opts.Dir)
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
//     a container around an unnamed literal (type Config = *struct{...}).
//     Ownership is reset at every alias to where that alias is declared, so
//     an upstream alias to a literal (type Spec = struct{...} upstream) stays
//     upstream however it is reached;
//   - a named container declared under prefix (type List []struct{...});
//   - an unnamed struct, or an unnamed interface with at least one method,
//     written in the signature itself — the parameter list belongs to this
//     tree when the function's own package is under prefix, so a literal
//     written there is defined here.
//
// A named type from outside prefix (the upstream API) is not entered, beyond
// the type arguments of an instantiated generic or generic alias; a type
// parameter is reported only when its constraint is — including the embedded
// types and union terms of its type set — and an interface with no methods (any) is not a
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
		// The type arguments of an instantiated generic alias are written in
		// the signature being walked, so they keep its ownership; the RHS
		// belongs to wherever the alias is declared.
		if args := x.TypeArgs(); args != nil {
			for i := 0; i < args.Len(); i++ {
				if ownSpecTypeIn(args.At(i), prefix, owned, seen) {
					return true
				}
			}
		}
		return ownSpecTypeIn(x.Rhs(), prefix, under(x.Obj().Pkg()), seen)
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
		// An instantiated generic carries its type arguments in the
		// signature even when the generic itself is upstream
		// (up.Wrapper[Config]); each argument is checked where it is written.
		if args := x.TypeArgs(); args != nil {
			for i := 0; i < args.Len(); i++ {
				if ownSpecTypeIn(args.At(i), prefix, owned, seen) {
					return true
				}
			}
		}
		switch x.Underlying().(type) {
		case *types.Struct, *types.Interface:
			return mine
		case *types.Pointer, *types.Slice, *types.Array, *types.Map, *types.Chan, *types.Signature:
			return ownSpecTypeIn(x.Underlying(), prefix, mine, seen)
		}
		return false
	case *types.TypeParam:
		// A type parameter is reported only through its constraint: one
		// constrained by a kure-defined interface ([T Variant]) is the
		// sealed-sum shape again; any, comparable and upstream constraints
		// are not.
		return ownSpecTypeIn(x.Constraint(), prefix, owned, seen)
	case *types.Struct:
		return owned
	case *types.Interface:
		// Embedded types carry a constraint's type set ([T Config],
		// [T interface{ Config | *Config }]); they are written where the
		// interface is, so they keep its ownership.
		for i := 0; i < x.NumEmbeddeds(); i++ {
			if ownSpecTypeIn(x.EmbeddedType(i), prefix, owned, seen) {
				return true
			}
		}
		return owned && x.NumMethods() > 0
	case *types.Union:
		for i := 0; i < x.Len(); i++ {
			if ownSpecTypeIn(x.Term(i).Type(), prefix, owned, seen) {
				return true
			}
		}
		return false
	}
	return false
}
