package io

import (
	stderrors "errors"
	"os"
	"path/filepath"
	"reflect"
	goruntime "runtime"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/kubernetes"
)

// ciliumPolicy is a CiliumNetworkPolicy with one ICMP field. A field that has
// no type, or a null one, makes the UnmarshalJSON of the upstream type
// dereference a nil pointer.
func ciliumPolicy(name, field string) string {
	return `{"apiVersion":"cilium.io/v2","kind":"CiliumNetworkPolicy","metadata":{"name":"` + name + `","namespace":"default"},` +
		`"spec":{"endpointSelector":{},"ingress":[{"icmps":[{"fields":[` + field + `]}]}]}}`
}

const (
	icmpNoType   = `{"family":"IPv4"}`
	icmpNullType = `{"family":"IPv4","type":null}`
	icmpTyped    = `{"family":"IPv4","type":8}`
)

func configMapDoc(name string) string {
	return `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"` + name + `"}}`
}

// parsers are the two ways into the parser: bytes, and a file.
var parsers = []struct {
	name  string
	parse func(t *testing.T, doc string, opts ParseOptions) ([]client.Object, error)
}{
	{"ParseYAML", func(_ *testing.T, doc string, opts ParseOptions) ([]client.Object, error) {
		return ParseYAMLWithOptions([]byte(doc), opts)
	}},
	{"ParseFile", func(t *testing.T, doc string, opts ParseOptions) ([]client.Object, error) {
		path := filepath.Join(t.TempDir(), "doc.yaml")
		if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
			t.Fatal(err)
		}
		return ParseFileWithOptions(path, opts)
	}},
}

// parseErrorsOf returns the errors a parse reported, each of which must be a
// parse error.
func parseErrorsOf(t *testing.T, err error) []error {
	t.Helper()
	var all *errors.ParseErrors
	if !stderrors.As(err, &all) {
		t.Fatalf("the error is %T, not the parser's ParseErrors: %v", err, err)
	}
	for _, e := range all.Errors {
		var pe *errors.ParseError
		if !stderrors.As(e, &pe) {
			t.Errorf("the error is %T, not a ParseError: %v", e, e)
		}
	}
	return all.Errors
}

// A registered type's UnmarshalJSON runs inside the scheme's deserializer. One
// that panics must come back as a parse error that names what was being
// decoded, in every position a document can have.
func TestParse_DecoderPanicIsAnError(t *testing.T) {
	const policy = `CiliumNetworkPolicy "default/p"`
	for _, tc := range []struct {
		name string
		doc  string
		opts ParseOptions
		want []string // what the one error must contain
	}{
		{"a document, type absent", ciliumPolicy("p", icmpNoType), ParseOptions{}, []string{policy}},
		{"a document, type null", ciliumPolicy("p", icmpNullType), ParseOptions{}, []string{policy}},
		{"a document, unstructured allowed", ciliumPolicy("p", icmpNoType), ParseOptions{AllowUnstructured: true}, []string{policy}},
		{
			"an item of a v1 List",
			`{"apiVersion":"v1","kind":"List","items":[` + ciliumPolicy("p", icmpNoType) + `]}`,
			ParseOptions{},
			[]string{policy, "item 0 of List"},
		},
		{
			"an item of a typed list",
			`{"apiVersion":"cilium.io/v2","kind":"CiliumNetworkPolicyList","items":[` + ciliumPolicy("p", icmpNoType) + `]}`,
			ParseOptions{},
			[]string{policy, "item 0 of CiliumNetworkPolicyList"},
		},
		{
			"an item of a typed list that leaves its kind out",
			`{"apiVersion":"cilium.io/v2","kind":"CiliumNetworkPolicyList","items":[` +
				`{"metadata":{"name":"p","namespace":"default"},"spec":{"endpointSelector":{},"ingress":[{"icmps":[{"fields":[` + icmpNoType + `]}]}]}}]}`,
			ParseOptions{},
			[]string{policy, "item 0 of CiliumNetworkPolicyList"},
		},
	} {
		for _, p := range parsers {
			t.Run(tc.name+", "+p.name, func(t *testing.T) {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("the parser panicked: %v", r)
					}
				}()
				objs, err := p.parse(t, tc.doc, tc.opts)
				if err == nil {
					t.Fatalf("a field without a type decoded without an error: %d objects", len(objs))
				}
				if len(objs) != 0 {
					t.Errorf("%d objects returned beside the error", len(objs))
				}
				errs := parseErrorsOf(t, err)
				if len(errs) != 1 {
					t.Fatalf("%d errors, want 1: %v", len(errs), err)
				}
				for _, want := range append(tc.want, "the decoder panicked", "nil pointer dereference") {
					if !strings.Contains(errs[0].Error(), want) {
						t.Errorf("the error does not say %q: %v", want, errs[0])
					}
				}
			})
		}
	}
}

// The message is part of what a caller reads: the position of an item first,
// then the object, then the panic's own value.
func TestParse_DecoderPanicMessage(t *testing.T) {
	const panicked = `the decoder panicked on CiliumNetworkPolicy "default/p": ` +
		`runtime error: invalid memory address or nil pointer dereference`
	for _, tc := range []struct{ name, doc, want string }{
		{
			"a document",
			ciliumPolicy("p", icmpNoType),
			"parse error in Kubernetes object: failed to decode object: " + panicked,
		},
		{
			"an item of a v1 List",
			`{"apiVersion":"v1","kind":"List","items":[` + ciliumPolicy("p", icmpNoType) + `]}`,
			"parse error in Kubernetes object: item 0 of List: " +
				"parse error in Kubernetes object: failed to decode object: " + panicked,
		},
		{
			"an item of a typed list",
			`{"apiVersion":"cilium.io/v2","kind":"CiliumNetworkPolicyList","items":[` + ciliumPolicy("p", icmpNoType) + `]}`,
			"parse error in Kubernetes object: item 0 of CiliumNetworkPolicyList: " + panicked,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseYAML([]byte(tc.doc))
			if err == nil || err.Error() != tc.want {
				t.Errorf("got  %v\nwant %s", err, tc.want)
			}
			var runtimeErr goruntime.Error
			if !stderrors.As(err, &runtimeErr) {
				t.Errorf("the panic's own error is not in the chain of %v", err)
			}
		})
	}
}

// A panic's value need not be an error, and the object it happened on need not
// state everything an error would name it by.
func TestDecoderPanic(t *testing.T) {
	deployment := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	sentinel := stderrors.New("boom")
	for _, tc := range []struct {
		name  string
		value any
		raw   string
		kind  *schema.GroupVersionKind
		want  string
	}{
		{"an error value", sentinel, `{"kind":"Pod","metadata":{"name":"web","namespace":"default"}}`, nil,
			`the decoder panicked on Pod "default/web": boom`},
		{"a value that is not an error", "out of range", `{"kind":"Pod","metadata":{"name":"web"}}`, nil,
			`the decoder panicked on Pod "web": out of range`},
		{"no name", 42, `{"kind":"Pod","metadata":{}}`, nil,
			`the decoder panicked on Pod: 42`},
		{"no kind, the list's stands in", "x", `{"metadata":{"name":"web"}}`, &deployment,
			`the decoder panicked on Deployment "web": x`},
		{"a stated kind wins over the list's", "x", `{"kind":"Pod","metadata":{"name":"web"}}`, &deployment,
			`the decoder panicked on Pod "web": x`},
		{"no kind at all", "x", `{"metadata":{"name":"web"}}`, nil,
			`the decoder panicked on the object "web": x`},
		{"a key that differs in case only is not read", "x", `{"Kind":"Pod","metadata":{"Name":"web"}}`, nil,
			`the decoder panicked on the object: x`},
		{"values of another type are left out", "x", `{"kind":7,"metadata":{"name":["web"],"namespace":{}}}`, nil,
			`the decoder panicked on the object: x`},
		{"metadata that is no object", "x", `{"kind":"Pod","metadata":"web"}`, nil,
			`the decoder panicked on Pod: x`},
		{"not an object", "x", `[1,2]`, nil,
			`the decoder panicked on the object: x`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := decoderPanic(tc.value, []byte(tc.raw), tc.kind)
			if err.Error() != tc.want {
				t.Errorf("got  %v\nwant %s", err, tc.want)
			}
			if valueErr, isErr := tc.value.(error); isErr && !stderrors.Is(err, valueErr) {
				t.Errorf("the panic's error is not in the chain of %v", err)
			}
		})
	}
}

// After a recovered panic the decoder returns the error alone: no object and no
// kind a caller could go on with. A decode that does not panic returns what the
// deserializer returned.
func TestDecodeRegistered_PanicReturnsTheErrorAlone(t *testing.T) {
	// A parse registers the schemes before it decodes; this test decodes
	// without a parse.
	if err := kubernetes.RegisterSchemes(); err != nil {
		t.Fatal(err)
	}
	policyKind := schema.GroupVersionKind{Group: "cilium.io", Version: "v2", Kind: "CiliumNetworkPolicy"}
	for _, tc := range []struct {
		name string
		kind *schema.GroupVersionKind
	}{
		{"a document", nil},
		{"a typed-list item", &policyKind},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obj, actual, err := decodeRegistered([]byte(ciliumPolicy("p", icmpNoType)), tc.kind)
			if err == nil || !strings.Contains(err.Error(), "the decoder panicked") {
				t.Fatalf("want the panic as an error, got %v", err)
			}
			if obj != nil {
				t.Errorf("an object is returned beside the error: %T", obj)
			}
			if actual != nil {
				t.Errorf("a kind is returned beside the error: %v", actual)
			}

			typed := []byte(ciliumPolicy("p", icmpTyped))
			wantObj, wantKind, err := kubernetes.Codecs.UniversalDeserializer().Decode(typed, tc.kind, nil)
			if err != nil {
				t.Fatal(err)
			}
			if wantKind == nil || *wantKind != policyKind || wantObj.(client.Object).GetName() != "p" {
				t.Fatalf("the deserializer returned %T and %v, not the policy", wantObj, wantKind)
			}
			obj, actual, err = decodeRegistered(typed, tc.kind)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(obj, wantObj) || !reflect.DeepEqual(actual, wantKind) {
				t.Errorf("got  %#v and %v\nwant %#v and %v, what the deserializer returns", obj, actual, wantObj, wantKind)
			}
		})
	}
}

// The panic of one document or item costs only that one: what stands beside it
// is still returned, and the same kind decodes when its field has a type.
func TestParse_DecoderPanicLeavesTheRest(t *testing.T) {
	for _, p := range parsers {
		t.Run("documents of a stream, "+p.name, func(t *testing.T) {
			doc := configMapDoc("before") + "\n---\n" + ciliumPolicy("p", icmpNoType) + "\n---\n" + configMapDoc("after") + "\n"
			objs, err := p.parse(t, doc, ParseOptions{})
			if errs := parseErrorsOf(t, err); len(errs) != 1 || !strings.Contains(errs[0].Error(), "the decoder panicked") {
				t.Errorf("want the one panic reported, got %v", err)
			}
			if len(objs) != 2 || objs[0].GetName() != "before" || objs[1].GetName() != "after" {
				t.Errorf("want the documents before and after, got %d objects", len(objs))
			}
		})

		t.Run("items of a list, "+p.name, func(t *testing.T) {
			doc := `{"apiVersion":"v1","kind":"List","items":[` +
				configMapDoc("before") + `,` + ciliumPolicy("p", icmpNoType) + `,` + configMapDoc("after") + `]}`
			objs, err := p.parse(t, doc, ParseOptions{})
			errs := parseErrorsOf(t, err)
			if len(errs) != 1 || !strings.Contains(errs[0].Error(), "item 1 of List") || !strings.Contains(errs[0].Error(), "the decoder panicked") {
				t.Errorf("want the one panic reported for item 1, got %v", err)
			}
			if len(objs) != 2 || objs[0].GetName() != "before" || objs[1].GetName() != "after" {
				t.Errorf("want the items before and after, got %d objects", len(objs))
			}
		})

		t.Run("items of a typed list, "+p.name, func(t *testing.T) {
			doc := `{"apiVersion":"cilium.io/v2","kind":"CiliumNetworkPolicyList","items":[` +
				ciliumPolicy("before", icmpTyped) + `,` + ciliumPolicy("p", icmpNoType) + `,` + ciliumPolicy("after", icmpTyped) + `]}`
			objs, err := p.parse(t, doc, ParseOptions{})
			errs := parseErrorsOf(t, err)
			if len(errs) != 1 || !strings.Contains(errs[0].Error(), "item 1 of CiliumNetworkPolicyList") || !strings.Contains(errs[0].Error(), "the decoder panicked") {
				t.Errorf("want the one panic reported for item 1, got %v", err)
			}
			if len(objs) != 2 || objs[0].GetName() != "before" || objs[1].GetName() != "after" {
				t.Errorf("want the items before and after, got %d objects", len(objs))
			}
		})

		t.Run("the same kind with a type, "+p.name, func(t *testing.T) {
			objs, err := p.parse(t, ciliumPolicy("p", icmpTyped), ParseOptions{})
			if err != nil {
				t.Fatal(err)
			}
			if len(objs) != 1 || objs[0].GetName() != "p" {
				t.Errorf("want the policy, got %d objects", len(objs))
			}
		})
	}
}
