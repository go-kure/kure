package io

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	reflectorv1 "github.com/fluxcd/image-reflector-controller/api/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	nodev1 "k8s.io/api/node/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	errors2 "github.com/go-kure/kure/pkg/errors"
)

// parseModes are the two modes every list behaviour is asserted in: a list of
// a registered kind never reaches the unstructured fallback, so the result
// must not depend on AllowUnstructured.
var parseModes = []struct {
	name string
	opts ParseOptions
}{
	{"strict", ParseOptions{}},
	{"allow-unstructured", ParseOptions{AllowUnstructured: true}},
}

// describe renders parsed objects as "Kind/name" in order, so a test compares
// the whole sequence and a failure shows it.
func describe(objs []client.Object) []string {
	out := make([]string, 0, len(objs))
	for _, o := range objs {
		out = append(out, o.GetObjectKind().GroupVersionKind().Kind+"/"+o.GetName())
	}
	return out
}

// A typed list of a registered kind is flattened into its items, typed, in the
// list's order. The first item states apiVersion and kind and the second
// leaves them out, as the API server's own list responses do; both come back
// with the kind's GVK. Deployment is the kind that was registered all along,
// the other ten were registered later: the rule is one rule for all of them.
func TestParseYAML_TypedListsAreFlattened(t *testing.T) {
	cases := []struct {
		apiVersion string
		kind       string
		want       client.Object
	}{
		{"apps/v1", "Deployment", &appsv1.Deployment{}},
		{"scheduling.k8s.io/v1", "PriorityClass", &schedulingv1.PriorityClass{}},
		{"discovery.k8s.io/v1", "EndpointSlice", &discoveryv1.EndpointSlice{}},
		{"coordination.k8s.io/v1", "Lease", &coordinationv1.Lease{}},
		{"node.k8s.io/v1", "RuntimeClass", &nodev1.RuntimeClass{}},
		{"admissionregistration.k8s.io/v1", "MutatingWebhookConfiguration", &admissionregistrationv1.MutatingWebhookConfiguration{}},
		{"admissionregistration.k8s.io/v1", "ValidatingWebhookConfiguration", &admissionregistrationv1.ValidatingWebhookConfiguration{}},
		{"admissionregistration.k8s.io/v1", "ValidatingAdmissionPolicy", &admissionregistrationv1.ValidatingAdmissionPolicy{}},
		{"admissionregistration.k8s.io/v1", "ValidatingAdmissionPolicyBinding", &admissionregistrationv1.ValidatingAdmissionPolicyBinding{}},
		{"admissionregistration.k8s.io/v1", "MutatingAdmissionPolicy", &admissionregistrationv1.MutatingAdmissionPolicy{}},
		{"admissionregistration.k8s.io/v1", "MutatingAdmissionPolicyBinding", &admissionregistrationv1.MutatingAdmissionPolicyBinding{}},
		// The kinds registered from a module other than k8s.io/api.
		{"image.toolkit.fluxcd.io/v1", "ImageRepository", &reflectorv1.ImageRepository{}},
		{"image.toolkit.fluxcd.io/v1", "ImagePolicy", &reflectorv1.ImagePolicy{}},
		{"apiregistration.k8s.io/v1", "APIService", &apiregistrationv1.APIService{}},
	}
	for _, c := range cases {
		doc := fmt.Sprintf(`apiVersion: %[1]s
kind: %[2]sList
items:
- apiVersion: %[1]s
  kind: %[2]s
  metadata:
    name: stated
- metadata:
    name: omitted
`, c.apiVersion, c.kind)
		for _, m := range parseModes {
			t.Run(c.kind+"List/"+m.name, func(t *testing.T) {
				objs, err := ParseYAMLWithOptions([]byte(doc), m.opts)
				if err != nil {
					t.Fatalf("parse: %v", err)
				}
				want := []string{c.kind + "/stated", c.kind + "/omitted"}
				if got := describe(objs); !reflect.DeepEqual(got, want) {
					t.Fatalf("objects = %v, want %v", got, want)
				}
				for i, o := range objs {
					if got, wantType := reflect.TypeOf(o), reflect.TypeOf(c.want); got != wantType {
						t.Errorf("item %d parsed as %v, want %v", i, got, wantType)
					}
					if gv := o.GetObjectKind().GroupVersionKind().GroupVersion().String(); gv != c.apiVersion {
						t.Errorf("item %d apiVersion = %q, want %q", i, gv, c.apiVersion)
					}
				}
			})
		}
	}
}

// An empty list is a document with nothing in it: no object, no error. Both
// spellings of "empty" are covered, and both list shapes.
func TestParseYAML_EmptyListYieldsNothing(t *testing.T) {
	docs := map[string]string{
		"typed, items empty":     "apiVersion: apps/v1\nkind: DeploymentList\nitems: []\n",
		"typed, items absent":    "apiVersion: apps/v1\nkind: DeploymentList\n",
		"generic, items empty":   "apiVersion: v1\nkind: List\nitems: []\n",
		"generic, items absent":  "apiVersion: v1\nkind: List\n",
		"typed, later base kind": "apiVersion: scheduling.k8s.io/v1\nkind: PriorityClassList\nitems: []\n",
	}
	for name, doc := range docs {
		for _, m := range parseModes {
			t.Run(name+"/"+m.name, func(t *testing.T) {
				objs, err := ParseYAMLWithOptions([]byte(doc), m.opts)
				if err != nil {
					t.Fatalf("parse: %v", err)
				}
				if len(objs) != 0 {
					t.Fatalf("got %v from an empty list, want nothing", describe(objs))
				}
			})
		}
	}
}

// mixedList is a generic v1 List holding a registered kind, a kind the scheme
// does not know, a typed list and another registered kind, in that order.
const mixedList = `apiVersion: v1
kind: List
items:
- apiVersion: v1
  kind: ConfigMap
  metadata:
    name: cm
- apiVersion: custom.example.com/v1
  kind: Widget
  metadata:
    name: widget
- apiVersion: apps/v1
  kind: DeploymentList
  items:
  - metadata:
      name: nested-a
  - metadata:
      name: nested-b
- apiVersion: v1
  kind: Secret
  metadata:
    name: secret
`

// Each item of a generic List is decoded as a document of its own. With
// AllowUnstructured the unregistered item comes back unstructured, in its
// place, and the nested typed list is flattened where it stands.
func TestParseYAML_GenericListWithMixedItems_AllowUnstructured(t *testing.T) {
	objs, err := ParseYAMLWithOptions([]byte(mixedList), ParseOptions{AllowUnstructured: true})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []string{"ConfigMap/cm", "Widget/widget", "Deployment/nested-a", "Deployment/nested-b", "Secret/secret"}
	if got := describe(objs); !reflect.DeepEqual(got, want) {
		t.Fatalf("objects = %v, want %v", got, want)
	}
	wantTypes := []client.Object{
		&corev1.ConfigMap{}, &unstructured.Unstructured{}, &appsv1.Deployment{}, &appsv1.Deployment{}, &corev1.Secret{},
	}
	for i, o := range objs {
		if got, wantType := reflect.TypeOf(o), reflect.TypeOf(wantTypes[i]); got != wantType {
			t.Errorf("item %d parsed as %v, want %v", i, got, wantType)
		}
	}
}

// A strict parse refuses the unregistered item, names its position in the
// list, and still returns the items that decoded, in order: the contract of a
// multi-document stream, applied to the items of a list.
func TestParseYAML_GenericListWithMixedItems_Strict(t *testing.T) {
	objs, err := ParseYAML([]byte(mixedList))
	if err == nil {
		t.Fatalf("a strict parse must refuse the unregistered item, got %v", describe(objs))
	}
	var parseErrs *errors2.ParseErrors
	if !errors.As(err, &parseErrs) {
		t.Fatalf("error is %T, want *errors.ParseErrors: %v", err, err)
	}
	if len(parseErrs.Errors) != 1 {
		t.Fatalf("got %d errors, want 1: %v", len(parseErrs.Errors), err)
	}
	if want := "item 1 of List"; !strings.Contains(err.Error(), want) {
		t.Errorf("error %q does not name the item (%q)", err, want)
	}
	want := []string{"ConfigMap/cm", "Deployment/nested-a", "Deployment/nested-b", "Secret/secret"}
	if got := describe(objs); !reflect.DeepEqual(got, want) {
		t.Fatalf("objects = %v, want %v", got, want)
	}
}

// The items of a list take the list's place in a multi-document stream: the
// order of the result is the order of the documents, with each list replaced
// by its items in the list's own order.
func TestParseYAML_ListItemsKeepTheStreamOrder(t *testing.T) {
	doc := `apiVersion: v1
kind: ServiceAccount
metadata:
  name: first
---
apiVersion: apps/v1
kind: DeploymentList
items:
- metadata:
    name: typed-b
- metadata:
    name: typed-a
---
apiVersion: v1
kind: List
items:
- apiVersion: v1
  kind: Secret
  metadata:
    name: generic-b
- apiVersion: v1
  kind: ConfigMap
  metadata:
    name: generic-a
---
apiVersion: apps/v1
kind: DeploymentList
items: []
---
apiVersion: v1
kind: Pod
metadata:
  name: last
`
	want := []string{
		"ServiceAccount/first",
		"Deployment/typed-b", "Deployment/typed-a",
		"Secret/generic-b", "ConfigMap/generic-a",
		"Pod/last",
	}
	for _, m := range parseModes {
		t.Run(m.name, func(t *testing.T) {
			objs, err := ParseYAMLWithOptions([]byte(doc), m.opts)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got := describe(objs); !reflect.DeepEqual(got, want) {
				t.Fatalf("objects = %v, want %v", got, want)
			}
		})
	}
}

// A typed list holds one kind. An item that states another kind is refused,
// not returned as the Go type of the list's kind under a kind it is not; the
// items beside it are still returned.
func TestParseYAML_TypedListItemOfAnotherKindIsRefused(t *testing.T) {
	doc := `apiVersion: apps/v1
kind: DeploymentList
items:
- metadata:
    name: good
- apiVersion: apps/v1
  kind: StatefulSet
  metadata:
    name: wrong-kind
`
	for _, m := range parseModes {
		t.Run(m.name, func(t *testing.T) {
			objs, err := ParseYAMLWithOptions([]byte(doc), m.opts)
			if err == nil {
				t.Fatalf("an item of another kind must be refused, got %v", describe(objs))
			}
			var parseErrs *errors2.ParseErrors
			if !errors.As(err, &parseErrs) {
				t.Fatalf("error is %T, want *errors.ParseErrors: %v", err, err)
			}
			if want := "item 1 of DeploymentList"; !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not name the item (%q)", err, want)
			}
			if got, want := describe(objs), []string{"Deployment/good"}; !reflect.DeepEqual(got, want) {
				t.Fatalf("objects = %v, want %v", got, want)
			}
		})
	}
}

// A null item of a generic List is an error naming its position, never a nil
// object in the result.
func TestParseYAML_GenericListNullItemIsRefused(t *testing.T) {
	doc := `apiVersion: v1
kind: List
items:
- apiVersion: v1
  kind: ConfigMap
  metadata:
    name: cm
- null
`
	for _, m := range parseModes {
		t.Run(m.name, func(t *testing.T) {
			objs, err := ParseYAMLWithOptions([]byte(doc), m.opts)
			if err == nil {
				t.Fatalf("a null item must be refused, got %v", describe(objs))
			}
			if want := "item 1 of List"; !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not name the item (%q)", err, want)
			}
			if !errors.Is(err, errors2.ErrNilRuntimeObject) {
				t.Errorf("error %q does not wrap ErrNilRuntimeObject", err)
			}
			if got, want := describe(objs), []string{"ConfigMap/cm"}; !reflect.DeepEqual(got, want) {
				t.Fatalf("objects = %v, want %v", got, want)
			}
		})
	}
}

// Each item of a typed list is decoded by itself. One that does not decode is
// an error naming its position, and the items on both sides of it are still
// returned: decoding the list as a whole would lose all three.
func TestParseYAML_TypedListMalformedItemKeepsItsSiblings(t *testing.T) {
	doc := `apiVersion: apps/v1
kind: DeploymentList
items:
- metadata:
    name: before
- metadata:
    name: malformed
  spec:
    replicas: oops
- metadata:
    name: after
`
	for _, m := range parseModes {
		t.Run(m.name, func(t *testing.T) {
			objs, err := ParseYAMLWithOptions([]byte(doc), m.opts)
			if err == nil {
				t.Fatalf("a malformed item must be refused, got %v", describe(objs))
			}
			var parseErrs *errors2.ParseErrors
			if !errors.As(err, &parseErrs) {
				t.Fatalf("error is %T, want *errors.ParseErrors: %v", err, err)
			}
			if len(parseErrs.Errors) != 1 {
				t.Fatalf("got %d errors, want 1: %v", len(parseErrs.Errors), err)
			}
			if want := "item 1 of DeploymentList"; !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not name the item (%q)", err, want)
			}
			if got, want := describe(objs), []string{"Deployment/before", "Deployment/after"}; !reflect.DeepEqual(got, want) {
				t.Fatalf("objects = %v, want %v", got, want)
			}
		})
	}
}

// A null item of a typed list is an error naming its position, as it is in a
// generic List. Decoding null into the list's Go slice would yield an empty
// object of the list's kind, which nothing in the document stated.
func TestParseYAML_TypedListNullItemIsRefused(t *testing.T) {
	docs := map[string]string{
		"yaml": "apiVersion: apps/v1\nkind: DeploymentList\nitems:\n- metadata:\n    name: good\n- null\n",
		"json": `{"apiVersion":"apps/v1","kind":"DeploymentList","items":[{"metadata":{"name":"good"}},null]}`,
	}
	for name, doc := range docs {
		for _, m := range parseModes {
			t.Run(name+"/"+m.name, func(t *testing.T) {
				objs, err := ParseYAMLWithOptions([]byte(doc), m.opts)
				if err == nil {
					t.Fatalf("a null item must be refused, got %v", describe(objs))
				}
				if want := "item 1 of DeploymentList"; !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not name the item (%q)", err, want)
				}
				if !errors.Is(err, errors2.ErrNilRuntimeObject) {
					t.Errorf("error %q does not wrap ErrNilRuntimeObject", err)
				}
				if got, want := describe(objs), []string{"Deployment/good"}; !reflect.DeepEqual(got, want) {
					t.Fatalf("objects = %v, want %v", got, want)
				}
			})
		}
	}
}

// An item of a typed list may state part of its identity. What it leaves out
// is the list's; what it states must be the list's as written, so the right
// kind at another version is refused like another kind is, and so is an
// apiVersion that names the group and no version: the decoder would fill the
// version in and return the item as if it had stated the list's. The last two
// items state another identity under apiVersion and kind and the list's under
// the same key in another case; the exact keys are the ones that count.
func TestParseYAML_TypedListItemStatingPartOfItsIdentity(t *testing.T) {
	doc := `apiVersion: apps/v1
kind: DeploymentList
items:
- kind: Deployment
  metadata:
    name: kind-only
- apiVersion: apps/v1
  metadata:
    name: version-only
- apiVersion: apps/v1beta1
  kind: Deployment
  metadata:
    name: other-version
- apiVersion: apps/
  kind: Deployment
  metadata:
    name: group-without-version
- apiVersion: apps/
  APIVersion: apps/v1
  metadata:
    name: version-under-another-case
- kind: StatefulSet
  Kind: Deployment
  metadata:
    name: kind-under-another-case
`
	for _, m := range parseModes {
		t.Run(m.name, func(t *testing.T) {
			objs, err := ParseYAMLWithOptions([]byte(doc), m.opts)
			if err == nil {
				t.Fatalf("an item at another version must be refused, got %v", describe(objs))
			}
			var parseErrs *errors2.ParseErrors
			if !errors.As(err, &parseErrs) {
				t.Fatalf("error is %T, want *errors.ParseErrors: %v", err, err)
			}
			if len(parseErrs.Errors) != 4 {
				t.Fatalf("got %d errors, want 4: %v", len(parseErrs.Errors), err)
			}
			for _, want := range []string{
				"item 2 of DeploymentList", "item 3 of DeploymentList",
				"item 4 of DeploymentList", "item 5 of DeploymentList",
			} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not name the item (%q)", err, want)
				}
			}
			if got, want := describe(objs), []string{"Deployment/kind-only", "Deployment/version-only"}; !reflect.DeepEqual(got, want) {
				t.Fatalf("objects = %v, want %v", got, want)
			}
			for i, o := range objs {
				if gv := o.GetObjectKind().GroupVersionKind().GroupVersion().String(); gv != "apps/v1" {
					t.Errorf("item %d apiVersion = %q, want apps/v1", i, gv)
				}
			}
		})
	}
}

// A registered kind that is not an object with metadata of its own, v1 Status
// for one, is refused as an item of a List with its position, like any other
// item that cannot be returned, and the items beside it are still returned.
func TestParseYAML_GenericListNonObjectItemIsRefused(t *testing.T) {
	doc := `apiVersion: v1
kind: List
items:
- apiVersion: v1
  kind: ConfigMap
  metadata:
    name: cm
- apiVersion: v1
  kind: Status
  status: Failure
`
	for _, m := range parseModes {
		t.Run(m.name, func(t *testing.T) {
			objs, err := ParseYAMLWithOptions([]byte(doc), m.opts)
			if err == nil {
				t.Fatalf("a Status item must be refused, got %v", describe(objs))
			}
			var parseErrs *errors2.ParseErrors
			if !errors.As(err, &parseErrs) {
				t.Fatalf("error is %T, want *errors.ParseErrors: %v", err, err)
			}
			if len(parseErrs.Errors) != 1 {
				t.Fatalf("got %d errors, want 1: %v", len(parseErrs.Errors), err)
			}
			for _, want := range []string{"item 1 of List", "does not implement client.Object"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not say %q", err, want)
				}
			}
			if got, want := describe(objs), []string{"ConfigMap/cm"}; !reflect.DeepEqual(got, want) {
				t.Fatalf("objects = %v, want %v", got, want)
			}
		})
	}
}

// A list is recognised, and its items are read, under the exact keys
// apiVersion, kind and items. A key that differs in case only is not one of
// them: it does not replace a list's items, does not hide that a document is a
// list, and does not turn a document of another kind into an empty list, which
// would return nothing and no error for a document that is refused otherwise.
func TestParseYAML_ListIsRecognisedByItsExactKeys(t *testing.T) {
	flattened := map[string]string{
		"typed list, Items beside items":   `{"apiVersion":"apps/v1","kind":"DeploymentList","items":[{"metadata":{"name":"good"}}],"Items":null}`,
		"generic List, Items beside items": `{"apiVersion":"v1","kind":"List","items":[{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"good"}}],"Items":null}`,
		"typed list, Kind beside kind":     `{"apiVersion":"apps/v1","kind":"DeploymentList","Kind":"Deployment","items":[{"metadata":{"name":"good"}}]}`,
	}
	for name, doc := range flattened {
		for _, m := range parseModes {
			t.Run(name+"/"+m.name, func(t *testing.T) {
				objs, err := ParseYAMLWithOptions([]byte(doc), m.opts)
				if err != nil {
					t.Fatalf("parse: %v", err)
				}
				if got, want := describe(objs), []string{"Deployment/good"}; !reflect.DeepEqual(got, want) {
					t.Fatalf("objects = %v, want %v", got, want)
				}
			})
		}
	}

	notAList := `{"apiVersion":"v1","kind":"ConfigMap","Kind":"List","metadata":{"name":"cm"}}`
	for _, m := range parseModes {
		t.Run("ConfigMap, Kind List beside kind/"+m.name, func(t *testing.T) {
			objs, err := ParseYAMLWithOptions([]byte(notAList), m.opts)
			if err == nil {
				t.Fatalf("the document must be refused as it was before lists were flattened, got %v", describe(objs))
			}
			if len(objs) != 0 {
				t.Fatalf("got %v, want nothing", describe(objs))
			}
		})
	}
}

// nestedLists wraps one ConfigMap in the given number of generic Lists.
func nestedLists(lists int) string {
	doc := `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"innermost"}}`
	for range lists {
		doc = `{"apiVersion":"v1","kind":"List","items":[` + doc + `]}`
	}
	return doc
}

// A list inside a generic List is flattened where it stands, but not to any
// depth: every level decodes what it holds again, so the depth is bounded. A
// list may sit inside maxListNesting Lists; one List more is refused, with no
// object returned for what it held.
func TestParseYAML_NestedListsAreBounded(t *testing.T) {
	for _, m := range parseModes {
		t.Run("at the bound/"+m.name, func(t *testing.T) {
			objs, err := ParseYAMLWithOptions([]byte(nestedLists(maxListNesting+1)), m.opts)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if got, want := describe(objs), []string{"ConfigMap/innermost"}; !reflect.DeepEqual(got, want) {
				t.Fatalf("objects = %v, want %v", got, want)
			}
		})
		t.Run("past the bound/"+m.name, func(t *testing.T) {
			objs, err := ParseYAMLWithOptions([]byte(nestedLists(maxListNesting+2)), m.opts)
			if err == nil {
				t.Fatalf("lists nested past the bound must be refused, got %v", describe(objs))
			}
			if want := fmt.Sprintf("nested more than %d lists deep", maxListNesting); !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not state the bound (%q)", err, want)
			}
			if len(objs) != 0 {
				t.Fatalf("got %v, want nothing", describe(objs))
			}
		})
	}
}
