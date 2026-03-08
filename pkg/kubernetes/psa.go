package kubernetes

import (
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
// compliant with the given PSA level. Returns nil when compliant, or an error
// describing the first violation found.
func ValidateContainerPSA(container *corev1.Container, level PSALevel) error {
	if container == nil {
		return errors.ErrNilContainer
	}

	switch level {
	case PSAPrivileged:
		return nil
	case PSABaseline:
		return validateContainerBaseline(container)
	case PSARestricted:
		return validateContainerRestricted(container)
	default:
		return errors.NewValidationError("level", string(level), "PSA", []string{
			string(PSARestricted), string(PSABaseline), string(PSAPrivileged),
		})
	}
}

// ValidatePodSpecPSA checks whether a PodSpec is compliant with the given PSA
// level. It validates both the pod-level security context and all containers
// (including init and ephemeral containers).
func ValidatePodSpecPSA(spec *corev1.PodSpec, level PSALevel) error {
	if spec == nil {
		return errors.ErrNilPodSpec
	}

	switch level {
	case PSAPrivileged:
		return nil
	case PSABaseline:
		if err := validatePodSpecBaseline(spec); err != nil {
			return err
		}
	case PSARestricted:
		if err := validatePodSpecRestricted(spec); err != nil {
			return err
		}
	default:
		return errors.NewValidationError("level", string(level), "PSA", []string{
			string(PSARestricted), string(PSABaseline), string(PSAPrivileged),
		})
	}

	return nil
}

func validateContainerBaseline(c *corev1.Container) error {
	sc := c.SecurityContext
	if sc == nil {
		return nil
	}

	if sc.Privileged != nil && *sc.Privileged {
		return errors.Errorf("container %q: privileged mode is not allowed at baseline level", c.Name)
	}

	if sc.Capabilities != nil {
		for _, cap := range sc.Capabilities.Add {
			if !isBaselineAllowedCapability(cap) {
				return errors.Errorf("container %q: capability %q is not allowed at baseline level", c.Name, cap)
			}
		}
	}

	return nil
}

func validateContainerRestricted(c *corev1.Container) error {
	if err := validateContainerBaseline(c); err != nil {
		return err
	}

	sc := c.SecurityContext
	if sc == nil {
		return errors.Errorf("container %q: security context must be set at restricted level", c.Name)
	}

	if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
		return errors.Errorf("container %q: allowPrivilegeEscalation must be false at restricted level", c.Name)
	}

	if sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot {
		return errors.Errorf("container %q: runAsNonRoot must be true at restricted level", c.Name)
	}

	if sc.Capabilities == nil || !hasDropAll(sc.Capabilities.Drop) {
		return errors.Errorf("container %q: must drop ALL capabilities at restricted level", c.Name)
	}

	if sc.SeccompProfile == nil ||
		(sc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault &&
			sc.SeccompProfile.Type != corev1.SeccompProfileTypeLocalhost) {
		return errors.Errorf("container %q: seccompProfile must be RuntimeDefault or Localhost at restricted level", c.Name)
	}

	return nil
}

func validatePodSpecBaseline(spec *corev1.PodSpec) error {
	if spec.HostNetwork {
		return errors.New("hostNetwork is not allowed at baseline level")
	}
	if spec.HostPID {
		return errors.New("hostPID is not allowed at baseline level")
	}
	if spec.HostIPC {
		return errors.New("hostIPC is not allowed at baseline level")
	}

	for i := range spec.Containers {
		if err := validateContainerBaseline(&spec.Containers[i]); err != nil {
			return err
		}
	}
	for i := range spec.InitContainers {
		if err := validateContainerBaseline(&spec.InitContainers[i]); err != nil {
			return err
		}
	}
	for i := range spec.EphemeralContainers {
		c := containerFromEphemeral(&spec.EphemeralContainers[i])
		if err := validateContainerBaseline(&c); err != nil {
			return err
		}
	}

	return nil
}

func validatePodSpecRestricted(spec *corev1.PodSpec) error {
	if err := validatePodSpecBaseline(spec); err != nil {
		return err
	}

	sc := spec.SecurityContext
	if sc == nil || sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot {
		return errors.New("pod runAsNonRoot must be true at restricted level")
	}

	if sc.SeccompProfile == nil ||
		(sc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault &&
			sc.SeccompProfile.Type != corev1.SeccompProfileTypeLocalhost) {
		return errors.New("pod seccompProfile must be RuntimeDefault or Localhost at restricted level")
	}

	for i := range spec.Containers {
		if err := validateContainerRestricted(&spec.Containers[i]); err != nil {
			return err
		}
	}
	for i := range spec.InitContainers {
		if err := validateContainerRestricted(&spec.InitContainers[i]); err != nil {
			return err
		}
	}
	for i := range spec.EphemeralContainers {
		c := containerFromEphemeral(&spec.EphemeralContainers[i])
		if err := validateContainerRestricted(&c); err != nil {
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
