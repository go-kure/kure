package stack

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
)

// TestContainerConfigGenerate verifies ports and volume mounts are propagated.
func TestContainerConfigGenerate(t *testing.T) {
	cfg := ContainerConfig{
		Name:         "ctr",
		Image:        "nginx",
		Ports:        []corev1.ContainerPort{{Name: "http", ContainerPort: 80, HostPort: 80}},
		VolumeMounts: map[string]string{"data": "/data"},
	}
	container, ports, err := cfg.Generate()
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if len(ports) != 1 || ports[0].Name != "http" {
		t.Fatalf("ports not returned correctly: %#v", ports)
	}
	if len(container.VolumeMounts) != 1 || container.VolumeMounts[0].Name != "data" {
		t.Fatalf("volume mounts not applied: %#v", container.VolumeMounts)
	}
}

// TestAppWorkloadGenerate ensures that different workload types produce expected objects.
func TestAppWorkloadGenerate(t *testing.T) {
	newBase := func() AppWorkloadConfig {
		return AppWorkloadConfig{
			Containers: []ContainerConfig{{
				Name:  "ctr",
				Image: "nginx",
				Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 80, HostPort: 80}},
			}},
		}
	}
	app := NewApplication("app", "ns", nil)

	// Deployment with service and ingress
	depCfg := newBase()
	depCfg.Workload = DeploymentWorkload
	depCfg.Ingress = &IngressConfig{Host: "example.com", ServiceName: "app", ServicePortName: "http"}
	objs, err := depCfg.Generate(app)
	if err != nil {
		t.Fatalf("deployment generate error: %v", err)
	}
	var hasDep, hasSvc, hasIng bool
	for _, o := range objs {
		switch (*o).(type) {
		case *appsv1.Deployment:
			hasDep = true
		case *corev1.Service:
			hasSvc = true
		case *netv1.Ingress:
			hasIng = true
		}
	}
	if !hasDep || !hasSvc || !hasIng {
		t.Fatalf("expected deployment, service and ingress, got: %#v", objs)
	}

	// StatefulSet without ports should only create workload
	stsCfg := newBase()
	stsCfg.Workload = StatefulSetWorkload
	stsCfg.Containers[0].Ports = nil
	objs, err = stsCfg.Generate(app)
	if err != nil {
		t.Fatalf("statefulset generate error: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("expected only statefulset, got %d objects", len(objs))
	}
	obj := *objs[0]
	if _, ok := obj.(*appsv1.StatefulSet); !ok {
		t.Fatalf("expected statefulset, got %T", objs[0])
	}

	// DaemonSet
	dsCfg := newBase()
	dsCfg.Workload = DaemonSetWorkload
	objs, err = dsCfg.Generate(app)
	if err != nil {
		t.Fatalf("daemonset generate error: %v", err)
	}
	if len(objs) != 2 {
		t.Fatalf("expected daemonset and service due to ports, got %d", len(objs))
	}
	foundDS := false
	for _, o := range objs {
		if _, ok := (*o).(*appsv1.DaemonSet); ok {
			foundDS = true
		}
	}
	if !foundDS {
		t.Fatalf("daemonset not found in objects: %#v", objs)
	}

	// Unsupported workload type
	badCfg := newBase()
	badCfg.Workload = "Unknown"
	if _, err := badCfg.Generate(app); err == nil {
		t.Fatalf("expected error for unsupported workload type")
	}
}
