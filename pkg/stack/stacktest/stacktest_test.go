package stacktest_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
	"github.com/go-kure/kure/pkg/stack/stacktest"
)

// genFunc is a config whose Generate is the function itself.
type genFunc func() ([]*client.Object, error)

func (f genFunc) Generate(*stack.Application) ([]*client.Object, error) { return f() }

// calls counts the calls of a genFunc, so a case can return something
// different from its second call.
func calls(f func(call int) []*client.Object) genFunc {
	n := 0
	return func() ([]*client.Object, error) {
		n++
		return f(n), nil
	}
}

func objs(os ...client.Object) []*client.Object {
	out := make([]*client.Object, len(os))
	for i, o := range os {
		out[i] = &o
	}
	return out
}

func app(cfg stack.ApplicationConfig) *stack.Application {
	return stack.NewApplication("web", "default", cfg)
}

// customSpec is a pointer target that is not an object.
type customSpec struct {
	Count  int32
	Labels map[string]string
}

// custom is a typed object outside any scheme, with fields a scheme type
// would not have: a zero-size pointee, an unexported map and a func.
type custom struct {
	metav1.TypeMeta
	metav1.ObjectMeta
	Spec   *customSpec
	Count  *int32
	Marker *struct{}
	notes  map[string]string
	hook   func()
}

func (c *custom) DeepCopyObject() runtime.Object {
	out := *c
	return &out
}

func freshDeployment() *appsv1.Deployment {
	dep := &appsv1.Deployment{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Labels: map[string]string{"app": "web"}, CreationTimestamp: metav1.Now()},
	}
	dep.Spec.Replicas = new(int32)
	dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}}
	dep.Spec.Template.Labels = map[string]string{"app": "web"}
	dep.Spec.Template.Spec.Containers = []corev1.Container{{
		Name:  "web",
		Image: "nginx",
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")},
		},
	}}
	return dep
}

func freshConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Data:       map[string]string{"key": "value"},
	}
}

func freshUnstructured() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.com/v1",
		"kind":       "Widget",
		"metadata":   map[string]any{"name": "web", "labels": map[string]any{"app": "web"}},
		"spec":       map[string]any{"sizes": []any{int64(1), int64(2)}},
	}}
}

func TestCheckGeneration_FreshConfigPasses(t *testing.T) {
	cfg := genFunc(func() ([]*client.Object, error) {
		svc := &corev1.Service{
			TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{"app": "web"},
				Ports:    []corev1.ServicePort{{Port: 80}},
			},
		}
		c := &custom{Spec: &customSpec{Labels: map[string]string{"a": "b"}}, Marker: &struct{}{}, notes: map[string]string{}}
		return objs(freshDeployment(), svc, freshConfigMap(), freshUnstructured(), c), nil
	})
	report := stacktest.AssertGeneration(t, app(cfg))
	if report == nil {
		t.Fatal("no report")
	}
	if len(report.Shared) != 0 || len(report.Internal) != 0 {
		t.Errorf("report = %+v, want nothing shared", report)
	}
}

func TestCheckGeneration_SharingConfigFails(t *testing.T) {
	cases := []struct {
		name         string
		cfg          func() genFunc
		wantShared   []stacktest.Sharing
		wantInternal []stacktest.Sharing
	}{
		{
			name: "cached object",
			cfg: func() genFunc {
				dep := freshDeployment()
				return calls(func(int) []*client.Object { return objs(dep) })
			},
			wantShared: []stacktest.Sharing{{Kind: "object", First: "[0].(*v1.Deployment)", Second: "[0].(*v1.Deployment)"}},
		},
		{
			name: "fresh object around a cached labels map",
			cfg: func() genFunc {
				labels := map[string]string{"app": "web"}
				return calls(func(int) []*client.Object {
					cm := freshConfigMap()
					cm.Labels = labels
					return objs(cm)
				})
			},
			wantShared: []stacktest.Sharing{{Kind: "map", First: "[0].(*v1.ConfigMap).ObjectMeta.Labels", Second: "[0].(*v1.ConfigMap).ObjectMeta.Labels"}},
		},
		{
			name: "cached slice re-sliced",
			cfg: func() genFunc {
				ports := []corev1.ServicePort{{Port: 80}, {Port: 443}}
				return calls(func(call int) []*client.Object {
					svc := &corev1.Service{}
					// Disjoint views, one backing array: an append to
					// the first would overwrite the second.
					if call == 1 {
						svc.Spec.Ports = ports[:1]
					} else {
						svc.Spec.Ports = ports[1:]
					}
					return objs(svc)
				})
			},
			wantShared: []stacktest.Sharing{{Kind: "slice", First: "[0].(*v1.Service).Spec.Ports", Second: "[0].(*v1.Service).Spec.Ports"}},
		},
		{
			name: "pointer to a config field",
			cfg: func() genFunc {
				replicas := struct{ Replicas int32 }{Replicas: 2}
				return calls(func(int) []*client.Object {
					dep := freshDeployment()
					dep.Spec.Replicas = &replicas.Replicas
					return objs(dep)
				})
			},
			wantShared: []stacktest.Sharing{{Kind: "pointer", First: "[0].(*v1.Deployment).Spec.Replicas", Second: "[0].(*v1.Deployment).Spec.Replicas"}},
		},
		{
			name: "the same result slice",
			cfg: func() genFunc {
				out := objs(freshConfigMap())
				return calls(func(int) []*client.Object { return out })
			},
			wantShared: []stacktest.Sharing{{Kind: "slice", First: "", Second: ""}},
		},
		{
			name: "map inside an unstructured object",
			cfg: func() genFunc {
				spec := map[string]any{"replicas": int64(1)}
				return calls(func(int) []*client.Object {
					u := freshUnstructured()
					u.Object["spec"] = spec
					return objs(u)
				})
			},
			wantShared: []stacktest.Sharing{{
				Kind:   "map",
				First:  "[0].(*unstructured.Unstructured).Object[spec].(map[string]interface {})",
				Second: "[0].(*unstructured.Unstructured).Object[spec].(map[string]interface {})",
			}},
		},
		{
			name: "pointer into the other result's slice storage",
			cfg: func() genFunc {
				groups := []int64{1000, 2000}
				return calls(func(call int) []*client.Object {
					pod := &corev1.Pod{Spec: corev1.PodSpec{SecurityContext: &corev1.PodSecurityContext{}}}
					if call == 1 {
						pod.Spec.SecurityContext.SupplementalGroups = groups
					} else {
						pod.Spec.SecurityContext.RunAsUser = &groups[1]
					}
					return objs(pod)
				})
			},
			wantShared: []stacktest.Sharing{{
				Kind:   "slice",
				First:  "[0].(*v1.Pod).Spec.SecurityContext.SupplementalGroups",
				Second: "[0].(*v1.Pod).Spec.SecurityContext.RunAsUser",
			}},
		},
		{
			name: "longer view of storage already walked",
			cfg: func() genFunc {
				cached := map[string]any{"cached": "yes"}
				return calls(func(int) []*client.Object {
					items := []any{map[string]any{"fresh": "yes"}, cached}
					// The short view is walked first; the longer one
					// still has to be walked to reach the cached map.
					return objs(&unstructured.Unstructured{Object: map[string]any{"a": items[:1], "b": items[:2]}})
				})
			},
			wantShared: []stacktest.Sharing{{
				Kind:   "map",
				First:  "[0].(*unstructured.Unstructured).Object[b].([]interface {})[1].(map[string]interface {})",
				Second: "[0].(*unstructured.Unstructured).Object[b].([]interface {})[1].(map[string]interface {})",
			}},
			wantInternal: []stacktest.Sharing{{
				Kind:   "slice",
				First:  "[0].(*unstructured.Unstructured).Object[a].([]interface {})",
				Second: "[0].(*unstructured.Unstructured).Object[b].([]interface {})",
			}},
		},
		{
			name: "pointer into a struct beside its cached map",
			cfg: func() genFunc {
				spec := &customSpec{Count: 1, Labels: map[string]string{"app": "web"}}
				return calls(func(call int) []*client.Object {
					if call == 1 {
						return objs(&custom{Spec: spec})
					}
					return objs(&custom{Spec: &customSpec{Labels: spec.Labels}, Count: &spec.Count})
				})
			},
			wantShared: []stacktest.Sharing{
				{Kind: "pointer", First: "[0].(*stacktest_test.custom).Spec", Second: "[0].(*stacktest_test.custom).Count"},
				{Kind: "map", First: "[0].(*stacktest_test.custom).Spec.Labels", Second: "[0].(*stacktest_test.custom).Spec.Labels"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report, err := stacktest.CheckGeneration(app(tc.cfg()))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(report.Shared, tc.wantShared) {
				t.Errorf("Shared:\n got %+v\nwant %+v", report.Shared, tc.wantShared)
			}
			if !reflect.DeepEqual(report.Internal, tc.wantInternal) {
				t.Errorf("Internal:\n got %+v\nwant %+v", report.Internal, tc.wantInternal)
			}
		})
	}
}

// fakeTB records what AssertGeneration reports.
type fakeTB struct {
	testing.TB
	errors []string
}

func (f *fakeTB) Helper() {}

func (f *fakeTB) Errorf(format string, args ...any) {
	f.errors = append(f.errors, fmt.Sprintf(format, args...))
}

func TestAssertGeneration_FailsOnSharing(t *testing.T) {
	dep := freshDeployment()
	cached := calls(func(int) []*client.Object { return objs(dep) })
	tb := &fakeTB{}
	if stacktest.AssertGeneration(tb, app(cached)) == nil {
		t.Fatal("no report")
	}
	if len(tb.errors) != 1 || !strings.Contains(tb.errors[0], `"[0].(*v1.Deployment)"`) {
		t.Errorf("errors = %q, want one naming the shared Deployment", tb.errors)
	}

	tb = &fakeTB{}
	if stacktest.AssertGeneration(tb, app(nil)) != nil {
		t.Error("a report after an error")
	}
	if len(tb.errors) != 1 || !strings.Contains(tb.errors[0], "call 1") {
		t.Errorf("errors = %q, want one naming the failed call", tb.errors)
	}
}

func TestCheckGeneration_Cycles(t *testing.T) {
	cfg := genFunc(func() ([]*client.Object, error) {
		m := map[string]any{"kind": "Loop"}
		m["self"] = m
		s := make([]any, 1)
		s[0] = s
		m["list"] = s
		return objs(&unstructured.Unstructured{Object: m}), nil
	})
	report, err := stacktest.CheckGeneration(app(cfg))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Shared) != 0 || len(report.Internal) != 0 {
		t.Errorf("report = %+v, want nothing shared: a value that contains itself is not reported", report)
	}
}

func TestCheckGeneration_DeepCopyCachePasses(t *testing.T) {
	cached := freshDeployment()
	cfg := genFunc(func() ([]*client.Object, error) {
		return objs(cached.DeepCopy()), nil
	})
	stacktest.AssertGeneration(t, app(cfg))
}

func TestCheckGeneration_WithinOneResult(t *testing.T) {
	cfg := genFunc(func() ([]*client.Object, error) {
		dep := freshDeployment()
		dep.Spec.Selector.MatchLabels = dep.Labels
		return objs(dep), nil
	})
	report, err := stacktest.CheckGeneration(app(cfg))
	if err != nil {
		t.Fatal(err)
	}
	want := []stacktest.Sharing{{
		Kind:   "map",
		First:  "[0].(*v1.Deployment).ObjectMeta.Labels",
		Second: "[0].(*v1.Deployment).Spec.Selector.MatchLabels",
	}}
	if len(report.Shared) != 0 {
		t.Errorf("Shared = %+v, want none", report.Shared)
	}
	if !reflect.DeepEqual(report.Internal, want) {
		t.Errorf("Internal:\n got %+v\nwant %+v", report.Internal, want)
	}
}

// failingValidator fails validation.
type failingValidator struct{ genFunc }

func (failingValidator) Validate() error { return errors.New("invalid config") }

func TestCheckGeneration_Errors(t *testing.T) {
	fresh := func() ([]*client.Object, error) { return objs(freshConfigMap()), nil }
	cases := []struct {
		name string
		app  *stack.Application
		want string
	}{
		{"nil application", nil, "nil application"},
		{"nil config", app(nil), "call 1"},
		{"validation", app(failingValidator{genFunc(fresh)}), "call 1: validation failed"},
		{"func value", app(genFunc(func() ([]*client.Object, error) {
			return objs(&custom{hook: func() {}}), nil
		})), `unsupported func value at "[0].(*stacktest_test.custom).hook"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report, err := stacktest.CheckGeneration(tc.app)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want one containing %q", err, tc.want)
			}
			if report != nil {
				t.Errorf("report = %+v, want none after an error", report)
			}
		})
	}

	t.Run("second call", func(t *testing.T) {
		cfg := genFunc(func() func() ([]*client.Object, error) {
			n := 0
			return func() ([]*client.Object, error) {
				if n++; n == 2 {
					return nil, errors.New("second call failed")
				}
				return objs(freshConfigMap()), nil
			}
		}())
		report, err := stacktest.CheckGeneration(app(cfg))
		if err == nil || !strings.Contains(err.Error(), "call 2: second call failed") {
			t.Errorf("err = %v, want the second call's error", err)
		}
		if report != nil {
			t.Errorf("report = %+v, want none after an error", report)
		}
	})
}

func TestCheckGeneration_UnexportedFields(t *testing.T) {
	notes := map[string]string{"cached": "yes"}
	cfg := genFunc(func() ([]*client.Object, error) {
		return objs(&custom{notes: notes}), nil
	})
	report, err := stacktest.CheckGeneration(app(cfg))
	if err != nil {
		t.Fatal(err)
	}
	want := []stacktest.Sharing{{Kind: "map", First: "[0].(*stacktest_test.custom).notes", Second: "[0].(*stacktest_test.custom).notes"}}
	if !reflect.DeepEqual(report.Shared, want) {
		t.Errorf("Shared:\n got %+v\nwant %+v", report.Shared, want)
	}
}

// fullConfig implements every optional interface.
type fullConfig struct{}

func (fullConfig) Generate(*stack.Application) ([]*client.Object, error) {
	return objs(freshConfigMap()), nil
}
func (fullConfig) Validate() error                            { return nil }
func (fullConfig) AugmentLayout(*layout.ManifestLayout) error { return nil }
func (fullConfig) WantsOwnLayout() bool                       { return true }

type embeddingWrapper struct{ stack.ApplicationConfig }

type concreteWrapper struct{ fullConfig }

type forwardingWrapper struct{ inner fullConfig }

func (w forwardingWrapper) Generate(app *stack.Application) ([]*client.Object, error) {
	return w.inner.Generate(app)
}
func (w forwardingWrapper) Validate() error { return w.inner.Validate() }
func (w forwardingWrapper) AugmentLayout(ml *layout.ManifestLayout) error {
	return w.inner.AugmentLayout(ml)
}
func (w forwardingWrapper) WantsOwnLayout() bool { return w.inner.WantsOwnLayout() }

type augmentOnlyWrapper struct{ inner fullConfig }

func (w augmentOnlyWrapper) Generate(app *stack.Application) ([]*client.Object, error) {
	return w.inner.Generate(app)
}
func (w augmentOnlyWrapper) AugmentLayout(ml *layout.ManifestLayout) error {
	return w.inner.AugmentLayout(ml)
}

func TestInterfacesOf_Wrappers(t *testing.T) {
	want := stacktest.InterfacesOf(fullConfig{})
	if want != (stacktest.Interfaces{Validator: true, LayoutAugmenter: true, LayoutIntentAugmenter: true}) {
		t.Fatalf("InterfacesOf(fullConfig) = %+v, want all three", want)
	}
	cases := []struct {
		name    string
		wrapper stack.ApplicationConfig
		missing []string
	}{
		{"embeds the interface", embeddingWrapper{fullConfig{}}, []string{"stack.Validator", "layout.LayoutAugmenter", "layout.LayoutIntentAugmenter"}},
		{"embeds the concrete type", concreteWrapper{}, nil},
		{"forwards each interface", forwardingWrapper{}, nil},
		{"forwards AugmentLayout only", augmentOnlyWrapper{}, []string{"stack.Validator", "layout.LayoutIntentAugmenter"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stacktest.InterfacesOf(tc.wrapper).Missing(want)
			if !reflect.DeepEqual(got, tc.missing) {
				t.Errorf("Missing = %q, want %q", got, tc.missing)
			}
		})
	}
	if got := stacktest.InterfacesOf(nil); got != (stacktest.Interfaces{}) {
		t.Errorf("InterfacesOf(nil) = %+v, want none", got)
	}
}
