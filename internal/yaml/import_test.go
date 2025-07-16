package yaml

import (
	"testing"

	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	imagev1 "github.com/fluxcd/image-automation-controller/api/v1beta2"
	notificationv1beta2 "github.com/fluxcd/notification-controller/api/v1beta2"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
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
	objs := parse([]byte(data))
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

func TestParseCustomObjects(t *testing.T) {
	data := `apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: hr
spec: {}
---
apiVersion: image.toolkit.fluxcd.io/v1beta2
kind: ImageUpdateAutomation
metadata:
  name: img
spec: {}
---
apiVersion: notification.toolkit.fluxcd.io/v1beta2
kind: Provider
metadata:
  name: prov
spec: {}
---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: repo
spec: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: cert
spec: {}
---
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: pool
spec: {}
`
	objs := parse([]byte(data))
	if len(objs) != 6 {
		t.Fatalf("expected 6 objects, got %d", len(objs))
	}
	if _, ok := objs[0].(*helmv2.HelmRelease); !ok {
		t.Fatalf("unexpected object 0: %#v", objs[0])
	}
	if _, ok := objs[1].(*imagev1.ImageUpdateAutomation); !ok {
		t.Fatalf("unexpected object 1: %#v", objs[1])
	}
	if _, ok := objs[2].(*notificationv1beta2.Provider); !ok {
		t.Fatalf("unexpected object 2: %#v", objs[2])
	}
	if _, ok := objs[3].(*sourcev1.GitRepository); !ok {
		t.Fatalf("unexpected object 3: %#v", objs[3])
	}
	if _, ok := objs[4].(*certv1.Certificate); !ok {
		t.Fatalf("unexpected object 4: %#v", objs[4])
	}
	if _, ok := objs[5].(*metallbv1beta1.IPAddressPool); !ok {
		t.Fatalf("unexpected object 5: %#v", objs[5])
	}
}
