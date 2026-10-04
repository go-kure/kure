package io

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
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
	modes := []struct {
		name string
		opts ParseOptions
	}{
		{"strict", ParseOptions{}},
		{"allow-unstructured", ParseOptions{AllowUnstructured: true}},
	}
	for _, c := range cases {
		doc := fmt.Sprintf("apiVersion: %s\nkind: %s\nmetadata:\n  name: sample\n", c.apiVersion, c.kind)
		for _, m := range modes {
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

// Registering a kind registers its list type with it, and a typed list is not a
// client.Object: the parser refuses it in both modes. Before these kinds were
// registered, AllowUnstructured flattened such a list into its items, so this is
// the second thing the registration changed for a caller. DeploymentList is the
// control: it was registered all along and is refused the same way, which is
// what makes this the rule for registered kinds and not a defect of the ten.
func TestParseYAML_BaseKindListsAreRefusedLikeEveryRegisteredList(t *testing.T) {
	cases := []struct {
		apiVersion string
		kind       string
	}{
		{"apps/v1", "Deployment"},
		{"scheduling.k8s.io/v1", "PriorityClass"},
		{"discovery.k8s.io/v1", "EndpointSlice"},
		{"coordination.k8s.io/v1", "Lease"},
		{"node.k8s.io/v1", "RuntimeClass"},
		{"admissionregistration.k8s.io/v1", "MutatingWebhookConfiguration"},
		{"admissionregistration.k8s.io/v1", "ValidatingWebhookConfiguration"},
		{"admissionregistration.k8s.io/v1", "ValidatingAdmissionPolicy"},
		{"admissionregistration.k8s.io/v1", "ValidatingAdmissionPolicyBinding"},
		{"admissionregistration.k8s.io/v1", "MutatingAdmissionPolicy"},
		{"admissionregistration.k8s.io/v1", "MutatingAdmissionPolicyBinding"},
	}
	modes := []struct {
		name string
		opts ParseOptions
	}{
		{"strict", ParseOptions{}},
		{"allow-unstructured", ParseOptions{AllowUnstructured: true}},
	}
	for _, c := range cases {
		doc := fmt.Sprintf("apiVersion: %[1]s\nkind: %[2]sList\nitems:\n- apiVersion: %[1]s\n  kind: %[2]s\n  metadata:\n    name: sample\n", c.apiVersion, c.kind)
		for _, m := range modes {
			t.Run(c.kind+"List/"+m.name, func(t *testing.T) {
				objs, err := ParseYAMLWithOptions([]byte(doc), m.opts)
				if err == nil {
					t.Fatalf("a typed list must be refused, got %d objects", len(objs))
				}
				var parseErrs *errors2.ParseErrors
				if !errors.As(err, &parseErrs) {
					t.Fatalf("error is %T, want *errors.ParseErrors: %v", err, err)
				}
				if len(objs) != 0 {
					t.Errorf("got %d objects from a refused list, want 0", len(objs))
				}
				if want := "List does not implement client.Object"; !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not say the list is not an object (%q)", err, want)
				}
			})
		}
	}
}

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
