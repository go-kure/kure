package internal

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
)

// TestToKubernetesResources tests resource requirements conversion
func TestToKubernetesResources(t *testing.T) {
	tests := []struct {
		name        string
		resources   *ResourceRequirements
		wantNil     bool
		wantErr     bool
		checkLimits func(t *testing.T, limits corev1.ResourceList)
	}{
		{
			name:      "nil resources",
			resources: nil,
			wantNil:   true,
		},
		{
			name: "valid limits and requests",
			resources: &ResourceRequirements{
				Limits: map[string]string{
					"cpu":    "1000m",
					"memory": "512Mi",
				},
				Requests: map[string]string{
					"cpu":    "100m",
					"memory": "128Mi",
				},
			},
			checkLimits: func(t *testing.T, limits corev1.ResourceList) {
				if limits == nil {
					t.Fatal("expected non-nil limits")
				}
				cpu := limits[corev1.ResourceCPU]
				if cpu.String() != "1" {
					t.Errorf("expected cpu limit 1, got %s", cpu.String())
				}
			},
		},
		{
			name: "invalid cpu format",
			resources: &ResourceRequirements{
				Limits: map[string]string{
					"cpu": "invalid",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid memory format",
			resources: &ResourceRequirements{
				Requests: map[string]string{
					"memory": "bad-format",
				},
			},
			wantErr: true,
		},
		{
			name: "empty maps",
			resources: &ResourceRequirements{
				Limits:   map[string]string{},
				Requests: map[string]string{},
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.resources.ToKubernetesResources()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil && result != nil {
				t.Fatalf("expected nil result, got %v", result)
			}
			if !tt.wantNil && result == nil {
				t.Fatal("expected non-nil result, got nil")
			}
			if tt.checkLimits != nil && result != nil {
				tt.checkLimits(t, result.Limits)
			}
		})
	}
}

// TestToKubernetesPVC tests PVC conversion
func TestToKubernetesPVC(t *testing.T) {
	tests := []struct {
		name    string
		vct     *VolumeClaimTemplate
		wantErr bool
		check   func(t *testing.T, pvc *corev1.PersistentVolumeClaim)
	}{
		{
			name: "basic pvc",
			vct: &VolumeClaimTemplate{
				Metadata: struct {
					Name string `json:"name" yaml:"name"`
				}{Name: "data"},
				Spec: struct {
					AccessModes      []string              `json:"accessModes,omitempty" yaml:"accessModes,omitempty"`
					StorageClassName *string               `json:"storageClassName,omitempty" yaml:"storageClassName,omitempty"`
					Resources        *ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
				}{
					AccessModes: []string{"ReadWriteOnce"},
				},
			},
			check: func(t *testing.T, pvc *corev1.PersistentVolumeClaim) {
				if pvc.Name != "data" {
					t.Errorf("expected name 'data', got %s", pvc.Name)
				}
				if len(pvc.Spec.AccessModes) != 1 {
					t.Fatalf("expected 1 access mode, got %d", len(pvc.Spec.AccessModes))
				}
				if pvc.Spec.AccessModes[0] != corev1.ReadWriteOnce {
					t.Errorf("expected ReadWriteOnce, got %s", pvc.Spec.AccessModes[0])
				}
			},
		},
		{
			name: "pvc with storage class",
			vct: func() *VolumeClaimTemplate {
				sc := "fast"
				vct := &VolumeClaimTemplate{
					Metadata: struct {
						Name string `json:"name" yaml:"name"`
					}{Name: "cache"},
				}
				vct.Spec.StorageClassName = &sc
				return vct
			}(),
			check: func(t *testing.T, pvc *corev1.PersistentVolumeClaim) {
				if pvc.Spec.StorageClassName == nil {
					t.Fatal("expected storage class name, got nil")
				}
				if *pvc.Spec.StorageClassName != "fast" {
					t.Errorf("expected 'fast', got %s", *pvc.Spec.StorageClassName)
				}
			},
		},
		{
			name: "pvc with resources",
			vct: func() *VolumeClaimTemplate {
				vct := &VolumeClaimTemplate{
					Metadata: struct {
						Name string `json:"name" yaml:"name"`
					}{Name: "storage"},
				}
				vct.Spec.Resources = &ResourceRequirements{
					Requests: map[string]string{
						"storage": "10Gi",
					},
				}
				return vct
			}(),
			check: func(t *testing.T, pvc *corev1.PersistentVolumeClaim) {
				if pvc.Spec.Resources.Requests == nil {
					t.Fatal("expected resources requests, got nil")
				}
				storage := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
				if storage.String() != "10Gi" {
					t.Errorf("expected 10Gi, got %s", storage.String())
				}
			},
		},
		{
			name: "pvc with invalid resource",
			vct: func() *VolumeClaimTemplate {
				vct := &VolumeClaimTemplate{
					Metadata: struct {
						Name string `json:"name" yaml:"name"`
					}{Name: "invalid"},
				}
				vct.Spec.Resources = &ResourceRequirements{
					Limits: map[string]string{
						"storage": "bad-format",
					},
				}
				return vct
			}(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pvc, err := tt.vct.ToKubernetesPVC()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if pvc == nil {
				t.Fatal("expected non-nil pvc")
			}
			if tt.check != nil {
				tt.check(t, pvc)
			}
		})
	}
}

// TestToKubernetesPort tests container port conversion
func TestToKubernetesPort(t *testing.T) {
	tests := []struct {
		name string
		port ContainerPort
		want corev1.ContainerPort
	}{
		{
			name: "basic port with TCP",
			port: ContainerPort{
				Name:          "http",
				ContainerPort: 8080,
				Protocol:      "TCP",
			},
			want: corev1.ContainerPort{
				Name:          "http",
				ContainerPort: 8080,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		{
			name: "port without protocol defaults to TCP",
			port: ContainerPort{
				Name:          "grpc",
				ContainerPort: 9090,
			},
			want: corev1.ContainerPort{
				Name:          "grpc",
				ContainerPort: 9090,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		{
			name: "UDP port",
			port: ContainerPort{
				Name:          "dns",
				ContainerPort: 53,
				Protocol:      "UDP",
			},
			want: corev1.ContainerPort{
				Name:          "dns",
				ContainerPort: 53,
				Protocol:      corev1.ProtocolUDP,
			},
		},
		{
			name: "port without name",
			port: ContainerPort{
				ContainerPort: 3000,
			},
			want: corev1.ContainerPort{
				ContainerPort: 3000,
				Protocol:      corev1.ProtocolTCP,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.port.ToKubernetesPort()
			if got.Name != tt.want.Name {
				t.Errorf("name: got %s, want %s", got.Name, tt.want.Name)
			}
			if got.ContainerPort != tt.want.ContainerPort {
				t.Errorf("containerPort: got %d, want %d", got.ContainerPort, tt.want.ContainerPort)
			}
			if got.Protocol != tt.want.Protocol {
				t.Errorf("protocol: got %s, want %s", got.Protocol, tt.want.Protocol)
			}
		})
	}
}

// TestToKubernetesVolumeMount tests volume mount conversion
func TestToKubernetesVolumeMount(t *testing.T) {
	tests := []struct {
		name  string
		mount VolumeMount
		want  corev1.VolumeMount
	}{
		{
			name: "basic volume mount",
			mount: VolumeMount{
				Name:      "data",
				MountPath: "/data",
			},
			want: corev1.VolumeMount{
				Name:      "data",
				MountPath: "/data",
			},
		},
		{
			name: "read-only mount",
			mount: VolumeMount{
				Name:      "config",
				MountPath: "/etc/config",
				ReadOnly:  true,
			},
			want: corev1.VolumeMount{
				Name:      "config",
				MountPath: "/etc/config",
				ReadOnly:  true,
			},
		},
		{
			name: "mount with subpath",
			mount: VolumeMount{
				Name:      "shared",
				MountPath: "/app/logs",
				SubPath:   "logs",
			},
			want: corev1.VolumeMount{
				Name:      "shared",
				MountPath: "/app/logs",
				SubPath:   "logs",
			},
		},
		{
			name: "mount with all fields",
			mount: VolumeMount{
				Name:      "full",
				MountPath: "/mnt/full",
				ReadOnly:  true,
				SubPath:   "sub/path",
			},
			want: corev1.VolumeMount{
				Name:      "full",
				MountPath: "/mnt/full",
				ReadOnly:  true,
				SubPath:   "sub/path",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.mount.ToKubernetesVolumeMount()
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestToKubernetesEnvVar tests environment variable conversion
func TestToKubernetesEnvVar(t *testing.T) {
	tests := []struct {
		name string
		env  EnvVar
		want corev1.EnvVar
	}{
		{
			name: "simple env var",
			env: EnvVar{
				Name:  "ENV_VAR",
				Value: "value",
			},
			want: corev1.EnvVar{
				Name:  "ENV_VAR",
				Value: "value",
			},
		},
		{
			name: "env var from secret",
			env: EnvVar{
				Name: "PASSWORD",
				ValueFrom: &EnvVarSource{
					SecretKeyRef: &SecretKeySelector{
						Name: "my-secret",
						Key:  "password",
					},
				},
			},
			want: corev1.EnvVar{
				Name: "PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "my-secret",
						},
						Key: "password",
					},
				},
			},
		},
		{
			name: "env var from configmap",
			env: EnvVar{
				Name: "CONFIG",
				ValueFrom: &EnvVarSource{
					ConfigMapKeyRef: &ConfigMapKeySelector{
						Name: "my-config",
						Key:  "config.yaml",
					},
				},
			},
			want: corev1.EnvVar{
				Name: "CONFIG",
				ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "my-config",
						},
						Key: "config.yaml",
					},
				},
			},
		},
		{
			name: "env var from field ref",
			env: EnvVar{
				Name: "POD_NAME",
				ValueFrom: &EnvVarSource{
					FieldRef: &ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
			want: corev1.EnvVar{
				Name: "POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.env.ToKubernetesEnvVar()
			if got.Name != tt.want.Name {
				t.Errorf("name: got %s, want %s", got.Name, tt.want.Name)
			}
			if got.Value != tt.want.Value {
				t.Errorf("value: got %s, want %s", got.Value, tt.want.Value)
			}
			if (got.ValueFrom == nil) != (tt.want.ValueFrom == nil) {
				t.Fatalf("valueFrom presence mismatch: got %v, want %v", got.ValueFrom, tt.want.ValueFrom)
			}
			if got.ValueFrom != nil {
				if (got.ValueFrom.SecretKeyRef == nil) != (tt.want.ValueFrom.SecretKeyRef == nil) {
					t.Errorf("secretKeyRef presence mismatch")
				}
				if got.ValueFrom.SecretKeyRef != nil {
					if got.ValueFrom.SecretKeyRef.Name != tt.want.ValueFrom.SecretKeyRef.Name {
						t.Errorf("secret name: got %s, want %s", got.ValueFrom.SecretKeyRef.Name, tt.want.ValueFrom.SecretKeyRef.Name)
					}
					if got.ValueFrom.SecretKeyRef.Key != tt.want.ValueFrom.SecretKeyRef.Key {
						t.Errorf("secret key: got %s, want %s", got.ValueFrom.SecretKeyRef.Key, tt.want.ValueFrom.SecretKeyRef.Key)
					}
				}
				if (got.ValueFrom.ConfigMapKeyRef == nil) != (tt.want.ValueFrom.ConfigMapKeyRef == nil) {
					t.Errorf("configMapKeyRef presence mismatch")
				}
				if got.ValueFrom.ConfigMapKeyRef != nil {
					if got.ValueFrom.ConfigMapKeyRef.Name != tt.want.ValueFrom.ConfigMapKeyRef.Name {
						t.Errorf("configmap name: got %s, want %s", got.ValueFrom.ConfigMapKeyRef.Name, tt.want.ValueFrom.ConfigMapKeyRef.Name)
					}
				}
				if (got.ValueFrom.FieldRef == nil) != (tt.want.ValueFrom.FieldRef == nil) {
					t.Errorf("fieldRef presence mismatch")
				}
				if got.ValueFrom.FieldRef != nil {
					if got.ValueFrom.FieldRef.FieldPath != tt.want.ValueFrom.FieldRef.FieldPath {
						t.Errorf("fieldPath: got %s, want %s", got.ValueFrom.FieldRef.FieldPath, tt.want.ValueFrom.FieldRef.FieldPath)
					}
				}
			}
		})
	}
}

// TestToKubernetesProbe tests probe conversion
func TestToKubernetesProbe(t *testing.T) {
	tests := []struct {
		name    string
		probe   Probe
		wantNil bool
		check   func(t *testing.T, p *corev1.Probe)
	}{
		{
			name:    "empty probe returns nil",
			probe:   Probe{},
			wantNil: true,
		},
		{
			name: "http get probe",
			probe: Probe{
				HTTPGet: &HTTPGetAction{
					Path:   "/health",
					Port:   8080,
					Scheme: "HTTP",
				},
				InitialDelaySeconds: 10,
				TimeoutSeconds:      5,
				PeriodSeconds:       30,
				SuccessThreshold:    1,
				FailureThreshold:    3,
			},
			check: func(t *testing.T, p *corev1.Probe) {
				if p.HTTPGet == nil {
					t.Fatal("expected HTTPGet, got nil")
				}
				if p.HTTPGet.Path != "/health" {
					t.Errorf("path: got %s, want /health", p.HTTPGet.Path)
				}
				if p.HTTPGet.Port.IntVal != 8080 {
					t.Errorf("port: got %d, want 8080", p.HTTPGet.Port.IntVal)
				}
				if p.HTTPGet.Scheme != corev1.URISchemeHTTP {
					t.Errorf("scheme: got %s, want HTTP", p.HTTPGet.Scheme)
				}
				if p.InitialDelaySeconds != 10 {
					t.Errorf("initialDelaySeconds: got %d, want 10", p.InitialDelaySeconds)
				}
			},
		},
		{
			name: "http get without scheme",
			probe: Probe{
				HTTPGet: &HTTPGetAction{
					Path: "/ready",
					Port: 9000,
				},
			},
			check: func(t *testing.T, p *corev1.Probe) {
				if p.HTTPGet == nil {
					t.Fatal("expected HTTPGet, got nil")
				}
				if p.HTTPGet.Scheme != "" {
					t.Errorf("expected empty scheme, got %s", p.HTTPGet.Scheme)
				}
			},
		},
		{
			name: "exec probe",
			probe: Probe{
				Exec: &ExecAction{
					Command: []string{"cat", "/tmp/healthy"},
				},
				PeriodSeconds: 10,
			},
			check: func(t *testing.T, p *corev1.Probe) {
				if p.Exec == nil {
					t.Fatal("expected Exec, got nil")
				}
				if len(p.Exec.Command) != 2 {
					t.Fatalf("expected 2 commands, got %d", len(p.Exec.Command))
				}
				if p.Exec.Command[0] != "cat" {
					t.Errorf("command[0]: got %s, want cat", p.Exec.Command[0])
				}
			},
		},
		{
			name: "probe with both http and exec",
			probe: Probe{
				HTTPGet: &HTTPGetAction{
					Path: "/health",
					Port: 8080,
				},
				Exec: &ExecAction{
					Command: []string{"test"},
				},
			},
			check: func(t *testing.T, p *corev1.Probe) {
				if p.HTTPGet == nil {
					t.Error("expected HTTPGet")
				}
				if p.Exec == nil {
					t.Error("expected Exec")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.probe.ToKubernetesProbe()
			if tt.wantNil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil probe")
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

// TestToKubernetesVolume tests volume conversion
func TestToKubernetesVolume(t *testing.T) {
	tests := []struct {
		name   string
		volume Volume
		check  func(t *testing.T, v corev1.Volume)
	}{
		{
			name: "emptydir volume",
			volume: Volume{
				Name: "cache",
				VolumeSource: &VolumeSource{
					EmptyDir: &EmptyDirVolumeSource{},
				},
			},
			check: func(t *testing.T, v corev1.Volume) {
				if v.Name != "cache" {
					t.Errorf("name: got %s, want cache", v.Name)
				}
				if v.EmptyDir == nil {
					t.Fatal("expected EmptyDir, got nil")
				}
			},
		},
		{
			name: "emptydir with size limit",
			volume: Volume{
				Name: "tmp",
				VolumeSource: &VolumeSource{
					EmptyDir: &EmptyDirVolumeSource{
						SizeLimit: "1Gi",
					},
				},
			},
			check: func(t *testing.T, v corev1.Volume) {
				if v.EmptyDir == nil {
					t.Fatal("expected EmptyDir, got nil")
				}
				if v.EmptyDir.SizeLimit == nil {
					t.Fatal("expected size limit, got nil")
				}
				expected := resource.MustParse("1Gi")
				if !v.EmptyDir.SizeLimit.Equal(expected) {
					t.Errorf("sizeLimit: got %s, want 1Gi", v.EmptyDir.SizeLimit.String())
				}
			},
		},
		{
			name: "emptydir with invalid size limit",
			volume: Volume{
				Name: "bad",
				VolumeSource: &VolumeSource{
					EmptyDir: &EmptyDirVolumeSource{
						SizeLimit: "invalid",
					},
				},
			},
			check: func(t *testing.T, v corev1.Volume) {
				if v.EmptyDir.SizeLimit != nil {
					t.Error("expected nil size limit for invalid format")
				}
			},
		},
		{
			name: "configmap volume",
			volume: Volume{
				Name: "config",
				VolumeSource: &VolumeSource{
					ConfigMap: &ConfigMapVolumeSource{
						Name: "app-config",
					},
				},
			},
			check: func(t *testing.T, v corev1.Volume) {
				if v.ConfigMap == nil {
					t.Fatal("expected ConfigMap, got nil")
				}
				if v.ConfigMap.Name != "app-config" {
					t.Errorf("name: got %s, want app-config", v.ConfigMap.Name)
				}
			},
		},
		{
			name: "secret volume",
			volume: Volume{
				Name: "secret",
				VolumeSource: &VolumeSource{
					Secret: &SecretVolumeSource{
						SecretName: "my-secret",
					},
				},
			},
			check: func(t *testing.T, v corev1.Volume) {
				if v.Secret == nil {
					t.Fatal("expected Secret, got nil")
				}
				if v.Secret.SecretName != "my-secret" {
					t.Errorf("secretName: got %s, want my-secret", v.Secret.SecretName)
				}
			},
		},
		{
			name: "hostpath volume",
			volume: Volume{
				Name: "host",
				VolumeSource: &VolumeSource{
					HostPath: &HostPathVolumeSource{
						Path: "/var/lib/data",
					},
				},
			},
			check: func(t *testing.T, v corev1.Volume) {
				if v.HostPath == nil {
					t.Fatal("expected HostPath, got nil")
				}
				if v.HostPath.Path != "/var/lib/data" {
					t.Errorf("path: got %s, want /var/lib/data", v.HostPath.Path)
				}
			},
		},
		{
			name: "volume without source",
			volume: Volume{
				Name: "empty",
			},
			check: func(t *testing.T, v corev1.Volume) {
				if v.Name != "empty" {
					t.Errorf("name: got %s, want empty", v.Name)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.volume.ToKubernetesVolume()
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

// TestContainerConfigGenerate verifies ports and volume mounts are propagated.
func TestContainerConfigGenerate(t *testing.T) {
	cfg := ContainerConfig{
		Name:         "ctr",
		Image:        "nginx",
		Ports:        []ContainerPort{{Name: "http", ContainerPort: 80}},
		VolumeMounts: []VolumeMount{{Name: "data", MountPath: "/data"}},
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

// TestContainerConfigGenerateAdvanced tests advanced container config generation
func TestContainerConfigGenerateAdvanced(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ContainerConfig
		wantErr bool
		check   func(t *testing.T, c *corev1.Container, ports []corev1.ContainerPort)
	}{
		{
			name: "container with env vars",
			cfg: ContainerConfig{
				Name:  "app",
				Image: "myapp:latest",
				Env: []EnvVar{
					{Name: "ENV", Value: "prod"},
					{Name: "SECRET", ValueFrom: &EnvVarSource{
						SecretKeyRef: &SecretKeySelector{Name: "secret", Key: "key"},
					}},
				},
			},
			check: func(t *testing.T, c *corev1.Container, ports []corev1.ContainerPort) {
				if len(c.Env) != 2 {
					t.Errorf("expected 2 env vars, got %d", len(c.Env))
				}
			},
		},
		{
			name: "container with resources",
			cfg: ContainerConfig{
				Name:  "app",
				Image: "myapp:latest",
				Resources: &ResourceRequirements{
					Limits: map[string]string{
						"cpu":    "500m",
						"memory": "256Mi",
					},
					Requests: map[string]string{
						"cpu":    "100m",
						"memory": "128Mi",
					},
				},
			},
			check: func(t *testing.T, c *corev1.Container, ports []corev1.ContainerPort) {
				if c.Resources.Limits == nil {
					t.Error("expected limits")
				}
				if c.Resources.Requests == nil {
					t.Error("expected requests")
				}
			},
		},
		{
			name: "container with invalid resources",
			cfg: ContainerConfig{
				Name:  "app",
				Image: "myapp:latest",
				Resources: &ResourceRequirements{
					Limits: map[string]string{
						"cpu": "invalid",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "container with probes",
			cfg: ContainerConfig{
				Name:  "app",
				Image: "myapp:latest",
				LivenessProbe: &Probe{
					HTTPGet: &HTTPGetAction{Path: "/health", Port: 8080},
				},
				ReadinessProbe: &Probe{
					HTTPGet: &HTTPGetAction{Path: "/ready", Port: 8080},
				},
				StartupProbe: &Probe{
					Exec: &ExecAction{Command: []string{"test"}},
				},
			},
			check: func(t *testing.T, c *corev1.Container, ports []corev1.ContainerPort) {
				if c.LivenessProbe == nil {
					t.Error("expected liveness probe")
				}
				if c.ReadinessProbe == nil {
					t.Error("expected readiness probe")
				}
				if c.StartupProbe == nil {
					t.Error("expected startup probe")
				}
			},
		},
		{
			name: "container with multiple ports",
			cfg: ContainerConfig{
				Name:  "app",
				Image: "myapp:latest",
				Ports: []ContainerPort{
					{Name: "http", ContainerPort: 8080, Protocol: "TCP"},
					{Name: "grpc", ContainerPort: 9090},
					{ContainerPort: 3000},
				},
			},
			check: func(t *testing.T, c *corev1.Container, ports []corev1.ContainerPort) {
				if len(ports) != 3 {
					t.Errorf("expected 3 ports, got %d", len(ports))
				}
				if len(c.Ports) != 3 {
					t.Errorf("expected 3 container ports, got %d", len(c.Ports))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container, ports, err := tt.cfg.Generate()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if container == nil {
				t.Fatal("expected non-nil container")
			}
			if tt.check != nil {
				tt.check(t, container, ports)
			}
		})
	}
}

// TestAppWorkloadGenerate ensures that different workload types produce expected objects.
func TestAppWorkloadGenerate(t *testing.T) {
	newBase := func() *Config {
		return &Config{
			Name:      "app",
			Namespace: "ns",
			Containers: []ContainerConfig{{
				Name:  "ctr",
				Image: "nginx",
				Ports: []ContainerPort{{Name: "http", ContainerPort: 80}},
			}},
		}
	}
	app := stack.NewApplication("app", "ns", nil)

	// Deployment with service and ingress
	depCfg := newBase()
	depCfg.Workload = DeploymentWorkload
	depCfg.Services = []ServiceConfig{{
		Name:       "app",
		Port:       80,
		TargetPort: 8080,
		Protocol:   corev1.ProtocolTCP,
	}}
	depCfg.Ingress = &IngressConfig{
		Host:            "example.com",
		ServiceName:     "app",
		ServicePortName: "http",
	}
	objs, err := GenerateResources(depCfg, app)
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
	stsCfg.Services = nil
	objs, err = GenerateResources(stsCfg, app)
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
	objs, err = GenerateResources(dsCfg, app)
	if err != nil {
		t.Fatalf("daemonset generate error: %v", err)
	}
	// Should have daemonset only (no auto-service unless explicitly configured)
	foundDS := false
	for _, o := range objs {
		if _, ok := (*o).(*appsv1.DaemonSet); ok {
			foundDS = true
		}
	}
	if !foundDS {
		t.Fatalf("expected daemonset in objects")
	}
}

// TestGenerateResourcesAdvanced tests advanced resource generation scenarios
func TestGenerateResourcesAdvanced(t *testing.T) {
	app := stack.NewApplication("app", "ns", nil)

	tests := []struct {
		name    string
		cfg     *Config
		wantErr bool
		check   func(t *testing.T, objs []*client.Object)
	}{
		{
			name: "deployment with volumes",
			cfg: &Config{
				Name:      "app",
				Namespace: "ns",
				Workload:  DeploymentWorkload,
				Replicas:  3,
				Containers: []ContainerConfig{{
					Name:  "app",
					Image: "nginx:latest",
					Ports: []ContainerPort{{Name: "http", ContainerPort: 80}},
					VolumeMounts: []VolumeMount{
						{Name: "data", MountPath: "/data"},
						{Name: "config", MountPath: "/etc/config", ReadOnly: true},
					},
				}},
				Volumes: []Volume{
					{
						Name: "data",
						VolumeSource: &VolumeSource{
							EmptyDir: &EmptyDirVolumeSource{SizeLimit: "1Gi"},
						},
					},
					{
						Name: "config",
						VolumeSource: &VolumeSource{
							ConfigMap: &ConfigMapVolumeSource{Name: "app-config"},
						},
					},
				},
			},
			check: func(t *testing.T, objs []*client.Object) {
				if len(objs) != 2 {
					t.Errorf("expected 2 objects (deployment + service), got %d", len(objs))
				}
				var foundDep bool
				for _, o := range objs {
					if dep, ok := (*o).(*appsv1.Deployment); ok {
						foundDep = true
						if *dep.Spec.Replicas != 3 {
							t.Errorf("expected 3 replicas, got %d", *dep.Spec.Replicas)
						}
						if len(dep.Spec.Template.Spec.Volumes) != 2 {
							t.Errorf("expected 2 volumes, got %d", len(dep.Spec.Template.Spec.Volumes))
						}
					}
				}
				if !foundDep {
					t.Error("deployment not found")
				}
			},
		},
		{
			name: "statefulset with volume claim templates",
			cfg: &Config{
				Name:      "db",
				Namespace: "ns",
				Workload:  StatefulSetWorkload,
				Replicas:  3,
				Containers: []ContainerConfig{{
					Name:  "postgres",
					Image: "postgres:14",
					Ports: []ContainerPort{{Name: "pg", ContainerPort: 5432}},
					VolumeMounts: []VolumeMount{
						{Name: "data", MountPath: "/var/lib/postgresql/data"},
					},
				}},
				VolumeClaimTemplates: []VolumeClaimTemplate{
					{
						Metadata: struct {
							Name string `json:"name" yaml:"name"`
						}{Name: "data"},
						Spec: struct {
							AccessModes      []string              `json:"accessModes,omitempty" yaml:"accessModes,omitempty"`
							StorageClassName *string               `json:"storageClassName,omitempty" yaml:"storageClassName,omitempty"`
							Resources        *ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
						}{
							AccessModes: []string{"ReadWriteOnce"},
							Resources: &ResourceRequirements{
								Requests: map[string]string{"storage": "10Gi"},
							},
						},
					},
				},
			},
			check: func(t *testing.T, objs []*client.Object) {
				var foundSts bool
				for _, o := range objs {
					if sts, ok := (*o).(*appsv1.StatefulSet); ok {
						foundSts = true
						if len(sts.Spec.VolumeClaimTemplates) != 1 {
							t.Errorf("expected 1 volume claim template, got %d", len(sts.Spec.VolumeClaimTemplates))
						}
						if sts.Spec.VolumeClaimTemplates[0].Name != "data" {
							t.Errorf("expected vct name 'data', got %s", sts.Spec.VolumeClaimTemplates[0].Name)
						}
					}
				}
				if !foundSts {
					t.Error("statefulset not found")
				}
			},
		},
		{
			name: "daemonset with volumes",
			cfg: &Config{
				Name:      "logging",
				Namespace: "ns",
				Workload:  DaemonSetWorkload,
				Containers: []ContainerConfig{{
					Name:  "fluentd",
					Image: "fluentd:latest",
					VolumeMounts: []VolumeMount{
						{Name: "varlog", MountPath: "/var/log", ReadOnly: true},
					},
				}},
				Volumes: []Volume{
					{
						Name: "varlog",
						VolumeSource: &VolumeSource{
							HostPath: &HostPathVolumeSource{Path: "/var/log"},
						},
					},
				},
			},
			check: func(t *testing.T, objs []*client.Object) {
				var foundDs bool
				for _, o := range objs {
					if ds, ok := (*o).(*appsv1.DaemonSet); ok {
						foundDs = true
						if len(ds.Spec.Template.Spec.Volumes) != 1 {
							t.Errorf("expected 1 volume, got %d", len(ds.Spec.Template.Spec.Volumes))
						}
					}
				}
				if !foundDs {
					t.Error("daemonset not found")
				}
			},
		},
		{
			name: "invalid workload type",
			cfg: &Config{
				Name:      "app",
				Namespace: "ns",
				Workload:  WorkloadType("InvalidType"),
				Containers: []ContainerConfig{{
					Name:  "app",
					Image: "nginx",
				}},
			},
			wantErr: true,
		},
		{
			name: "deployment without ports no service",
			cfg: &Config{
				Name:      "worker",
				Namespace: "ns",
				Workload:  DeploymentWorkload,
				Containers: []ContainerConfig{{
					Name:  "worker",
					Image: "worker:latest",
				}},
			},
			check: func(t *testing.T, objs []*client.Object) {
				if len(objs) != 1 {
					t.Errorf("expected 1 object (deployment only), got %d", len(objs))
				}
				if _, ok := (*objs[0]).(*appsv1.Deployment); !ok {
					t.Errorf("expected deployment, got %T", *objs[0])
				}
			},
		},
		{
			name: "deployment with ingress but no service",
			cfg: &Config{
				Name:      "app",
				Namespace: "ns",
				Workload:  DeploymentWorkload,
				Containers: []ContainerConfig{{
					Name:  "app",
					Image: "nginx",
				}},
				Ingress: &IngressConfig{
					Host:            "example.com",
					ServiceName:     "app",
					ServicePortName: "http",
				},
			},
			check: func(t *testing.T, objs []*client.Object) {
				// No ports, so no service, so no ingress should be created
				if len(objs) != 1 {
					t.Errorf("expected 1 object (deployment only), got %d", len(objs))
				}
			},
		},
		{
			name: "ingress with custom path",
			cfg: &Config{
				Name:      "api",
				Namespace: "ns",
				Workload:  DeploymentWorkload,
				Containers: []ContainerConfig{{
					Name:  "api",
					Image: "api:latest",
					Ports: []ContainerPort{{Name: "http", ContainerPort: 8080}},
				}},
				Ingress: &IngressConfig{
					Host:            "api.example.com",
					Path:            "/v1",
					ServiceName:     "api",
					ServicePortName: "http",
				},
			},
			check: func(t *testing.T, objs []*client.Object) {
				var foundIng bool
				for _, o := range objs {
					if ing, ok := (*o).(*netv1.Ingress); ok {
						foundIng = true
						if len(ing.Spec.Rules) != 1 {
							t.Fatalf("expected 1 rule, got %d", len(ing.Spec.Rules))
						}
						if ing.Spec.Rules[0].Host != "api.example.com" {
							t.Errorf("expected host api.example.com, got %s", ing.Spec.Rules[0].Host)
						}
						if len(ing.Spec.Rules[0].HTTP.Paths) != 1 {
							t.Fatalf("expected 1 path, got %d", len(ing.Spec.Rules[0].HTTP.Paths))
						}
						if ing.Spec.Rules[0].HTTP.Paths[0].Path != "/v1" {
							t.Errorf("expected path /v1, got %s", ing.Spec.Rules[0].HTTP.Paths[0].Path)
						}
					}
				}
				if !foundIng {
					t.Error("ingress not found")
				}
			},
		},
		{
			name: "container generation error propagates",
			cfg: &Config{
				Name:      "app",
				Namespace: "ns",
				Workload:  DeploymentWorkload,
				Containers: []ContainerConfig{{
					Name:  "app",
					Image: "nginx",
					Resources: &ResourceRequirements{
						Limits: map[string]string{"cpu": "invalid"},
					},
				}},
			},
			wantErr: true,
		},
		{
			name: "statefulset with invalid volume claim template",
			cfg: &Config{
				Name:      "app",
				Namespace: "ns",
				Workload:  StatefulSetWorkload,
				Containers: []ContainerConfig{{
					Name:  "app",
					Image: "nginx",
				}},
				VolumeClaimTemplates: []VolumeClaimTemplate{
					{
						Metadata: struct {
							Name string `json:"name" yaml:"name"`
						}{Name: "data"},
						Spec: struct {
							AccessModes      []string              `json:"accessModes,omitempty" yaml:"accessModes,omitempty"`
							StorageClassName *string               `json:"storageClassName,omitempty" yaml:"storageClassName,omitempty"`
							Resources        *ResourceRequirements `json:"resources,omitempty" yaml:"resources,omitempty"`
						}{
							Resources: &ResourceRequirements{
								Requests: map[string]string{"storage": "bad-format"},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "multiple containers in deployment",
			cfg: &Config{
				Name:      "app",
				Namespace: "ns",
				Workload:  DeploymentWorkload,
				Containers: []ContainerConfig{
					{
						Name:  "app",
						Image: "nginx",
						Ports: []ContainerPort{{Name: "http", ContainerPort: 8080}},
					},
					{
						Name:  "sidecar",
						Image: "proxy:latest",
						Ports: []ContainerPort{{Name: "metrics", ContainerPort: 9090}},
					},
				},
			},
			check: func(t *testing.T, objs []*client.Object) {
				var foundDep bool
				for _, o := range objs {
					if dep, ok := (*o).(*appsv1.Deployment); ok {
						foundDep = true
						if len(dep.Spec.Template.Spec.Containers) != 2 {
							t.Errorf("expected 2 containers, got %d", len(dep.Spec.Template.Spec.Containers))
						}
					}
					// Should also have a service with 2 ports
					if svc, ok := (*o).(*corev1.Service); ok {
						if len(svc.Spec.Ports) != 2 {
							t.Errorf("expected 2 service ports, got %d", len(svc.Spec.Ports))
						}
					}
				}
				if !foundDep {
					t.Error("deployment not found")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objs, err := GenerateResources(tt.cfg, app)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, objs)
			}
		})
	}
}
