package io

import (
	"bytes"
	stderrors "errors"
	"io"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// countedDecoder is the stream decoder of a parse, allowed a number of calls.
// A parse that keeps calling it is stopped by a panic the test recovers, so a
// parse that does not end fails its test and takes nothing else down.
type countedDecoder struct {
	inner documentDecoder
	calls int
	limit int
}

// runawayParse is the value countedDecoder panics with.
type runawayParse struct{}

func (d *countedDecoder) Decode(into any) error {
	d.calls++
	if d.calls > d.limit {
		panic(runawayParse{})
	}
	return d.inner.Decode(into)
}

// parseCounted parses doc as ParseYAML does, through a decoder that is allowed
// a hundred calls, many times what any stream of these tests needs.
func parseCounted(t *testing.T, doc string) ([]client.Object, error) {
	t.Helper()
	decoder := &countedDecoder{
		inner: yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(doc)), 4096),
		limit: 100,
	}
	defer func() {
		if r := recover(); r != nil {
			if _, runaway := r.(runawayParse); !runaway {
				panic(r)
			}
			t.Fatalf("the parse did not end: it called the decoder more than %d times", decoder.limit)
		}
	}()
	return parseStream(decoder, ParseOptions{})
}

func yamlConfigMap(name string) string {
	return "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: " + name + "\n"
}

const (
	// malformedJSON opens an object and closes an array.
	malformedJSON = "{]\n"
	// truncatedJSON ends inside an object.
	truncatedJSON = `{"apiVersion":"v1","kind":` + "\n"
	// malformedYAML opens a flow sequence and does not close it.
	malformedYAML = "key: [unclosed\n"

	jsonSyntax = "invalid character ']'"
)

// streamCases are streams with a place the decoder reports an error for. The
// first group are streams it cannot move past: the parse ends there. The
// second group are YAML documents that do not parse: the decoder goes on to
// the document after them.
var streamCases = []struct {
	name     string
	doc      string
	want     []string   // the names of the objects returned, in order
	wantErrs [][]string // per error, what it must contain
}{
	{
		name:     "malformed JSON alone",
		doc:      malformedJSON,
		wantErrs: [][]string{{jsonSyntax}},
	},
	{
		name:     "one JSON document, then malformed JSON",
		doc:      configMapDoc("a") + "\n" + malformedJSON,
		want:     []string{"a"},
		wantErrs: [][]string{{jsonSyntax}},
	},
	{
		// Without the newline the decoder is left with no reader after the
		// JSON error; with it, the call after that error is the end of the
		// stream.
		name:     "one JSON document, then malformed JSON that ends the stream without a newline",
		doc:      configMapDoc("a") + "\n" + strings.TrimSuffix(malformedJSON, "\n"),
		want:     []string{"a"},
		wantErrs: [][]string{{jsonSyntax}},
	},
	{
		name:     "two JSON documents, then malformed JSON",
		doc:      configMapDoc("a") + "\n" + configMapDoc("b") + "\n" + malformedJSON,
		want:     []string{"a", "b"},
		wantErrs: [][]string{{jsonSyntax}},
	},
	{
		name:     "three JSON documents, malformed JSON, then a document that is not reached",
		doc:      configMapDoc("a") + "\n" + configMapDoc("b") + "\n" + configMapDoc("c") + "\n" + malformedJSON + configMapDoc("d") + "\n",
		want:     []string{"a", "b", "c"},
		wantErrs: [][]string{{jsonSyntax}},
	},
	{
		name:     "one JSON document, malformed JSON, then a document that is not reached",
		doc:      configMapDoc("a") + "\n" + malformedJSON + configMapDoc("d") + "\n",
		want:     []string{"a"},
		wantErrs: [][]string{{jsonSyntax}},
	},
	{
		// After two JSON documents the decoder reads JSON only, and a
		// separator line is not JSON.
		name:     "two JSON documents, a separator line, then a document that is not reached",
		doc:      configMapDoc("a") + "\n" + configMapDoc("b") + "\n---\n" + configMapDoc("d") + "\n",
		want:     []string{"a", "b"},
		wantErrs: [][]string{{"invalid character '-'"}},
	},
	{
		name:     "malformed JSON, then text",
		doc:      malformedJSON + "some more text here\n",
		wantErrs: [][]string{{jsonSyntax}},
	},
	{
		name:     "truncated JSON alone",
		doc:      truncatedJSON,
		wantErrs: [][]string{{"unexpected EOF"}},
	},
	{
		name:     "two JSON documents, then truncated JSON",
		doc:      configMapDoc("a") + "\n" + configMapDoc("b") + "\n" + truncatedJSON,
		want:     []string{"a", "b"},
		wantErrs: [][]string{{"unexpected EOF"}},
	},

	{
		name:     "a YAML document that does not parse, between two that do",
		doc:      yamlConfigMap("a") + "---\n" + malformedYAML + "---\n" + yamlConfigMap("b"),
		want:     []string{"a", "b"},
		wantErrs: [][]string{{"yaml:"}},
	},
	{
		name:     "two YAML documents that do not parse, between two that do",
		doc:      yamlConfigMap("a") + "---\n" + malformedYAML + "---\n" + malformedYAML + "---\n" + yamlConfigMap("b"),
		want:     []string{"a", "b"},
		wantErrs: [][]string{{"yaml:"}, {"yaml:"}},
	},
	{
		name:     "a YAML document that does not parse, last",
		doc:      yamlConfigMap("a") + "---\n" + malformedYAML,
		want:     []string{"a"},
		wantErrs: [][]string{{"yaml:"}},
	},
	{
		// JSON documents with a separator line between them are YAML
		// documents to the decoder, and so is the malformed one.
		name:     "malformed JSON among JSON documents with a separator line between them",
		doc:      configMapDoc("a") + "\n---\n" + configMapDoc("b") + "\n---\n" + malformedJSON + "---\n" + configMapDoc("c") + "\n",
		want:     []string{"a", "b", "c"},
		wantErrs: [][]string{{"yaml:"}},
	},
	{
		// After one JSON document the decoder can still fall back to its
		// YAML reader, which reads on from the separator line.
		name:     "one JSON document, malformed JSON, a separator line, then a document",
		doc:      configMapDoc("a") + "\n" + malformedJSON + "---\n" + configMapDoc("c") + "\n",
		want:     []string{"a", "c"},
		wantErrs: [][]string{{jsonSyntax}},
	},
	{
		name:     "malformed JSON, then YAML documents",
		doc:      malformedJSON + "---\n" + malformedYAML + "---\n" + yamlConfigMap("b"),
		want:     []string{"b"},
		wantErrs: [][]string{{jsonSyntax}, {"yaml:"}},
	},
}

// checkStream compares what a parse returned with what a stream case wants.
func checkStream(t *testing.T, objs []client.Object, err error, want []string, wantErrs [][]string) {
	t.Helper()
	names := make([]string, 0, len(objs))
	for _, obj := range objs {
		names = append(names, obj.GetName())
	}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("objects %v, want %v", names, want)
	}
	errs := parseErrorsOf(t, err)
	if len(errs) != len(wantErrs) {
		t.Fatalf("%d errors, want %d: %v", len(errs), len(wantErrs), err)
	}
	for i, contains := range wantErrs {
		for _, s := range append([]string{"failed to decode document"}, contains...) {
			if !strings.Contains(errs[i].Error(), s) {
				t.Errorf("error %d does not say %q: %v", i, s, errs[i])
			}
		}
	}
}

// scriptedDecoder returns its steps in order, one per call: an error, or the
// JSON of a document. Past the last step it returns that step again, as a
// decoder does that has stopped moving; a script that ends ends with io.EOF.
type scriptedDecoder struct {
	steps []any
	calls int
}

func (d *scriptedDecoder) Decode(into any) error {
	d.calls++
	if d.calls > len(d.steps)+100 {
		panic(runawayParse{})
	}
	step := d.steps[min(d.calls, len(d.steps))-1]
	if err, isErr := step.(error); isErr {
		return err
	}
	into.(*runtime.RawExtension).Raw = []byte(step.(string))
	return nil
}

// yamlDocumentError is the error the stream decoder returns for a YAML
// document that does not parse.
func yamlDocumentError(t *testing.T) error {
	t.Helper()
	var raw runtime.RawExtension
	err := yamlutil.NewYAMLOrJSONDecoder(strings.NewReader(malformedYAML), 4096).Decode(&raw)
	var syntax yamlutil.YAMLSyntaxError
	if !stderrors.As(err, &syntax) {
		t.Fatalf("the decoder returned %T for a YAML document that does not parse: %v", err, err)
	}
	return err
}

// The rule that ends a parse, on a decoder that returns what the test says: a
// second error in a row ends it and is not kept, unless it is the error of a
// YAML document, after which the decoder stands at the next one.
func TestParseStream_EndsWhenTheDecoderStopsMoving(t *testing.T) {
	first, second := stderrors.New("first"), stderrors.New("second")
	yamlErr := yamlDocumentError(t)
	for _, tc := range []struct {
		name     string
		steps    []any
		want     []string
		wantErrs [][]string
		calls    int
	}{
		{"the same error on every call", []any{first}, nil, [][]string{{"first"}}, 2},
		{"another error on every further call", []any{first, second}, nil, [][]string{{"first"}}, 2},
		{"an error, then the end", []any{first, io.EOF}, nil, [][]string{{"first"}}, 2},
		{
			"an error on either side of a document",
			[]any{first, configMapDoc("a"), second, io.EOF},
			[]string{"a"},
			[][]string{{"first"}, {"second"}},
			4,
		},
		{
			"a YAML document's error, then an error on every call",
			[]any{yamlErr, first},
			nil,
			[][]string{{"yaml:"}, {"first"}},
			3,
		},
		{
			"an error, then YAML documents' errors, then a document",
			[]any{first, yamlErr, yamlErr, configMapDoc("a"), io.EOF},
			[]string{"a"},
			[][]string{{"first"}, {"yaml:"}, {"yaml:"}},
			5,
		},
		{
			"an error, a YAML document's error, then an error on every call",
			[]any{first, yamlErr, second},
			nil,
			[][]string{{"first"}, {"yaml:"}, {"second"}},
			4,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decoder := &scriptedDecoder{steps: tc.steps}
			defer func() {
				if r := recover(); r != nil {
					if _, runaway := r.(runawayParse); !runaway {
						panic(r)
					}
					t.Fatalf("the parse did not end: %d calls of the decoder", decoder.calls)
				}
			}()
			objs, err := parseStream(decoder, ParseOptions{})
			checkStream(t, objs, err, tc.want, tc.wantErrs)
			if decoder.calls != tc.calls {
				t.Errorf("%d calls of the decoder, want %d", decoder.calls, tc.calls)
			}
		})
	}
}

// A stream the decoder cannot move past ends the parse, with one error for the
// place and the objects decoded before it. A YAML document that does not parse
// costs only itself. Each stream is parsed through a counted decoder first;
// only a parse that ended there is run through the parsers, which have nothing
// to stop them.
func TestParse_StreamErrors(t *testing.T) {
	for _, tc := range streamCases {
		t.Run(tc.name, func(t *testing.T) {
			objs, err := parseCounted(t, tc.doc)
			checkStream(t, objs, err, tc.want, tc.wantErrs)
			if t.Failed() {
				return
			}
			for _, p := range parsers {
				t.Run(p.name, func(t *testing.T) {
					objs, err := p.parse(t, tc.doc, ParseOptions{})
					checkStream(t, objs, err, tc.want, tc.wantErrs)
				})
			}
		})
	}
}
