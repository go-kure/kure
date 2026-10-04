package io

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"os"
	"reflect"

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

	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(yamlbytes), 4096)
	retVal := make([]runtime.Object, 0)

	if err := kubernetes.RegisterSchemes(); err != nil {
		return nil, errors.Wrapf(err, "register schemes")
	}

	var errs []error

	for {
		var raw runtime.RawExtension
		if err := decoder.Decode(&raw); err != nil {
			if stderrors.Is(err, io.EOF) {
				break
			}
			errs = append(errs, errors.NewParseError("YAML document", "failed to decode document", 0, 0, err))
			continue
		}
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

// maxListNesting is how deep a list may sit inside generic Lists. Each level
// decodes what it holds again, so an unbounded depth would make the cost of a
// document quadratic in its size; no manifest nests lists this deep.
const maxListNesting = 8

// decodeDocument decodes one document, as JSON, into the objects it holds. A
// document of a single kind yields that object. A list document yields its
// items in the order the document gives them, never the list itself; see
// flattenList. The objects that decoded are returned next to the errors of the
// ones that did not. nesting is the number of generic Lists the document sits
// inside: zero for a document of the stream.
func decodeDocument(raw []byte, opts ParseOptions, nesting int) ([]runtime.Object, []error) {
	if list, items, ok := registeredList(raw); ok {
		if nesting > maxListNesting {
			return nil, []error{errors.NewParseError("Kubernetes object",
				fmt.Sprintf("%s is nested more than %d lists deep", list.GetObjectKind().GroupVersionKind().Kind, maxListNesting),
				0, 0, nil)}
		}
		return flattenList(items, list, opts, nesting)
	}
	obj, _, err := kubernetes.Codecs.UniversalDeserializer().Decode(raw, nil, nil)
	if err != nil {
		if opts.AllowUnstructured && runtime.IsNotRegisteredError(err) {
			unstObj, _, unstErr := unstructured.UnstructuredJSONScheme.Decode(raw, nil, nil)
			if unstErr != nil {
				return nil, []error{errors.NewParseError("Kubernetes object", "failed to decode unstructured object", 0, 0, unstErr)}
			}
			if list, ok := unstObj.(*unstructured.UnstructuredList); ok {
				items := make([]runtime.Object, 0, len(list.Items))
				for i := range list.Items {
					items = append(items, &list.Items[i])
				}
				return items, nil
			}
			return []runtime.Object{unstObj}, nil
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

// registeredList reports whether raw is a list document of a kind the scheme
// registers. When it is, it returns an empty list of that kind, carrying its
// GroupVersionKind, and the document's items, undecoded. The document is
// recognised by the kind it states and is not decoded as a whole: one item
// that does not decode must not take the items beside it down with it.
//
// apiVersion, kind and items are read under exactly those keys, the ones the
// Kubernetes decoder reads an object from. A key that differs in case only
// (Kind, Items) is not one of them: it must not turn another document into a
// list, nor replace a list's items.
func registeredList(raw []byte) (list runtime.Object, items json.RawMessage, ok bool) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, nil, false
	}
	apiVersion, err := stringField(fields, "apiVersion")
	if err != nil {
		return nil, nil, false
	}
	kind, err := stringField(fields, "kind")
	if err != nil || kind == "" {
		return nil, nil, false
	}
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return nil, nil, false
	}
	gvk := gv.WithKind(kind)
	list, err = kubernetes.Scheme.New(gvk)
	if err != nil {
		return nil, nil, false
	}
	if _, isObject := list.(client.Object); isObject || !meta.IsListType(list) {
		return nil, nil, false
	}
	list.GetObjectKind().SetGroupVersionKind(gvk)
	return list, fields["items"], true
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

// flattenList returns the items of a list document of a registered kind, in
// the list's own order. rawItems is the document's items field and list an
// empty list of its kind. It covers the two shapes such a list has:
//
//   - a typed list (DeploymentList): its items are of the one kind the list
//     holds. An item that leaves apiVersion and kind out, as the API server's
//     own list responses do, is given the kind's; an item that states them must
//     state the kind the list holds.
//   - the generic v1 List: each item is a document of its own and is decoded
//     like one, so an item of an unregistered kind follows opts.AllowUnstructured
//     and an item that is itself a list is flattened in place, up to
//     maxListNesting deep.
//
// Each item is decoded by itself: one that does not decode, a null among them,
// is an error naming its position, and the items beside it are still returned.
// An empty list yields no object and no error. Only the items are read; the
// list's own metadata is not.
func flattenList(rawItems json.RawMessage, list runtime.Object, opts ParseOptions, nesting int) ([]runtime.Object, []error) {
	listGVK := list.GetObjectKind().GroupVersionKind()
	listErr := func(cause error) []error {
		return []error{errors.NewParseError("Kubernetes object",
			fmt.Sprintf("failed to read the items of %s", listGVK.Kind), 0, 0, cause)}
	}
	var items []json.RawMessage
	if len(rawItems) > 0 {
		if err := json.Unmarshal(rawItems, &items); err != nil {
			return nil, listErr(err)
		}
	}
	want, generic, err := listItemKind(list, listGVK.GroupVersion())
	if err != nil {
		return nil, listErr(err)
	}

	objs := make([]runtime.Object, 0, len(items))
	var errs []error
	for i, item := range items {
		itemErr := func(cause error) error {
			return errors.NewParseError("Kubernetes object",
				fmt.Sprintf("item %d of %s", i, listGVK.Kind), 0, 0, cause)
		}
		switch {
		case bytes.Equal(bytes.TrimSpace(item), []byte("null")):
			errs = append(errs, itemErr(errors.ErrNilRuntimeObject))
		case generic:
			itemObjs, itemErrs := decodeDocument(item, opts, nesting+1)
			objs = append(objs, itemObjs...)
			for _, e := range itemErrs {
				errs = append(errs, itemErr(e))
			}
		default:
			obj, err := decodeTypedItem(item, want)
			if err != nil {
				errs = append(errs, itemErr(err))
				continue
			}
			objs = append(objs, obj)
		}
	}
	return objs, errs
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
// from: a key that differs in case only must not stand in for them. An empty
// or null value counts as left out, as it does for the decoder.
func decodeTypedItem(raw []byte, want schema.GroupVersionKind) (runtime.Object, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	for _, id := range []struct{ key, want string }{
		{"apiVersion", want.GroupVersion().String()},
		{"kind", want.Kind},
	} {
		s, err := stringField(fields, id.key)
		if err != nil {
			return nil, err
		}
		if s != "" && s != id.want {
			return nil, errors.Errorf("the item states %s %q, the list holds %s", id.key, s, want)
		}
	}
	obj, actual, err := kubernetes.Codecs.UniversalDeserializer().Decode(raw, &want, nil)
	if err != nil {
		return nil, err
	}
	if actual == nil || *actual != want {
		return nil, errors.Errorf("the item states %v, the list holds %s", actual, want)
	}
	obj.GetObjectKind().SetGroupVersionKind(want)
	if err := checkType(obj); err != nil {
		return nil, err
	}
	if err := requireObject(obj); err != nil {
		return nil, err
	}
	return obj, nil
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
