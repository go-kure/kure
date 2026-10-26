package stack

import (
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestBundleGenerate_SharedMapIsNotWritten: a config that uses one map as a
// Deployment's labels, its selector and its pod template labels keeps that
// map as it was. The Deployment's own labels get the bundle's, the selector
// and the template do not (go-kure/kure#1083).
func TestBundleGenerate_SharedMapIsNotWritten(t *testing.T) {
	shared := map[string]string{"app": "web"}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Labels: shared, Annotations: shared},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: shared},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: shared}},
		},
	}
	obj := client.Object(dep)
	b := &Bundle{
		Name:         "web",
		Applications: []*Application{NewApplication("web", "default", &fakeConfig{objs: []*client.Object{&obj}})},
		Labels:       map[string]string{"team": "shop", "app": "bundle"},
		Annotations:  map[string]string{"owner": "shop"},
	}
	if _, err := b.Generate(); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	if want := map[string]string{"app": "web"}; !reflect.DeepEqual(shared, want) {
		t.Errorf("the config's map = %v, want %v unchanged", shared, want)
	}
	if want := map[string]string{"app": "web"}; !reflect.DeepEqual(dep.Spec.Selector.MatchLabels, want) {
		t.Errorf("selector = %v, want %v", dep.Spec.Selector.MatchLabels, want)
	}
	if want := map[string]string{"app": "web"}; !reflect.DeepEqual(dep.Spec.Template.Labels, want) {
		t.Errorf("pod template labels = %v, want %v", dep.Spec.Template.Labels, want)
	}
	if want := map[string]string{"app": "web", "team": "shop"}; !reflect.DeepEqual(dep.Labels, want) {
		t.Errorf("Deployment labels = %v, want %v (its own app wins)", dep.Labels, want)
	}
	if want := map[string]string{"app": "web", "owner": "shop"}; !reflect.DeepEqual(dep.Annotations, want) {
		t.Errorf("Deployment annotations = %v, want %v", dep.Annotations, want)
	}
	if want := map[string]string{"team": "shop", "app": "bundle"}; !reflect.DeepEqual(b.Labels, want) {
		t.Errorf("bundle labels = %v, want %v unchanged", b.Labels, want)
	}
}

// TestBundleGenerate_ObjectsDoNotShareBundleMaps: two objects without labels
// or annotations each get a map of their own, and neither is the bundle's.
func TestBundleGenerate_ObjectsDoNotShareBundleMaps(t *testing.T) {
	obj1 := client.Object(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "a"}})
	obj2 := client.Object(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "b"}})
	b := &Bundle{
		Name:         "web",
		Applications: []*Application{NewApplication("web", "default", &fakeConfig{objs: []*client.Object{&obj1, &obj2}})},
		Labels:       map[string]string{"team": "shop"},
		Annotations:  map[string]string{"owner": "shop"},
	}
	res, err := b.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	(*res[0]).GetLabels()["edited"] = "x"
	(*res[0]).GetAnnotations()["edited"] = "x"

	if want := map[string]string{"team": "shop"}; !reflect.DeepEqual((*res[1]).GetLabels(), want) {
		t.Errorf("second object's labels = %v, want %v", (*res[1]).GetLabels(), want)
	}
	if want := map[string]string{"owner": "shop"}; !reflect.DeepEqual((*res[1]).GetAnnotations(), want) {
		t.Errorf("second object's annotations = %v, want %v", (*res[1]).GetAnnotations(), want)
	}
	if want := map[string]string{"team": "shop"}; !reflect.DeepEqual(b.Labels, want) {
		t.Errorf("bundle labels = %v, want %v", b.Labels, want)
	}
	if want := map[string]string{"owner": "shop"}; !reflect.DeepEqual(b.Annotations, want) {
		t.Errorf("bundle annotations = %v, want %v", b.Annotations, want)
	}
}

// TestBundleGenerate_NilEntrySkipped: a nil entry, an entry holding a nil
// object and one holding a nil pointer of an object type are returned as they
// are and get no labels, with no panic; the other objects still get the
// bundle's labels and annotations.
func TestBundleGenerate_NilEntrySkipped(t *testing.T) {
	var nilObj client.Object
	var nilCM *corev1.ConfigMap
	typedNil := client.Object(nilCM)
	obj := client.Object(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "a"}})
	b := &Bundle{
		Name:         "web",
		Applications: []*Application{NewApplication("web", "default", &fakeConfig{objs: []*client.Object{nil, &nilObj, &typedNil, &obj}})},
		Labels:       map[string]string{"team": "shop"},
		Annotations:  map[string]string{"owner": "shop"},
	}
	res, err := b.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(res) != 4 || res[0] != nil || res[1] != &nilObj || *res[1] != nil || res[2] != &typedNil || *res[2] != typedNil {
		t.Fatalf("Generate = %v, want the nil entries returned as they are", res)
	}
	if want := map[string]string{"team": "shop"}; !reflect.DeepEqual(obj.GetLabels(), want) {
		t.Errorf("labels = %v, want %v", obj.GetLabels(), want)
	}
	if want := map[string]string{"owner": "shop"}; !reflect.DeepEqual(obj.GetAnnotations(), want) {
		t.Errorf("annotations = %v, want %v", obj.GetAnnotations(), want)
	}
}
