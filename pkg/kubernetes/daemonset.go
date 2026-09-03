package kubernetes

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/go-kure/kure/pkg/errors"
)

// SetDaemonSetPodSpec assigns a PodSpec to the DaemonSet template.
func SetDaemonSetPodSpec(ds *appsv1.DaemonSet, spec *corev1.PodSpec) error {
	if ds == nil {
		return errors.ErrNilDaemonSet
	}
	if spec == nil {
		return errors.ErrNilSpec
	}
	ds.Spec.Template.Spec = *spec
	return nil
}

// AddDaemonSetContainer appends a container to the DaemonSet pod template.
func AddDaemonSetContainer(ds *appsv1.DaemonSet, c *corev1.Container) error {
	if ds == nil {
		return errors.ErrNilDaemonSet
	}
	return AddPodSpecContainer(&ds.Spec.Template.Spec, c)
}

// AddDaemonSetInitContainer appends an init container to the pod template.
func AddDaemonSetInitContainer(ds *appsv1.DaemonSet, c *corev1.Container) error {
	if ds == nil {
		return errors.ErrNilDaemonSet
	}
	return AddPodSpecInitContainer(&ds.Spec.Template.Spec, c)
}

// AddDaemonSetVolume appends a volume to the pod template.
func AddDaemonSetVolume(ds *appsv1.DaemonSet, v *corev1.Volume) error {
	if ds == nil {
		return errors.ErrNilDaemonSet
	}
	return AddPodSpecVolume(&ds.Spec.Template.Spec, v)
}

// AddDaemonSetImagePullSecret appends an image pull secret to the pod template.
func AddDaemonSetImagePullSecret(ds *appsv1.DaemonSet, s *corev1.LocalObjectReference) error {
	if ds == nil {
		return errors.ErrNilDaemonSet
	}
	return AddPodSpecImagePullSecret(&ds.Spec.Template.Spec, s)
}

// AddDaemonSetToleration appends a toleration to the pod template.
func AddDaemonSetToleration(ds *appsv1.DaemonSet, t *corev1.Toleration) error {
	if ds == nil {
		return errors.ErrNilDaemonSet
	}
	return AddPodSpecToleration(&ds.Spec.Template.Spec, t)
}

// AddDaemonSetTopologySpreadConstraints appends a topology spread constraint if not nil.
func AddDaemonSetTopologySpreadConstraints(ds *appsv1.DaemonSet, c *corev1.TopologySpreadConstraint) error {
	if ds == nil {
		return errors.ErrNilDaemonSet
	}
	return AddPodSpecTopologySpreadConstraints(&ds.Spec.Template.Spec, c)
}

// SetDaemonSetServiceAccountName sets the service account name.
func SetDaemonSetServiceAccountName(ds *appsv1.DaemonSet, name string) {
	if ds == nil {
		panic("SetDaemonSetServiceAccountName: ds must not be nil")
	}
	ds.Spec.Template.Spec.ServiceAccountName = name
}

// SetDaemonSetSecurityContext sets the pod security context.
func SetDaemonSetSecurityContext(ds *appsv1.DaemonSet, sc *corev1.PodSecurityContext) {
	if ds == nil {
		panic("SetDaemonSetSecurityContext: ds must not be nil")
	}
	SetPodSpecSecurityContext(&ds.Spec.Template.Spec, sc)
}

// SetDaemonSetAffinity sets the pod affinity rules.
func SetDaemonSetAffinity(ds *appsv1.DaemonSet, aff *corev1.Affinity) {
	if ds == nil {
		panic("SetDaemonSetAffinity: ds must not be nil")
	}
	SetPodSpecAffinity(&ds.Spec.Template.Spec, aff)
}

// SetDaemonSetNodeSelector sets the node selector.
func SetDaemonSetNodeSelector(ds *appsv1.DaemonSet, ns map[string]string) {
	if ds == nil {
		panic("SetDaemonSetNodeSelector: ds must not be nil")
	}
	ds.Spec.Template.Spec.NodeSelector = ns
}

// SetDaemonSetRevisionHistoryLimit sets the revision history limit.
func SetDaemonSetRevisionHistoryLimit(ds *appsv1.DaemonSet, limit *int32) {
	if ds == nil {
		panic("SetDaemonSetRevisionHistoryLimit: ds must not be nil")
	}
	ds.Spec.RevisionHistoryLimit = limit
}
