package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile writes a Go source file under dir and returns its path.
func writeFile(t *testing.T, dir, name, src string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// runIndex runs the command over the given paths and returns its streams.
func runIndex(t *testing.T, mode string, paths ...string) (code int, stdout, stderr string) {
	t.Helper()
	in := strings.Join(paths, "\x00") + "\x00"
	var out, errOut bytes.Buffer
	code = run([]string{mode}, strings.NewReader(in), &out, &errOut)
	return code, out.String(), errOut.String()
}

const symbolsSrc = `package a

/*
func CreateGoneFromComment() {}
*/

var example = ` + "`\nfunc CreateGoneFromString() {}\n`" + `

func CreatePlain() {}
func (r *Recv) CreatePtr() {}
func (Recv) CreateBare() {}
func (g *Gen[T]) CreateGeneric() {}
func (g Gen[K, V]) CreateGenericPair() {}
func (r (*Recv)) CreateParen() {}
func CreateGenericFunc[T any](v T) T { return v }
func (r *Recv) unexported() {}
func helper() {}

const (
	SetDefault = "x"
	hidden     = 1
)

var AddOne, addTwo = 1, 2

type SetOptions struct{}
`

func TestSymbols(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "a.go", symbolsSrc)
	code, out, errOut := runIndex(t, "symbols", path)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	want := []string{
		"AddOne -",
		"CreateBare Recv",
		"CreateGeneric Gen",
		"CreateGenericFunc -",
		"CreateGenericPair Gen",
		"CreateParen Recv",
		"CreatePlain -",
		"CreatePtr Recv",
		"SetDefault -",
		"SetOptions -",
	}
	for i, row := range want {
		want[i] = dir + " " + row
	}
	if got := strings.Split(strings.TrimSuffix(out, "\n"), "\n"); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("symbols:\n got %q\nwant %q", got, want)
	}
}

const typesSrc = `package a

type Recv struct{}
type Gen[K comparable, V any] struct{}
type Alias = Recv
type Iface interface{ CreateX() }
type hidden struct{}

type (
	Grouped      struct{}
	GroupedAlias = Recv
)

type Slice[T ~[]E, E any] struct{}

/*
type CommentGhost struct{}
*/
`

func TestTypes(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "types.go", typesSrc)
	code, out, errOut := runIndex(t, "types", path)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	want := strings.Join([]string{
		dir + " Gen",
		dir + " Grouped",
		dir + " Recv",
		dir + " Slice",
	}, "\n") + "\n"
	if out != want {
		t.Errorf("types:\n got %q\nwant %q", out, want)
	}
}

// A receiver shape the parser accepts and the type checker rejects must not
// read as a plain function, nor resolve against a real type.
func TestMalformedReceivers(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "bad.go", `package a

func () CreateNoRecv() {}
func (a A, b B) CreateTwoRecv() {}
func (r pkg.T) CreateQualified() {}
`)
	code, out, errOut := runIndex(t, "symbols", path)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	want := dir + " CreateNoRecv ?\n" +
		dir + " CreateQualified ?*ast.SelectorExpr\n" +
		dir + " CreateTwoRecv ?\n"
	if out != want {
		t.Errorf("malformed receivers:\n got %q\nwant %q", out, want)
	}
}

func TestParseErrorFails(t *testing.T) {
	dir := t.TempDir()
	good := writeFile(t, dir, "good.go", "package a\n\nfunc CreateGood() {}\n")
	bad := writeFile(t, dir, "bad.go", "package a\n\nfunc CreateBroken( {\n")
	for _, mode := range []string{"symbols", "types"} {
		code, out, errOut := runIndex(t, mode, good, bad)
		if code != 1 {
			t.Errorf("%s: exit %d, want 1", mode, code)
		}
		if out != "" {
			t.Errorf("%s: printed a partial index %q", mode, out)
		}
		if !strings.Contains(errOut, "parsing "+bad) {
			t.Errorf("%s: stderr %q does not name the file", mode, errOut)
		}
	}
}

func TestMissingFileFails(t *testing.T) {
	code, _, errOut := runIndex(t, "symbols", filepath.Join(t.TempDir(), "absent.go"))
	if code != 1 || !strings.Contains(errOut, "absent.go") {
		t.Errorf("exit %d, stderr %q", code, errOut)
	}
}

func TestUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"funcs"}, {"symbols", "types"}} {
		var out, errOut bytes.Buffer
		if code := run(args, strings.NewReader(""), &out, &errOut); code != 2 {
			t.Errorf("%q: exit %d, want 2", args, code)
		}
		if !strings.Contains(errOut.String(), "usage:") {
			t.Errorf("%q: stderr %q", args, errOut.String())
		}
	}
}

func TestEmptyInput(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"symbols"}, strings.NewReader(""), &out, &errOut); code != 0 || out.Len() != 0 {
		t.Errorf("exit %d, stdout %q, stderr %q", code, out.String(), errOut.String())
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read refused") }

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write refused") }

func TestIOErrorsFail(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"symbols"}, failingReader{}, &out, &errOut); code != 1 ||
		!strings.Contains(errOut.String(), "read refused") {
		t.Errorf("read: exit %d, stderr %q", code, errOut.String())
	}

	path := writeFile(t, t.TempDir(), "a.go", "package a\n\nfunc CreateA() {}\n")
	errOut.Reset()
	if code := run([]string{"symbols"}, strings.NewReader(path+"\x00"), failingWriter{}, &errOut); code != 1 ||
		!strings.Contains(errOut.String(), "write refused") {
		t.Errorf("write: exit %d, stderr %q", code, errOut.String())
	}
}
