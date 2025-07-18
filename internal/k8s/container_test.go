package k8s

import (
	"fmt"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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

			if err := AddContainerEnvFrom(container, tt.newEnvFrom); err != nil {
				t.Fatalf("AddContainerEnvFrom returned error: %v", err)
			}
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
func TestCreateContainer(t *testing.T) {
	tests := []struct {
		name           string
		inputName      string
		inputImage     string
		inputCommand   []string
		inputArgs      []string
		expectedResult corev1.Container
	}{
		{
			name:         "simple container",
			inputName:    "test-container",
			inputImage:   "test-image",
			inputCommand: []string{"echo"},
			inputArgs:    []string{"hello"},
			expectedResult: corev1.Container{
				Name:    "test-container",
				Image:   "test-image",
				Command: []string{"echo"},
				Args:    []string{"hello"},
				Ports:   []corev1.ContainerPort{},
				EnvFrom: []corev1.EnvFromSource{},
				Env:     []corev1.EnvVar{},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"memory": resource.MustParse("256Mi"),
					},
					Requests: corev1.ResourceList{
						"cpu":    resource.MustParse("100m"),
						"memory": resource.MustParse("256Mi"),
					},
				},
				VolumeMounts:    []corev1.VolumeMount{},
				VolumeDevices:   []corev1.VolumeDevice{},
				LivenessProbe:   &corev1.Probe{},
				ReadinessProbe:  &corev1.Probe{},
				StartupProbe:    &corev1.Probe{},
				ImagePullPolicy: corev1.PullIfNotPresent,
				SecurityContext: &corev1.SecurityContext{},
			},
		},
		{
			name:         "container with empty command and args",
			inputName:    "container-no-cmd",
			inputImage:   "empty-cmd-image",
			inputCommand: nil,
			inputArgs:    nil,
			expectedResult: corev1.Container{
				Name:    "container-no-cmd",
				Image:   "empty-cmd-image",
				Command: nil,
				Args:    nil,
				Ports:   []corev1.ContainerPort{},
				EnvFrom: []corev1.EnvFromSource{},
				Env:     []corev1.EnvVar{},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"memory": resource.MustParse("256Mi"),
					},
					Requests: corev1.ResourceList{
						"cpu":    resource.MustParse("100m"),
						"memory": resource.MustParse("256Mi"),
					},
				},
				VolumeMounts:    []corev1.VolumeMount{},
				VolumeDevices:   []corev1.VolumeDevice{},
				LivenessProbe:   &corev1.Probe{},
				ReadinessProbe:  &corev1.Probe{},
				StartupProbe:    &corev1.Probe{},
				ImagePullPolicy: corev1.PullIfNotPresent,
				SecurityContext: &corev1.SecurityContext{},
			},
		},
		{
			name:         "container with args only",
			inputName:    "args-only",
			inputImage:   "args-only-image",
			inputCommand: []string{},
			inputArgs:    []string{"arg1", "arg2"},
			expectedResult: corev1.Container{
				Name:    "args-only",
				Image:   "args-only-image",
				Command: []string{},
				Args:    []string{"arg1", "arg2"},
				Ports:   []corev1.ContainerPort{},
				EnvFrom: []corev1.EnvFromSource{},
				Env:     []corev1.EnvVar{},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"memory": resource.MustParse("256Mi"),
					},
					Requests: corev1.ResourceList{
						"cpu":    resource.MustParse("100m"),
						"memory": resource.MustParse("256Mi"),
					},
				},
				VolumeMounts:    []corev1.VolumeMount{},
				VolumeDevices:   []corev1.VolumeDevice{},
				LivenessProbe:   &corev1.Probe{},
				ReadinessProbe:  &corev1.Probe{},
				StartupProbe:    &corev1.Probe{},
				ImagePullPolicy: corev1.PullIfNotPresent,
				SecurityContext: &corev1.SecurityContext{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CreateContainer(tt.inputName, tt.inputImage, tt.inputCommand, tt.inputArgs)
			assertContainerEqual(t, result, &tt.expectedResult)
		})
	}
}
func assertContainerEqual(t *testing.T, got, want *corev1.Container) {
	t.Helper()

	if got.Name != want.Name {
		t.Errorf("container name mismatch: got %q, want %q", got.Name, want.Name)
	}
	if got.Image != want.Image {
		t.Errorf("container image mismatch: got %q, want %q", got.Image, want.Image)
	}
	if got.ImagePullPolicy != want.ImagePullPolicy {
		t.Errorf("image pull policy mismatch: got %v, want %v", got.ImagePullPolicy, want.ImagePullPolicy)
	}

	if !reflect.DeepEqual(got.Command, want.Command) {
		t.Errorf("command mismatch: got %v, want %v", got.Command, want.Command)
	}
	if !reflect.DeepEqual(got.Args, want.Args) {
		t.Errorf("args mismatch: got %v, want %v", got.Args, want.Args)
	}
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

			if err := AddContainerPort(container, tt.newPort); err != nil {
				t.Fatalf("AddContainerPort returned error: %v", err)
			}

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

			if err := AddContainerEnv(container, tt.newEnv); err != nil {
				t.Fatalf("AddContainerEnv returned error: %v", err)
			}

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
	if err := AddContainerVolumeMount(c, mount); err != nil {
		t.Fatalf("AddContainerVolumeMount returned error: %v", err)
	}
	if len(c.VolumeMounts) != 1 || c.VolumeMounts[0] != mount {
		t.Errorf("volume mount not added")
	}

	dev := corev1.VolumeDevice{Name: "block", DevicePath: "/dev/block"}
	if err := AddContainerVolumeDevice(c, dev); err != nil {
		t.Fatalf("AddContainerVolumeDevice returned error: %v", err)
	}
	if len(c.VolumeDevices) != 1 || c.VolumeDevices[0] != dev {
		t.Errorf("volume device not added")
	}

	probe := corev1.Probe{TimeoutSeconds: 5}
	if err := SetContainerLivenessProbe(c, probe); err != nil {
		t.Fatalf("SetContainerLivenessProbe returned error: %v", err)
	}
	if c.LivenessProbe == nil || *c.LivenessProbe != probe {
		t.Errorf("liveness probe not set")
	}

	if err := SetContainerReadinessProbe(c, probe); err != nil {
		t.Fatalf("SetContainerReadinessProbe returned error: %v", err)
	}
	if c.ReadinessProbe == nil || *c.ReadinessProbe != probe {
		t.Errorf("readiness probe not set")
	}

	if err := SetContainerStartupProbe(c, probe); err != nil {
		t.Fatalf("SetContainerStartupProbe returned error: %v", err)
	}
	if c.StartupProbe == nil || *c.StartupProbe != probe {
		t.Errorf("startup probe not set")
	}

	resources := corev1.ResourceRequirements{}
	if err := SetContainerResources(c, resources); err != nil {
		t.Fatalf("SetContainerResources returned error: %v", err)
	}
	if !reflect.DeepEqual(c.Resources, resources) {
		t.Errorf("resources not set")
	}

	if err := SetContainerImagePullPolicy(c, corev1.PullAlways); err != nil {
		t.Fatalf("SetContainerImagePullPolicy returned error: %v", err)
	}
	if c.ImagePullPolicy != corev1.PullAlways {
		t.Errorf("image pull policy not set")
	}

	sc := corev1.SecurityContext{RunAsUser: new(int64)}
	if err := SetContainerSecurityContext(c, sc); err != nil {
		t.Fatalf("SetContainerSecurityContext returned error: %v", err)
	}
	if c.SecurityContext == nil || *c.SecurityContext != sc {
		t.Errorf("security context not set")
	}
}

func TestContainerMiscFunctions(t *testing.T) {
	c := &corev1.Container{}

	SetContainerWorkingDir(c, "/work")
	if c.WorkingDir != "/work" {
		t.Errorf("working dir not set")
	}

	lc := &corev1.Lifecycle{}
	SetContainerLifecycle(c, lc)
	if c.Lifecycle != lc {
		t.Errorf("lifecycle not set")
	}

	SetContainerTerminationMessagePath(c, "/tmp/msg")
	if c.TerminationMessagePath != "/tmp/msg" {
		t.Errorf("termination message path not set")
	}

	SetContainerTerminationMessagePolicy(c, corev1.TerminationMessageReadFile)
	if c.TerminationMessagePolicy != corev1.TerminationMessageReadFile {
		t.Errorf("termination message policy not set")
	}

	SetContainerStdin(c, true)
	if !c.Stdin {
		t.Errorf("stdin not set")
	}

	SetContainerStdinOnce(c, true)
	if !c.StdinOnce {
		t.Errorf("stdin once not set")
	}

	SetContainerTTY(c, true)
	if !c.TTY {
		t.Errorf("tty not set")
	}
}

func TestContainerSetters(t *testing.T) {
	c := &corev1.Container{}

	SetContainerImage(c, "nginx")
	if c.Image != "nginx" {
		t.Errorf("image not set")
	}

	cmd := []string{"/bin/sh"}
	SetContainerCommand(c, cmd)
	if !reflect.DeepEqual(c.Command, cmd) {
		t.Errorf("command not set")
	}

	args := []string{"-c", "echo"}
	SetContainerArgs(c, args)
	if !reflect.DeepEqual(c.Args, args) {
		t.Errorf("args not set")
	}
}
