package kubernetes_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kureio "github.com/go-kure/kure/pkg/io"
	. "github.com/go-kure/kure/pkg/kubernetes"
)

var update = flag.Bool("update", false, "update golden files")

func testInitContainer() *corev1.Container {
	return &corev1.Container{
		Name:    "init-data",
		Image:   "busybox:1.36",
		Command: []string{"/bin/sh", "-c"},
		Args:    []string{"cp /src/config.yaml /data/config.yaml"},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "data-vol", MountPath: "/data"},
		},
		Env: []corev1.EnvVar{
			{Name: "INIT_MODE", Value: "copy"},
		},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("50m"),
				corev1.ResourceMemory: resource.MustParse("64Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
		},
	}
}

func testAppContainer() *corev1.Container {
	return &corev1.Container{
		Name:  "app",
		Image: "myapp:latest",
	}
}

func testDataVolume() *corev1.Volume {
	return &corev1.Volume{
		Name: "data-vol",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
}

func goldenTest(t *testing.T, filename string, obj client.Object) {
	t.Helper()

	objects := []*client.Object{&obj}
	got, err := kureio.EncodeObjectsToYAMLWithOptions(objects, kureio.EncodeOptions{
		KubernetesFieldOrder: true,
	})
	if err != nil {
		t.Fatalf("encoding to YAML: %v", err)
	}

	golden := filepath.Join("testdata", filename)

	if *update {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatalf("updating golden file: %v", err)
		}
		return
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading golden file (run with -update to create): %v", err)
	}

	if string(got) != string(want) {
		t.Errorf("output does not match golden file %s\n\ngot:\n%s\nwant:\n%s", golden, got, want)
	}
}

func TestDeploymentInitContainer_Golden(t *testing.T) {
	dep := CreateDeployment("my-app", "default")
	if err := AddDeploymentContainer(dep, testAppContainer()); err != nil {
		t.Fatal(err)
	}
	if err := AddDeploymentInitContainer(dep, testInitContainer()); err != nil {
		t.Fatal(err)
	}
	if err := AddDeploymentVolume(dep, testDataVolume()); err != nil {
		t.Fatal(err)
	}

	goldenTest(t, "deployment-with-init-container.yaml", dep)
}

func TestStatefulSetInitContainer_Golden(t *testing.T) {
	sts := CreateStatefulSet("my-app", "default")
	if err := AddStatefulSetContainer(sts, testAppContainer()); err != nil {
		t.Fatal(err)
	}
	if err := AddStatefulSetInitContainer(sts, testInitContainer()); err != nil {
		t.Fatal(err)
	}
	if err := AddStatefulSetVolume(sts, testDataVolume()); err != nil {
		t.Fatal(err)
	}

	goldenTest(t, "statefulset-with-init-container.yaml", sts)
}

func TestDaemonSetInitContainer_Golden(t *testing.T) {
	ds := CreateDaemonSet("my-app", "default")
	if err := AddDaemonSetContainer(ds, testAppContainer()); err != nil {
		t.Fatal(err)
	}
	if err := AddDaemonSetInitContainer(ds, testInitContainer()); err != nil {
		t.Fatal(err)
	}
	if err := AddDaemonSetVolume(ds, testDataVolume()); err != nil {
		t.Fatal(err)
	}

	goldenTest(t, "daemonset-with-init-container.yaml", ds)
}
