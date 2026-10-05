package io

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"slices"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/kubernetes"
)

// ParseOptions controls how Kubernetes YAML documents are decoded.
type ParseOptions struct {
	// AllowUnstructured enables fallback decoding for GVKs not registered
	// in the kure scheme. When true, unknown objects are returned as
	// *unstructured.Unstructured instead of producing an error.
	AllowUnstructured bool
}

func parse(yamlbytes []byte, opts ParseOptions) ([]client.Object, error) {
	// Parsing approach adapted from
	// https://dx13.co.uk/articles/2021/01/15/kubernetes-types-using-go/

	return parseStream(yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(yamlbytes), 4096), opts)
}

// documentDecoder is what a parse reads the documents of its stream from, one
// per call, until the error that marks the end of the stream.
type documentDecoder interface {
	Decode(into any) error
}

// parseStream decodes the documents decoder yields into the objects they
// hold. It reads until the stream ends, or until the decoder stops moving.
//
// The decoder reports a place of the stream it cannot read as an error, and
// only some of those errors leave it at the next document. A YAML document
// that does not parse does: the decoder has split it off already, and its
// error is a YAMLSyntaxError. Malformed JSON does not always: the decoder may
// return an error on every further call and never the end of the stream. So a
// second error in a row that is not such a YAML error ends the parse. The
// first error of the row stands for the place; the one that ends the parse
// says nothing more about it and is not kept.
func parseStream(decoder documentDecoder, opts ParseOptions) ([]client.Object, error) {
	retVal := make([]runtime.Object, 0)

	if err := kubernetes.RegisterSchemes(); err != nil {
		return nil, errors.Wrapf(err, "register schemes")
	}

	var errs []error

	// unsure is true after an error that may have left the decoder where it
	// was.
	unsure := false
	for {
		var raw runtime.RawExtension
		if err := decoder.Decode(&raw); err != nil {
			if stderrors.Is(err, io.EOF) {
				break
			}
			var split yamlutil.YAMLSyntaxError
			movedOn := stderrors.As(err, &split)
			if unsure && !movedOn {
				break
			}
			errs = append(errs, errors.NewParseError("YAML document", "failed to decode document", 0, 0, err))
			unsure = !movedOn
			continue
		}
		unsure = false
		if len(bytes.TrimSpace(raw.Raw)) == 0 {
			continue
		}
		objs, docErrs := decodeDocument(raw.Raw, opts, 0)
		retVal = append(retVal, objs...)
		errs = append(errs, docErrs...)
	}

	retValCO := make([]client.Object, 0, len(retVal))
	for _, obj := range retVal {
		co, ok := obj.(client.Object)
		if !ok {
			errs = append(errs, errors.NewParseError("Kubernetes object",
				fmt.Sprintf("object of type %T does not implement client.Object", obj),
				0, 0, nil))
			continue
		}
		retValCO = append(retValCO, co)
	}
	if len(errs) > 0 {
		return retValCO, &errors.ParseErrors{Errors: errs}
	}
	return retValCO, nil
}

// ParseFile reads the YAML file at path and returns the runtime objects
// defined within. Each object is decoded using the k8s scheme. An error is
// returned if the file cannot be read or if decoding any document fails.
func ParseFile(path string) ([]client.Object, error) {
	return ParseFileWithOptions(path, ParseOptions{})
}

// ParseFileWithOptions reads the YAML file at path and returns the runtime
// objects defined within. Behavior is controlled by opts; see [ParseOptions].
func ParseFileWithOptions(path string, opts ParseOptions) ([]client.Object, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parse(data, opts)
}

// ParseYAML parses YAML bytes and returns the runtime objects
// defined within. Each object is decoded using the k8s scheme. An error is
// returned if decoding any document fails.
func ParseYAML(data []byte) ([]client.Object, error) {
	return ParseYAMLWithOptions(data, ParseOptions{})
}

// ParseYAMLWithOptions parses YAML bytes and returns the runtime objects
// defined within. Behavior is controlled by opts; see [ParseOptions].
func ParseYAMLWithOptions(data []byte, opts ParseOptions) ([]client.Object, error) {
	return parse(data, opts)
}

// maxListNesting is how deep a list may sit inside other lists. Each level
// decodes what it holds again, so an unbounded depth would make the cost of a
// document quadratic in its size; no manifest nests lists this deep.
const maxListNesting = 8

// decodeDocument decodes one document, as JSON, into the objects it holds. A
// document of a single kind yields that object. A list document yields its
// items in the order the document gives them, never the list itself; see
// flattenList. The objects that decoded are returned next to the errors of the
// ones that did not. nesting is the number of lists the document sits inside
// whose items are documents of their own, the v1 List and the list of an
// unregistered kind: zero for a document of the stream.
//
// A document has one reading. The Kubernetes decoder finds apiVersion and kind
// under those keys in any case, while a list is recognised by the exact keys,
// so a document that states either under another case would be one thing to
// one reader and another to the other. It is refused before anything reads
// it; see foldedKeys.
func decodeDocument(raw []byte, opts ParseOptions, nesting int) ([]runtime.Object, []error) {
	// fields stays nil for a document that is not a JSON object. The decoder
	// below says what is wrong with such a document.
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	if refused := foldedKeys(fields, "apiVersion", "kind"); len(refused) > 0 {
		return nil, parseErrorsOfKeys(refused)
	}

	if list, ok := registeredList(fields); ok {
		listGVK := list.GetObjectKind().GroupVersionKind()
		want, generic, err := listItemKind(list, listGVK.GroupVersion())
		if err != nil {
			return nil, []error{errors.NewParseError("Kubernetes object",
				fmt.Sprintf("failed to read the items of %s", listGVK.Kind), 0, 0, err)}
		}
		decodeItem := func(item []byte) ([]runtime.Object, []error) {
			return decodeTypedItem(item, want)
		}
		if generic {
			decodeItem = func(item []byte) ([]runtime.Object, []error) {
				return decodeDocument(item, opts, nesting+1)
			}
		}
		return flattenList(raw, fields, listGVK.Kind, nesting, decodeItem)
	}

	obj, _, err := decodeRegistered(raw, nil)
	if err != nil {
		if opts.AllowUnstructured && runtime.IsNotRegisteredError(err) {
			return decodeUnregistered(raw, fields, opts, nesting)
		}
		return nil, []error{errors.NewParseError("Kubernetes object", "failed to decode object", 0, 0, err)}
	}
	if err := checkType(obj); err != nil {
		return nil, []error{err}
	}
	if err := requireObject(obj); err != nil {
		return nil, []error{err}
	}
	return []runtime.Object{obj}, nil
}

// foldedKeys returns an error for each key of fields that equals one of exact
// only after case folding (Kind, ITEMS), sorted by key. Such a key is not the
// one the document's readers agree on, and nothing says which of them the
// author meant, so the document that carries it is refused and the error names
// the key and the spelling that is read.
func foldedKeys(fields map[string]json.RawMessage, exact ...string) []error {
	var keys []string
	spelling := make(map[string]string)
	for key := range fields {
		for _, want := range exact {
			if key != want && strings.EqualFold(key, want) {
				keys = append(keys, key)
				spelling[key] = want
			}
		}
	}
	sort.Strings(keys)
	errs := make([]error, 0, len(keys))
	for _, key := range keys {
		errs = append(errs, errors.Errorf("the key %+q equals %+q only after case folding; write it %+q or remove it",
			key, spelling[key], spelling[key]))
	}
	return errs
}

// parseErrorsOfKeys turns the errors of foldedKeys into the parse errors of the
// document that carries the keys.
func parseErrorsOfKeys(refused []error) []error {
	errs := make([]error, 0, len(refused))
	for _, err := range refused {
		errs = append(errs, errors.NewParseError("Kubernetes object", err.Error(), 0, 0, nil))
	}
	return errs
}

// decodeUnregistered decodes a document of a kind the scheme does not know,
// for a parse that allows unstructured objects. fields is the document's
// top level.
//
// A kind that ends in List and states items is a list, and is taken through
// the item handling every list gets: see flattenList. Its items are documents
// of their own, like those of the v1 List, so an item of a registered kind
// comes back as its Go type and a list among them is opened in place. An item
// that leaves apiVersion or kind out is given the list's: the list's
// apiVersion, and its kind without the List. A kind that ends in List and
// states its items only under a key in another case (Items) goes the same way
// and is refused there: nothing says whether its author meant a list.
//
// Any other document is one object, whatever fields it has: a kind that does
// not end in List keeps a field named items as part of its content.
func decodeUnregistered(raw []byte, fields map[string]json.RawMessage, opts ParseOptions, nesting int) ([]runtime.Object, []error) {
	// Both were read as strings by the decoder that found the kind unregistered.
	apiVersion, _ := stringField(fields, "apiVersion")
	kind, _ := stringField(fields, "kind")
	if _, stated := fields["items"]; strings.HasSuffix(kind, "List") && (stated || len(foldedKeys(fields, "items")) > 0) {
		itemKind := strings.TrimSuffix(kind, "List")
		return flattenList(raw, fields, kind, nesting, func(item []byte) ([]runtime.Object, []error) {
			return decodeDocument(withListIdentity(item, apiVersion, itemKind), opts, nesting+1)
		})
	}
	obj := &unstructured.Unstructured{}
	if _, _, err := unstructured.UnstructuredJSONScheme.Decode(raw, nil, obj); err != nil {
		return nil, []error{errors.NewParseError("Kubernetes object", "failed to decode unstructured object", 0, 0, err)}
	}
	return []runtime.Object{obj}, nil
}

// withListIdentity returns item with apiVersion and kind stated: what the item
// leaves out of the two, under the exact key, is added with the given value.
// It is added behind the item's last field and the item's own bytes are kept
// as they are: an item is not written out again from what was read of it,
// which would drop whatever that reading does not hold, a field stated twice
// for one. An item that states both, and one that is not a JSON object, is
// returned as it is.
func withListIdentity(item []byte, apiVersion, kind string) []byte {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(item, &fields); err != nil || fields == nil {
		return item
	}
	var added []byte
	for _, id := range []struct{ key, value string }{{"apiVersion", apiVersion}, {"kind", kind}} {
		stated, err := stringField(fields, id.key)
		if err != nil || stated != "" || id.value == "" {
			continue
		}
		value, err := json.Marshal(id.value)
		if err != nil {
			continue
		}
		if len(fields) > 0 || len(added) > 0 {
			added = append(added, ',')
		}
		added = append(added, `"`+id.key+`":`...)
		added = append(added, value...)
	}
	// The item is a JSON object, so its last brace is the one that closes it.
	end := bytes.LastIndexByte(item, '}')
	if len(added) == 0 || end < 0 {
		return item
	}
	return slices.Concat(item[:end], added, item[end:])
}

// registeredList reports whether fields, the top level of a document, is that
// of a list document of a kind the scheme registers. When it is, it returns an
// empty list of that kind, carrying its GroupVersionKind. The document is
// recognised by the kind it states and is not decoded as a whole: one item
// that does not decode must not take the items beside it down with it.
//
// apiVersion and kind are read under exactly those keys. A document that
// states either under another case never gets here; see decodeDocument.
func registeredList(fields map[string]json.RawMessage) (list runtime.Object, ok bool) {
	apiVersion, err := stringField(fields, "apiVersion")
	if err != nil {
		return nil, false
	}
	kind, err := stringField(fields, "kind")
	if err != nil || kind == "" {
		return nil, false
	}
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, false
	}
	gvk := gv.WithKind(kind)
	list, err = kubernetes.Scheme.New(gvk)
	if err != nil {
		return nil, false
	}
	if _, isObject := list.(client.Object); isObject || !meta.IsListType(list) {
		return nil, false
	}
	list.GetObjectKind().SetGroupVersionKind(gvk)
	return list, true
}

// stringField returns the string a document states under key, exactly as
// spelled. A key that is absent, null or empty yields "": the field is left
// out.
func stringField(fields map[string]json.RawMessage, key string) (string, error) {
	value, stated := fields[key]
	if !stated {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(value, &s); err != nil {
		return "", errors.Wrapf(err, "read %s", key)
	}
	return s, nil
}

// flattenList returns the items of a list document, in the list's own order.
// raw is the document, fields its top level, kind the kind it states, and
// decodeItem what turns one item into objects. It is the one path for the
// three shapes a list has:
//
//   - a typed list (DeploymentList): its items are of the one kind the list
//     holds. An item that leaves apiVersion and kind out, as the API server's
//     own list responses do, is given the kind's; an item that states them must
//     state the kind the list holds. See decodeTypedItem.
//   - the generic v1 List: each item is a document of its own and is decoded
//     like one, so an item of an unregistered kind follows
//     ParseOptions.AllowUnstructured and an item that is itself a list is
//     flattened in place.
//   - the list of an unregistered kind (WidgetList), in a parse that allows
//     unstructured objects: its items are documents of their own too. See
//     decodeUnregistered.
//
// A list is refused as a whole, with no item returned, when it sits inside
// more than maxListNesting lists, when it states its items under a key in
// another case (Items), which the readers of a list do not agree on, and when
// its own metadata carries labels or annotations: the items are all a parse
// returns of a list, and they cannot keep what was said about the list.
//
// Each item is decoded by itself: one that does not decode, a null among them,
// is an error naming its position, and the items beside it are still returned.
// An empty list yields no object and no error.
func flattenList(raw []byte, fields map[string]json.RawMessage, kind string, nesting int, decodeItem func(item []byte) ([]runtime.Object, []error)) ([]runtime.Object, []error) {
	refuse := func(reason string, cause error) []error {
		return []error{errors.NewParseError("Kubernetes object", reason, 0, 0, cause)}
	}
	if nesting > maxListNesting {
		return nil, refuse(fmt.Sprintf("%s is nested more than %d lists deep", kind, maxListNesting), nil)
	}
	if refused := foldedKeys(fields, "items"); len(refused) > 0 {
		return nil, parseErrorsOfKeys(refused)
	}
	carried, err := listMetadata(raw)
	if err != nil {
		return nil, refuse(fmt.Sprintf("failed to read the metadata of %s", kind), err)
	}
	if carried != "" {
		return nil, refuse(fmt.Sprintf("%s has metadata of its own that its items cannot keep: %s", kind, carried), nil)
	}
	var items []json.RawMessage
	if rawItems := fields["items"]; len(rawItems) > 0 {
		if err := json.Unmarshal(rawItems, &items); err != nil {
			return nil, refuse(fmt.Sprintf("failed to read the items of %s", kind), err)
		}
	}

	objs := make([]runtime.Object, 0, len(items))
	var errs []error
	for i, item := range items {
		itemErr := func(cause error) error {
			return errors.NewParseError("Kubernetes object",
				fmt.Sprintf("item %d of %s", i, kind), 0, 0, cause)
		}
		if bytes.Equal(bytes.TrimSpace(item), []byte("null")) {
			errs = append(errs, itemErr(nullItemError{}))
			continue
		}
		itemObjs, itemErrs := decodeItem(item)
		objs = append(objs, itemObjs...)
		for _, e := range itemErrs {
			errs = append(errs, itemErr(e))
		}
	}
	return objs, errs
}

// nullItemError is the error for a null among the items of a list. Its text
// says what the author wrote; it is errors.ErrNilRuntimeObject to errors.Is.
type nullItemError struct{}

func (nullItemError) Error() string { return "the item is null, not an object" }

func (nullItemError) Unwrap() error { return errors.ErrNilRuntimeObject }

// listMetadata returns the labels and annotations a list document states for
// itself, as the text an error names them by ("annotations a, b; labels app"),
// and "" when it states none. Only those two are read: a list's
// resourceVersion, continue and the like describe the response the list came
// in, and nothing is lost with them. Metadata that cannot be read is an error:
// it cannot be shown to carry no label and no annotation.
//
// raw is the list document, not a reading of it. A document can state
// metadata, labels or annotations more than once, and its readers do not keep
// the same occurrence. What any occurrence carries is carried: a hook
// annotation followed by an empty annotations field is still one the items
// cannot keep.
func listMetadata(raw []byte) (string, error) {
	metadata, err := occurrences(raw, "metadata")
	if err != nil {
		return "", err
	}
	var carried []string
	for _, field := range []string{"annotations", "labels"} {
		named := make(map[string]struct{})
		for _, object := range metadata {
			stated, err := occurrences(object, field)
			if err != nil {
				return "", err
			}
			for _, value := range stated {
				var entries map[string]json.RawMessage
				if err := json.Unmarshal(value, &entries); err != nil {
					return "", err
				}
				for key := range entries {
					named[key] = struct{}{}
				}
			}
		}
		if len(named) == 0 {
			continue
		}
		keys := make([]string, 0, len(named))
		for key := range named {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		carried = append(carried, field+" "+strings.Join(keys, ", "))
	}
	return strings.Join(carried, "; "), nil
}

// occurrences returns every value a JSON object states under key, in the order
// it states them: more than one where it states the key more than once. A null
// in place of the object states none; anything else that is no object is the
// error of reading it as one, which says what the document states there.
func occurrences(object []byte, key string) ([]json.RawMessage, error) {
	if !bytes.HasPrefix(bytes.TrimSpace(object), []byte("{")) {
		// The value is read as an object only to have the error of that
		// reading, or none for a null; the walk below never sees it.
		var fields map[string]json.RawMessage
		return nil, json.Unmarshal(object, &fields)
	}
	dec := json.NewDecoder(bytes.NewReader(object))
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	var values []json.RawMessage
	for dec.More() {
		name, err := dec.Token()
		if err != nil {
			return nil, err
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, err
		}
		if stated, isString := name.(string); isString && stated == key {
			values = append(values, value)
		}
	}
	return values, nil
}

// listItemKind returns what a registered list holds: generic is true for the
// v1 List, whose items are documents of any kind, and want is the one kind a
// typed list holds, in the list's group version.
func listItemKind(list runtime.Object, listGV schema.GroupVersion) (want schema.GroupVersionKind, generic bool, err error) {
	itemsPtr, err := meta.GetItemsPtr(list)
	if err != nil {
		return schema.GroupVersionKind{}, false, err
	}
	elem := reflect.TypeOf(itemsPtr).Elem().Elem()
	if elem == reflect.TypeOf(runtime.RawExtension{}) {
		return schema.GroupVersionKind{}, true, nil
	}
	if elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}
	item, ok := reflect.New(elem).Interface().(runtime.Object)
	if !ok {
		return schema.GroupVersionKind{}, false, errors.Errorf("items of type %s are not Kubernetes objects", elem)
	}
	want, err = itemKind(item, listGV)
	return want, false, err
}

// decodeTypedItem decodes one item of a typed list as the kind the list holds.
// What the item leaves out of apiVersion and kind is taken from want; what it
// states must be want's, as written. The stated strings are compared before
// decoding, because the decoder fills in what an apiVersion such as "apps/"
// leaves out and would hide that the item stated something else. They are read
// under the exact keys apiVersion and kind, the ones the object is decoded
// from; an item that states either under a key in another case is refused like
// a document that does, see foldedKeys. An empty or null value counts as left
// out, as it does for the decoder.
func decodeTypedItem(raw []byte, want schema.GroupVersionKind) ([]runtime.Object, []error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, []error{err}
	}
	if refused := foldedKeys(fields, "apiVersion", "kind"); len(refused) > 0 {
		return nil, refused
	}
	for _, id := range []struct{ key, want string }{
		{"apiVersion", want.GroupVersion().String()},
		{"kind", want.Kind},
	} {
		s, err := stringField(fields, id.key)
		if err != nil {
			return nil, []error{err}
		}
		if s != "" && s != id.want {
			return nil, []error{errors.Errorf("the item states %s %q, the list holds %s", id.key, s, want)}
		}
	}
	obj, actual, err := decodeRegistered(raw, &want)
	if err != nil {
		return nil, []error{err}
	}
	if actual == nil || *actual != want {
		return nil, []error{errors.Errorf("the item states %v, the list holds %s", actual, want)}
	}
	obj.GetObjectKind().SetGroupVersionKind(want)
	if err := checkType(obj); err != nil {
		return nil, []error{err}
	}
	if err := requireObject(obj); err != nil {
		return nil, []error{err}
	}
	return []runtime.Object{obj}, nil
}

// decodeRegistered decodes raw with the scheme's deserializer; kind is what raw
// is decoded as where it leaves apiVersion or kind out, and may be nil. The
// deserializer runs the UnmarshalJSON of the registered types, which is code
// kure does not own. A panic in one must not take the caller down: it comes
// back as an error that names the object being decoded and carries the panic's
// value, and no object is returned.
func decodeRegistered(raw []byte, kind *schema.GroupVersionKind) (obj runtime.Object, actual *schema.GroupVersionKind, err error) {
	defer func() {
		if r := recover(); r != nil {
			obj, actual, err = nil, nil, decoderPanic(r, raw, kind)
		}
	}()
	return kubernetes.Codecs.UniversalDeserializer().Decode(raw, kind, nil)
}

// decoderPanic is the error for a panic with value r while raw was decoded. A
// value that is an error stays in the chain; any other is printed.
func decoderPanic(r any, raw []byte, kind *schema.GroupVersionKind) error {
	cause, isErr := r.(error)
	if !isErr {
		cause = errors.Errorf("%v", r)
	}
	return errors.Wrapf(cause, "the decoder panicked on %s", describeObject(raw, kind))
}

// describeObject names the object raw states, for an error about it: its kind
// and its name, with the namespace when it has one (Deployment "default/web").
// kind stands in for a kind raw leaves out. It reads the exact keys kind and
// metadata, and leaves out whatever it cannot read: without a kind it says
// "the object".
func describeObject(raw []byte, kind *schema.GroupVersionKind) string {
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	what, _ := stringField(fields, "kind")
	if what == "" && kind != nil {
		what = kind.Kind
	}
	if what == "" {
		what = "the object"
	}
	var metadata map[string]json.RawMessage
	_ = json.Unmarshal(fields["metadata"], &metadata)
	name, _ := stringField(metadata, "name")
	if name == "" {
		return what
	}
	if namespace, _ := stringField(metadata, "namespace"); namespace != "" {
		name = namespace + "/" + name
	}
	return fmt.Sprintf("%s %q", what, name)
}

// requireObject refuses a decoded value of a registered kind that is not a
// Kubernetes object with metadata of its own, v1 Status for one. It is checked
// where a document is decoded, so that the error of a list item can name the
// item's position.
func requireObject(obj runtime.Object) error {
	if _, ok := obj.(client.Object); !ok {
		return errors.NewParseError("Kubernetes object",
			fmt.Sprintf("object of type %T does not implement client.Object", obj), 0, 0, nil)
	}
	return nil
}

// itemKind returns the kind the scheme registers for a typed list item in the
// group version of its list. A Go type registered under one kind only is
// returned as that kind whatever the list's group version is.
func itemKind(item runtime.Object, listGV schema.GroupVersion) (schema.GroupVersionKind, error) {
	gvks, _, err := kubernetes.Scheme.ObjectKinds(item)
	if err != nil {
		return schema.GroupVersionKind{}, errors.Wrapf(err, "look up the kind of %T", item)
	}
	for _, gvk := range gvks {
		if gvk.GroupVersion() == listGV {
			return gvk, nil
		}
	}
	if len(gvks) == 1 {
		return gvks[0], nil
	}
	return schema.GroupVersionKind{}, errors.Errorf("type %T is registered under %d kinds, none of them in %s", item, len(gvks), listGV)
}

func checkType(obj runtime.Object) error {
	if obj == nil {
		return errors.ErrNilRuntimeObject
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	if err := kubernetes.RegisterSchemes(); err != nil {
		return errors.Wrapf(err, "register schemes")
	}
	expected, ok := kubernetes.Scheme.AllKnownTypes()[gvk]
	if !ok {
		return errors.Wrapf(errors.ErrUnsupportedKind, "kind %s", gvk.String())
	}

	objType := reflect.TypeOf(obj)
	if objType != expected && objType != reflect.PointerTo(expected) {
		return errors.NewParseError("Kubernetes object", fmt.Sprintf("kind %s expected type %v but got %T", gvk.Kind, expected, obj), 0, 0, nil)
	}

	return nil
}
