package kubernetes

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestRestrictedPodSecurityContext(t *testing.T) {
	sc := RestrictedPodSecurityContext()
	if sc == nil {
		t.Fatal("expected non-nil PodSecurityContext")
	}
	if sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot {
		t.Error("expected RunAsNonRoot to be true")
	}
	if sc.SeccompProfile == nil || sc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Error("expected SeccompProfile RuntimeDefault")
	}
}

func TestRestrictedSecurityContext(t *testing.T) {
	sc := RestrictedSecurityContext()
	if sc == nil {
		t.Fatal("expected non-nil SecurityContext")
	}
	if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
		t.Error("expected AllowPrivilegeEscalation to be false")
	}
	if sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot {
		t.Error("expected RunAsNonRoot to be true")
	}
	if sc.ReadOnlyRootFilesystem == nil || !*sc.ReadOnlyRootFilesystem {
		t.Error("expected ReadOnlyRootFilesystem to be true")
	}
	if sc.Capabilities == nil || len(sc.Capabilities.Drop) != 1 || sc.Capabilities.Drop[0] != "ALL" {
		t.Error("expected capabilities to drop ALL")
	}
	if sc.SeccompProfile == nil || sc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Error("expected SeccompProfile RuntimeDefault")
	}
}

func TestBaselinePodSecurityContext(t *testing.T) {
	sc := BaselinePodSecurityContext()
	if sc == nil {
		t.Fatal("expected non-nil PodSecurityContext")
	}
	if sc.SeccompProfile == nil || sc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Error("expected SeccompProfile RuntimeDefault")
	}
}

func TestBaselineSecurityContext(t *testing.T) {
	sc := BaselineSecurityContext()
	if sc == nil {
		t.Fatal("expected non-nil SecurityContext")
	}
	if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
		t.Error("expected AllowPrivilegeEscalation to be false")
	}
}

func TestPrivilegedPodSecurityContext(t *testing.T) {
	sc := PrivilegedPodSecurityContext()
	if sc == nil {
		t.Fatal("expected non-nil PodSecurityContext")
	}
}

func TestPrivilegedSecurityContext(t *testing.T) {
	sc := PrivilegedSecurityContext()
	if sc == nil {
		t.Fatal("expected non-nil SecurityContext")
	}
}

func TestPodSecurityContextForLevel(t *testing.T) {
	tests := []struct {
		name    string
		level   PSALevel
		wantErr bool
	}{
		{"restricted", PSARestricted, false},
		{"baseline", PSABaseline, false},
		{"privileged", PSAPrivileged, false},
		{"invalid", PSALevel("invalid"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc, err := PodSecurityContextForLevel(tt.level)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sc == nil {
				t.Error("expected non-nil PodSecurityContext")
			}
		})
	}
}

func TestSecurityContextForLevel(t *testing.T) {
	tests := []struct {
		name    string
		level   PSALevel
		wantErr bool
	}{
		{"restricted", PSARestricted, false},
		{"baseline", PSABaseline, false},
		{"privileged", PSAPrivileged, false},
		{"invalid", PSALevel("invalid"), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc, err := SecurityContextForLevel(tt.level)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sc == nil {
				t.Error("expected non-nil SecurityContext")
			}
		})
	}
}

func TestValidateContainerPSA(t *testing.T) {
	tests := []struct {
		name      string
		container *corev1.Container
		level     PSALevel
		wantErr   bool
	}{
		{
			name:      "nil container",
			container: nil,
			level:     PSARestricted,
			wantErr:   true,
		},
		{
			name: "restricted compliant container",
			container: &corev1.Container{
				Name:            "test",
				SecurityContext: RestrictedSecurityContext(),
			},
			level:   PSARestricted,
			wantErr: false,
		},
		{
			name: "restricted missing security context",
			container: &corev1.Container{
				Name: "test",
			},
			level:   PSARestricted,
			wantErr: true,
		},
		{
			name: "restricted with privilege escalation",
			container: &corev1.Container{
				Name: "test",
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: boolPtr(true),
					RunAsNonRoot:             boolPtr(true),
					Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
					SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
				},
			},
			level:   PSARestricted,
			wantErr: true,
		},
		{
			name: "restricted missing drop ALL",
			container: &corev1.Container{
				Name: "test",
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: boolPtr(false),
					RunAsNonRoot:             boolPtr(true),
					SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
				},
			},
			level:   PSARestricted,
			wantErr: true,
		},
		{
			name: "baseline compliant container",
			container: &corev1.Container{
				Name: "test",
				SecurityContext: &corev1.SecurityContext{
					AllowPrivilegeEscalation: boolPtr(false),
				},
			},
			level:   PSABaseline,
			wantErr: false,
		},
		{
			name: "baseline with privileged mode",
			container: &corev1.Container{
				Name: "test",
				SecurityContext: &corev1.SecurityContext{
					Privileged: boolPtr(true),
				},
			},
			level:   PSABaseline,
			wantErr: true,
		},
		{
			name: "baseline with disallowed capability",
			container: &corev1.Container{
				Name: "test",
				SecurityContext: &corev1.SecurityContext{
					Capabilities: &corev1.Capabilities{
						Add: []corev1.Capability{"SYS_ADMIN"},
					},
				},
			},
			level:   PSABaseline,
			wantErr: true,
		},
		{
			name: "baseline with allowed capability",
			container: &corev1.Container{
				Name: "test",
				SecurityContext: &corev1.SecurityContext{
					Capabilities: &corev1.Capabilities{
						Add: []corev1.Capability{"NET_BIND_SERVICE"},
					},
				},
			},
			level:   PSABaseline,
			wantErr: false,
		},
		{
			name: "privileged allows anything",
			container: &corev1.Container{
				Name: "test",
				SecurityContext: &corev1.SecurityContext{
					Privileged: boolPtr(true),
				},
			},
			level:   PSAPrivileged,
			wantErr: false,
		},
		{
			name: "invalid level",
			container: &corev1.Container{
				Name: "test",
			},
			level:   PSALevel("invalid"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateContainerPSA(tt.container, tt.level)
			if tt.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidatePodSpecPSA(t *testing.T) {
	restrictedContainer := corev1.Container{
		Name:            "test",
		SecurityContext: RestrictedSecurityContext(),
	}

	tests := []struct {
		name    string
		spec    *corev1.PodSpec
		level   PSALevel
		wantErr bool
	}{
		{
			name:    "nil spec",
			spec:    nil,
			level:   PSARestricted,
			wantErr: true,
		},
		{
			name: "restricted compliant",
			spec: &corev1.PodSpec{
				SecurityContext: RestrictedPodSecurityContext(),
				Containers:      []corev1.Container{restrictedContainer},
			},
			level:   PSARestricted,
			wantErr: false,
		},
		{
			name: "restricted missing pod RunAsNonRoot",
			spec: &corev1.PodSpec{
				SecurityContext: &corev1.PodSecurityContext{
					SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
				},
				Containers: []corev1.Container{restrictedContainer},
			},
			level:   PSARestricted,
			wantErr: true,
		},
		{
			name: "baseline with hostNetwork",
			spec: &corev1.PodSpec{
				HostNetwork: true,
			},
			level:   PSABaseline,
			wantErr: true,
		},
		{
			name: "baseline with hostPID",
			spec: &corev1.PodSpec{
				HostPID: true,
			},
			level:   PSABaseline,
			wantErr: true,
		},
		{
			name: "baseline with hostIPC",
			spec: &corev1.PodSpec{
				HostIPC: true,
			},
			level:   PSABaseline,
			wantErr: true,
		},
		{
			name: "privileged allows anything",
			spec: &corev1.PodSpec{
				HostNetwork: true,
				HostPID:     true,
			},
			level:   PSAPrivileged,
			wantErr: false,
		},
		{
			name:    "invalid level",
			spec:    &corev1.PodSpec{},
			level:   PSALevel("invalid"),
			wantErr: true,
		},
		{
			name: "restricted with non-compliant init container",
			spec: &corev1.PodSpec{
				SecurityContext: RestrictedPodSecurityContext(),
				Containers:      []corev1.Container{restrictedContainer},
				InitContainers: []corev1.Container{
					{Name: "init", SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: boolPtr(true),
					}},
				},
			},
			level:   PSARestricted,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePodSpecPSA(tt.spec, tt.level)
			if tt.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
