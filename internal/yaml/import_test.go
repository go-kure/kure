package yaml

import (
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type dummy struct{ runtime.TypeMeta }

func (d *dummy) DeepCopyObject() runtime.Object { return &dummy{} }

func TestParse(t *testing.T) {
	data := `apiVersion: v1
kind: ServiceAccount
metadata:
  name: sa
---
apiVersion: v1
kind: Pod
metadata:
  name: pod
spec:
  containers: []
`
	objs, err := parse([]byte(data))
	if err != nil {
		t.Fatalf("parse returned unexpected error: %v", err)
	}
	if len(objs) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(objs))
	}
	if sa, ok := objs[0].(*corev1.ServiceAccount); !ok || sa.Name != "sa" {
		t.Fatalf("unexpected first object: %#v", objs[0])
	}
	if pod, ok := objs[1].(*corev1.Pod); !ok || pod.Name != "pod" {
		t.Fatalf("unexpected second object: %#v", objs[1])
	}
}

func TestCheckType(t *testing.T) {
	pod := &corev1.Pod{}
	pod.TypeMeta = metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"}
	if err := checkType(pod); err != nil {
		t.Fatalf("expected pod to be supported: %v", err)
	}

	var unknown runtime.Object = &dummy{}
	if err := checkType(unknown); err == nil {
		t.Fatalf("expected unsupported object error")
	}

	bad := &corev1.Pod{TypeMeta: metav1.TypeMeta{Kind: "ServiceAccount", APIVersion: "v1"}}
	if err := checkType(bad); err == nil {
		t.Fatalf("expected type mismatch error")
	}
}

func TestParseDecodeError(t *testing.T) {
	data := `apiVersion: v1
kind: ServiceAccount
metadata:
  name: sa
---
this is : [ invalid yaml
`
	objs, err := parse([]byte(data))
	if len(objs) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objs))
	}
	if _, ok := objs[0].(*corev1.ServiceAccount); !ok {
		t.Fatalf("unexpected object parsed: %#v", objs[0])
	}
	var pErr *ParseErrors
	if err == nil || !errors.As(err, &pErr) {
		t.Fatalf("expected ParseErrors, got %v", err)
	}
	if len(pErr.Errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(pErr.Errs))
	}
}

func TestParseUnsupportedObject(t *testing.T) {
	data := `apiVersion: v1
kind: ServiceAccount
metadata:
  name: sa
---
apiVersion: foo/v1
kind: Dummy
metadata:
  name: d
`
	objs, err := parse([]byte(data))
	if len(objs) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objs))
	}
	if _, ok := objs[0].(*corev1.ServiceAccount); !ok {
		t.Fatalf("unexpected object parsed: %#v", objs[0])
	}
	var pErr *ParseErrors
	if err == nil || !errors.As(err, &pErr) {
		t.Fatalf("expected ParseErrors, got %v", err)
	}
	if len(pErr.Errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(pErr.Errs))
	}
}
