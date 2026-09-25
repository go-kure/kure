// Command docapiindex builds the declaration index scripts/check-doc-api-refs.sh
// resolves documentation references against, by parsing the Go files it is
// given rather than matching their text.
//
//	printf '%s\0' pkg/a/a.go ... | go run ./scripts/docapiindex symbols
//	printf '%s\0' pkg/a/a.go ... | go run ./scripts/docapiindex types
//
// The input is a NUL-separated list of Go files on stdin. Each row names the
// directory of the file that declares it, cleaned as filepath.Dir cleans it
// (`./pkg/a/a.go` gives `pkg/a`).
//
// symbols prints "<dir> <name> <receiver>" for every exported package-level
// declaration -- func, const, var and type, `-` in the receiver column -- and
// for every method with an exported name, its receiver column holding the bare
// receiver type: the pointer and any type-parameter list dropped.
//
// types prints "<dir> <type>" for every exported type a method can be declared
// on: aliases (whose methods are their target's) and interfaces (whose methods
// are not func declarations the symbol index could hold) are left out.
//
// A text match over the same files cannot give these answers exactly: it reads
// a declaration inside a comment or a raw string as real, it misses one inside
// a grouped `type ( ... )`, `const ( ... )` or `var ( ... )` block, and a
// type-parameter list holding a bracket defeats it. A file that does not parse
// is an error, not a skipped file: a thinner index is a greener check.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/go-kure/kure/pkg/errors"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run is main without the process: it returns the exit status.
//
// A failed write to stderr has nowhere to be reported, so its error is
// dropped; the exit status still says the run failed. A failed write to
// stdout is not dropped: bufio keeps the first error and Flush returns it.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) != 1 || (args[0] != "symbols" && args[0] != "types") {
		_, _ = fmt.Fprintln(stderr, "usage: docapiindex symbols|types < NUL-separated Go file paths")
		return 2
	}
	input, err := io.ReadAll(stdin)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "docapiindex: reading file list: %v\n", err)
		return 1
	}
	var rows []string
	for _, path := range bytes.Split(input, []byte{0}) {
		if len(path) == 0 {
			continue
		}
		fileRows, err := index(args[0], string(path))
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "docapiindex: %v\n", err)
			return 1
		}
		rows = append(rows, fileRows...)
	}
	sort.Strings(rows)
	w := bufio.NewWriter(stdout)
	for _, row := range rows {
		_, _ = fmt.Fprintln(w, row)
	}
	if err := w.Flush(); err != nil {
		_, _ = fmt.Fprintf(stderr, "docapiindex: writing index: %v\n", err)
		return 1
	}
	return 0
}

// index parses one file and returns its rows for the given mode.
func index(mode, path string) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, errors.Wrap(err, "parsing "+path)
	}
	dir := filepath.Dir(path)
	if mode == "types" {
		return typeRows(dir, file), nil
	}
	return symbolRows(dir, file), nil
}

// symbolRows is the symbols mode for one parsed file.
func symbolRows(dir string, file *ast.File) []string {
	var rows []string
	add := func(name *ast.Ident, recv string) {
		if name.IsExported() {
			rows = append(rows, dir+" "+name.Name+" "+recv)
		}
	}
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			recv := "-"
			if d.Recv != nil {
				// The parser accepts an empty or multiple receiver list,
				// which the type checker rejects; such a method is still
				// not a plain function.
				recv = "?"
				if len(d.Recv.List) == 1 {
					recv = receiverType(d.Recv.List[0].Type)
				}
			}
			add(d.Name, recv)
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					add(s.Name, "-")
				case *ast.ValueSpec:
					for _, name := range s.Names {
						add(name, "-")
					}
				}
			}
		}
	}
	return rows
}

// typeRows is the types mode for one parsed file.
func typeRows(dir string, file *ast.File) []string {
	var rows []string
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			s := spec.(*ast.TypeSpec)
			if !s.Name.IsExported() || s.Assign.IsValid() {
				continue
			}
			if _, isInterface := s.Type.(*ast.InterfaceType); isInterface {
				continue
			}
			rows = append(rows, dir+" "+s.Name.Name)
		}
	}
	return rows
}

// receiverType reduces a receiver's type expression to the name of the type:
// `*T`, `T`, `T[K]`, `*T[K, V]` and a parenthesised `(*T)` all give T.
func receiverType(expr ast.Expr) string {
	for {
		switch e := expr.(type) {
		case *ast.Ident:
			return e.Name
		case *ast.StarExpr:
			expr = e.X
		case *ast.ParenExpr:
			expr = e.X
		case *ast.IndexExpr:
			expr = e.X
		case *ast.IndexListExpr:
			expr = e.X
		default:
			// The parser accepts receiver shapes the type checker rejects,
			// such as a qualified pkg.T; such a file does not compile, so no
			// method of it can be live API. Name the shape so it cannot
			// resolve against a real type.
			return fmt.Sprintf("?%T", e)
		}
	}
}
