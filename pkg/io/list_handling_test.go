package io

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// outcome is what a parse returns, in a form a table states whole.
type outcome struct {
	// objs is each returned object as "<apiVersion> <Kind>/<name> <Go type>",
	// in order.
	objs []string
	// errs is the text of each error. One that ends in … states the beginning
	// of a text whose rest is another library's wording.
	errs []string
}

func returns(objs ...string) outcome { return outcome{objs: objs} }

func refuses(errs ...string) outcome { return outcome{errs: errs} }

// parseCase is one document and what a parse returns for it: strict without
// AllowUnstructured, loose with it.
type parseCase struct {
	name   string
	doc    string
	strict outcome
	loose  outcome
}

// both is a document that is read the same way in both modes.
func both(name, doc string, want outcome) parseCase {
	return parseCase{name: name, doc: doc, strict: want, loose: want}
}

// each is a document whose reading depends on AllowUnstructured.
func each(name, doc string, strict, loose outcome) parseCase {
	return parseCase{name: name, doc: doc, strict: strict, loose: loose}
}

// inObject starts the text of every error a document or a list is refused
// with.
const inObject = "parse error in Kubernetes object: "

// inItem is the text of an error about item n of a list of the given kind.
func inItem(n int, list, text string) string {
	return fmt.Sprintf("%sitem %d of %s: %s", inObject, n, list, text)
}

// inLists is the text of an error about a document that sits inside the given
// number of v1 Lists, as the first item of each.
func inLists(lists int, text string) string {
	return inListsOf("List", lists, text)
}

// inListsOf is inLists for lists of the given kind.
func inListsOf(kind string, lists int, text string) string {
	for range lists {
		text = inItem(0, kind, text)
	}
	return text
}

// anotherCase is the text that refuses a key which is one of the three exact
// keys only after case folding.
func anotherCase(key, exact string) string {
	return `the key "` + key + `" equals "` + exact + `" only after case folding; write it "` + exact + `" or remove it`
}

// carries is the text that refuses a list for the labels and annotations it
// states for itself.
func carries(list, what string) string {
	return inObject + list + " has metadata of its own that its items cannot keep: " + what
}

// notRegistered starts the text that refuses a kind the scheme does not know.
func notRegistered(kind, apiVersion string) string {
	return inObject + `failed to decode object: no kind "` + kind + `" is registered for version "` + apiVersion + `"…`
}

// wrapInLists wraps a document in the given number of v1 Lists.
func wrapInLists(doc string, lists int) string {
	for range lists {
		doc = `{"apiVersion":"v1","kind":"List","items":[` + doc + `]}`
	}
	return doc
}

// wrapInWidgetLists wraps a document in the given number of lists of an
// unregistered kind.
func wrapInWidgetLists(doc string, lists int) string {
	for range lists {
		doc = `{"apiVersion":"example.com/v1","kind":"WidgetList","items":[` + doc + `]}`
	}
	return doc
}

func outcomeOf(t *testing.T, objs []client.Object, err error) outcome {
	t.Helper()
	var got outcome
	for _, o := range objs {
		gvk := o.GetObjectKind().GroupVersionKind()
		got.objs = append(got.objs, fmt.Sprintf("%s %s/%s %T", gvk.GroupVersion(), gvk.Kind, o.GetName(), o))
	}
	if err != nil {
		for _, e := range parseErrorsOf(t, err) {
			got.errs = append(got.errs, e.Error())
		}
	}
	return got
}

// sameTexts reports whether got is want, text by text. A wanted text that ends
// in … is the beginning of the one it matches.
func sameTexts(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if beginning, open := strings.CutSuffix(want[i], "…"); open {
			if !strings.HasPrefix(got[i], beginning) {
				return false
			}
			continue
		}
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// runParseCases parses every case from bytes and from a file, in both modes,
// and compares everything the parse returned.
func runParseCases(t *testing.T, cases []parseCase) {
	t.Helper()
	for _, c := range cases {
		for _, p := range parsers {
			for _, m := range parseModes {
				want := c.strict
				if m.opts.AllowUnstructured {
					want = c.loose
				}
				t.Run(c.name+"/"+p.name+"/"+m.name, func(t *testing.T) {
					objs, err := p.parse(t, c.doc, m.opts)
					got := outcomeOf(t, objs, err)
					if !sameTexts(got.objs, want.objs) {
						t.Errorf("objects:\n got %q\nwant %q", got.objs, want.objs)
					}
					if !sameTexts(got.errs, want.errs) {
						t.Errorf("errors:\n got %q\nwant %q", got.errs, want.errs)
					}
				})
			}
		}
	}
}

const (
	heldConfigMap  = "v1 ConfigMap/held *v1.ConfigMap"
	hookAnnotation = `"annotations":{"helm.sh/hook":"pre-install"}`
	widgetV1       = "example.com/v1"
)

// The items are all a parse returns of a list, and they cannot keep what the
// list says about itself. A list whose own metadata carries labels or
// annotations is refused as a whole, with an error that names them, whichever
// of the three shapes it has; a Helm hook on a list is the known case. What
// describes the response a list came in (resourceVersion, continue and the
// like) is ignored as before, and so is a name on a list.
func TestParse_ListWithMetadataOfItsOwnIsRefused(t *testing.T) {
	held := configMapDoc("held")
	widget := `{"apiVersion":"example.com/v1","kind":"Widget","metadata":{"name":"w"}}`
	pagination := `"metadata":{"resourceVersion":"12","continue":"abc","selfLink":"/x","remainingItemCount":3}`
	runParseCases(t, []parseCase{
		both("v1 List, annotations",
			`{"apiVersion":"v1","kind":"List","metadata":{`+hookAnnotation+`},"items":[`+held+`]}`,
			refuses(carries("List", "annotations helm.sh/hook"))),
		both("v1 List, annotations, YAML",
			"apiVersion: v1\nkind: List\nmetadata:\n  annotations:\n    helm.sh/hook: pre-install\nitems:\n- apiVersion: v1\n  kind: ConfigMap\n  metadata:\n    name: held\n",
			refuses(carries("List", "annotations helm.sh/hook"))),
		both("v1 List, labels",
			`{"apiVersion":"v1","kind":"List","metadata":{"labels":{"app":"x"}},"items":[`+held+`]}`,
			refuses(carries("List", "labels app"))),
		both("v1 List, both, each sorted by key",
			`{"apiVersion":"v1","kind":"List","metadata":{"labels":{"app":"x"},"annotations":{"b":"1","a":"2"}},"items":[`+held+`]}`,
			refuses(carries("List", "annotations a, b; labels app"))),
		both("v1 List, empty, annotations",
			`{"apiVersion":"v1","kind":"List","metadata":{"annotations":{"a":"b"}},"items":[]}`,
			refuses(carries("List", "annotations a"))),
		both("typed list, annotations",
			`{"apiVersion":"v1","kind":"ConfigMapList","metadata":{`+hookAnnotation+`},"items":[{"metadata":{"name":"held"}}]}`,
			refuses(carries("ConfigMapList", "annotations helm.sh/hook"))),
		both("typed list, annotations, YAML",
			"apiVersion: v1\nkind: ConfigMapList\nmetadata:\n  annotations:\n    helm.sh/hook: pre-install\nitems:\n- metadata:\n    name: held\n",
			refuses(carries("ConfigMapList", "annotations helm.sh/hook"))),
		both("typed list, labels",
			`{"apiVersion":"v1","kind":"ConfigMapList","metadata":{"labels":{"app":"x"}},"items":[{"metadata":{"name":"held"}}]}`,
			refuses(carries("ConfigMapList", "labels app"))),
		each("unregistered list, annotations",
			`{"apiVersion":"example.com/v1","kind":"WidgetList","metadata":{`+hookAnnotation+`},"items":[`+widget+`]}`,
			refuses(notRegistered("WidgetList", widgetV1)),
			refuses(carries("WidgetList", "annotations helm.sh/hook"))),
		each("unregistered list, labels",
			`{"apiVersion":"example.com/v1","kind":"WidgetList","metadata":{"labels":{"app":"x"}},"items":[`+widget+`]}`,
			refuses(notRegistered("WidgetList", widgetV1)),
			refuses(carries("WidgetList", "labels app"))),
		// A key stated twice is read whole: what any of its occurrences carries
		// is carried, whichever of them another reader would keep.
		both("v1 List, annotations stated twice, the last empty",
			`{"apiVersion":"v1","kind":"List","metadata":{`+hookAnnotation+`,"annotations":{}},"items":[`+held+`]}`,
			refuses(carries("List", "annotations helm.sh/hook"))),
		both("v1 List, annotations stated twice, named once",
			`{"apiVersion":"v1","kind":"List","metadata":{"annotations":{"b":"1"},"annotations":{"a":"2","b":"3"}},"items":[`+held+`]}`,
			refuses(carries("List", "annotations a, b"))),
		both("v1 List, metadata stated twice, the last empty",
			`{"apiVersion":"v1","kind":"List","metadata":{"labels":{"app":"x"}},"metadata":{},"items":[`+held+`]}`,
			refuses(carries("List", "labels app"))),
		both("typed list, labels stated twice, the last null",
			`{"apiVersion":"v1","kind":"ConfigMapList","metadata":{"labels":{"app":"x"},"labels":null},"items":[{"metadata":{"name":"held"}}]}`,
			refuses(carries("ConfigMapList", "labels app"))),
		each("unregistered list, metadata stated twice, the last null",
			`{"apiVersion":"example.com/v1","kind":"WidgetList","metadata":{`+hookAnnotation+`},"metadata":null,"items":[`+widget+`]}`,
			refuses(notRegistered("WidgetList", widgetV1)),
			refuses(carries("WidgetList", "annotations helm.sh/hook"))),
		both("v1 List, metadata stated twice, the first a string",
			`{"apiVersion":"v1","kind":"List","metadata":"x","metadata":{},"items":[`+held+`]}`,
			refuses(inObject+"failed to read the metadata of List: …")),
		// The list beside the refused one is not refused with it.
		{
			name: "a List with annotations inside a List",
			doc:  `{"apiVersion":"v1","kind":"List","items":[` + held + `,{"apiVersion":"v1","kind":"List","metadata":{"annotations":{"a":"b"}},"items":[` + held + `]}]}`,
			strict: outcome{
				objs: []string{heldConfigMap},
				errs: []string{inItem(1, "List", carries("List", "annotations a"))},
			},
			loose: outcome{
				objs: []string{heldConfigMap},
				errs: []string{inItem(1, "List", carries("List", "annotations a"))},
			},
		},

		// Metadata that cannot be read cannot be shown to carry neither.
		both("v1 List, metadata a string",
			`{"apiVersion":"v1","kind":"List","metadata":"x","items":[`+held+`]}`,
			refuses(inObject+"failed to read the metadata of List: …")),
		both("v1 List, annotations a string",
			`{"apiVersion":"v1","kind":"List","metadata":{"annotations":"x"},"items":[`+held+`]}`,
			refuses(inObject+"failed to read the metadata of List: …")),

		// Not refused.
		both("v1 List, pagination only",
			`{"apiVersion":"v1","kind":"List",`+pagination+`,"items":[`+held+`]}`,
			returns(heldConfigMap)),
		both("typed list, pagination only",
			`{"apiVersion":"v1","kind":"ConfigMapList",`+pagination+`,"items":[{"metadata":{"name":"held"}}]}`,
			returns(heldConfigMap)),
		each("unregistered list, pagination only",
			`{"apiVersion":"example.com/v1","kind":"WidgetList",`+pagination+`,"items":[`+widget+`]}`,
			refuses(notRegistered("WidgetList", widgetV1)),
			returns("example.com/v1 Widget/w *unstructured.Unstructured")),
		both("v1 List, empty annotations and labels",
			`{"apiVersion":"v1","kind":"List","metadata":{"annotations":{},"labels":{}},"items":[`+held+`]}`,
			returns(heldConfigMap)),
		both("v1 List, null annotations",
			`{"apiVersion":"v1","kind":"List","metadata":{"annotations":null},"items":[`+held+`]}`,
			returns(heldConfigMap)),
		both("v1 List, metadata null",
			`{"apiVersion":"v1","kind":"List","metadata":null,"items":[`+held+`]}`,
			returns(heldConfigMap)),
		both("v1 List, name and namespace",
			`{"apiVersion":"v1","kind":"List","metadata":{"name":"l","namespace":"n"},"items":[`+held+`]}`,
			returns(heldConfigMap)),
		// Neither is a key the rule names: only the list's metadata.annotations
		// and metadata.labels are read, under those keys.
		both("v1 List, Annotations in another case",
			`{"apiVersion":"v1","kind":"List","metadata":{"Annotations":{"a":"b"}},"items":[`+held+`]}`,
			returns(heldConfigMap)),
		both("v1 List, Metadata in another case",
			`{"apiVersion":"v1","kind":"List","Metadata":{"annotations":{"a":"b"}},"items":[`+held+`]}`,
			returns(heldConfigMap)),
	})
}

// The refusal of metadata that cannot be read carries the JSON error of the
// value that could not be: a caller can still ask it which type the document
// states there, wherever among several values the unreadable one stands.
func TestParse_UnreadableListMetadataKeepsItsCause(t *testing.T) {
	const held = `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"held"}}`
	for _, tt := range []struct {
		name, doc, stated string
	}{
		{"metadata a string",
			`{"apiVersion":"v1","kind":"List","metadata":"x","items":[` + held + `]}`, "string"},
		{"metadata an array",
			`{"apiVersion":"v1","kind":"List","metadata":[],"items":[` + held + `]}`, "array"},
		{"metadata a number",
			`{"apiVersion":"v1","kind":"List","metadata":1,"items":[` + held + `]}`, "number"},
		{"metadata a number no float holds",
			`{"apiVersion":"v1","kind":"List","metadata":1e1000,"items":[` + held + `]}`, "number"},
		{"metadata a boolean",
			`{"apiVersion":"v1","kind":"List","metadata":true,"items":[` + held + `]}`, "bool"},
		{"metadata stated twice, the first a string",
			`{"apiVersion":"v1","kind":"List","metadata":"x","metadata":{},"items":[` + held + `]}`, "string"},
		{"annotations a string",
			`{"apiVersion":"v1","kind":"List","metadata":{"annotations":"x"},"items":[` + held + `]}`, "string"},
		{"labels stated twice, the last an array",
			`{"apiVersion":"v1","kind":"List","metadata":{"labels":{},"labels":[]},"items":[` + held + `]}`, "array"},
	} {
		for _, m := range parseModes {
			t.Run(tt.name+"/"+m.name, func(t *testing.T) {
				objs, err := ParseYAMLWithOptions([]byte(tt.doc), m.opts)
				if err == nil || len(objs) != 0 {
					t.Fatalf("the list must be refused whole, got %v and error %v", describe(objs), err)
				}
				if want := "failed to read the metadata of List: json: cannot unmarshal " + tt.stated + " "; !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not say what the document states (%q)", err, want)
				}
				var cause *json.UnmarshalTypeError
				if !errors.As(err, &cause) {
					t.Fatalf("error %q does not carry the JSON type error", err)
				}
				if cause.Value != tt.stated {
					t.Errorf("the JSON type error names a %s, want a %s", cause.Value, tt.stated)
				}
			})
		}
	}
}

// A document has one reading. The Kubernetes decoder finds apiVersion and kind
// under those keys in any case, while a list is recognised and its items are
// read under the exact keys, so a key in another case made one document a list
// to one reader and a single object to the other, or an empty list to both. A
// key that equals apiVersion or kind only after case folding is refused on
// every document and every item of a list, and one that equals items on every
// document that is read as a list; each error names the key.
func TestParse_KeyInAnotherCaseIsRefused(t *testing.T) {
	held := configMapDoc("held")
	kind := inObject + anotherCase("Kind", "kind")
	items := inObject + anotherCase("Items", "items")
	configMap := "v1 ConfigMap/c *v1.ConfigMap"
	// kind, spelled with the Kelvin sign (U+212A) for its k.
	kelvinKind := string(rune(0x212A)) + "ind"
	runParseCases(t, []parseCase{
		// A list that stated its items under another key yielded no object and
		// no error.
		both("v1 List, Items",
			`{"apiVersion":"v1","kind":"List","Items":[`+held+`]}`, refuses(items)),
		both("v1 List, Items, YAML",
			"apiVersion: v1\nkind: List\nItems:\n- "+held+"\n", refuses(items)),
		both("v1 List, ITEMS",
			`{"apiVersion":"v1","kind":"List","ITEMS":[`+held+`]}`,
			refuses(inObject+anotherCase("ITEMS", "items"))),
		both("v1 List, items and Items",
			`{"apiVersion":"v1","kind":"List","items":[`+held+`],"Items":[]}`, refuses(items)),
		both("v1 List, items and a null Items",
			`{"apiVersion":"v1","kind":"List","items":[`+held+`],"Items":null}`, refuses(items)),
		both("typed list, Items only",
			`{"apiVersion":"v1","kind":"ConfigMapList","Items":[{"metadata":{"name":"b"}}]}`, refuses(items)),
		both("typed list, items and Items",
			`{"apiVersion":"v1","kind":"ConfigMapList","items":[{"metadata":{"name":"a"}}],"Items":[{"metadata":{"name":"b"}}]}`,
			refuses(items)),
		each("unregistered list, Items",
			`{"apiVersion":"example.com/v1","kind":"WidgetList","Items":[{"apiVersion":"example.com/v1","kind":"Widget","metadata":{"name":"w"}}]}`,
			refuses(notRegistered("WidgetList", widgetV1)),
			refuses(items)),

		// Items on a document that is not read as a list is a field like any
		// other the kind does not have, and is not refused.
		both("ConfigMap, Items",
			`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"c"},"Items":[`+held+`]}`, returns(configMap)),
		both("ConfigMap, items",
			`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"c"},"items":[`+held+`]}`, returns(configMap)),
		each("unregistered kind, Items",
			`{"apiVersion":"example.com/v1","kind":"Widget","metadata":{"name":"outer"},"Items":[`+held+`]}`,
			refuses(notRegistered("Widget", widgetV1)),
			returns("example.com/v1 Widget/outer *unstructured.Unstructured")),
		// Only the top level of a document is its identity.
		both("ConfigMap, Items and Kind below the top level",
			`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"c"},"data":{"Items":"x","Kind":"y"}}`, returns(configMap)),

		// kind and Kind in one document: the decoder took the last of the two,
		// list detection the exact one.
		both("kind: List, then Kind: ConfigMap",
			`{"apiVersion":"v1","kind":"List","Kind":"ConfigMap","metadata":{"name":"outer"},"items":[`+held+`]}`, refuses(kind)),
		both("kind: ConfigMap, then Kind: List",
			`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"outer"},"Kind":"List","items":[`+held+`]}`, refuses(kind)),
		both("typed list, Kind beside kind",
			`{"apiVersion":"apps/v1","kind":"DeploymentList","Kind":"Deployment","items":[{"metadata":{"name":"good"}}]}`, refuses(kind)),
		both("kind and Kind, both ConfigMap",
			`{"apiVersion":"v1","kind":"ConfigMap","Kind":"ConfigMap","metadata":{"name":"c"}}`, refuses(kind)),
		both("kind and Kind, both ConfigMap, YAML",
			"apiVersion: v1\nkind: ConfigMap\nKind: ConfigMap\nmetadata:\n  name: c\n", refuses(kind)),
		both("Kind, then kind, both ConfigMap",
			`{"apiVersion":"v1","Kind":"ConfigMap","kind":"ConfigMap","metadata":{"name":"c"}}`, refuses(kind)),
		both("kind: ConfigMap, then Kind: Secret",
			`{"apiVersion":"v1","kind":"ConfigMap","Kind":"Secret","metadata":{"name":"c"}}`, refuses(kind)),
		both("Kind: Secret, then kind: ConfigMap",
			`{"apiVersion":"v1","Kind":"Secret","kind":"ConfigMap","metadata":{"name":"c"}}`, refuses(kind)),
		both("unregistered kind, Kind beside kind",
			`{"apiVersion":"example.com/v1","kind":"Widget","Kind":"Gadget","metadata":{"name":"outer"}}`, refuses(kind)),

		// Kind alone: refused before too, by a text that named no key.
		both("Kind: List",
			`{"apiVersion":"v1","Kind":"List","items":[`+held+`]}`, refuses(kind)),
		both("Kind: List, YAML",
			"apiVersion: v1\nKind: List\nitems:\n- "+held+"\n", refuses(kind)),
		both("Kind: ConfigMap",
			`{"apiVersion":"v1","Kind":"ConfigMap","metadata":{"name":"c"}}`, refuses(kind)),
		both("KIND: ConfigMap",
			`{"apiVersion":"v1","KIND":"ConfigMap","metadata":{"name":"c"}}`,
			refuses(inObject+anotherCase("KIND", "kind"))),
		both("unregistered kind under Kind",
			`{"apiVersion":"example.com/v1","Kind":"Widget","metadata":{"name":"outer"}}`, refuses(kind)),

		// apiVersion.
		both("ApiVersion",
			`{"ApiVersion":"v1","kind":"ConfigMap","metadata":{"name":"c"}}`,
			refuses(inObject+anotherCase("ApiVersion", "apiVersion"))),
		both("APIVersion",
			`{"APIVersion":"v1","kind":"ConfigMap","metadata":{"name":"c"}}`,
			refuses(inObject+anotherCase("APIVersion", "apiVersion"))),
		both("apiVersion and APIVERSION",
			`{"apiVersion":"v1","APIVERSION":"apps/v1","kind":"ConfigMap","metadata":{"name":"c"}}`,
			refuses(inObject+anotherCase("APIVERSION", "apiVersion"))),

		// Each key is its own error, in the order of the keys.
		both("ApiVersion and Kind",
			`{"Kind":"ConfigMap","ApiVersion":"v1","metadata":{"name":"c"}}`,
			refuses(inObject+anotherCase("ApiVersion", "apiVersion"), kind)),
		// Case folding is the decoder's: the Kelvin sign folds to k. The error
		// shows the key in a form that tells it from the one that is read.
		both("kind with the Kelvin sign",
			`{"apiVersion":"v1","`+kelvinKind+`":"ConfigMap","metadata":{"name":"c"}}`,
			refuses(inObject+anotherCase("\\"+"u212aind", "kind"))),

		// The items of a list are documents too, whichever list holds them.
		both("item of a v1 List, Kind only",
			`{"apiVersion":"v1","kind":"List","items":[{"apiVersion":"v1","Kind":"ConfigMap","metadata":{"name":"i"}}]}`,
			refuses(inItem(0, "List", kind))),
		both("item of a v1 List, kind and Kind",
			`{"apiVersion":"v1","kind":"List","items":[{"apiVersion":"v1","kind":"ConfigMap","Kind":"Secret","metadata":{"name":"i"}}]}`,
			refuses(inItem(0, "List", kind))),
		both("item of a typed list, Kind: ConfigMap",
			`{"apiVersion":"v1","kind":"ConfigMapList","items":[{"Kind":"ConfigMap","metadata":{"name":"i"}}]}`,
			refuses(inItem(0, "ConfigMapList", anotherCase("Kind", "kind")))),
		both("item of a typed list, Kind: Secret",
			`{"apiVersion":"v1","kind":"ConfigMapList","items":[{"Kind":"Secret","metadata":{"name":"i"}}]}`,
			refuses(inItem(0, "ConfigMapList", anotherCase("Kind", "kind")))),
		both("item of a typed list, ApiVersion",
			`{"apiVersion":"v1","kind":"ConfigMapList","items":[{"ApiVersion":"apps/v1","metadata":{"name":"i"}}]}`,
			refuses(inItem(0, "ConfigMapList", anotherCase("ApiVersion", "apiVersion")))),
		each("item of an unregistered list, Kind",
			`{"apiVersion":"example.com/v1","kind":"WidgetList","items":[{"Kind":"Widget","metadata":{"name":"w"}}]}`,
			refuses(notRegistered("WidgetList", widgetV1)),
			refuses(inItem(0, "WidgetList", kind))),
	})
}

// With AllowUnstructured, a list of a kind the scheme does not know is a list
// like the other two: a kind that ends in List and states items. Each item is
// decoded the way the same item is in a v1 List, a list among the items is
// opened in place and counted against the same nesting bound, and what makes a
// list refused makes this one refused. A kind that does not end in List is one
// object, whatever fields it has. A strict parse refuses every one of them as
// before: the kind is not registered.
func TestParse_ListOfAnUnregisteredKind(t *testing.T) {
	held := configMapDoc("held")
	widgetList := func(items string) string {
		return `{"apiVersion":"example.com/v1","kind":"WidgetList","items":[` + items + `]}`
	}
	strict := refuses(notRegistered("WidgetList", widgetV1))
	noWidget := refuses(notRegistered("Widget", widgetV1))
	outer := returns("example.com/v1 Widget/outer *unstructured.Unstructured")
	const notDecoded = inObject + "failed to decode object: …"
	runParseCases(t, []parseCase{
		// An item is decoded the same way whichever list holds it.
		each("an item of a registered kind is typed", widgetList(held), strict, returns(heldConfigMap)),
		each("an item of an unregistered kind is unstructured",
			widgetList(`{"apiVersion":"example.com/v1","kind":"Widget","metadata":{"name":"w"}}`),
			strict, returns("example.com/v1 Widget/w *unstructured.Unstructured")),
		each("an item of a registered kind that does not decode as it",
			widgetList(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"bad"},"data":{"a":1}}`),
			strict, refuses(inItem(0, "WidgetList", notDecoded))),
		each("an item of a registered kind that is no object",
			widgetList(`{"apiVersion":"v1","kind":"Status","status":"Failure"}`),
			strict, refuses(inItem(0, "WidgetList", inObject+"object of type *v1.Status does not implement client.Object"))),
		each("a null item, and the item beside it",
			widgetList(`null,`+held),
			strict, outcome{
				objs: []string{heldConfigMap},
				errs: []string{inItem(0, "WidgetList", "the item is null, not an object")},
			}),
		each("an item that is a string", widgetList(`"x"`), strict, refuses(inItem(0, "WidgetList", notDecoded))),

		// What an item leaves out of apiVersion and kind is the list's: its
		// apiVersion, and its kind without the List.
		each("an item with neither",
			widgetList(`{"metadata":{"name":"nameless"}}`),
			strict, returns("example.com/v1 Widget/nameless *unstructured.Unstructured")),
		each("an item with a kind and no apiVersion",
			widgetList(`{"kind":"Widget","metadata":{"name":"w"}}`),
			strict, returns("example.com/v1 Widget/w *unstructured.Unstructured")),
		each("an item with an apiVersion and no kind",
			widgetList(`{"apiVersion":"example.org/v2","metadata":{"name":"w"}}`),
			strict, returns("example.org/v2 Widget/w *unstructured.Unstructured")),
		each("an item with a null kind and an empty apiVersion",
			widgetList(`{"apiVersion":"","kind":null,"metadata":{"name":"w"}}`),
			strict, returns("example.com/v1 Widget/w *unstructured.Unstructured")),
		each("an item with nothing in it",
			widgetList(`{ }`),
			strict, returns("example.com/v1 Widget/ *unstructured.Unstructured")),

		// A list among the items is opened in place.
		each("a v1 List inside",
			widgetList(`{"apiVersion":"v1","kind":"List","items":[`+held+`]}`), strict, returns(heldConfigMap)),
		each("an unregistered list inside", widgetList(widgetList(held)), strict, returns(heldConfigMap)),
		each("a typed list inside",
			widgetList(`{"apiVersion":"v1","kind":"ConfigMapList","items":[{"metadata":{"name":"deep"}}]}`),
			strict, returns("v1 ConfigMap/deep *v1.ConfigMap")),
		each("inside a v1 List",
			wrapInLists(widgetList(held), 1),
			refuses(inLists(1, notRegistered("WidgetList", widgetV1))), returns(heldConfigMap)),
		each("kind List of an unregistered group",
			`{"apiVersion":"example.com/v1","kind":"List","items":[`+held+`]}`,
			refuses(notRegistered("List", widgetV1)), returns(heldConfigMap)),

		// A list inside the list that is itself refused.
		each("a v1 List with annotations inside",
			widgetList(`{"apiVersion":"v1","kind":"List","metadata":{"annotations":{"a":"b"}},"items":[`+held+`]}`),
			strict, refuses(inItem(0, "WidgetList", carries("List", "annotations a")))),
		each("a v1 List with Items inside",
			widgetList(`{"apiVersion":"v1","kind":"List","Items":[`+held+`]}`),
			strict, refuses(inItem(0, "WidgetList", inObject+anotherCase("Items", "items")))),
		each("a typed list with an item of another kind inside",
			widgetList(`{"apiVersion":"apps/v1","kind":"DeploymentList","items":[{"kind":"StatefulSet","metadata":{"name":"s"}}]}`),
			strict, refuses(inItem(0, "WidgetList", inItem(0, "DeploymentList", `the item states kind "StatefulSet", the list holds apps/v1, Kind=Deployment`)))),

		// The nesting bound is one bound for every kind of list.
		each("at the nesting bound",
			wrapInLists(widgetList(held), maxListNesting),
			refuses(inLists(maxListNesting, notRegistered("WidgetList", widgetV1))), returns(heldConfigMap)),
		each("past the nesting bound",
			wrapInLists(widgetList(held), maxListNesting+1),
			refuses(inLists(maxListNesting+1, notRegistered("WidgetList", widgetV1))),
			refuses(inLists(maxListNesting+1, inObject+"WidgetList is nested more than 8 lists deep"))),
		both("a v1 List at the nesting bound",
			wrapInLists(held, maxListNesting+1), returns(heldConfigMap)),
		both("a v1 List past the nesting bound",
			wrapInLists(held, maxListNesting+2),
			refuses(inLists(maxListNesting+1, inObject+"List is nested more than 8 lists deep"))),
		// A list of an unregistered kind counts against the bound as a v1 List
		// does, and a typed list is refused past it like any other.
		each("unregistered lists at the nesting bound",
			wrapInWidgetLists(held, maxListNesting+1), strict, returns(heldConfigMap)),
		each("unregistered lists past the nesting bound",
			wrapInWidgetLists(held, maxListNesting+2), strict,
			refuses(inListsOf("WidgetList", maxListNesting+1, inObject+"WidgetList is nested more than 8 lists deep"))),
		each("a typed list inside unregistered lists at the nesting bound",
			wrapInWidgetLists(`{"apiVersion":"v1","kind":"ConfigMapList","items":[{"metadata":{"name":"deep"}}]}`, maxListNesting),
			strict, returns("v1 ConfigMap/deep *v1.ConfigMap")),
		each("a typed list inside unregistered lists past the nesting bound",
			wrapInWidgetLists(`{"apiVersion":"v1","kind":"ConfigMapList","items":[{"metadata":{"name":"deep"}}]}`, maxListNesting+1),
			strict,
			refuses(inListsOf("WidgetList", maxListNesting+1, inObject+"ConfigMapList is nested more than 8 lists deep"))),
		both("a typed list at the nesting bound",
			wrapInLists(`{"apiVersion":"v1","kind":"ConfigMapList","items":[{"metadata":{"name":"deep"}}]}`, maxListNesting),
			returns("v1 ConfigMap/deep *v1.ConfigMap")),
		both("a typed list past the nesting bound",
			wrapInLists(`{"apiVersion":"v1","kind":"ConfigMapList","items":[{"metadata":{"name":"deep"}}]}`, maxListNesting+1),
			refuses(inLists(maxListNesting+1, inObject+"ConfigMapList is nested more than 8 lists deep"))),

		// The items of a list, read: empty, null, and not an array.
		each("no item", widgetList(``), strict, returns()),
		each("items null", `{"apiVersion":"example.com/v1","kind":"WidgetList","items":null}`, strict, returns()),
		each("items an object",
			`{"apiVersion":"example.com/v1","kind":"WidgetList","items":{"a":1}}`,
			strict, refuses(inObject+"failed to read the items of WidgetList: …")),

		// One object: a kind that ends in List and states no items, and a kind
		// that does not end in List whatever it states.
		each("a kind that ends in List, no items",
			`{"apiVersion":"example.com/v1","kind":"WidgetList","metadata":{"name":"n"}}`,
			strict, returns("example.com/v1 WidgetList/n *unstructured.Unstructured")),
		each("another kind that ends in List, no items",
			`{"apiVersion":"example.com/v1","kind":"AllowList","metadata":{"name":"allow"},"spec":{"cidrs":["10.0.0.0/8"]}}`,
			refuses(notRegistered("AllowList", widgetV1)),
			returns("example.com/v1 AllowList/allow *unstructured.Unstructured")),
		each("another kind that ends in List, items that are no objects",
			`{"apiVersion":"example.com/v1","kind":"AllowList","metadata":{"name":"allow"},"items":["10.0.0.0/8"]}`,
			refuses(notRegistered("AllowList", widgetV1)),
			refuses(inItem(0, "AllowList", notDecoded))),
		each("a kind that is no List, items",
			`{"apiVersion":"example.com/v1","kind":"Widget","metadata":{"name":"outer"},"items":[`+held+`]}`, noWidget, outer),
		each("a kind that is no List, items without a kind",
			`{"apiVersion":"example.com/v1","kind":"Widget","metadata":{"name":"outer"},"items":[{"metadata":{"name":"nameless"}}]}`, noWidget, outer),
		each("a kind that is no List, no item",
			`{"apiVersion":"example.com/v1","kind":"Widget","metadata":{"name":"outer"},"items":[]}`, noWidget, outer),
		each("a kind that is no List, items of strings",
			`{"apiVersion":"example.com/v1","kind":"Widget","metadata":{"name":"outer"},"items":["a","b"]}`, noWidget, outer),
		each("a kind that is no List, items an object",
			`{"apiVersion":"example.com/v1","kind":"Widget","metadata":{"name":"outer"},"items":{"a":1}}`, noWidget, outer),
		each("a kind that is no List, inside a v1 List",
			wrapInLists(`{"apiVersion":"example.com/v1","kind":"Widget","metadata":{"name":"outer"},"items":[`+held+`]}`, 1),
			refuses(inLists(1, notRegistered("Widget", widgetV1))), outer),
	})
}

// An object of an unregistered kind that is not a list keeps a field named
// items, or Items, as part of its content: nothing of the document is dropped
// and nothing in it is returned as an object of its own.
func TestParse_UnregisteredKindKeepsItsItemsField(t *testing.T) {
	for _, key := range []string{"items", "Items"} {
		t.Run(key, func(t *testing.T) {
			doc := `{"apiVersion":"example.com/v1","kind":"Widget","metadata":{"name":"outer"},"` + key + `":[` + configMapDoc("held") + `]}`
			objs, err := ParseYAMLWithOptions([]byte(doc), ParseOptions{AllowUnstructured: true})
			if err != nil {
				t.Fatal(err)
			}
			if len(objs) != 1 {
				t.Fatalf("got %d objects, want the Widget alone", len(objs))
			}
			widget, ok := objs[0].(*unstructured.Unstructured)
			if !ok {
				t.Fatalf("the Widget is %T, want unstructured", objs[0])
			}
			field, found, err := unstructured.NestedSlice(widget.Object, key)
			if err != nil || !found || len(field) != 1 {
				t.Fatalf("the field %s is %v (found %v, error %v), want its one entry", key, field, found, err)
			}
			entry, ok := field[0].(map[string]any)
			if !ok || entry["kind"] != "ConfigMap" {
				t.Errorf("the entry of %s is %v, want the ConfigMap as written", key, field[0])
			}
		})
	}
}

// What an item of a list of an unregistered kind leaves out of apiVersion and
// kind is added to it, and everything the item states stays as written. The
// item is the object the same text is as a document of its own with its
// identity stated, down to a field it states twice, which the decoders read
// in their own way.
func TestParse_ItemOfAnUnregisteredListKeepsWhatItStates(t *testing.T) {
	for _, c := range []struct{ name, list, item, alone string }{
		{
			name:  "a field stated twice",
			list:  `"apiVersion":"v1","kind":"WidgetList"`,
			item:  `{"kind":"ConfigMap","metadata":{"name":"c"},"data":{"a":"1"},"data":{"b":"2"}}`,
			alone: `{"kind":"ConfigMap","metadata":{"name":"c"},"data":{"a":"1"},"data":{"b":"2"},"apiVersion":"v1"}`,
		},
		{
			name:  "numbers and escapes",
			list:  `"apiVersion":"example.com/v1","kind":"WidgetList"`,
			item:  `{"metadata":{"name":"w"},"spec":{"n":12345678901234567890,"f":1.0,"s":"aé\n"}}`,
			alone: `{"metadata":{"name":"w"},"spec":{"n":12345678901234567890,"f":1.0,"s":"aé\n"},"apiVersion":"example.com/v1","kind":"Widget"}`,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			opts := ParseOptions{AllowUnstructured: true}
			inList, err := ParseYAMLWithOptions([]byte(`{`+c.list+`,"items":[`+c.item+`]}`), opts)
			if err != nil {
				t.Fatal(err)
			}
			alone, err := ParseYAMLWithOptions([]byte(c.alone), opts)
			if err != nil {
				t.Fatal(err)
			}
			if len(inList) != 1 || len(alone) != 1 {
				t.Fatalf("got %d objects from the list and %d from the document, want one of each", len(inList), len(alone))
			}
			if !reflect.DeepEqual(inList[0], alone[0]) {
				t.Errorf("the list changed what its item states:\n in the list %#v\n       alone %#v", inList[0], alone[0])
			}
		})
	}
}

// The bytes an item is decoded from, one for one: what the item leaves out of
// apiVersion and kind stands behind its last field, and every byte the item
// has is kept, in its order, white space and a field stated twice included;
// the closing brace and what follows it stand behind what was added.
func TestWithListIdentity_ByteForByte(t *testing.T) {
	const listVersion, itemKind = "example.com/v1", "Widget"
	for _, c := range []struct{ name, item, apiVersion, kind, want string }{
		{"both stated", `{"apiVersion":"v1","kind":"ConfigMap"}`, listVersion, itemKind,
			`{"apiVersion":"v1","kind":"ConfigMap"}`},
		{"no kind", `{"apiVersion":"v1","metadata":{"name":"a"}}`, listVersion, itemKind,
			`{"apiVersion":"v1","metadata":{"name":"a"},"kind":"Widget"}`},
		{"no apiVersion", `{"kind":"ConfigMap","metadata":{"name":"a"}}`, listVersion, itemKind,
			`{"kind":"ConfigMap","metadata":{"name":"a"},"apiVersion":"example.com/v1"}`},
		{"neither", `{"metadata":{"name":"a"}}`, listVersion, itemKind,
			`{"metadata":{"name":"a"},"apiVersion":"example.com/v1","kind":"Widget"}`},
		{"an empty object", `{ }`, listVersion, itemKind,
			`{ "apiVersion":"example.com/v1","kind":"Widget"}`},
		{"a null kind and an empty apiVersion", `{"apiVersion":"","kind":null}`, listVersion, itemKind,
			`{"apiVersion":"","kind":null,"apiVersion":"example.com/v1","kind":"Widget"}`},
		{"a kind that is no string", `{"kind":1}`, listVersion, itemKind,
			`{"kind":1,"apiVersion":"example.com/v1"}`},
		{"a field stated twice, white space, numbers and escapes",
			"{\"data\":{\"a\":\"1\"}, \"data\" : {\"b\":\"2\"},\n \"n\":1.0e3,\"s\":\"\\u00e9\" }\n", listVersion, itemKind,
			"{\"data\":{\"a\":\"1\"}, \"data\" : {\"b\":\"2\"},\n \"n\":1.0e3,\"s\":\"\\u00e9\" ,\"apiVersion\":\"example.com/v1\",\"kind\":\"Widget\"}\n"},
		{"a brace inside the last value", `{"s":"}"}`, listVersion, itemKind,
			`{"s":"}","apiVersion":"example.com/v1","kind":"Widget"}`},
		{"a list that states no apiVersion", `{"metadata":{"name":"a"}}`, "", itemKind,
			`{"metadata":{"name":"a"},"kind":"Widget"}`},
		{"a list that states neither", `{"metadata":{"name":"a"}}`, "", "",
			`{"metadata":{"name":"a"}}`},
		{"an identity that needs escaping", `{}`, `a"b`, `c\d`,
			`{"apiVersion":"a\"b","kind":"c\\d"}`},
		{"a string", `"x"`, listVersion, itemKind, `"x"`},
		{"an array", `[{}]`, listVersion, itemKind, `[{}]`},
		{"a null", `null`, listVersion, itemKind, `null`},
		{"not JSON", `{"a":`, listVersion, itemKind, `{"a":`},
	} {
		t.Run(c.name, func(t *testing.T) {
			item := []byte(c.item)
			got := withListIdentity(item, c.apiVersion, c.kind)
			if string(got) != c.want {
				t.Errorf("withListIdentity(%q)\n got %q\nwant %q", c.item, got, c.want)
			}
			if string(item) != c.item {
				t.Errorf("the item was written to: %q, was %q", item, c.item)
			}
		})
	}
}
