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
				Spec: corev1.PodSpec{
					Containers:                    []corev1.Container{},
					InitContainers:                []corev1.Container{},
					Volumes:                       []corev1.Volume{},
					RestartPolicy:                 corev1.RestartPolicyAlways,
					TerminationGracePeriodSeconds: new(int64),
					SecurityContext:               &corev1.PodSecurityContext{},
					ImagePullSecrets:              []corev1.LocalObjectReference{},
					ServiceAccountName:            "",
					NodeSelector:                  map[string]string{},
					Affinity:                      &corev1.Affinity{},
					Tolerations:                   []corev1.Toleration{},
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{},
		},
	}
	return obj
}

// AddDaemonSetContainer appends a container to the DaemonSet pod template.
func AddDaemonSetContainer(ds *appsv1.DaemonSet, c *corev1.Container) error {
	if ds == nil || c == nil {
		return errors.New("nil daemonset or container")
	}
	ds.Spec.Template.Spec.Containers = append(ds.Spec.Template.Spec.Containers, *c)
	return nil
}

// AddDaemonSetInitContainer appends an init container to the pod template.
func AddDaemonSetInitContainer(ds *appsv1.DaemonSet, c *corev1.Container) error {
	if ds == nil || c == nil {
		return errors.New("nil daemonset or container")
	}
	ds.Spec.Template.Spec.InitContainers = append(ds.Spec.Template.Spec.InitContainers, *c)
	return nil
}

// AddDaemonSetVolume appends a volume to the pod template.
func AddDaemonSetVolume(ds *appsv1.DaemonSet, v *corev1.Volume) error {
	if ds == nil || v == nil {
		return errors.New("nil daemonset or volume")
	}
	ds.Spec.Template.Spec.Volumes = append(ds.Spec.Template.Spec.Volumes, *v)
	return nil
}

// AddDaemonSetImagePullSecret appends an image pull secret to the pod template.
func AddDaemonSetImagePullSecret(ds *appsv1.DaemonSet, s *corev1.LocalObjectReference) error {
	if ds == nil || s == nil {
		return errors.New("nil daemonset or secret")
	}
	ds.Spec.Template.Spec.ImagePullSecrets = append(ds.Spec.Template.Spec.ImagePullSecrets, *s)
	return nil
}

// AddDaemonSetToleration appends a toleration to the pod template.
func AddDaemonSetToleration(ds *appsv1.DaemonSet, t *corev1.Toleration) error {
	if ds == nil || t == nil {
		return errors.New("nil daemonset or toleration")
	}
	ds.Spec.Template.Spec.Tolerations = append(ds.Spec.Template.Spec.Tolerations, *t)
	return nil
}

// AddDaemonSetTopologySpreadConstraints appends a topology spread constraint if not nil.
func AddDaemonSetTopologySpreadConstraints(ds *appsv1.DaemonSet, c *corev1.TopologySpreadConstraint) error {
	if ds == nil {
		return errors.New("nil daemonset")
	}
	if c == nil {
		return nil
	}
	ds.Spec.Template.Spec.TopologySpreadConstraints = append(ds.Spec.Template.Spec.TopologySpreadConstraints, *c)
	return nil
}

// SetDaemonSetServiceAccountName sets the service account name.
func SetDaemonSetServiceAccountName(ds *appsv1.DaemonSet, name string) error {
	if ds == nil {
		return errors.New("nil daemonset")
	}
	ds.Spec.Template.Spec.ServiceAccountName = name
	return nil
}

// SetDaemonSetSecurityContext sets the pod security context.
func SetDaemonSetSecurityContext(ds *appsv1.DaemonSet, sc *corev1.PodSecurityContext) error {
	if ds == nil {
		return errors.New("nil daemonset")
	}
	ds.Spec.Template.Spec.SecurityContext = sc
	return nil
}

// SetDaemonSetAffinity sets the pod affinity rules.
func SetDaemonSetAffinity(ds *appsv1.DaemonSet, aff *corev1.Affinity) error {
	if ds == nil {
		return errors.New("nil daemonset")
	}
	ds.Spec.Template.Spec.Affinity = aff
	return nil
}

// SetDaemonSetNodeSelector sets the node selector.
func SetDaemonSetNodeSelector(ds *appsv1.DaemonSet, ns map[string]string) error {
	if ds == nil {
		return errors.New("nil daemonset")
	}
	ds.Spec.Template.Spec.NodeSelector = ns
	return nil
}

// SetDaemonSetUpdateStrategy sets the update strategy.
func SetDaemonSetUpdateStrategy(ds *appsv1.DaemonSet, strat appsv1.DaemonSetUpdateStrategy) error {
	if ds == nil {
		return errors.New("nil daemonset")
	}
	ds.Spec.UpdateStrategy = strat
	return nil
}
