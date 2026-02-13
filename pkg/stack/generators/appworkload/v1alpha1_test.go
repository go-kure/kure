package appworkload

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/generators"
	"github.com/go-kure/kure/pkg/stack/generators/appworkload/internal"
)

func TestConfigV1Alpha1_GetAPIVersion(t *testing.T) {
	cfg := &ConfigV1Alpha1{}
	expected := "generators.gokure.dev/v1alpha1"
	if got := cfg.GetAPIVersion(); got != expected {
		t.Errorf("GetAPIVersion() = %s, want %s", got, expected)
	}
}

func TestConfigV1Alpha1_GetKind(t *testing.T) {
	cfg := &ConfigV1Alpha1{}
	expected := "AppWorkload"
	if got := cfg.GetKind(); got != expected {
		t.Errorf("GetKind() = %s, want %s", got, expected)
	}
}

func TestConfigV1Alpha1_Generate_Deployment(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		BaseMetadata: generators.BaseMetadata{
			Name:      "test-app",
			Namespace: "test-ns",
		},
		Workload: internal.DeploymentWorkload,
		Replicas: 3,
		Labels: map[string]string{
			"app":     "test",
			"version": "v1.0",
		},
		Containers: []internal.ContainerConfig{
			{
				Name:  "web",
				Image: "nginx:1.21",
				Ports: []internal.ContainerPort{
					{Name: "http", ContainerPort: 80, Protocol: "TCP"},
				},
				Env: []internal.EnvVar{
					{Name: "ENV", Value: "production"},
				},
			},
		},
		Services: []internal.ServiceConfig{
			{
				Name:       "web-service",
				Type:       corev1.ServiceTypeLoadBalancer,
				Port:       80,
				TargetPort: 8080,
				Protocol:   corev1.ProtocolTCP,
			},
		},
		Ingress: &internal.IngressConfig{
			Host:            "test.example.com",
			Path:            "/",
			ServiceName:     "web-service",
			ServicePortName: "http",
		},
	}

	app := stack.NewApplication("test-app", "test-ns", cfg)
	objs, err := cfg.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(objs) == 0 {
		t.Fatal("Generate() returned no objects")
	}

	// Verify we get expected resource types
	var deployment *appsv1.Deployment
	var service *corev1.Service
	var ingress *netv1.Ingress

	for _, obj := range objs {
		switch v := (*obj).(type) {
		case *appsv1.Deployment:
			deployment = v
		case *corev1.Service:
			service = v
		case *netv1.Ingress:
			ingress = v
		}
	}

	if deployment == nil {
		t.Error("Expected Deployment object")
	} else {
		if deployment.Name != "test-app" {
			t.Errorf("Deployment name = %s, want test-app", deployment.Name)
		}
		if deployment.Namespace != "test-ns" {
			t.Errorf("Deployment namespace = %s, want test-ns", deployment.Namespace)
		}
		if *deployment.Spec.Replicas != 3 {
			t.Errorf("Deployment replicas = %d, want 3", *deployment.Spec.Replicas)
		}
	}

	if service == nil {
		t.Error("Expected Service object")
	} else {
		if service.Name != "test-app" {
			t.Errorf("Service name = %s, want test-app", service.Name)
		}
	}

	if ingress == nil {
		t.Error("Expected Ingress object")
	} else {
		if ingress.Name != "test-app" {
			t.Errorf("Ingress name = %s, want test-app", ingress.Name)
		}
	}
}

func TestConfigV1Alpha1_Generate_StatefulSet(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		BaseMetadata: generators.BaseMetadata{
			Name:      "test-statefulset",
			Namespace: "default",
		},
		Workload: internal.StatefulSetWorkload,
		Replicas: 2,
		Containers: []internal.ContainerConfig{
			{
				Name:  "database",
				Image: "postgres:13",
				Env: []internal.EnvVar{
					{Name: "POSTGRES_DB", Value: "testdb"},
				},
				VolumeMounts: []internal.VolumeMount{
					{Name: "data", MountPath: "/var/lib/postgresql/data"},
				},
			},
		},
		Volumes: []internal.Volume{
			{
				Name: "config",
				VolumeSource: &internal.VolumeSource{
					ConfigMap: &internal.ConfigMapVolumeSource{
						Name: "db-config",
					},
				},
			},
		},
		VolumeClaimTemplates: []internal.VolumeClaimTemplate{
			{
				Metadata: struct {
					Name string `json:"name" yaml:"name"`
				}{Name: "data"},
				Spec: struct {
					AccessModes      []string                       `json:"accessModes,omitempty" yaml:"accessModes,omitempty"`
					StorageClassName *string                        `json:"storageClassName,omitempty" yaml:"storageClassName,omitempty"`
					Resources        *internal.ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
				}{
					AccessModes: []string{"ReadWriteOnce"},
					Resources: &internal.ResourceRequirements{
						Requests: map[string]string{"storage": "1Gi"},
					},
				},
			},
		},
	}

	app := stack.NewApplication("test-statefulset", "default", cfg)
	objs, err := cfg.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(objs) != 1 {
		t.Errorf("Generate() returned %d objects, want 1", len(objs))
	}

	statefulSet, ok := (*objs[0]).(*appsv1.StatefulSet)
	if !ok {
		t.Errorf("Expected StatefulSet, got %T", *objs[0])
		return
	}

	if statefulSet.Name != "test-statefulset" {
		t.Errorf("StatefulSet name = %s, want test-statefulset", statefulSet.Name)
	}

	if *statefulSet.Spec.Replicas != 2 {
		t.Errorf("StatefulSet replicas = %d, want 2", *statefulSet.Spec.Replicas)
	}

	// Verify volume claim templates
	if len(statefulSet.Spec.VolumeClaimTemplates) != 1 {
		t.Errorf("VolumeClaimTemplates count = %d, want 1", len(statefulSet.Spec.VolumeClaimTemplates))
	}
}

func TestConfigV1Alpha1_Generate_DaemonSet(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		BaseMetadata: generators.BaseMetadata{
			Name:      "test-daemonset",
			Namespace: "kube-system",
		},
		Workload: internal.DaemonSetWorkload,
		Containers: []internal.ContainerConfig{
			{
				Name:  "agent",
				Image: "monitoring-agent:latest",
				VolumeMounts: []internal.VolumeMount{
					{Name: "host-logs", MountPath: "/var/log", ReadOnly: true},
				},
			},
		},
	}

	app := stack.NewApplication("test-daemonset", "kube-system", cfg)
	objs, err := cfg.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(objs) != 1 {
		t.Errorf("Generate() returned %d objects, want 1", len(objs))
	}

	daemonSet, ok := (*objs[0]).(*appsv1.DaemonSet)
	if !ok {
		t.Errorf("Expected DaemonSet, got %T", *objs[0])
		return
	}

	if daemonSet.Name != "test-daemonset" {
		t.Errorf("DaemonSet name = %s, want test-daemonset", daemonSet.Name)
	}

	if daemonSet.Namespace != "kube-system" {
		t.Errorf("DaemonSet namespace = %s, want kube-system", daemonSet.Namespace)
	}
}

func TestConfigV1Alpha1_Generate_InvalidWorkload(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		BaseMetadata: generators.BaseMetadata{
			Name:      "test-invalid",
			Namespace: "default",
		},
		Workload: internal.WorkloadType("InvalidWorkload"),
		Containers: []internal.ContainerConfig{
			{
				Name:  "test",
				Image: "test:latest",
			},
		},
	}

	app := stack.NewApplication("test-invalid", "default", cfg)
	_, err := cfg.Generate(app)
	if err == nil {
		t.Error("Generate() should return error for invalid workload type")
	}
}

func TestConfigV1Alpha1_Generate_WithComplexContainer(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		BaseMetadata: generators.BaseMetadata{
			Name:      "complex-app",
			Namespace: "default",
		},
		Workload: internal.DeploymentWorkload,
		Replicas: 1,
		Containers: []internal.ContainerConfig{
			{
				Name:  "web",
				Image: "nginx:1.21",
				Ports: []internal.ContainerPort{
					{Name: "http", ContainerPort: 80, Protocol: "TCP"},
					{Name: "metrics", ContainerPort: 9090, Protocol: "TCP"},
				},
				Env: []internal.EnvVar{
					{Name: "SIMPLE_VAR", Value: "simple-value"},
					{
						Name: "SECRET_VAR",
						ValueFrom: &internal.EnvVarSource{
							SecretKeyRef: &internal.SecretKeySelector{
								Name: "app-secret",
								Key:  "secret-key",
							},
						},
					},
					{
						Name: "CONFIG_VAR",
						ValueFrom: &internal.EnvVarSource{
							ConfigMapKeyRef: &internal.ConfigMapKeySelector{
								Name: "app-config",
								Key:  "config-key",
							},
						},
					},
					{
						Name: "FIELD_VAR",
						ValueFrom: &internal.EnvVarSource{
							FieldRef: &internal.ObjectFieldSelector{
								FieldPath: "metadata.name",
							},
						},
					},
				},
				VolumeMounts: []internal.VolumeMount{
					{Name: "data", MountPath: "/data", ReadOnly: false},
					{Name: "config", MountPath: "/etc/config", ReadOnly: true, SubPath: "app.conf"},
				},
				Resources: &internal.ResourceRequirements{
					Limits: map[string]string{
						"cpu":    "500m",
						"memory": "512Mi",
					},
					Requests: map[string]string{
						"cpu":    "100m",
						"memory": "128Mi",
					},
				},
				LivenessProbe: &internal.Probe{
					HTTPGet: &internal.HTTPGetAction{
						Path:   "/health",
						Port:   80,
						Scheme: "HTTP",
					},
					InitialDelaySeconds: 30,
					TimeoutSeconds:      5,
					PeriodSeconds:       10,
					SuccessThreshold:    1,
					FailureThreshold:    3,
				},
				ReadinessProbe: &internal.Probe{
					HTTPGet: &internal.HTTPGetAction{
						Path: "/ready",
						Port: 80,
					},
					InitialDelaySeconds: 5,
					TimeoutSeconds:      3,
					PeriodSeconds:       5,
				},
				StartupProbe: &internal.Probe{
					Exec: &internal.ExecAction{
						Command: []string{"/bin/sh", "-c", "curl -f http://localhost/startup"},
					},
					InitialDelaySeconds: 10,
					TimeoutSeconds:      1,
					PeriodSeconds:       10,
					SuccessThreshold:    1,
					FailureThreshold:    30,
				},
			},
		},
		Volumes: []internal.Volume{
			{
				Name: "data",
				VolumeSource: &internal.VolumeSource{
					EmptyDir: &internal.EmptyDirVolumeSource{
						SizeLimit: "1Gi",
					},
				},
			},
			{
				Name: "config",
				VolumeSource: &internal.VolumeSource{
					ConfigMap: &internal.ConfigMapVolumeSource{
						Name: "app-config",
					},
				},
			},
		},
	}

	app := stack.NewApplication("complex-app", "default", cfg)
	objs, err := cfg.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(objs) != 2 { // Deployment + Service (auto-created due to ports)
		t.Errorf("Generate() returned %d objects, want 2", len(objs))
	}

	deployment := findDeployment(objs)
	if deployment == nil {
		t.Fatal("Expected Deployment object")
	}

	// Verify container configuration
	if len(deployment.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("Expected 1 container, got %d", len(deployment.Spec.Template.Spec.Containers))
	}

	container := deployment.Spec.Template.Spec.Containers[0]

	// Verify ports
	if len(container.Ports) != 2 {
		t.Errorf("Container ports count = %d, want 2", len(container.Ports))
	}

	// Verify environment variables
	if len(container.Env) != 4 {
		t.Errorf("Container env count = %d, want 4", len(container.Env))
	}

	// Verify volume mounts
	if len(container.VolumeMounts) != 2 {
		t.Errorf("Container volume mounts count = %d, want 2", len(container.VolumeMounts))
	}

	// Verify probes are set
	if container.LivenessProbe == nil {
		t.Error("Expected liveness probe")
	}
	if container.ReadinessProbe == nil {
		t.Error("Expected readiness probe")
	}
	if container.StartupProbe == nil {
		t.Error("Expected startup probe")
	}

	// Verify volumes
	if len(deployment.Spec.Template.Spec.Volumes) != 2 {
		t.Errorf("Pod volumes count = %d, want 2", len(deployment.Spec.Template.Spec.Volumes))
	}
}

func TestConfigV1Alpha1_Generate_NilApp(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		BaseMetadata: generators.BaseMetadata{
			Name:      "test",
			Namespace: "default",
		},
		Workload: internal.DeploymentWorkload,
		Containers: []internal.ContainerConfig{
			{Name: "test", Image: "test:latest"},
		},
	}

	// Test that Generate panics with nil app (expected behavior)
	defer func() {
		if r := recover(); r == nil {
			t.Error("Generate() should panic with nil application")
		}
	}()

	cfg.Generate(nil)
}

func TestConfigV1Alpha1_Generate_EmptyContainers(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		BaseMetadata: generators.BaseMetadata{
			Name:      "test",
			Namespace: "default",
		},
		Workload:   internal.DeploymentWorkload,
		Containers: []internal.ContainerConfig{}, // Empty containers
	}

	app := stack.NewApplication("test", "default", cfg)
	objs, err := cfg.Generate(app)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if len(objs) != 1 {
		t.Errorf("Generate() returned %d objects, want 1", len(objs))
	}
}

func TestRegistration(t *testing.T) {
	// Test that the AppWorkload generator is properly registered in the stack registry
	config, err := stack.CreateApplicationConfig("generators.gokure.dev/v1alpha1", "AppWorkload")
	if err != nil {
		t.Fatalf("AppWorkload generator not registered in stack package: %v", err)
	}

	if config == nil {
		t.Fatal("CreateApplicationConfig returned nil config")
	}

	appWorkloadConfig, ok := config.(*ConfigV1Alpha1)
	if !ok {
		t.Fatalf("CreateApplicationConfig returned wrong type: %T, want *ConfigV1Alpha1", config)
	}

	if appWorkloadConfig.GetAPIVersion() != "generators.gokure.dev/v1alpha1" {
		t.Errorf("Config APIVersion = %s, want generators.gokure.dev/v1alpha1", appWorkloadConfig.GetAPIVersion())
	}

	if appWorkloadConfig.GetKind() != "AppWorkload" {
		t.Errorf("Config Kind = %s, want AppWorkload", appWorkloadConfig.GetKind())
	}
}

func TestConfigV1Alpha1_BaseMetadata(t *testing.T) {
	cfg := &ConfigV1Alpha1{
		BaseMetadata: generators.BaseMetadata{
			Name:      "test-base",
			Namespace: "test-namespace",
		},
		Workload: internal.DeploymentWorkload,
		Containers: []internal.ContainerConfig{
			{Name: "test", Image: "test:latest"},
		},
	}

	// Verify BaseMetadata fields are accessible
	if cfg.Name != "test-base" {
		t.Errorf("Name = %s, want test-base", cfg.Name)
	}

	if cfg.Namespace != "test-namespace" {
		t.Errorf("Namespace = %s, want test-namespace", cfg.Namespace)
	}
}

// Helper function to find deployment in objects slice
func findDeployment(objs []*client.Object) *appsv1.Deployment {
	for _, obj := range objs {
		if deployment, ok := (*obj).(*appsv1.Deployment); ok {
			return deployment
		}
	}
	return nil
}

// Test workload type validation
func TestWorkloadTypeValidation(t *testing.T) {
	tests := []struct {
		name        string
		workload    internal.WorkloadType
		expectError bool
	}{
		{"Deployment", internal.DeploymentWorkload, false},
		{"StatefulSet", internal.StatefulSetWorkload, false},
		{"DaemonSet", internal.DaemonSetWorkload, false},
		{"Invalid", internal.WorkloadType("Pod"), true},
		{"Empty", internal.WorkloadType(""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &ConfigV1Alpha1{
				BaseMetadata: generators.BaseMetadata{
					Name:      "test",
					Namespace: "default",
				},
				Workload: tt.workload,
				Containers: []internal.ContainerConfig{
					{Name: "test", Image: "test:latest"},
				},
			}

			app := stack.NewApplication("test", "default", cfg)
			_, err := cfg.Generate(app)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			} else if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}
