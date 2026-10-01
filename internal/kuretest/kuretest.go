// Package kuretest holds the test helpers that hold kure's output to what a
// cluster would accept: every object is validated, as the YAML a user would
// apply, against the CustomResourceDefinitions of the exact module versions
// the kinds registry pins, with the API server's own validation routines
// ([crdvalidate]). A kind whose module ships no definition gets the
// server's ObjectMeta validation and nothing more; TestEveryKindHasOneSource
// names each such kind and why.
//
// It is for tests only: it registers the -update flag and reads the module
// cache through go list.
package kuretest

import (
	"bytes"
	"context"
	stderrors "errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/internal/crdvalidate"
	"github.com/go-kure/kure/pkg/errors"
	kureio "github.com/go-kure/kure/pkg/io"
	"github.com/go-kure/kure/pkg/kubernetes"
)

var update = flag.Bool("update", false, "rewrite golden files with the current output")

// kustomizeGroup is the API group of kustomization.yaml. It is kustomize's
// configuration, not an object a cluster accepts, and a generated layout
// carries one per directory.
const kustomizeGroup = "kustomize.config.k8s.io"

// Golden encodes obj the way the golden fixtures are written, validates the
// result, then compares it with testdata/filename — or rewrites the file
// under -update. Validation comes first, so -update can never write a
// fixture a cluster would reject.
func Golden(t testing.TB, filename string, obj client.Object) {
	t.Helper()
	got, err := kureio.EncodeObjectsToYAMLWithOptions([]*client.Object{&obj}, kureio.EncodeOptions{KubernetesFieldOrder: true})
	if err != nil {
		t.Fatalf("encoding to YAML: %v", err)
	}
	AssertValidYAML(t, got)

	golden := filepath.Join("testdata", filename)
	if *update {
		if err := os.WriteFile(golden, got, 0o644); err != nil { //nolint:gosec // a test fixture, world-readable by design
			t.Fatalf("updating golden file: %v", err)
		}
		return
	}
	want, err := os.ReadFile(golden) //nolint:gosec // a test fixture named by the test
	if err != nil {
		t.Fatalf("reading golden file (run with -update to create): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output does not match golden file %s\n\ngot:\n%s\nwant:\n%s", golden, got, want)
	}
}

// AssertValid encodes the objects with kure's default options and validates
// the YAML.
func AssertValid(t testing.TB, objs ...client.Object) {
	t.Helper()
	ptrs := make([]*client.Object, len(objs))
	for i := range objs {
		ptrs[i] = &objs[i]
	}
	data, err := kureio.EncodeObjectsToYAML(ptrs)
	if err != nil {
		t.Fatalf("encoding to YAML: %v", err)
	}
	AssertValidYAML(t, data)
}

// AssertValidYAML validates every document in data and fails the test with
// all of the findings at once.
func AssertValidYAML(t testing.TB, data []byte) {
	t.Helper()
	findings, err := validateYAML(data)
	if err != nil {
		t.Fatalf("kuretest: %v", err)
	}
	if len(findings) > 0 {
		t.Fatalf("%s", format(findings))
	}
}

// AssertValidDir validates every .yaml and .yml file under dir. A directory
// with no manifests fails: there is nothing a pass could be about.
func AssertValidDir(t testing.TB, dir string) {
	t.Helper()
	var all []finding
	files := 0
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || (filepath.Ext(path) != ".yaml" && filepath.Ext(path) != ".yml") {
			return nil
		}
		files++
		data, err := os.ReadFile(path) //nolint:gosec // a test output file under the directory named
		if err != nil {
			return err
		}
		findings, err := validateYAML(data)
		if err != nil {
			return errors.Wrap(err, path)
		}
		for _, f := range findings {
			f.doc = path + ": " + f.doc
			all = append(all, f)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("kuretest: %v", err)
	}
	if files == 0 {
		t.Fatalf("kuretest: no manifests under %s", dir)
	}
	if len(all) > 0 {
		t.Fatalf("%s", format(all))
	}
}

// finding is one document's errors.
type finding struct {
	doc  string
	errs field.ErrorList
}

func format(findings []finding) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d object(s) a cluster would reject:\n", len(findings))
	for _, f := range findings {
		fmt.Fprintf(&b, "  %s\n", f.doc)
		for _, e := range f.errs {
			fmt.Fprintf(&b, "    %s\n", e)
		}
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// validateYAML decodes data as the server decodes a request body and
// validates each object. The error is for what stops validation itself: a
// document that does not decode, or a validator that cannot be built.
func validateYAML(data []byte) ([]finding, error) {
	objs, err := decode(data)
	if err != nil {
		return nil, err
	}
	var findings []finding
	for i, u := range objs {
		errs, err := validateObject(u)
		if err != nil {
			return nil, errors.Wrapf(err, "object %d", i)
		}
		if len(errs) > 0 {
			findings = append(findings, finding{doc: describe(i, u), errs: errs})
		}
	}
	return findings, nil
}

// describe names an object by its position in decode order (a list counts
// as its items), kind, name and apiVersion.
func describe(i int, u *unstructured.Unstructured) string {
	name := u.GetName()
	if ns := u.GetNamespace(); ns != "" {
		name = ns + "/" + name
	}
	return fmt.Sprintf("object %d: %s %s (%s)", i, u.GetKind(), name, u.GetAPIVersion())
}

// decode is the unstructured path of pkg/io: every document, through the
// server's own JSON decoder, with a list flattened to its items.
func decode(data []byte) ([]*unstructured.Unstructured, error) {
	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	var out []*unstructured.Unstructured
	for i := 0; ; i++ {
		var raw runtime.RawExtension
		if err := decoder.Decode(&raw); err != nil {
			if stderrors.Is(err, io.EOF) {
				return out, nil
			}
			return nil, errors.Wrapf(err, "document %d", i)
		}
		if len(bytes.TrimSpace(raw.Raw)) == 0 {
			continue
		}
		obj, _, err := unstructured.UnstructuredJSONScheme.Decode(raw.Raw, nil, nil)
		if err != nil {
			return nil, errors.Wrapf(err, "document %d", i)
		}
		switch o := obj.(type) {
		case *unstructured.UnstructuredList:
			for j := range o.Items {
				out = append(out, &o.Items[j])
			}
		case *unstructured.Unstructured:
			out = append(out, o)
		default:
			return nil, errors.Errorf("document %d decoded to %T", i, obj)
		}
	}
}

// validateObject holds one object to its kind's source: the pinned
// definition, or ObjectMeta alone for a kind that has none. A kind kure does
// not register is a finding: kure cannot have produced it on purpose.
func validateObject(u *unstructured.Unstructured) (field.ErrorList, error) {
	gvk := u.GroupVersionKind()
	if gvk.Group == kustomizeGroup {
		return nil, nil
	}
	info, ok := kubernetes.KindFor(u.GetAPIVersion(), u.GetKind())
	if !ok {
		return field.ErrorList{field.Invalid(field.NewPath("kind"), u.GetKind(), fmt.Sprintf("%s is not a kind kure registers", gvk))}, nil
	}
	src, ok := modules[info.Module]
	if !ok {
		return nil, errors.Errorf("%s comes from %s, which the source table does not name", gvk, info.Module)
	}
	if src.Uncovered != "" {
		return crdvalidate.ValidateObjectMeta(context.Background(), u, info.Namespaced), nil
	}
	v, err := validator()
	if err != nil {
		return nil, err
	}
	return v.Validate(context.Background(), u)
}
