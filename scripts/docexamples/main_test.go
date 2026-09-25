package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// exampleSrc declares two Examples in pkg/x: one with an output comment, a
// nested block and a blank line, and one without.
const exampleSrc = `package x_test

import "fmt"

func ExampleNew() {
	v := 1
	if v > 0 {
		fmt.Println(v)
	}

	fmt.Println("done")
	// Output:
	// 1
	// done
}

func ExampleOther() {
	fmt.Println("other")
}
`

const generatedNew = "```go\n" +
	"v := 1\n" +
	"if v > 0 {\n" +
	"    fmt.Println(v)\n" +
	"}\n" +
	"\n" +
	"fmt.Println(\"done\")\n" +
	"```\n"

// tree writes pkg/x/example_test.go and the given page into a fresh directory,
// makes it the working directory, and returns the page's path.
func tree(t *testing.T, page string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "pkg", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg", "x", "example_test.go"), []byte(exampleSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "page.md"), []byte(page), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	return "page.md"
}

// runMode runs the command in mode over page and returns the exit status,
// stderr and the page as it is afterwards.
func runMode(t *testing.T, mode, page string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := run([]string{mode, page}, &out, &errOut)
	after, err := os.ReadFile(page)
	if err != nil {
		t.Fatal(err)
	}
	return code, errOut.String(), string(after)
}

// TestGenerate: generate fills an empty region with the Example's body, less
// its output comment, one tab of indentation, and with the remaining tabs as
// four spaces; a second run changes nothing, and check then passes.
func TestGenerate(t *testing.T) {
	page := tree(t, "# Page\n\n<!-- doc-example: pkg/x ExampleNew -->\n<!-- doc-example:end -->\n\nAfter.\n")
	code, stderr, after := runMode(t, "generate", page)
	if code != 0 {
		t.Fatalf("generate: exit %d, stderr:\n%s", code, stderr)
	}
	want := "# Page\n\n<!-- doc-example: pkg/x ExampleNew -->\n" + generatedNew + "<!-- doc-example:end -->\n\nAfter.\n"
	if after != want {
		t.Fatalf("generated page:\n%s\nwant:\n%s", after, want)
	}
	if code, stderr, again := runMode(t, "generate", page); code != 0 || again != want {
		t.Fatalf("second generate: exit %d, stderr %q, page changed: %v", code, stderr, again != want)
	}
	if code, stderr, _ := runMode(t, "check", page); code != 0 {
		t.Fatalf("check after generate: exit %d, stderr:\n%s", code, stderr)
	}
}

// TestGenerate_Indented: a marker inside a list item gives the generated
// block its indentation; blank lines stay empty.
func TestGenerate_Indented(t *testing.T) {
	page := tree(t, "- item\n\n  <!-- doc-example: pkg/x ExampleNew -->\n  <!-- doc-example:end -->\n")
	if code, stderr, _ := runMode(t, "generate", page); code != 0 {
		t.Fatalf("generate: exit %d, stderr:\n%s", code, stderr)
	}
	after, _ := os.ReadFile(page)
	var want strings.Builder
	want.WriteString("- item\n\n  <!-- doc-example: pkg/x ExampleNew -->\n")
	for _, l := range strings.SplitAfter(strings.TrimSuffix(generatedNew, "\n"), "\n") {
		if l != "\n" {
			want.WriteString("  ")
		}
		want.WriteString(l)
	}
	want.WriteString("\n  <!-- doc-example:end -->\n")
	if string(after) != want.String() {
		t.Fatalf("generated page:\n%s\nwant:\n%s", after, want.String())
	}
}

// TestCheck_Failures: every way a page can drift from its Examples, or carry
// a Go block nobody vouches for, fails check and names the line; check
// writes nothing.
func TestCheck_Failures(t *testing.T) {
	cases := map[string]struct {
		page string
		want string
	}{
		"drifted block": {
			page: "<!-- doc-example: pkg/x ExampleOther -->\n```go\nfmt.Println(\"old\")\n```\n<!-- doc-example:end -->\n",
			want: "page.md: a generated block differs from its Example",
		},
		"unmarked go block": {
			page: "Text.\n\n```go\nx := 1\n```\n",
			want: "page.md:3: a ```go block that is neither generated nor marked",
		},
		"excerpt marker without a reason": {
			page: "<!-- doc-example:excerpt -->\n```go\nx := 1\n```\n",
			want: "page.md:1: an excerpt marker needs a reason",
		},
		"excerpt marker not above a fence": {
			page: "<!-- doc-example:excerpt shows a type -->\n\n```go\nx := 1\n```\n",
			want: "page.md:1: an excerpt marker must be directly above a ```go fence",
		},
		"missing Example": {
			page: "<!-- doc-example: pkg/x ExampleGone -->\n<!-- doc-example:end -->\n",
			want: "page.md:1: pkg/x declares no function ExampleGone in its test files",
		},
		"missing end marker": {
			page: "<!-- doc-example: pkg/x ExampleOther -->\n```go\nfmt.Println(\"other\")\n```\n",
			want: "page.md:1: no <!-- doc-example:end --> after this doc-example marker",
		},
		"unrecognised marker": {
			page: "<!-- doc-example pkg/x ExampleOther -->\n",
			want: "page.md:1: unrecognised doc-example marker",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			page := tree(t, tc.page)
			code, stderr, after := runMode(t, "check", page)
			if code == 0 || !strings.Contains(stderr, tc.want) {
				t.Errorf("check: exit %d, stderr %q, want it to contain %q", code, stderr, tc.want)
			}
			if after != tc.page {
				t.Errorf("check changed the page:\n%s", after)
			}
		})
	}
}

// TestCheck_Accepted: an excerpt with a reason, a block in another language,
// a ```go block and a marker inside another fence, and a generated block
// that matches its Example all pass.
func TestCheck_Accepted(t *testing.T) {
	page := tree(t, "<!-- doc-example:excerpt shows a type declaration -->\n```go\ntype T struct{}\n```\n\n"+
		"```bash\ngo test ./...\n```\n\n"+
		"````markdown\n```go\nx := 1\n```\n<!-- doc-example: pkg/x ExampleGone -->\n````\n\n"+
		"<!-- doc-example: pkg/x ExampleOther -->\n```go\nfmt.Println(\"other\")\n```\n<!-- doc-example:end -->\n")
	if code, stderr, _ := runMode(t, "check", page); code != 0 {
		t.Fatalf("check: exit %d, stderr:\n%s", code, stderr)
	}
}

// TestGenerate_UnparseableTestFile: a test file that does not parse is an
// error, not a missing Example.
func TestGenerate_UnparseableTestFile(t *testing.T) {
	page := tree(t, "<!-- doc-example: pkg/x ExampleNew -->\n<!-- doc-example:end -->\n")
	if err := os.WriteFile(filepath.Join("pkg", "x", "broken_test.go"), []byte("package x_test\nfunc {"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stderr, _ := runMode(t, "generate", page)
	if code == 0 || !strings.Contains(stderr, "parsing pkg/x/broken_test.go") {
		t.Fatalf("generate: exit %d, stderr %q", code, stderr)
	}
}
