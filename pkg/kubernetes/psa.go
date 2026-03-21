package kubernetes

import (
	"fmt"

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
func ValidateContainerPSA(container *corev1.Container, level PSALevel) error {
	if container == nil {
		return errors.ErrNilContainer
	}

	switch level {
	case PSAPrivileged:
		return nil
	case PSABaseline:
		return validateContainerBaseline(container, "", string(level))
	case PSARestricted:
		return validateContainerRestricted(container, "", string(level))
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
func ValidatePodSpecPSA(spec *corev1.PodSpec, level PSALevel) error {
	if spec == nil {
		return errors.ErrNilPodSpec
	}

	switch level {
	case PSAPrivileged:
		return nil
	case PSABaseline:
		return validatePodSpecBaseline(spec, string(level))
	case PSARestricted:
		return validatePodSpecRestricted(spec, string(level))
	default:
		return errors.NewValidationError("level", string(level), "PSA", []string{
			string(PSARestricted), string(PSABaseline), string(PSAPrivileged),
		})
	}
}

func psaField(prefix, suffix string) string {
	if prefix == "" {
		return suffix
	}
	return prefix + "." + suffix
}

func validateContainerBaseline(c *corev1.Container, fieldPrefix, level string) error {
	sc := c.SecurityContext
	if sc == nil {
		return nil
	}

	if sc.Privileged != nil && *sc.Privileged {
		return errors.NewPSAViolationError(
			psaField(fieldPrefix, "securityContext.privileged"),
			level,
			fmt.Sprintf("container %q: privileged mode is not allowed at %s level", c.Name, level),
		)
	}

	if sc.Capabilities != nil {
		for _, cap := range sc.Capabilities.Add {
			if !isBaselineAllowedCapability(cap) {
				return errors.NewPSAViolationError(
					psaField(fieldPrefix, "securityContext.capabilities.add"),
					level,
					fmt.Sprintf("container %q: capability %q is not allowed at %s level", c.Name, cap, level),
				)
			}
		}
	}

	return nil
}

func validateContainerRestricted(c *corev1.Container, fieldPrefix, level string) error {
	if err := validateContainerBaseline(c, fieldPrefix, level); err != nil {
		return err
	}

	sc := c.SecurityContext
	if sc == nil {
		return errors.NewPSAViolationError(
			psaField(fieldPrefix, "securityContext"),
			level,
			fmt.Sprintf("container %q: security context must be set at %s level", c.Name, level),
		)
	}

	if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
		return errors.NewPSAViolationError(
			psaField(fieldPrefix, "securityContext.allowPrivilegeEscalation"),
			level,
			fmt.Sprintf("container %q: allowPrivilegeEscalation must be false at %s level", c.Name, level),
		)
	}

	if sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot {
		return errors.NewPSAViolationError(
			psaField(fieldPrefix, "securityContext.runAsNonRoot"),
			level,
			fmt.Sprintf("container %q: runAsNonRoot must be true at %s level", c.Name, level),
		)
	}

	if sc.Capabilities == nil || !hasDropAll(sc.Capabilities.Drop) {
		return errors.NewPSAViolationError(
			psaField(fieldPrefix, "securityContext.capabilities"),
			level,
			fmt.Sprintf("container %q: must drop ALL capabilities at %s level", c.Name, level),
		)
	}

	if sc.SeccompProfile == nil ||
		(sc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault &&
			sc.SeccompProfile.Type != corev1.SeccompProfileTypeLocalhost) {
		return errors.NewPSAViolationError(
			psaField(fieldPrefix, "securityContext.seccompProfile"),
			level,
			fmt.Sprintf("container %q: seccompProfile must be RuntimeDefault or Localhost at %s level", c.Name, level),
		)
	}

	return nil
}

func validatePodSpecBaseline(spec *corev1.PodSpec, level string) error {
	if spec.HostNetwork {
		return errors.NewPSAViolationError("hostNetwork", level, fmt.Sprintf("hostNetwork is not allowed at %s level", level))
	}
	if spec.HostPID {
		return errors.NewPSAViolationError("hostPID", level, fmt.Sprintf("hostPID is not allowed at %s level", level))
	}
	if spec.HostIPC {
		return errors.NewPSAViolationError("hostIPC", level, fmt.Sprintf("hostIPC is not allowed at %s level", level))
	}

	for i := range spec.Containers {
		if err := validateContainerBaseline(&spec.Containers[i], fmt.Sprintf("containers[%d]", i), level); err != nil {
			return err
		}
	}
	for i := range spec.InitContainers {
		if err := validateContainerBaseline(&spec.InitContainers[i], fmt.Sprintf("initContainers[%d]", i), level); err != nil {
			return err
		}
	}
	for i := range spec.EphemeralContainers {
		c := containerFromEphemeral(&spec.EphemeralContainers[i])
		if err := validateContainerBaseline(&c, fmt.Sprintf("ephemeralContainers[%d]", i), level); err != nil {
			return err
		}
	}

	return nil
}

func validatePodSpecRestricted(spec *corev1.PodSpec, level string) error {
	if err := validatePodSpecBaseline(spec, level); err != nil {
		return err
	}

	sc := spec.SecurityContext
	if sc == nil || sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot {
		return errors.NewPSAViolationError(
			"securityContext.runAsNonRoot",
			level,
			fmt.Sprintf("pod runAsNonRoot must be true at %s level", level),
		)
	}

	if sc.SeccompProfile == nil ||
		(sc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault &&
			sc.SeccompProfile.Type != corev1.SeccompProfileTypeLocalhost) {
		return errors.NewPSAViolationError(
			"securityContext.seccompProfile",
			level,
			fmt.Sprintf("pod seccompProfile must be RuntimeDefault or Localhost at %s level", level),
		)
	}

	for i := range spec.Containers {
		if err := validateContainerRestricted(&spec.Containers[i], fmt.Sprintf("containers[%d]", i), level); err != nil {
			return err
		}
	}
	for i := range spec.InitContainers {
		if err := validateContainerRestricted(&spec.InitContainers[i], fmt.Sprintf("initContainers[%d]", i), level); err != nil {
			return err
		}
	}
	for i := range spec.EphemeralContainers {
		c := containerFromEphemeral(&spec.EphemeralContainers[i])
		if err := validateContainerRestricted(&c, fmt.Sprintf("ephemeralContainers[%d]", i), level); err != nil {
			return err
		}
	}

	return nil
}

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

// containerFromEphemeral converts an EphemeralContainer to a Container for
// validation purposes.
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
