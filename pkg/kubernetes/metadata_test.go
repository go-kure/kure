package kubernetes

import (
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestAddLabel_InitialisesNilMapAndPreservesExisting(t *testing.T) {
	d := Create[appsv1.Deployment]("web", "default")
	if d.Labels != nil {
		t.Fatal("precondition: constructor must leave labels nil")
	}

	AddLabel(d, "app", "web")
	AddLabel(d, "tier", "frontend")
	AddLabel(d, "app", "web2")

	want := map[string]string{"app": "web2", "tier": "frontend"}
	if !reflect.DeepEqual(d.Labels, want) {
		t.Errorf("labels = %v, want %v", d.Labels, want)
	}
}

func TestAddAnnotation_InitialisesNilMapAndPreservesExisting(t *testing.T) {
	ns := Create[corev1.Namespace]("platform", "")

	AddAnnotation(ns, "owner", "platform")
	AddAnnotation(ns, "team", "infra")

	want := map[string]string{"owner": "platform", "team": "infra"}
	if !reflect.DeepEqual(ns.Annotations, want) {
		t.Errorf("annotations = %v, want %v", ns.Annotations, want)
	}
}

func TestSetLabelsAndSetAnnotations_ReplaceWholesale(t *testing.T) {
	svc := Create[corev1.Service]("api", "default")
	AddLabel(svc, "stale", "yes")
	AddAnnotation(svc, "stale", "yes")

	SetLabels(svc, map[string]string{"app": "api"})
	SetAnnotations(svc, map[string]string{"note": "x"})

	if !reflect.DeepEqual(svc.Labels, map[string]string{"app": "api"}) {
		t.Errorf("labels = %v", svc.Labels)
	}
	if !reflect.DeepEqual(svc.Annotations, map[string]string{"note": "x"}) {
		t.Errorf("annotations = %v", svc.Annotations)
	}

	SetLabels(svc, nil)
	if svc.Labels != nil {
		t.Errorf("SetLabels(nil) must clear the map, got %v", svc.Labels)
	}
}

func TestMetadataHelpers_PanicOnNil(t *testing.T) {
	assertPanics(t, func() { AddLabel(nil, "k", "v") })
	assertPanics(t, func() { AddAnnotation(nil, "k", "v") })
	assertPanics(t, func() { SetLabels(nil, nil) })
	assertPanics(t, func() { SetAnnotations(nil, nil) })
}
