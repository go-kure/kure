package io

import (
	"math"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// largeInteger is 2^53 + 1, the first integer a float64 cannot hold: read into
// one it becomes 9007199254740992.
const largeInteger = int64(9007199254740993)

func largeIntegerDeployment() client.Object {
	grace := largeInteger
	d := &appsv1.Deployment{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{Name: "typed", Namespace: "default"},
	}
	d.Spec.Template.Spec.TerminationGracePeriodSeconds = &grace
	return d
}

func largeIntegerWidget(value any) client.Object {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.com/v1",
		"kind":       "Widget",
		"metadata":   map[string]any{"name": "loose"},
		"spec":       map[string]any{"serial": value},
	}}
}

// The writer takes every object through JSON before it writes it. An integer
// must come out as the integer it is, also above 2^53.
func TestEncodeObjectsToYAML_KeepsLargeIntegers(t *testing.T) {
	// Each want is a whole line: the same digits as a quoted string would be
	// another value.
	const (
		wantTyped    = "terminationGracePeriodSeconds: 9007199254740993\n"
		wantLoose    = "serial: 9007199254740993\n"
		wantNegative = "serial: -9007199254740993\n"
	)
	for _, tc := range []struct {
		name string
		obj  client.Object
		opts EncodeOptions
		want string
	}{
		{"typed", largeIntegerDeployment(), EncodeOptions{}, wantTyped},
		{"typed, Kubernetes field order", largeIntegerDeployment(), EncodeOptions{KubernetesFieldOrder: true}, wantTyped},
		{"unstructured", largeIntegerWidget(largeInteger), EncodeOptions{}, wantLoose},
		{"unstructured, Kubernetes field order", largeIntegerWidget(largeInteger), EncodeOptions{KubernetesFieldOrder: true}, wantLoose},
		{"unstructured, no stripping", largeIntegerWidget(largeInteger), EncodeOptions{ServerFieldStripping: StripServerFieldsNone}, wantLoose},
		{"unstructured, no stripping, Kubernetes field order", largeIntegerWidget(largeInteger), EncodeOptions{KubernetesFieldOrder: true, ServerFieldStripping: StripServerFieldsNone}, wantLoose},
		{"negative", largeIntegerWidget(-largeInteger), EncodeOptions{}, wantNegative},
		{"negative, Kubernetes field order", largeIntegerWidget(-largeInteger), EncodeOptions{KubernetesFieldOrder: true}, wantNegative},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := EncodeObjectsToYAMLWithOptions([]*client.Object{&tc.obj}, tc.opts)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(out), tc.want) {
				t.Errorf("want %q in:\n%s", tc.want, out)
			}
		})
	}

	t.Run("EncodeObjectsToYAML", func(t *testing.T) {
		for want, obj := range map[string]client.Object{
			wantTyped: largeIntegerDeployment(),
			wantLoose: largeIntegerWidget(largeInteger),
		} {
			out, err := EncodeObjectsToYAML([]*client.Object{&obj})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(out), want) {
				t.Errorf("want %q in:\n%s", want, out)
			}
		}
	})

	t.Run("parsed and written back", func(t *testing.T) {
		in := "apiVersion: example.com/v1\nkind: Widget\nmetadata:\n  name: loose\nspec:\n  " + wantLoose
		objs, err := ParseYAMLWithOptions([]byte(in), ParseOptions{AllowUnstructured: true})
		if err != nil {
			t.Fatal(err)
		}
		for _, opts := range []EncodeOptions{{}, {KubernetesFieldOrder: true}} {
			out, err := EncodeObjectsToYAMLWithOptions([]*client.Object{&objs[0]}, opts)
			if err != nil {
				t.Fatal(err)
			}
			if string(out) != in {
				t.Errorf("field order %v: a parsed document is written back as another one:\n%s", opts.KubernetesFieldOrder, out)
			}
		}
	})
}

// The bounds of the integer cases, and what they leave alone: a number that is
// not an integer, and one too large for a uint64, are written as they were.
// The two field orders already differ for a float beyond the uint64 range;
// that is recorded here, not changed.
func TestEncodeObjectsToYAML_OtherNumbers(t *testing.T) {
	for _, tc := range []struct {
		name        string
		value       any
		want        string
		wantOrdered string
	}{
		{"small integer", int64(8080), "serial: 8080\n", "serial: 8080\n"},
		{"zero", int64(0), "serial: 0\n", "serial: 0\n"},
		{"integral float", float64(8080), "serial: 8080\n", "serial: 8080\n"},
		{"fraction", 0.5, "serial: 0.5\n", "serial: 0.5\n"},
		{"largest int64", int64(9223372036854775807), "serial: 9223372036854775807\n", "serial: 9223372036854775807\n"},
		{"smallest int64", int64(-9223372036854775808), "serial: -9223372036854775808\n", "serial: -9223372036854775808\n"},
		{"first integer above int64", uint64(9223372036854775808), "serial: 9223372036854775808\n", "serial: 9223372036854775808\n"},
		{"largest uint64", uint64(18446744073709551615), "serial: 18446744073709551615\n", "serial: 18446744073709551615\n"},
		{"float between int64 and uint64", float64(1e19), "serial: 10000000000000000000\n", "serial: 10000000000000000000\n"},
		{"float beyond uint64", float64(1e20), "serial: 1e+20\n", "serial: !!int 100000000000000000000\n"},
		{"float with an exponent", float64(1e21), "serial: 1e+21\n", "serial: !!int 1000000000000000000000\n"},
		// JSON writes negative zero as -0, which reads back as the integer 0.
		{"negative zero", math.Copysign(0, -1), "serial: 0\n", "serial: 0\n"},
	} {
		for _, ordered := range []bool{false, true} {
			name, want := tc.name, tc.want
			if ordered {
				name += ", Kubernetes field order"
				want = tc.wantOrdered
			}
			t.Run(name, func(t *testing.T) {
				obj := largeIntegerWidget(tc.value)
				out, err := EncodeObjectsToYAMLWithOptions([]*client.Object{&obj}, EncodeOptions{KubernetesFieldOrder: ordered})
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(string(out), want) {
					t.Errorf("want %q in:\n%s", want, out)
				}
			})
		}
	}
}

// A status that holds only zero values is stripped whatever Go type the JSON
// round trip gives its numbers.
func TestEncodeObjectsToYAML_ZeroIntegerStatusIsEmpty(t *testing.T) {
	obj := client.Object(&unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.com/v1",
		"kind":       "Widget",
		"metadata":   map[string]any{"name": "loose"},
		"status":     map[string]any{"replicas": int64(0), "nested": map[string]any{"ready": int64(0)}},
	}})
	out, err := EncodeObjectsToYAML([]*client.Object{&obj})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "status") {
		t.Errorf("a status of zero values is written:\n%s", out)
	}

	obj = &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.com/v1",
		"kind":       "Widget",
		"metadata":   map[string]any{"name": "loose"},
		"status":     map[string]any{"replicas": int64(3)},
	}}
	out, err = EncodeObjectsToYAML([]*client.Object{&obj})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "replicas: 3") {
		t.Errorf("a status with a non-zero integer is stripped:\n%s", out)
	}
}
