package kubernetes

import (
	"fmt"
	"maps"
	"slices"

	corev1 "k8s.io/api/core/v1"

	"github.com/go-kure/kure/pkg/errors"
)

// PSALevel represents the Pod Security Standards level.
type PSALevel string

const (
	// PSARestricted is the most restrictive Pod Security Standards level.
	// It follows the current best practices for hardening pods.
	PSARestricted PSALevel = "restricted"

	// PSABaseline provides a minimally restrictive policy that prevents
	// known privilege escalations while allowing most workloads.
	PSABaseline PSALevel = "baseline"

	// PSAPrivileged is an unrestricted policy providing the widest possible
	// level of permissions.
	PSAPrivileged PSALevel = "privileged"
)

// PSALabels returns the Pod Security Admission labels for the given modes. It
// writes nothing; apply the result with AddLabel:
//
//	for k, v := range kubernetes.PSALabels(kubernetes.PSARestricted, "", "", "v1.28") {
//		kubernetes.AddLabel(ns, k, v)
//	}
//
// An empty level omits that mode's labels; an empty version omits the
// *-version labels. This is a value helper, not builder sugar — one argument
// expands into up to six labels, which no Set<Field> helper is allowed to do
// (see the builder contract in this package's README).
func PSALabels(enforce, warn, audit PSALevel, version string) map[string]string {
	labels := map[string]string{}
	for _, mode := range []struct {
		name  string
		level PSALevel
	}{
		{"enforce", enforce},
		{"warn", warn},
		{"audit", audit},
	} {
		if mode.level == "" {
			continue
		}
		labels["pod-security.kubernetes.io/"+mode.name] = string(mode.level)
		if version != "" {
			labels["pod-security.kubernetes.io/"+mode.name+"-version"] = version
		}
	}
	return labels
}

// RestrictedPodSecurityContext returns a PodSecurityContext compliant with the
// restricted Pod Security Standards level. It sets RunAsNonRoot and configures
// SeccompProfile to RuntimeDefault.
func RestrictedPodSecurityContext() *corev1.PodSecurityContext {
	return &corev1.PodSecurityContext{
		RunAsNonRoot: boolPtr(true),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
}

// RestrictedSecurityContext returns a container SecurityContext compliant with
// the restricted Pod Security Standards level. It drops all capabilities,
// disallows privilege escalation, sets a read-only root filesystem, and
// configures SeccompProfile to RuntimeDefault.
func RestrictedSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: boolPtr(false),
		RunAsNonRoot:             boolPtr(true),
		ReadOnlyRootFilesystem:   boolPtr(true),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
}

// BaselinePodSecurityContext returns a PodSecurityContext compliant with the
// baseline Pod Security Standards level.
func BaselinePodSecurityContext() *corev1.PodSecurityContext {
	return &corev1.PodSecurityContext{
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
}

// BaselineSecurityContext returns a container SecurityContext compliant with
// the baseline Pod Security Standards level. It disallows privilege escalation
// and sets SeccompProfile to RuntimeDefault.
func BaselineSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: boolPtr(false),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
	}
}

// PrivilegedPodSecurityContext returns a PodSecurityContext with no
// restrictions, matching the privileged Pod Security Standards level.
func PrivilegedPodSecurityContext() *corev1.PodSecurityContext {
	return &corev1.PodSecurityContext{}
}

// PrivilegedSecurityContext returns a container SecurityContext with no
// restrictions, matching the privileged Pod Security Standards level.
func PrivilegedSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{}
}

// PodSecurityContextForLevel returns a PodSecurityContext for the given PSA level.
func PodSecurityContextForLevel(level PSALevel) (*corev1.PodSecurityContext, error) {
	switch level {
	case PSARestricted:
		return RestrictedPodSecurityContext(), nil
	case PSABaseline:
		return BaselinePodSecurityContext(), nil
	case PSAPrivileged:
		return PrivilegedPodSecurityContext(), nil
	default:
		return nil, errors.NewValidationError("level", string(level), "PSA", []string{
			string(PSARestricted), string(PSABaseline), string(PSAPrivileged),
		})
	}
}

// SecurityContextForLevel returns a container SecurityContext for the given PSA level.
func SecurityContextForLevel(level PSALevel) (*corev1.SecurityContext, error) {
	switch level {
	case PSARestricted:
		return RestrictedSecurityContext(), nil
	case PSABaseline:
		return BaselineSecurityContext(), nil
	case PSAPrivileged:
		return PrivilegedSecurityContext(), nil
	default:
		return nil, errors.NewValidationError("level", string(level), "PSA", []string{
			string(PSARestricted), string(PSABaseline), string(PSAPrivileged),
		})
	}
}

// ValidateContainerPSA checks whether a container's SecurityContext is
// compliant with the given PSA level. Returns nil when compliant, or a
// *errors.PSAViolationError describing the first violation found.
// Field paths are relative to the container (no container prefix).
//
// It checks the securityContext rows of the Pod Security Standards (see
// [ValidatePodSpecPSA] for the list). A container has no pod, so nothing is
// inherited from a pod-level securityContext and no pod-dependent exemption
// applies: at the restricted level the container must set runAsNonRoot and a
// seccomp profile itself, and the user-namespace and Windows exemptions do not
// apply. Use [ValidatePodSpecPSA] for those.
func ValidateContainerPSA(container *corev1.Container, level PSALevel) error {
	if container == nil {
		return errors.ErrNilContainer
	}

	switch level {
	case PSAPrivileged:
		return nil
	case PSABaseline:
		return validateContainerBaseline(container, "", string(level), psaPod{}, false)
	case PSARestricted:
		if err := validateContainerBaseline(container, "", string(level), psaPod{}, true); err != nil {
			return err
		}
		return validateContainerRestricted(container, "", string(level), psaPod{})
	default:
		return errors.NewValidationError("level", string(level), "PSA", []string{
			string(PSARestricted), string(PSABaseline), string(PSAPrivileged),
		})
	}
}

// ValidatePodSpecPSA checks whether a PodSpec is compliant with the given PSA
// level. It validates both the pod-level security context and all containers
// (including init and ephemeral containers). Returns nil when compliant, or a
// *errors.PSAViolationError describing the first violation found.
// Field paths are relative to the PodSpec.
//
// A nil error means the host namespaces and the securityContext fields meet
// the Pod Security Standards rows below, as of Kubernetes 1.37. It does not
// mean admission accepts the pod: host ports, volume types, host probes and
// lifecycle hooks, and the legacy seccomp and AppArmor annotations are not
// checked.
//
// Baseline: hostNetwork, hostPID, hostIPC; privileged; capabilities.add
// against [BaselineAllowedCapabilities]; procMount; windowsOptions.hostProcess;
// seccompProfile and appArmorProfile types other than RuntimeDefault and
// Localhost; seLinuxOptions; pod sysctls outside the safe list.
//
// Restricted adds: allowPrivilegeEscalation=false; runAsNonRoot=true and
// runAsUser other than 0; capabilities.drop ALL with at most the
// [RestrictedAllowedCapabilities] added; a seccompProfile of RuntimeDefault or
// Localhost. runAsNonRoot and seccompProfile are accepted when the pod sets
// them, or when every container does; a container value overrides the pod
// value, but a pod value the standard forbids (runAsNonRoot false, an
// Unconfined seccompProfile) stays a violation whatever the containers set.
//
// Two pod-level exemptions follow the Pod Security Standards. With
// spec.hostUsers set to false, runAsNonRoot and runAsUser are not checked, and
// procMount is not checked at the baseline level only: the restricted level
// still refuses a procMount other than Default. With spec.os.name set to
// windows, allowPrivilegeEscalation, capabilities and seccompProfile are not
// checked at the restricted level.
func ValidatePodSpecPSA(spec *corev1.PodSpec, level PSALevel) error {
	if spec == nil {
		return errors.ErrNilPodSpec
	}

	switch level {
	case PSAPrivileged:
		return nil
	case PSABaseline:
		return validatePodSpec(spec, string(level), false)
	case PSARestricted:
		return validatePodSpec(spec, string(level), true)
	default:
		return errors.NewValidationError("level", string(level), "PSA", []string{
			string(PSARestricted), string(PSABaseline), string(PSAPrivileged),
		})
	}
}

// psaPod carries the pod-level facts a container check depends on. The zero
// value means there is no pod: ValidateContainerPSA passes it, so nothing is
// inherited and no exemption applies.
type psaPod struct {
	sc *corev1.PodSecurityContext
	// userNamespace is true when spec.hostUsers is false.
	userNamespace bool
	// windows is true when spec.os.name is windows.
	windows bool
}

func newPSAPod(spec *corev1.PodSpec) psaPod {
	return psaPod{
		sc:            spec.SecurityContext,
		userNamespace: spec.HostUsers != nil && !*spec.HostUsers,
		windows:       spec.OS != nil && spec.OS.Name == corev1.Windows,
	}
}

func psaField(prefix, suffix string) string {
	if prefix == "" {
		return suffix
	}
	return prefix + "." + suffix
}

func psaViolation(prefix, suffix, level, format string, args ...any) error {
	return errors.NewPSAViolationError(psaField(prefix, suffix), level, fmt.Sprintf(format, args...))
}

// validateContainerBaseline checks the baseline securityContext rows of one
// container. restricted is true when the restricted level is being checked:
// the restricted capabilities and seccomp rows replace their baseline rows, as
// in the Pod Security Standards, so those two baseline rows are skipped.
func validateContainerBaseline(c *corev1.Container, fieldPrefix, level string, pod psaPod, restricted bool) error {
	sc := c.SecurityContext
	if sc == nil {
		return nil
	}

	if sc.Privileged != nil && *sc.Privileged {
		return psaViolation(fieldPrefix, "securityContext.privileged", level,
			"container %q: privileged mode is not allowed at %s level", c.Name, level)
	}

	if !restricted && sc.Capabilities != nil {
		for _, cap := range sc.Capabilities.Add {
			if !isBaselineAllowedCapability(cap) {
				return psaViolation(fieldPrefix, "securityContext.capabilities.add", level,
					"container %q: capability %q is not allowed at %s level", c.Name, cap, level)
			}
		}
	}

	// A user-namespace pod may use another procMount at baseline; restricted
	// requires the default whatever the namespace.
	if (restricted || !pod.userNamespace) && sc.ProcMount != nil && *sc.ProcMount != corev1.DefaultProcMount {
		return psaViolation(fieldPrefix, "securityContext.procMount", level,
			"container %q: procMount %q is not allowed at %s level", c.Name, *sc.ProcMount, level)
	}

	if sc.WindowsOptions != nil && sc.WindowsOptions.HostProcess != nil && *sc.WindowsOptions.HostProcess {
		return psaViolation(fieldPrefix, "securityContext.windowsOptions.hostProcess", level,
			"container %q: windowsOptions.hostProcess is not allowed at %s level", c.Name, level)
	}

	if !restricted && sc.SeccompProfile != nil && !seccompTypeAllowed(sc.SeccompProfile.Type) {
		return psaViolation(fieldPrefix, "securityContext.seccompProfile.type", level,
			"container %q: seccompProfile type %q is not allowed at %s level", c.Name, sc.SeccompProfile.Type, level)
	}

	if sc.AppArmorProfile != nil && !appArmorTypeAllowed(sc.AppArmorProfile.Type) {
		return psaViolation(fieldPrefix, "securityContext.appArmorProfile.type", level,
			"container %q: appArmorProfile type %q is not allowed at %s level", c.Name, sc.AppArmorProfile.Type, level)
	}

	if err := validateSELinuxOptions(sc.SELinuxOptions, fieldPrefix, "securityContext.seLinuxOptions", level,
		fmt.Sprintf("container %q", c.Name)); err != nil {
		return err
	}

	return nil
}

// validateContainerRestricted checks the restricted securityContext rows of one
// container, on top of the baseline rows. pod supplies the pod-level values a
// container may inherit and the pod-level exemptions.
func validateContainerRestricted(c *corev1.Container, fieldPrefix, level string, pod psaPod) error {
	sc := c.SecurityContext
	if sc == nil {
		// allowPrivilegeEscalation and capabilities are container-only fields,
		// so a container without a securityContext cannot comply, except on a
		// Windows pod where both rows are exempt.
		if !pod.windows {
			return psaViolation(fieldPrefix, "securityContext", level,
				"container %q: security context must be set at %s level", c.Name, level)
		}
		sc = &corev1.SecurityContext{}
	}

	if !pod.windows && (sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation) {
		return psaViolation(fieldPrefix, "securityContext.allowPrivilegeEscalation", level,
			"container %q: allowPrivilegeEscalation must be false at %s level", c.Name, level)
	}

	if !pod.userNamespace {
		podRunAsNonRoot := pod.sc != nil && pod.sc.RunAsNonRoot != nil && *pod.sc.RunAsNonRoot
		if sc.RunAsNonRoot == nil && !podRunAsNonRoot || sc.RunAsNonRoot != nil && !*sc.RunAsNonRoot {
			return psaViolation(fieldPrefix, "securityContext.runAsNonRoot", level,
				"container %q: runAsNonRoot must be true, on the container or the pod, at %s level", c.Name, level)
		}
		if sc.RunAsUser != nil && *sc.RunAsUser == 0 {
			return psaViolation(fieldPrefix, "securityContext.runAsUser", level,
				"container %q: runAsUser must not be 0 at %s level", c.Name, level)
		}
	}

	if !pod.windows {
		if sc.Capabilities != nil {
			for _, cap := range sc.Capabilities.Add {
				if cap != capabilityNetBindService {
					return psaViolation(fieldPrefix, "securityContext.capabilities.add", level,
						"container %q: capability %q is not allowed at %s level; only %s may be added",
						c.Name, cap, level, capabilityNetBindService)
				}
			}
		}
		if sc.Capabilities == nil || !hasDropAll(sc.Capabilities.Drop) {
			return psaViolation(fieldPrefix, "securityContext.capabilities", level,
				"container %q: must drop ALL capabilities at %s level", c.Name, level)
		}

		podSeccomp := pod.sc != nil && pod.sc.SeccompProfile != nil && seccompTypeAllowed(pod.sc.SeccompProfile.Type)
		if sc.SeccompProfile == nil && !podSeccomp ||
			sc.SeccompProfile != nil && !seccompTypeAllowed(sc.SeccompProfile.Type) {
			return psaViolation(fieldPrefix, "securityContext.seccompProfile", level,
				"container %q: seccompProfile must be RuntimeDefault or Localhost, on the container or the pod, at %s level",
				c.Name, level)
		}
	}

	return nil
}

// validatePodContextBaseline checks the baseline rows that live on the pod
// securityContext. restricted skips the seccomp row, which the restricted
// check replaces.
func validatePodContextBaseline(spec *corev1.PodSpec, level string, restricted bool) error {
	sc := spec.SecurityContext
	if sc == nil {
		return nil
	}

	if sc.WindowsOptions != nil && sc.WindowsOptions.HostProcess != nil && *sc.WindowsOptions.HostProcess {
		return psaViolation("", "securityContext.windowsOptions.hostProcess", level,
			"pod windowsOptions.hostProcess is not allowed at %s level", level)
	}

	if !restricted && sc.SeccompProfile != nil && !seccompTypeAllowed(sc.SeccompProfile.Type) {
		return psaViolation("", "securityContext.seccompProfile.type", level,
			"pod seccompProfile type %q is not allowed at %s level", sc.SeccompProfile.Type, level)
	}

	if sc.AppArmorProfile != nil && !appArmorTypeAllowed(sc.AppArmorProfile.Type) {
		return psaViolation("", "securityContext.appArmorProfile.type", level,
			"pod appArmorProfile type %q is not allowed at %s level", sc.AppArmorProfile.Type, level)
	}

	if err := validateSELinuxOptions(sc.SELinuxOptions, "", "securityContext.seLinuxOptions", level, "pod"); err != nil {
		return err
	}

	for _, sysctl := range sc.Sysctls {
		if !baselineSafeSysctls[sysctl.Name] {
			return psaViolation("", "securityContext.sysctls", level,
				"pod sysctl %q is not allowed at %s level", sysctl.Name, level)
		}
	}

	return nil
}

// validatePodContextRestricted checks the restricted rows that the pod
// securityContext sets explicitly. A pod value that is unset is not a
// violation here: the container checks require the setting on the container
// when the pod does not provide it.
func validatePodContextRestricted(spec *corev1.PodSpec, level string, pod psaPod) error {
	sc := spec.SecurityContext
	if sc == nil {
		return nil
	}

	if !pod.userNamespace {
		if sc.RunAsNonRoot != nil && !*sc.RunAsNonRoot {
			return psaViolation("", "securityContext.runAsNonRoot", level,
				"pod runAsNonRoot must not be false at %s level", level)
		}
		if sc.RunAsUser != nil && *sc.RunAsUser == 0 {
			return psaViolation("", "securityContext.runAsUser", level,
				"pod runAsUser must not be 0 at %s level", level)
		}
	}

	if !pod.windows && sc.SeccompProfile != nil && !seccompTypeAllowed(sc.SeccompProfile.Type) {
		return psaViolation("", "securityContext.seccompProfile", level,
			"pod seccompProfile must be RuntimeDefault or Localhost at %s level", level)
	}

	return nil
}

func validatePodSpec(spec *corev1.PodSpec, level string, restricted bool) error {
	if spec.HostNetwork {
		return errors.NewPSAViolationError("hostNetwork", level, fmt.Sprintf("hostNetwork is not allowed at %s level", level))
	}
	if spec.HostPID {
		return errors.NewPSAViolationError("hostPID", level, fmt.Sprintf("hostPID is not allowed at %s level", level))
	}
	if spec.HostIPC {
		return errors.NewPSAViolationError("hostIPC", level, fmt.Sprintf("hostIPC is not allowed at %s level", level))
	}

	pod := newPSAPod(spec)

	if err := validatePodContextBaseline(spec, level, restricted); err != nil {
		return err
	}
	if err := eachContainer(spec, func(c *corev1.Container, prefix string) error {
		return validateContainerBaseline(c, prefix, level, pod, restricted)
	}); err != nil {
		return err
	}

	if !restricted {
		return nil
	}

	if err := validatePodContextRestricted(spec, level, pod); err != nil {
		return err
	}
	return eachContainer(spec, func(c *corev1.Container, prefix string) error {
		return validateContainerRestricted(c, prefix, level, pod)
	})
}

// eachContainer calls fn for every container, init container and ephemeral
// container of spec with its field path, and returns the first error.
func eachContainer(spec *corev1.PodSpec, fn func(c *corev1.Container, prefix string) error) error {
	for i := range spec.Containers {
		if err := fn(&spec.Containers[i], fmt.Sprintf("containers[%d]", i)); err != nil {
			return err
		}
	}
	for i := range spec.InitContainers {
		if err := fn(&spec.InitContainers[i], fmt.Sprintf("initContainers[%d]", i)); err != nil {
			return err
		}
	}
	for i := range spec.EphemeralContainers {
		c := containerFromEphemeral(&spec.EphemeralContainers[i])
		if err := fn(&c, fmt.Sprintf("ephemeralContainers[%d]", i)); err != nil {
			return err
		}
	}
	return nil
}

// validateSELinuxOptions applies the baseline seLinuxOptions row: the type must
// be one of the allowed types and the user and role must be empty. who names
// the owner of the options in the message.
func validateSELinuxOptions(opts *corev1.SELinuxOptions, fieldPrefix, field, level, who string) error {
	if opts == nil {
		return nil
	}
	if !baselineAllowedSELinuxTypes[opts.Type] {
		return psaViolation(fieldPrefix, field+".type", level,
			"%s: seLinuxOptions type %q is not allowed at %s level", who, opts.Type, level)
	}
	if opts.User != "" {
		return psaViolation(fieldPrefix, field+".user", level,
			"%s: seLinuxOptions user must not be set at %s level", who, level)
	}
	if opts.Role != "" {
		return psaViolation(fieldPrefix, field+".role", level,
			"%s: seLinuxOptions role must not be set at %s level", who, level)
	}
	return nil
}

func seccompTypeAllowed(t corev1.SeccompProfileType) bool {
	return t == corev1.SeccompProfileTypeRuntimeDefault || t == corev1.SeccompProfileTypeLocalhost
}

func appArmorTypeAllowed(t corev1.AppArmorProfileType) bool {
	return t == corev1.AppArmorProfileTypeRuntimeDefault || t == corev1.AppArmorProfileTypeLocalhost
}

const capabilityNetBindService corev1.Capability = "NET_BIND_SERVICE"

// baselineAllowedCapabilities lists capabilities permitted at the baseline
// Pod Security Standards level.
var baselineAllowedCapabilities = map[corev1.Capability]bool{
	"AUDIT_WRITE":      true,
	"CHOWN":            true,
	"DAC_OVERRIDE":     true,
	"FOWNER":           true,
	"FSETID":           true,
	"KILL":             true,
	"MKNOD":            true,
	"NET_BIND_SERVICE": true,
	"SETFCAP":          true,
	"SETGID":           true,
	"SETPCAP":          true,
	"SETUID":           true,
	"SYS_CHROOT":       true,
}

func isBaselineAllowedCapability(cap corev1.Capability) bool {
	return baselineAllowedCapabilities[cap]
}

// BaselineAllowedCapabilities returns the capabilities that the baseline Pod
// Security Standards level allows in capabilities.add, sorted by name.
// [ValidateContainerPSA] and [ValidatePodSpecPSA] check against this list. The
// caller owns the returned slice and may change it.
func BaselineAllowedCapabilities() []corev1.Capability {
	return slices.Sorted(maps.Keys(baselineAllowedCapabilities))
}

// RestrictedAllowedCapabilities returns the capabilities that the restricted
// Pod Security Standards level allows in capabilities.add, which is only
// NET_BIND_SERVICE. The restricted level also requires capabilities.drop to
// contain ALL. The caller owns the returned slice and may change it.
func RestrictedAllowedCapabilities() []corev1.Capability {
	return []corev1.Capability{capabilityNetBindService}
}

// baselineAllowedSELinuxTypes lists the seLinuxOptions types the baseline
// level allows (Pod Security Standards 1.31 and later); the empty type means
// unset.
var baselineAllowedSELinuxTypes = map[string]bool{
	"":                   true,
	"container_t":        true,
	"container_init_t":   true,
	"container_kvm_t":    true,
	"container_engine_t": true,
}

// baselineSafeSysctls lists the sysctls the baseline level allows on a pod
// (Pod Security Standards 1.37).
var baselineSafeSysctls = map[string]bool{
	"kernel.shm_rmid_forced":              true,
	"net.ipv4.ip_local_port_range":        true,
	"net.ipv4.tcp_syncookies":             true,
	"net.ipv4.ping_group_range":           true,
	"net.ipv4.ip_unprivileged_port_start": true,
	"net.ipv4.ip_local_reserved_ports":    true,
	"net.ipv4.tcp_keepalive_time":         true,
	"net.ipv4.tcp_fin_timeout":            true,
	"net.ipv4.tcp_keepalive_intvl":        true,
	"net.ipv4.tcp_keepalive_probes":       true,
	"net.ipv4.tcp_rmem":                   true,
	"net.ipv4.tcp_wmem":                   true,
	"net.ipv4.tcp_slow_start_after_idle":  true,
	"net.ipv4.tcp_notsent_lowat":          true,
}

// containerFromEphemeral converts an EphemeralContainer to a Container for
// validation purposes. Only the name and the securityContext are copied; they
// are all the securityContext rules read.
func containerFromEphemeral(ec *corev1.EphemeralContainer) corev1.Container {
	return corev1.Container{
		Name:            ec.Name,
		SecurityContext: ec.SecurityContext,
	}
}

func hasDropAll(caps []corev1.Capability) bool {
	for _, cap := range caps {
		if cap == "ALL" {
			return true
		}
	}
	return false
}

func boolPtr(b bool) *bool {
	return &b
}
