package kubernetes

import (
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCreate_InfersPointerTypeAndSetsIdentity(t *testing.T) {
	// One type argument: PT is inferred from *T (core-type inference).
	d := Create[appsv1.Deployment]("web", "default")

	if d.APIVersion != "apps/v1" || d.Kind != "Deployment" {
		t.Errorf("TypeMeta = %s/%s, want apps/v1/Deployment", d.APIVersion, d.Kind)
	}
	if d.Name != "web" || d.Namespace != "default" {
		t.Errorf("identity = %s/%s, want default/web", d.Namespace, d.Name)
	}
}

func TestCreate_EmitsNothingBeyondIdentity(t *testing.T) {
	got := Create[appsv1.Deployment]("web", "default")

	want := &appsv1.Deployment{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Create injected values beyond identity:\n got %+v\nwant %+v", got, want)
	}
	if got.Labels != nil || got.Annotations != nil || got.Spec.Selector != nil {
		t.Errorf("labels/annotations/selector must stay nil: %+v", got.ObjectMeta)
	}
}

func TestCreate_ClusterScopedTakesEmptyNamespace(t *testing.T) {
	ns := Create[corev1.Namespace]("platform", "")

	if ns.APIVersion != "v1" || ns.Kind != "Namespace" || ns.Name != "platform" || ns.Namespace != "" {
		t.Errorf("unexpected object %+v", ns)
	}
}

// unregistered is a client.Object that no scheme knows about.
type unregistered struct {
	metav1.TypeMeta
	metav1.ObjectMeta
}

func (u *unregistered) DeepCopyObject() runtime.Object {
	c := *u
	return &c
}

func TestCreate_PanicsOnUnregisteredType(t *testing.T) {
	assertPanics(t, func() { Create[unregistered]("a", "b") })
}
