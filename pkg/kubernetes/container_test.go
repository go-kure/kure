package kubernetes

import (
	"fmt"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestAddContainerEnvFrom(t *testing.T) {
	tests := []struct {
		name           string
		initialEnvFrom []corev1.EnvFromSource
		newEnvFrom     corev1.EnvFromSource
		expectedResult []corev1.EnvFromSource
	}{
		{
			name:           "add single envFrom to empty list",
			initialEnvFrom: []corev1.EnvFromSource{},
			newEnvFrom: corev1.EnvFromSource{
				Prefix: "CONFIG_",
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "config-map-name",
					},
				},
			},
			expectedResult: []corev1.EnvFromSource{
				{
					Prefix: "CONFIG_",
					ConfigMapRef: &corev1.ConfigMapEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "config-map-name",
						},
					},
				},
			},
		},
		{
			name: "add envFrom to existing list",
			initialEnvFrom: []corev1.EnvFromSource{
				{
					Prefix: "EXISTING_",
					ConfigMapRef: &corev1.ConfigMapEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "existing-config-map",
						},
					},
				},
			},
			newEnvFrom: corev1.EnvFromSource{
				Prefix: "NEW_",
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "new-secret",
					},
				},
			},
			expectedResult: []corev1.EnvFromSource{
				{
					Prefix: "EXISTING_",
					ConfigMapRef: &corev1.ConfigMapEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "existing-config-map",
						},
					},
				},
				{
					Prefix: "NEW_",
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "new-secret",
						},
					},
				},
			},
		},
		{
			name: "add to list with duplicate prefix",
			initialEnvFrom: []corev1.EnvFromSource{
				{
					Prefix: "DUPLICATE_",
					ConfigMapRef: &corev1.ConfigMapEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "config-map1",
						},
					},
				},
			},
			newEnvFrom: corev1.EnvFromSource{
				Prefix: "DUPLICATE_",
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "secret-duplicate",
					},
				},
			},
			expectedResult: []corev1.EnvFromSource{
				{
					Prefix: "DUPLICATE_",
					ConfigMapRef: &corev1.ConfigMapEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "config-map1",
						},
					},
				},
				{
					Prefix: "DUPLICATE_",
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "secret-duplicate",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := &corev1.Container{
				EnvFrom: tt.initialEnvFrom,
			}

			AddContainerEnvFrom(container, tt.newEnvFrom)
			if err := compareEnvFromSources(container.EnvFrom, tt.expectedResult); err != nil {
				t.Error(err)
			}
		})
	}
}
func compareEnvFromSources(got, want []corev1.EnvFromSource) error {
	if len(got) != len(want) {
		return fmt.Errorf("length mismatch: got %d elements, want %d elements", len(got), len(want))
	}

	for i := range got {
		if !reflect.DeepEqual(got[i], want[i]) {
			return fmt.Errorf("mismatch at index %d:\ngot:  %+v\nwant: %+v", i, got[i], want[i])
		}
	}

	return nil
}
func TestAddContainerPort(t *testing.T) {
	tests := []struct {
		name           string
		initialPorts   []corev1.ContainerPort
		newPort        corev1.ContainerPort
		expectedResult []corev1.ContainerPort
	}{
		{
			name:         "add single port to empty ports",
			initialPorts: []corev1.ContainerPort{},
			newPort: corev1.ContainerPort{
				Name:          "http",
				ContainerPort: 8080,
				Protocol:      corev1.ProtocolTCP,
			},
			expectedResult: []corev1.ContainerPort{
				{
					Name:          "http",
					ContainerPort: 8080,
					Protocol:      corev1.ProtocolTCP,
				},
			},
		},
		{
			name: "add port to existing ports",
			initialPorts: []corev1.ContainerPort{
				{
					Name:          "http",
					ContainerPort: 8080,
					Protocol:      corev1.ProtocolTCP,
				},
			},
			newPort: corev1.ContainerPort{
				Name:          "https",
				ContainerPort: 443,
				Protocol:      corev1.ProtocolTCP,
			},
			expectedResult: []corev1.ContainerPort{
				{
					Name:          "http",
					ContainerPort: 8080,
					Protocol:      corev1.ProtocolTCP,
				},
				{
					Name:          "https",
					ContainerPort: 443,
					Protocol:      corev1.ProtocolTCP,
				},
			},
		},
		{
			name: "add duplicate port (same port and protocol)",
			initialPorts: []corev1.ContainerPort{
				{
					Name:          "http",
					ContainerPort: 8080,
					Protocol:      corev1.ProtocolTCP,
				},
			},
			newPort: corev1.ContainerPort{
				Name:          "duplicate-http",
				ContainerPort: 8080,
				Protocol:      corev1.ProtocolTCP,
			},
			expectedResult: []corev1.ContainerPort{
				{
					Name:          "http",
					ContainerPort: 8080,
					Protocol:      corev1.ProtocolTCP,
				},
				{
					Name:          "duplicate-http",
					ContainerPort: 8080,
					Protocol:      corev1.ProtocolTCP,
				},
			},
		},
		{
			name: "add port with different protocol",
			initialPorts: []corev1.ContainerPort{
				{
					Name:          "http",
					ContainerPort: 8080,
					Protocol:      corev1.ProtocolTCP,
				},
			},
			newPort: corev1.ContainerPort{
				Name:          "http-udp",
				ContainerPort: 8080,
				Protocol:      corev1.ProtocolUDP,
			},
			expectedResult: []corev1.ContainerPort{
				{
					Name:          "http",
					ContainerPort: 8080,
					Protocol:      corev1.ProtocolTCP,
				},
				{
					Name:          "http-udp",
					ContainerPort: 8080,
					Protocol:      corev1.ProtocolUDP,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := &corev1.Container{
				Ports: tt.initialPorts,
			}

			AddContainerPort(container, tt.newPort)

			if len(container.Ports) != len(tt.expectedResult) {
				t.Errorf("unexpected number of ports: got %d, want %d", len(container.Ports), len(tt.expectedResult))
			}

			for i, port := range tt.expectedResult {
				if container.Ports[i] != port {
					t.Errorf("unexpected port at index %d: got %+v, want %+v", i, container.Ports[i], port)
				}
			}
		})
	}
}

func TestAddContainerEnv(t *testing.T) {
	tests := []struct {
		name           string
		initialEnv     []corev1.EnvVar
		newEnv         corev1.EnvVar
		expectedResult []corev1.EnvVar
	}{
		{
			name:       "add single env to empty list",
			initialEnv: []corev1.EnvVar{},
			newEnv: corev1.EnvVar{
				Name:  "ENV_VAR",
				Value: "value",
			},
			expectedResult: []corev1.EnvVar{
				{
					Name:  "ENV_VAR",
					Value: "value",
				},
			},
		},
		{
			name: "add env to existing list",
			initialEnv: []corev1.EnvVar{
				{
					Name:  "OLD_VAR",
					Value: "old_value",
				},
			},
			newEnv: corev1.EnvVar{
				Name:  "NEW_VAR",
				Value: "new_value",
			},
			expectedResult: []corev1.EnvVar{
				{
					Name:  "OLD_VAR",
					Value: "old_value",
				},
				{
					Name:  "NEW_VAR",
					Value: "new_value",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := &corev1.Container{
				Env: tt.initialEnv,
			}

			AddContainerEnv(container, tt.newEnv)

			if len(container.Env) != len(tt.expectedResult) {
				t.Errorf("unexpected number of env vars: got %d, want %d", len(container.Env), len(tt.expectedResult))
			}

			for i, env := range tt.expectedResult {
				if container.Env[i] != env {
					t.Errorf("unexpected env at index %d: got %+v, want %+v", i, container.Env[i], env)
				}
			}
		})
	}
}

func TestAdditionalContainerFunctions(t *testing.T) {
	c := &corev1.Container{}

	mount := corev1.VolumeMount{Name: "data", MountPath: "/data"}
	AddContainerVolumeMount(c, mount)
	if len(c.VolumeMounts) != 1 || !reflect.DeepEqual(c.VolumeMounts[0], mount) {
		t.Errorf("volume mount not added")
	}

	dev := corev1.VolumeDevice{Name: "block", DevicePath: "/dev/block"}
	AddContainerVolumeDevice(c, dev)
	if len(c.VolumeDevices) != 1 || c.VolumeDevices[0] != dev {
		t.Errorf("volume device not added")
	}

	probe := corev1.Probe{TimeoutSeconds: 5}
	SetContainerLivenessProbe(c, probe)
	if c.LivenessProbe == nil || *c.LivenessProbe != probe {
		t.Errorf("liveness probe not set")
	}

	SetContainerReadinessProbe(c, probe)
	if c.ReadinessProbe == nil || *c.ReadinessProbe != probe {
		t.Errorf("readiness probe not set")
	}

	SetContainerStartupProbe(c, probe)
	if c.StartupProbe == nil || *c.StartupProbe != probe {
		t.Errorf("startup probe not set")
	}

	sc := corev1.SecurityContext{RunAsUser: new(int64)}
	SetContainerSecurityContext(c, sc)
	if c.SecurityContext == nil || *c.SecurityContext != sc {
		t.Errorf("security context not set")
	}
}

func TestContainerMiscFunctions(t *testing.T) {
	c := &corev1.Container{}

	lc := &corev1.Lifecycle{}
	SetContainerLifecycle(c, lc)
	if c.Lifecycle != lc {
		t.Errorf("lifecycle not set")
	}
}

func TestContainerNilGuards(t *testing.T) {
	assertPanics(t, func() { AddContainerPort(nil, corev1.ContainerPort{}) })
	assertPanics(t, func() { AddContainerEnv(nil, corev1.EnvVar{}) })
	assertPanics(t, func() { AddContainerEnvFrom(nil, corev1.EnvFromSource{}) })
	assertPanics(t, func() { AddContainerVolumeMount(nil, corev1.VolumeMount{}) })
	assertPanics(t, func() { AddContainerVolumeDevice(nil, corev1.VolumeDevice{}) })
	assertPanics(t, func() { SetContainerLivenessProbe(nil, corev1.Probe{}) })
	assertPanics(t, func() { SetContainerReadinessProbe(nil, corev1.Probe{}) })
	assertPanics(t, func() { SetContainerStartupProbe(nil, corev1.Probe{}) })
	assertPanics(t, func() { SetContainerSecurityContext(nil, corev1.SecurityContext{}) })
}
