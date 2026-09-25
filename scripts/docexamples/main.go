// Command docexamples keeps the Go blocks of documentation pages in step with
// compiled Example functions, so a page cannot show code that no longer
// builds.
//
//	go run ./scripts/docexamples generate page.md ...
//	go run ./scripts/docexamples check page.md ...
//
// A page marks a generated block with the package directory and the name of
// an Example function in that package's test files:
//
//	<!-- doc-example: pkg/kubernetes/metallb ExampleCreateIPAddressPool -->
//	```go
//	pool := metallb.CreateIPAddressPool("my-pool", "metallb-system")
//	```
//	<!-- doc-example:end -->
//
// and a block that cannot be one, with the reason:
//
//	<!-- doc-example:excerpt shows a type declaration, not a call -->
//	```go
//
// generate replaces everything between a doc-example marker and its end
// marker with the Example's body: the statements between its braces, less
// one level of indentation, each remaining leading tab written as four
// spaces, and without its `// Output:` comment or anything after it. A new
// block is added by writing the two markers and running generate.
//
// check changes nothing and fails when generate would change a page, or when
// a page has a ```go block that is neither generated nor marked as an
// excerpt. A marker line it does not recognise, an excerpt marker with no
// reason or not directly above a ```go fence, a doc-example marker with no
// end marker, and an Example that does not exist are errors in both modes.
//
// Markers inside a fenced block are text, not markers, and a ```go block
// inside another fence is not a block of the page. A marker may be indented,
// as inside a list item; the generated block takes its indentation.
package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/go-kure/kure/pkg/errors"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is main without the process: it returns the exit status.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 || (args[0] != "generate" && args[0] != "check") {
		_, _ = fmt.Fprintln(stderr, "usage: docexamples generate|check page.md ...")
		return 2
	}
	mode, pages := args[0], args[1:]
	examples := exampleCache{}
	failed := false
	for _, page := range pages {
		src, err := os.ReadFile(page) //nolint:gosec // G703: the page is this tool's own argument (the G304 rationale in .golangci.yml)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "docexamples: %v\n", err)
			return 1
		}
		out, problems, err := process(page, src, examples)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "docexamples: %v\n", err)
			return 1
		}
		for _, p := range problems {
			_, _ = fmt.Fprintln(stderr, p)
			failed = true
		}
		if bytes.Equal(out, src) {
			continue
		}
		if mode == "check" {
			_, _ = fmt.Fprintf(stderr, "%s: a generated block differs from its Example; run scripts/gen-doc-examples.sh\n", page)
			failed = true
			continue
		}
		if err := os.WriteFile(page, out, 0o644); err != nil { //nolint:gosec // G703: rewrites the page it was given, as above
			_, _ = fmt.Fprintf(stderr, "docexamples: %v\n", err)
			return 1
		}
		_, _ = fmt.Fprintf(stdout, "generated %s\n", page)
	}
	if failed {
		return 1
	}
	return 0
}

var (
	fenceRE   = regexp.MustCompile("^([ \t]*)(`{3,}|~{3,})[ \t]*([^ \t`]*)")
	exampleRE = regexp.MustCompile(`^<!-- doc-example: (\S+) (Example\S*) -->$`)
	excerptRE = regexp.MustCompile(`^<!-- doc-example:excerpt(.*)-->$`)
	endMarker = "<!-- doc-example:end -->"
)

// process returns the page as generate would write it, and every problem
// found on it as "page:line: message". A generated block whose Example cannot
// be found is a problem, and the page keeps its current text there.
func process(page string, src []byte, examples exampleCache) ([]byte, []string, error) {
	lines := strings.SplitAfter(string(src), "\n")
	var out strings.Builder
	var problems []string
	report := func(line int, format string, args ...any) {
		problems = append(problems, fmt.Sprintf("%s:%d: %s", page, line, fmt.Sprintf(format, args...)))
	}
	excerpted := false // the previous line was an excerpt marker
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		text := strings.TrimRight(line, "\r\n")
		trimmed := strings.TrimSpace(text)
		if m := fenceRE.FindStringSubmatch(text); m != nil {
			end := closingFence(lines, i, m[2])
			if m[3] == "go" && !excerpted {
				report(i+1, "a ```go block that is neither generated nor marked <!-- doc-example:excerpt <reason> -->")
			}
			excerpted = false
			for ; i <= end && i < len(lines); i++ {
				out.WriteString(lines[i])
			}
			i--
			continue
		}
		if excerpted {
			report(i, "an excerpt marker must be directly above a ```go fence")
			excerpted = false
		}
		switch {
		case !strings.HasPrefix(trimmed, "<!-- doc-example"):
			out.WriteString(line)
		case excerptRE.MatchString(trimmed):
			if strings.TrimSpace(excerptRE.FindStringSubmatch(trimmed)[1]) == "" {
				report(i+1, "an excerpt marker needs a reason: <!-- doc-example:excerpt <reason> -->")
			}
			excerpted = true
			out.WriteString(line)
		case exampleRE.MatchString(trimmed):
			m := exampleRE.FindStringSubmatch(trimmed)
			end := -1
			for j := i + 1; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) == endMarker {
					end = j
					break
				}
				if strings.HasPrefix(strings.TrimSpace(lines[j]), "<!-- doc-example") {
					break
				}
			}
			if end < 0 {
				report(i+1, "no %s after this doc-example marker", endMarker)
				out.WriteString(line)
				continue
			}
			body, err := examples.body(m[1], m[2])
			if err != nil {
				return nil, nil, err
			}
			out.WriteString(line)
			if body == nil {
				report(i+1, "%s declares no function %s in its test files", m[1], m[2])
				for j := i + 1; j <= end; j++ {
					out.WriteString(lines[j])
				}
			} else {
				indent := text[:len(text)-len(strings.TrimLeft(text, " \t"))]
				fmt.Fprintf(&out, "%s```go\n", indent)
				for _, b := range body {
					if b != "" {
						out.WriteString(indent)
						out.WriteString(b)
					}
					out.WriteString("\n")
				}
				fmt.Fprintf(&out, "%s```\n", indent)
				out.WriteString(lines[end])
			}
			i = end
		default:
			report(i+1, "unrecognised doc-example marker %q", trimmed)
			out.WriteString(line)
		}
	}
	if excerpted {
		report(len(lines), "an excerpt marker must be directly above a ```go fence")
	}
	return []byte(out.String()), problems, nil
}

// closingFence returns the index of the line closing the fence opened at
// lines[open] with the fence string marker, or the last line: an unclosed
// fence runs to the end of the page.
func closingFence(lines []string, open int, marker string) int {
	for j := open + 1; j < len(lines); j++ {
		t := strings.TrimSpace(lines[j])
		if strings.HasPrefix(t, marker[:3]) && strings.Trim(t, marker[:1]) == "" && len(t) >= len(marker) {
			return j
		}
	}
	return len(lines) - 1
}

// exampleCache holds, per package directory, the body of every Example
// function its test files declare, so each directory is parsed once.
type exampleCache map[string]map[string][]string

// body returns the generated lines for the named Example in dir, or nil when
// dir's test files declare no such function. A test file that does not parse
// is an error: the Example may be in it.
func (c exampleCache) body(dir, name string) ([]string, error) {
	if _, ok := c[dir]; !ok {
		bodies, err := parseExamples(dir)
		if err != nil {
			return nil, err
		}
		c[dir] = bodies
	}
	return c[dir][name], nil
}

// parseExamples returns the body of every top-level Example function in
// dir's *_test.go files.
func parseExamples(dir string) (map[string][]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	if err != nil {
		return nil, errors.Wrap(err, "listing "+dir)
	}
	sort.Strings(files)
	bodies := map[string][]string{}
	for _, path := range files {
		src, err := os.ReadFile(path) //nolint:gosec // G703: a *_test.go file in the package a marker names
		if err != nil {
			return nil, errors.Wrap(err, "reading "+path)
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, src, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return nil, errors.Wrap(err, "parsing "+path)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil || !strings.HasPrefix(fn.Name.Name, "Example") {
				continue
			}
			bodies[fn.Name.Name] = exampleBody(fset, src, file, fn.Body)
		}
	}
	return bodies, nil
}

// exampleBody is the text between body's braces, up to its output comment,
// with blank lines at either end dropped, one leading tab removed from each
// line and every remaining leading tab written as four spaces.
func exampleBody(fset *token.FileSet, src []byte, file *ast.File, body *ast.BlockStmt) []string {
	start := fset.Position(body.Lbrace).Offset + 1
	end := fset.Position(body.Rbrace).Offset
	for _, cg := range file.Comments {
		if cg.Pos() < body.Lbrace || cg.End() > body.Rbrace {
			continue
		}
		text := strings.TrimSpace(cg.Text())
		if strings.HasPrefix(text, "Output:") || strings.HasPrefix(text, "Unordered output:") {
			end = fset.Position(cg.Pos()).Offset
			break
		}
	}
	lines := strings.Split(string(src[start:end]), "\n")
	for i, l := range lines {
		l = strings.TrimRight(l, " \t\r")
		l = strings.TrimPrefix(l, "\t")
		tabs := len(l) - len(strings.TrimLeft(l, "\t"))
		lines[i] = strings.Repeat("    ", tabs) + l[tabs:]
	}
	for len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
