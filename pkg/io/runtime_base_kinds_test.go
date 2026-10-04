package io

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	nodev1 "k8s.io/api/node/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	errors2 "github.com/go-kure/kure/pkg/errors"
)

// The base kinds registered from scheduling/v1, discovery/v1, coordination/v1,
// node/v1 and admissionregistration/v1 parse to their upstream Go types. Before
// they were registered a strict parse refused them and a parse with
// AllowUnstructured returned *unstructured.Unstructured, so both modes are
// asserted: the second is the one a caller's type switch notices.
func TestParseYAML_BaseKindsAreTyped(t *testing.T) {
	cases := []struct {
		apiVersion string
		kind       string
		want       client.Object
	}{
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
	}
	for _, c := range cases {
		doc := fmt.Sprintf("apiVersion: %s\nkind: %s\nmetadata:\n  name: sample\n", c.apiVersion, c.kind)
		for _, m := range parseModes {
			t.Run(c.kind+"/"+m.name, func(t *testing.T) {
				objs, err := ParseYAMLWithOptions([]byte(doc), m.opts)
				if err != nil {
					t.Fatalf("parse: %v", err)
				}
				if len(objs) != 1 {
					t.Fatalf("got %d objects, want 1", len(objs))
				}
				if got, want := reflect.TypeOf(objs[0]), reflect.TypeOf(c.want); got != want {
					t.Errorf("parsed as %v, want %v", got, want)
				}
				if objs[0].GetName() != "sample" {
					t.Errorf("name = %q, want %q", objs[0].GetName(), "sample")
				}
				gvk := objs[0].GetObjectKind().GroupVersionKind()
				if gvk.GroupVersion().String() != c.apiVersion || gvk.Kind != c.kind {
					t.Errorf("GVK = %s, want %s %s", gvk, c.apiVersion, c.kind)
				}
			})
		}
	}
}

// Registering a kind registers its list type with it. What the parser does
// with a list of these kinds is asserted in runtime_lists_test.go, next to the
// control that was registered all along.

// APIService is a base kind kure does not register, because its Go type lives in
// a module kure does not depend on. It is the control for the test above: a
// strict parse still refuses it, so "typed" there is a statement about the
// registered kinds and not about every built-in.
func TestParseYAML_UnregisteredBaseKindIsStillRefused(t *testing.T) {
	doc := "apiVersion: apiregistration.k8s.io/v1\nkind: APIService\nmetadata:\n  name: v1.example.com\n"
	_, err := ParseYAML([]byte(doc))
	if err == nil {
		t.Fatal("a strict parse must refuse a kind the scheme does not register")
	}
	var parseErrs *errors2.ParseErrors
	if !errors.As(err, &parseErrs) {
		t.Fatalf("error is %T, want *errors.ParseErrors: %v", err, err)
	}
}
