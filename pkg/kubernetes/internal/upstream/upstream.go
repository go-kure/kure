// Package upstream loads the pinned API module sources that back the
// registered kinds, so scope and maturity are read from the modules kure
// actually builds against rather than from a table maintained by hand.
//
// The pinned version is whatever the build resolves — this package never globs
// the module cache. Several minor versions of a module are routinely present
// there at once (three cilium releases, at the time of writing), and picking
// the wrong one would describe an API kure does not compile against.
package upstream

import (
	"go/ast"
	"go/token"
	"reflect"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/go-kure/kure/pkg/errors"
)

// Type is one named type in a pinned upstream package, with the doc comment
// its markers live in.
type Type struct {
	ImportPath string
	Name       string
	Doc        string // the type's doc comment, marker lines included
	Module     string // module path, "" for the standard library
	Version    string // pinned module version
	// ModuleDir is the module's directory in the module cache, "" when the
	// loader reported none. It is what lets a caller read the artifacts a
	// module ships alongside its Go source — the generated CRDs, in
	// particular, which answer the scope question for a type whose own doc
	// comment does not.
	ModuleDir string
	Fields    []Field
	// Imports maps the import aliases in scope in the file that declares this
	// type to their import paths, so a field written as metav1.ObjectMeta can
	// be resolved to the package that declares it. Aliases are file-scoped in
	// Go, so this cannot be held per package.
	Imports map[string]string
}

// Field is one struct field of an upstream type.
type Field struct {
	Name     string // Go field name
	JSONName string // the name in the json tag, "" when the tag is absent
	TypeExpr string // the field's type, as written in source
	Doc      string // the field's doc comment, marker lines included
	Inline   bool   // the json tag carries ",inline"
}

// Load parses the given import paths and returns every named type in them,
// keyed by "importPath.TypeName".
//
// Load fails when any requested package reports an error. A partial load is
// worse than no load: a package that failed to type-check yields types with no
// doc comments, which would read as "no markers" and silently produce
// namespaced entries for cluster-scoped kinds.
func Load(importPaths []string) (map[string]Type, error) {
	if len(importPaths) == 0 {
		return map[string]Type{}, nil
	}
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax |
			packages.NeedModule | packages.NeedCompiledGoFiles,
	}
	pkgs, err := packages.Load(cfg, importPaths...)
	if err != nil {
		return nil, errors.Wrap(err, "upstream: load API packages")
	}
	var loadErrs []string
	for _, p := range pkgs {
		for _, e := range p.Errors {
			loadErrs = append(loadErrs, p.PkgPath+": "+e.Msg)
		}
	}
	if len(loadErrs) > 0 {
		return nil, errors.Errorf("upstream: %d package error(s): %s", len(loadErrs), strings.Join(loadErrs, "; "))
	}
	if len(pkgs) != len(importPaths) {
		return nil, errors.Errorf("upstream: asked for %d packages, loaded %d", len(importPaths), len(pkgs))
	}

	out := map[string]Type{}
	for _, p := range pkgs {
		if len(p.Syntax) == 0 {
			return nil, errors.Errorf("upstream: %s loaded no syntax; the module cache may be cold", p.PkgPath)
		}
		var mod module
		if p.Module != nil {
			mod = module{path: p.Module.Path, version: p.Module.Version, dir: p.Module.Dir}
		}
		for _, file := range p.Syntax {
			collectTypes(file, p.PkgPath, mod, out)
		}
	}
	return out, nil
}

// Key is the map key Load uses for a type.
func Key(importPath, name string) string { return importPath + "." + name }

// module is the provenance every type collected from a package carries.
type module struct {
	path    string
	version string
	dir     string
}

// collectTypes records every named type declared in file.
func collectTypes(file *ast.File, importPath string, mod module, out map[string]Type) {
	imports := fileImports(file)
	// Lower bound for the detached-marker scan: the package clause, so a
	// file's license header is never mistaken for the first type's markers.
	prevEnd := file.Name.End()
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			prevEnd = decl.End()
			continue
		}
		detached := detachedMarkers(file, gen, prevEnd)
		for _, spec := range gen.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			t := Type{
				ImportPath: importPath,
				Name:       ts.Name.Name,
				Doc:        typeDoc(gen, ts, detached),
				Module:     mod.path,
				Version:    mod.version,
				ModuleDir:  mod.dir,
				Imports:    imports,
			}
			if st, ok := ts.Type.(*ast.StructType); ok {
				t.Fields = structFields(st)
			}
			out[Key(importPath, ts.Name.Name)] = t
		}
		prevEnd = gen.End()
	}
}

// fileImports maps the import aliases in scope in a file to their paths. An
// import with no explicit alias is keyed by the last segment of its path,
// which is what the source refers to it by in the API packages kure reads
// (none of them import a package whose name differs from its directory).
func fileImports(file *ast.File) map[string]string {
	out := map[string]string{}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		alias := path
		if i := strings.LastIndex(path, "/"); i >= 0 {
			alias = path[i+1:]
		}
		if spec.Name != nil {
			alias = spec.Name.Name
		}
		out[alias] = path
	}
	return out
}

// detachedMarkers returns the comment groups that sit between the previous
// declaration and this one but are not attached to it as Doc.
//
// controller-gen conventionally separates its marker block from the type's
// prose doc comment with a blank line:
//
//	// +kubebuilder:resource:scope=Cluster
//
//	// ClusterImageCatalog is the Schema for the clusterimagecatalogs API
//	type ClusterImageCatalog struct{ … }
//
// That blank line is what detaches the block: go/ast attaches only the
// immediately preceding comment group as Doc, so the markers become
// free-floating comments in File.Comments and a reader that consults Doc alone
// sees no markers at all. Every CRD module kure pins uses this layout, so
// consulting Doc alone derived 31 of 33 cluster-scoped kinds as namespaced.
//
// Only a single-spec declaration takes detached markers. In a grouped
// `type ( … )` block a preceding block cannot be attributed to one spec, and
// giving every spec the same markers would be worse than giving them none.
func detachedMarkers(file *ast.File, gen *ast.GenDecl, after token.Pos) string {
	if len(gen.Specs) != 1 {
		return ""
	}
	var b strings.Builder
	for _, cg := range file.Comments {
		if cg == gen.Doc {
			continue // already read as Doc
		}
		if cg.Pos() > after && cg.End() < gen.Pos() {
			b.WriteString(cg.Text())
		}
	}
	return b.String()
}

// typeDoc returns every comment that can carry a type's markers: the attached
// doc comment (on the TypeSpec inside a grouped declaration, on the GenDecl
// otherwise) plus any marker block detached from it by a blank line.
func typeDoc(gen *ast.GenDecl, ts *ast.TypeSpec, detached string) string {
	var parts []string
	if detached != "" {
		parts = append(parts, detached)
	}
	if ts.Doc != nil {
		parts = append(parts, ts.Doc.Text())
	} else if len(gen.Specs) == 1 && gen.Doc != nil {
		parts = append(parts, gen.Doc.Text())
	}
	return strings.Join(parts, "\n")
}

// structFields records a struct's fields, including embedded ones (which carry
// the json ",inline" tag and whose type name is the field name).
func structFields(st *ast.StructType) []Field {
	var out []Field
	for _, f := range st.Fields.List {
		jsonName, inline, omitted := jsonTag(f.Tag)
		if omitted {
			// json:"-" means the field is not serialised at all, so it is not
			// part of the API this package describes: it has no name a manifest
			// could carry, and recording it would file every such field under
			// the key "-" and walk a type nothing can reach through the API.
			continue
		}
		doc := ""
		if f.Doc != nil {
			doc = f.Doc.Text()
		}
		typeExpr := exprString(f.Type)
		if len(f.Names) == 0 {
			// Embedded field: its name is the type's own name.
			out = append(out, Field{
				Name:     embeddedName(typeExpr),
				JSONName: jsonName,
				TypeExpr: typeExpr,
				Doc:      doc,
				Inline:   inline,
			})
			continue
		}
		for _, n := range f.Names {
			out = append(out, Field{
				Name:     n.Name,
				JSONName: jsonName,
				TypeExpr: typeExpr,
				Doc:      doc,
				Inline:   inline,
			})
		}
	}
	return out
}

// jsonTag returns the field's json name, whether it is inlined, and whether the
// tag excludes it from serialisation entirely.
//
// The literal is unquoted with strconv rather than trimmed with a cutset: a
// cutset of "`\"" strips the tag's own closing quote along with the backticks,
// turning `json:"spec"` into `json:"spec` and losing every name.
//
// omitted follows encoding/json exactly: only a tag whose whole value is "-"
// excludes the field. `json:"-,"` names the field "-" and does not, which is
// obscure but is the rule, and guessing the other way would drop a field the
// API really does carry.
func jsonTag(tag *ast.BasicLit) (name string, inline, omitted bool) {
	if tag == nil {
		return "", false, false
	}
	raw := tag.Value
	if unquoted, err := strconv.Unquote(raw); err == nil {
		raw = unquoted
	}
	value, ok := reflect.StructTag(raw).Lookup("json")
	if !ok {
		return "", false, false
	}
	if value == "-" {
		return "", false, true
	}
	parts := strings.Split(value, ",")
	for _, p := range parts[1:] {
		if p == "inline" {
			return parts[0], true, false
		}
	}
	return parts[0], false, false
}

// exprString renders a type expression the way it is written in source, for
// the subset of expressions API types use.
func exprString(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + exprString(t.X)
	case *ast.SelectorExpr:
		return exprString(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + exprString(t.Elt)
	case *ast.MapType:
		return "map[" + exprString(t.Key) + "]" + exprString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	default:
		return ""
	}
}

// embeddedName returns the Go field name of an embedded field: its type's own
// name, without pointer or package qualifier.
func embeddedName(typeExpr string) string {
	bare := strings.TrimPrefix(typeExpr, "*")
	if i := strings.LastIndex(bare, "."); i >= 0 {
		return bare[i+1:]
	}
	return bare
}
