package kubernetes

import (
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"

	kerrors "github.com/go-kure/kure/pkg/errors"
)

func psaInt64(v int64) *int64 { return &v }

// baselinePod returns a pod that meets the baseline level: one container with
// no securityContext. mutate changes it into the case under test.
func baselinePod(mutate func(*corev1.PodSpec)) *corev1.PodSpec {
	spec := &corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}}
	if mutate != nil {
		mutate(spec)
	}
	return spec
}

// restrictedPod returns a pod that meets the restricted level, with the
// settings on the container and the pod. mutate changes it into the case.
func restrictedPod(mutate func(*corev1.PodSpec)) *corev1.PodSpec {
	spec := &corev1.PodSpec{
		SecurityContext: RestrictedPodSecurityContext(),
		Containers:      []corev1.Container{{Name: "app", SecurityContext: RestrictedSecurityContext()}},
	}
	if mutate != nil {
		mutate(spec)
	}
	return spec
}

// podLevelOnlyPod returns a restricted pod whose runAsNonRoot and seccomp
// profile sit on the pod only; the container sets just the container-only
// fields.
func podLevelOnlyPod(mutate func(*corev1.PodSpec)) *corev1.PodSpec {
	spec := &corev1.PodSpec{
		SecurityContext: RestrictedPodSecurityContext(),
		Containers: []corev1.Container{{Name: "app", SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: boolPtr(false),
			Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		}}},
	}
	if mutate != nil {
		mutate(spec)
	}
	return spec
}

func containerSC(spec *corev1.PodSpec) *corev1.SecurityContext {
	if spec.Containers[0].SecurityContext == nil {
		spec.Containers[0].SecurityContext = &corev1.SecurityContext{}
	}
	return spec.Containers[0].SecurityContext
}

func podSC(spec *corev1.PodSpec) *corev1.PodSecurityContext {
	if spec.SecurityContext == nil {
		spec.SecurityContext = &corev1.PodSecurityContext{}
	}
	return spec.SecurityContext
}

type psaCase struct {
	name      string
	spec      *corev1.PodSpec
	level     PSALevel
	wantField string // empty means the spec must be compliant
}

func runPSACases(t *testing.T, cases []psaCase) {
	t.Helper()
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePodSpecPSA(tt.spec, tt.level)
			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("expected compliant, got: %v", err)
				}
				return
			}
			var psaErr *kerrors.PSAViolationError
			if !errors.As(err, &psaErr) {
				t.Fatalf("expected *PSAViolationError with field %q, got %v", tt.wantField, err)
			}
			if psaErr.Field != tt.wantField {
				t.Errorf("Field = %q, want %q (%v)", psaErr.Field, tt.wantField, err)
			}
		})
	}
}

func TestValidatePodSpecPSA_BaselineRows(t *testing.T) {
	hostProcess := func(v bool) *corev1.WindowsSecurityContextOptions {
		return &corev1.WindowsSecurityContextOptions{HostProcess: &v}
	}
	unmasked := corev1.UnmaskedProcMount
	runtimeDefault := &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}
	unconfinedSeccomp := &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeUnconfined}
	unconfinedAppArmor := &corev1.AppArmorProfile{Type: corev1.AppArmorProfileTypeUnconfined}

	runPSACases(t, []psaCase{
		{"hostProcess on the pod", baselinePod(func(s *corev1.PodSpec) {
			podSC(s).WindowsOptions = hostProcess(true)
		}), PSABaseline, "securityContext.windowsOptions.hostProcess"},
		{"hostProcess on a container", baselinePod(func(s *corev1.PodSpec) {
			containerSC(s).WindowsOptions = hostProcess(true)
		}), PSABaseline, "containers[0].securityContext.windowsOptions.hostProcess"},
		{"hostProcess on an init container", baselinePod(func(s *corev1.PodSpec) {
			s.InitContainers = []corev1.Container{{Name: "init", SecurityContext: &corev1.SecurityContext{WindowsOptions: hostProcess(true)}}}
		}), PSABaseline, "initContainers[0].securityContext.windowsOptions.hostProcess"},
		{"hostProcess false is allowed", baselinePod(func(s *corev1.PodSpec) {
			podSC(s).WindowsOptions = hostProcess(false)
			containerSC(s).WindowsOptions = hostProcess(false)
		}), PSABaseline, ""},

		{"procMount Unmasked", baselinePod(func(s *corev1.PodSpec) {
			containerSC(s).ProcMount = &unmasked
		}), PSABaseline, "containers[0].securityContext.procMount"},
		{"procMount Default is allowed", baselinePod(func(s *corev1.PodSpec) {
			def := corev1.DefaultProcMount
			containerSC(s).ProcMount = &def
		}), PSABaseline, ""},
		{"procMount Unmasked in a user namespace is allowed", baselinePod(func(s *corev1.PodSpec) {
			s.HostUsers = boolPtr(false)
			containerSC(s).ProcMount = &unmasked
		}), PSABaseline, ""},
		{"procMount Unmasked with hostUsers true", baselinePod(func(s *corev1.PodSpec) {
			s.HostUsers = boolPtr(true)
			containerSC(s).ProcMount = &unmasked
		}), PSABaseline, "containers[0].securityContext.procMount"},

		{"seccomp Unconfined on the pod", baselinePod(func(s *corev1.PodSpec) {
			podSC(s).SeccompProfile = unconfinedSeccomp
		}), PSABaseline, "securityContext.seccompProfile.type"},
		{"seccomp Unconfined on a container", baselinePod(func(s *corev1.PodSpec) {
			containerSC(s).SeccompProfile = unconfinedSeccomp
		}), PSABaseline, "containers[0].securityContext.seccompProfile.type"},
		{"seccomp RuntimeDefault and Localhost are allowed", baselinePod(func(s *corev1.PodSpec) {
			podSC(s).SeccompProfile = runtimeDefault
			containerSC(s).SeccompProfile = &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeLocalhost}
		}), PSABaseline, ""},

		{"appArmor Unconfined on the pod", baselinePod(func(s *corev1.PodSpec) {
			podSC(s).AppArmorProfile = unconfinedAppArmor
		}), PSABaseline, "securityContext.appArmorProfile.type"},
		{"appArmor Unconfined on an ephemeral container", baselinePod(func(s *corev1.PodSpec) {
			s.EphemeralContainers = []corev1.EphemeralContainer{{EphemeralContainerCommon: corev1.EphemeralContainerCommon{
				Name: "debug", SecurityContext: &corev1.SecurityContext{AppArmorProfile: unconfinedAppArmor},
			}}}
		}), PSABaseline, "ephemeralContainers[0].securityContext.appArmorProfile.type"},
		{"appArmor RuntimeDefault is allowed", baselinePod(func(s *corev1.PodSpec) {
			containerSC(s).AppArmorProfile = &corev1.AppArmorProfile{Type: corev1.AppArmorProfileTypeRuntimeDefault}
		}), PSABaseline, ""},

		{"seLinux type outside the allowed set on the pod", baselinePod(func(s *corev1.PodSpec) {
			podSC(s).SELinuxOptions = &corev1.SELinuxOptions{Type: "spc_t"}
		}), PSABaseline, "securityContext.seLinuxOptions.type"},
		{"seLinux user on a container", baselinePod(func(s *corev1.PodSpec) {
			containerSC(s).SELinuxOptions = &corev1.SELinuxOptions{User: "system_u"}
		}), PSABaseline, "containers[0].securityContext.seLinuxOptions.user"},
		{"seLinux role on a container", baselinePod(func(s *corev1.PodSpec) {
			containerSC(s).SELinuxOptions = &corev1.SELinuxOptions{Role: "system_r"}
		}), PSABaseline, "containers[0].securityContext.seLinuxOptions.role"},
		{"seLinux allowed types and a level are allowed", baselinePod(func(s *corev1.PodSpec) {
			podSC(s).SELinuxOptions = &corev1.SELinuxOptions{Type: "container_engine_t", Level: "s0:c1,c2"}
			containerSC(s).SELinuxOptions = &corev1.SELinuxOptions{Type: "container_kvm_t"}
		}), PSABaseline, ""},

		{"unsafe sysctl", baselinePod(func(s *corev1.PodSpec) {
			podSC(s).Sysctls = []corev1.Sysctl{{Name: "net.ipv4.tcp_rmem", Value: "1"}, {Name: "kernel.msgmax", Value: "1"}}
		}), PSABaseline, "securityContext.sysctls"},
		{"safe sysctls are allowed", baselinePod(func(s *corev1.PodSpec) {
			podSC(s).Sysctls = []corev1.Sysctl{
				{Name: "net.ipv4.tcp_rmem", Value: "1"},
				{Name: "net.ipv4.tcp_notsent_lowat", Value: "1"},
				{Name: "kernel.shm_rmid_forced", Value: "1"},
			}
		}), PSABaseline, ""},
	})
}

func TestValidatePodSpecPSA_RestrictedRows(t *testing.T) {
	runPSACases(t, []psaCase{
		{"compliant with settings on container and pod", restrictedPod(nil), PSARestricted, ""},

		// capabilities.add
		{"add a baseline capability", restrictedPod(func(s *corev1.PodSpec) {
			containerSC(s).Capabilities.Add = []corev1.Capability{"CHOWN"}
		}), PSARestricted, "containers[0].securityContext.capabilities.add"},
		{"add only NET_BIND_SERVICE", restrictedPod(func(s *corev1.PodSpec) {
			containerSC(s).Capabilities.Add = []corev1.Capability{"NET_BIND_SERVICE"}
		}), PSARestricted, ""},
		{"a baseline capability stays allowed at baseline", baselinePod(func(s *corev1.PodSpec) {
			containerSC(s).Capabilities = &corev1.Capabilities{Add: []corev1.Capability{"CHOWN"}}
		}), PSABaseline, ""},

		// runAsUser
		{"runAsUser 0 on a container", restrictedPod(func(s *corev1.PodSpec) {
			containerSC(s).RunAsUser = psaInt64(0)
		}), PSARestricted, "containers[0].securityContext.runAsUser"},
		{"runAsUser 0 on the pod", restrictedPod(func(s *corev1.PodSpec) {
			podSC(s).RunAsUser = psaInt64(0)
		}), PSARestricted, "securityContext.runAsUser"},
		{"runAsUser 1000", restrictedPod(func(s *corev1.PodSpec) {
			containerSC(s).RunAsUser = psaInt64(1000)
		}), PSARestricted, ""},
		{"runAsUser 0 is not a baseline rule", baselinePod(func(s *corev1.PodSpec) {
			containerSC(s).RunAsUser = psaInt64(0)
		}), PSABaseline, ""},

		// pod value or every container value
		{"runAsNonRoot and seccomp on the pod only", podLevelOnlyPod(nil), PSARestricted, ""},
		{"runAsNonRoot on every container only", podLevelOnlyPod(func(s *corev1.PodSpec) {
			podSC(s).RunAsNonRoot = nil
			containerSC(s).RunAsNonRoot = boolPtr(true)
		}), PSARestricted, ""},
		{"runAsNonRoot on neither the pod nor the container", podLevelOnlyPod(func(s *corev1.PodSpec) {
			podSC(s).RunAsNonRoot = nil
		}), PSARestricted, "containers[0].securityContext.runAsNonRoot"},
		{"runAsNonRoot false on a container over a true pod", podLevelOnlyPod(func(s *corev1.PodSpec) {
			containerSC(s).RunAsNonRoot = boolPtr(false)
		}), PSARestricted, "containers[0].securityContext.runAsNonRoot"},
		{"second container misses runAsNonRoot", podLevelOnlyPod(func(s *corev1.PodSpec) {
			podSC(s).RunAsNonRoot = nil
			containerSC(s).RunAsNonRoot = boolPtr(true)
			s.Containers = append(s.Containers, corev1.Container{Name: "side", SecurityContext: &corev1.SecurityContext{
				AllowPrivilegeEscalation: boolPtr(false),
				Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
				SeccompProfile:           &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
			}})
		}), PSARestricted, "containers[1].securityContext.runAsNonRoot"},
		{"seccomp unset on the pod and the container", podLevelOnlyPod(func(s *corev1.PodSpec) {
			podSC(s).SeccompProfile = nil
		}), PSARestricted, "containers[0].securityContext.seccompProfile"},
		{"seccomp Unconfined on a container over a valid pod", podLevelOnlyPod(func(s *corev1.PodSpec) {
			containerSC(s).SeccompProfile = &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeUnconfined}
		}), PSARestricted, "containers[0].securityContext.seccompProfile"},
		{"seccomp Localhost on a container over an Unconfined pod", podLevelOnlyPod(func(s *corev1.PodSpec) {
			podSC(s).SeccompProfile = &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeUnconfined}
			containerSC(s).SeccompProfile = &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeLocalhost}
		}), PSARestricted, "securityContext.seccompProfile"},

		// container without a securityContext
		{"container without a securityContext", podLevelOnlyPod(func(s *corev1.PodSpec) {
			s.Containers[0].SecurityContext = nil
		}), PSARestricted, "containers[0].securityContext"},

		// settings on the containers only, no pod securityContext
		{"no pod securityContext when the container sets everything", restrictedPod(func(s *corev1.PodSpec) {
			s.SecurityContext = nil
		}), PSARestricted, ""},
		{"drop list without ALL", restrictedPod(func(s *corev1.PodSpec) {
			containerSC(s).Capabilities.Drop = []corev1.Capability{"NET_RAW"}
		}), PSARestricted, "containers[0].securityContext.capabilities"},
		{"privileged container at restricted", restrictedPod(func(s *corev1.PodSpec) {
			containerSC(s).Privileged = boolPtr(true)
		}), PSARestricted, "containers[0].securityContext.privileged"},
	})
}

func TestValidatePodSpecPSA_UserNamespaceExemption(t *testing.T) {
	userNS := func(s *corev1.PodSpec) { s.HostUsers = boolPtr(false) }
	runPSACases(t, []psaCase{
		{"runAsUser 0 and no runAsNonRoot in a user namespace", restrictedPod(func(s *corev1.PodSpec) {
			userNS(s)
			podSC(s).RunAsNonRoot = nil
			containerSC(s).RunAsNonRoot = boolPtr(false)
			containerSC(s).RunAsUser = psaInt64(0)
		}), PSARestricted, ""},
		{"the same pod with hostUsers true", restrictedPod(func(s *corev1.PodSpec) {
			s.HostUsers = boolPtr(true)
			podSC(s).RunAsNonRoot = nil
			containerSC(s).RunAsNonRoot = boolPtr(false)
			containerSC(s).RunAsUser = psaInt64(0)
		}), PSARestricted, "containers[0].securityContext.runAsNonRoot"},
		{"the same pod with hostUsers unset", restrictedPod(func(s *corev1.PodSpec) {
			podSC(s).RunAsUser = psaInt64(0)
		}), PSARestricted, "securityContext.runAsUser"},
		{"a user namespace does not exempt procMount at restricted", restrictedPod(func(s *corev1.PodSpec) {
			userNS(s)
			unmasked := corev1.UnmaskedProcMount
			containerSC(s).ProcMount = &unmasked
		}), PSARestricted, "containers[0].securityContext.procMount"},
		{"a user namespace does not exempt allowPrivilegeEscalation", restrictedPod(func(s *corev1.PodSpec) {
			userNS(s)
			containerSC(s).AllowPrivilegeEscalation = boolPtr(true)
		}), PSARestricted, "containers[0].securityContext.allowPrivilegeEscalation"},
		{"a user namespace does not exempt capabilities", restrictedPod(func(s *corev1.PodSpec) {
			userNS(s)
			containerSC(s).Capabilities = nil
		}), PSARestricted, "containers[0].securityContext.capabilities"},
		{"a user namespace does not exempt seccomp", restrictedPod(func(s *corev1.PodSpec) {
			userNS(s)
			podSC(s).SeccompProfile = &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeUnconfined}
		}), PSARestricted, "securityContext.seccompProfile"},
	})
}

func TestValidatePodSpecPSA_WindowsExemption(t *testing.T) {
	windows := func(s *corev1.PodSpec) { s.OS = &corev1.PodOS{Name: corev1.Windows} }
	runPSACases(t, []psaCase{
		{"windows pod without allowPrivilegeEscalation, capabilities or seccomp", podLevelOnlyPod(func(s *corev1.PodSpec) {
			windows(s)
			podSC(s).SeccompProfile = nil
			s.Containers[0].SecurityContext = &corev1.SecurityContext{}
			podSC(s).RunAsNonRoot = boolPtr(true)
		}), PSARestricted, ""},
		{"windows pod without any container securityContext", restrictedPod(func(s *corev1.PodSpec) {
			windows(s)
			s.Containers[0].SecurityContext = nil
		}), PSARestricted, ""},
		{"the same pod on linux", restrictedPod(func(s *corev1.PodSpec) {
			s.OS = &corev1.PodOS{Name: corev1.Linux}
			s.Containers[0].SecurityContext = nil
		}), PSARestricted, "containers[0].securityContext"},
		{"windows pod adding a capability", restrictedPod(func(s *corev1.PodSpec) {
			windows(s)
			containerSC(s).Capabilities.Add = []corev1.Capability{"CHOWN"}
		}), PSARestricted, ""},
		// The restricted capabilities and seccomp rows replace the baseline
		// ones upstream, so a Windows pod skips both there.
		{"windows pod adding a capability outside the baseline list", restrictedPod(func(s *corev1.PodSpec) {
			windows(s)
			containerSC(s).Capabilities.Add = []corev1.Capability{"SYS_ADMIN"}
		}), PSARestricted, ""},
		{"windows pod with Unconfined seccomp on the pod", restrictedPod(func(s *corev1.PodSpec) {
			windows(s)
			podSC(s).SeccompProfile = &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeUnconfined}
		}), PSARestricted, ""},
		{"windows pod with Unconfined seccomp on the container", restrictedPod(func(s *corev1.PodSpec) {
			windows(s)
			containerSC(s).SeccompProfile = &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeUnconfined}
		}), PSARestricted, ""},
		{"windows pod with Unconfined seccomp still fails baseline", baselinePod(func(s *corev1.PodSpec) {
			windows(s)
			podSC(s).SeccompProfile = &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeUnconfined}
		}), PSABaseline, "securityContext.seccompProfile.type"},
		{"windows does not exempt runAsNonRoot", restrictedPod(func(s *corev1.PodSpec) {
			windows(s)
			podSC(s).RunAsNonRoot = nil
			containerSC(s).RunAsNonRoot = nil
		}), PSARestricted, "containers[0].securityContext.runAsNonRoot"},
		{"windows does not exempt runAsUser", restrictedPod(func(s *corev1.PodSpec) {
			windows(s)
			containerSC(s).RunAsUser = psaInt64(0)
		}), PSARestricted, "containers[0].securityContext.runAsUser"},
		{"windows does not exempt privileged", restrictedPod(func(s *corev1.PodSpec) {
			windows(s)
			containerSC(s).Privileged = boolPtr(true)
		}), PSARestricted, "containers[0].securityContext.privileged"},
		{"windows does not exempt hostProcess", restrictedPod(func(s *corev1.PodSpec) {
			windows(s)
			containerSC(s).WindowsOptions = &corev1.WindowsSecurityContextOptions{HostProcess: boolPtr(true)}
		}), PSARestricted, "containers[0].securityContext.windowsOptions.hostProcess"},
	})
}

func TestBaselineAllowedCapabilities(t *testing.T) {
	got := BaselineAllowedCapabilities()
	want := []corev1.Capability{
		"AUDIT_WRITE", "CHOWN", "DAC_OVERRIDE", "FOWNER", "FSETID", "KILL", "MKNOD",
		"NET_BIND_SERVICE", "SETFCAP", "SETGID", "SETPCAP", "SETUID", "SYS_CHROOT",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d capabilities %v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("capability %d = %q, want %q", i, got[i], want[i])
		}
	}

	// The list and the validator agree: every listed capability passes
	// baseline, and one outside the list does not.
	for _, c := range got {
		spec := baselinePod(func(s *corev1.PodSpec) {
			containerSC(s).Capabilities = &corev1.Capabilities{Add: []corev1.Capability{c}}
		})
		if err := ValidatePodSpecPSA(spec, PSABaseline); err != nil {
			t.Errorf("listed capability %q refused at baseline: %v", c, err)
		}
	}

	// The caller owns the slice: changing it does not change the next call.
	got[0] = "CHANGED"
	if again := BaselineAllowedCapabilities(); again[0] != "AUDIT_WRITE" {
		t.Errorf("a changed result leaked into the next call: %q", again[0])
	}
}

func TestRestrictedAllowedCapabilities(t *testing.T) {
	got := RestrictedAllowedCapabilities()
	if len(got) != 1 || got[0] != "NET_BIND_SERVICE" {
		t.Fatalf("got %v, want [NET_BIND_SERVICE]", got)
	}
	got[0] = "CHANGED"
	if again := RestrictedAllowedCapabilities(); again[0] != "NET_BIND_SERVICE" {
		t.Errorf("a changed result leaked into the next call: %q", again[0])
	}
}

// ValidateContainerPSA has no pod: nothing is inherited and no pod-dependent
// exemption applies.
func TestValidateContainerPSA_Rows(t *testing.T) {
	unmasked := corev1.UnmaskedProcMount
	tests := []struct {
		name      string
		sc        *corev1.SecurityContext
		level     PSALevel
		wantField string
	}{
		{"hostProcess", &corev1.SecurityContext{WindowsOptions: &corev1.WindowsSecurityContextOptions{HostProcess: boolPtr(true)}}, PSABaseline, "securityContext.windowsOptions.hostProcess"},
		{"procMount", &corev1.SecurityContext{ProcMount: &unmasked}, PSABaseline, "securityContext.procMount"},
		{"seccomp Unconfined", &corev1.SecurityContext{SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeUnconfined}}, PSABaseline, "securityContext.seccompProfile.type"},
		{"appArmor Unconfined", &corev1.SecurityContext{AppArmorProfile: &corev1.AppArmorProfile{Type: corev1.AppArmorProfileTypeUnconfined}}, PSABaseline, "securityContext.appArmorProfile.type"},
		{"seLinux type", &corev1.SecurityContext{SELinuxOptions: &corev1.SELinuxOptions{Type: "spc_t"}}, PSABaseline, "securityContext.seLinuxOptions.type"},
		{"restricted capability add", func() *corev1.SecurityContext {
			sc := RestrictedSecurityContext()
			sc.Capabilities.Add = []corev1.Capability{"CHOWN"}
			return sc
		}(), PSARestricted, "securityContext.capabilities.add"},
		{"restricted runAsUser 0", func() *corev1.SecurityContext {
			sc := RestrictedSecurityContext()
			sc.RunAsUser = psaInt64(0)
			return sc
		}(), PSARestricted, "securityContext.runAsUser"},
		{"restricted without runAsNonRoot: no pod to inherit from", func() *corev1.SecurityContext {
			sc := RestrictedSecurityContext()
			sc.RunAsNonRoot = nil
			return sc
		}(), PSARestricted, "securityContext.runAsNonRoot"},
		{"restricted without seccomp: no pod to inherit from", func() *corev1.SecurityContext {
			sc := RestrictedSecurityContext()
			sc.SeccompProfile = nil
			return sc
		}(), PSARestricted, "securityContext.seccompProfile"},
		{"restricted privileged", func() *corev1.SecurityContext {
			sc := RestrictedSecurityContext()
			sc.Privileged = boolPtr(true)
			return sc
		}(), PSARestricted, "securityContext.privileged"},
		{"restricted compliant", RestrictedSecurityContext(), PSARestricted, ""},
		{"restricted with NET_BIND_SERVICE", func() *corev1.SecurityContext {
			sc := RestrictedSecurityContext()
			sc.Capabilities.Add = []corev1.Capability{"NET_BIND_SERVICE"}
			return sc
		}(), PSARestricted, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateContainerPSA(&corev1.Container{Name: "app", SecurityContext: tt.sc}, tt.level)
			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("expected compliant, got: %v", err)
				}
				return
			}
			var psaErr *kerrors.PSAViolationError
			if !errors.As(err, &psaErr) {
				t.Fatalf("expected *PSAViolationError with field %q, got %v", tt.wantField, err)
			}
			if psaErr.Field != tt.wantField {
				t.Errorf("Field = %q, want %q (%v)", psaErr.Field, tt.wantField, err)
			}
		})
	}
}
