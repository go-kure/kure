package yaml

import (
	"errors"
	"os"
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
		t.Fatalf("parse returned error: %v", err)
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

func TestParseFile(t *testing.T) {
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
	dir := t.TempDir()
	path := dir + "/objects.yaml"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	objs, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}
	if len(objs) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(objs))
	}
}

func TestParseErrors(t *testing.T) {
	data := `apiVersion: v1
kind: ServiceAccount
metadata:
  name: sa
---
notvalid
---
apiVersion: foo/v1
kind: Unsupported
metadata:
  name: x
`
	objs, err := parse([]byte(data))
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if len(objs) != 1 {
		t.Fatalf("expected 1 valid object, got %d", len(objs))
	}
	var pe *ParseErrors
	if !errors.As(err, &pe) {
		t.Fatalf("expected ParseErrors, got %T", err)
	}
	if len(pe.Errors) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(pe.Errors))
	}
}

func TestParseCRDs(t *testing.T) {
	data := `apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: hr
spec: {}
---
apiVersion: image.toolkit.fluxcd.io/v1beta2
kind: ImageUpdateAutomation
metadata:
  name: ia
spec:
  interval: 1m
  sourceRef:
    kind: GitRepository
    name: repo
  git:
    checkout: {}
    commit:
      author:
        name: bot
        email: bot@example.com
    push: {}
---
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Provider
metadata:
  name: prov
spec:
  type: slack
---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: repo
spec:
  url: https://example.com/git.git
  interval: 1m
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: cert
spec:
  secretName: s
  issuerRef:
    name: issuer
---
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: pool
spec:
  addresses:
  - 1.2.3.4-1.2.3.5
`
	objs, err := parse([]byte(data))
	if err != nil {
		t.Fatalf("parse returned error: %v", err)
	}
	if len(objs) != 6 {
		t.Fatalf("expected 6 objects, got %d", len(objs))
	}
}
