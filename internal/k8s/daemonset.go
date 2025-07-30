package k8s

import (
	"errors"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateDaemonSet returns a DaemonSet with sane defaults.
func CreateDaemonSet(name, namespace string) *appsv1.DaemonSet {
	obj := &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DaemonSet",
			APIVersion: appsv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": name,
			},
			Annotations: map[string]string{
				"app": name,
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": name}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": name}},
				Spec:       corev1.PodSpec{},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{},
		},
	}
	return obj
}

// SetDaemonSetPodSpec assigns a PodSpec to the DaemonSet template.
func SetDaemonSetPodSpec(ds *appsv1.DaemonSet, spec *corev1.PodSpec) error {
	if ds == nil || spec == nil {
		return errors.New("nil daemonset or spec")
	}
	ds.Spec.Template.Spec = *spec
	return nil
}

// AddDaemonSetContainer appends a container to the DaemonSet pod template.
func AddDaemonSetContainer(ds *appsv1.DaemonSet, c *corev1.Container) error {
	if ds == nil {
		return errors.New("nil daemonset")
	}
	return AddPodSpecContainer(&ds.Spec.Template.Spec, c)
}

// AddDaemonSetInitContainer appends an init container to the pod template.
func AddDaemonSetInitContainer(ds *appsv1.DaemonSet, c *corev1.Container) error {
	if ds == nil {
		return errors.New("nil daemonset")
	}
	return AddPodSpecInitContainer(&ds.Spec.Template.Spec, c)
}

// AddDaemonSetVolume appends a volume to the pod template.
func AddDaemonSetVolume(ds *appsv1.DaemonSet, v *corev1.Volume) error {
	if ds == nil {
		return errors.New("nil daemonset")
	}
	return AddPodSpecVolume(&ds.Spec.Template.Spec, v)
}

// AddDaemonSetImagePullSecret appends an image pull secret to the pod template.
func AddDaemonSetImagePullSecret(ds *appsv1.DaemonSet, s *corev1.LocalObjectReference) error {
	if ds == nil {
		return errors.New("nil daemonset")
	}
	return AddPodSpecImagePullSecret(&ds.Spec.Template.Spec, s)
}

// AddDaemonSetToleration appends a toleration to the pod template.
func AddDaemonSetToleration(ds *appsv1.DaemonSet, t *corev1.Toleration) error {
	if ds == nil {
		return errors.New("nil daemonset")
	}
	return AddPodSpecToleration(&ds.Spec.Template.Spec, t)
}

// AddDaemonSetTopologySpreadConstraints appends a topology spread constraint if not nil.
func AddDaemonSetTopologySpreadConstraints(ds *appsv1.DaemonSet, c *corev1.TopologySpreadConstraint) error {
	if ds == nil {
		return errors.New("nil daemonset")
	}
	return AddPodSpecTopologySpreadConstraints(&ds.Spec.Template.Spec, c)
}

// SetDaemonSetServiceAccountName sets the service account name.
func SetDaemonSetServiceAccountName(ds *appsv1.DaemonSet, name string) error {
	if ds == nil {
		return errors.New("nil daemonset")
	}
	return SetPodSpecServiceAccountName(&ds.Spec.Template.Spec, name)
}

// SetDaemonSetSecurityContext sets the pod security context.
func SetDaemonSetSecurityContext(ds *appsv1.DaemonSet, sc *corev1.PodSecurityContext) error {
	if ds == nil {
		return errors.New("nil daemonset")
	}
	return SetPodSpecSecurityContext(&ds.Spec.Template.Spec, sc)
}

// SetDaemonSetAffinity sets the pod affinity rules.
func SetDaemonSetAffinity(ds *appsv1.DaemonSet, aff *corev1.Affinity) error {
	if ds == nil {
		return errors.New("nil daemonset")
	}
	return SetPodSpecAffinity(&ds.Spec.Template.Spec, aff)
}

// SetDaemonSetNodeSelector sets the node selector.
func SetDaemonSetNodeSelector(ds *appsv1.DaemonSet, ns map[string]string) error {
	if ds == nil {
		return errors.New("nil daemonset")
	}
	return SetPodSpecNodeSelector(&ds.Spec.Template.Spec, ns)
}

// SetDaemonSetUpdateStrategy sets the update strategy.
func SetDaemonSetUpdateStrategy(ds *appsv1.DaemonSet, strat appsv1.DaemonSetUpdateStrategy) error {
	if ds == nil {
		return errors.New("nil daemonset")
	}
	ds.Spec.UpdateStrategy = strat
	return nil
}

// SetDaemonSetRevisionHistoryLimit sets the revision history limit.
func SetDaemonSetRevisionHistoryLimit(ds *appsv1.DaemonSet, limit *int32) error {
	if ds == nil {
		return errors.New("nil daemonset")
	}
	ds.Spec.RevisionHistoryLimit = limit
	return nil
}
