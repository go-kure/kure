package k8s

import (
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
func AddDaemonSetContainer(ds *appsv1.DaemonSet, c *corev1.Container) {
	ds.Spec.Template.Spec.Containers = append(ds.Spec.Template.Spec.Containers, *c)
}

// AddDaemonSetInitContainer appends an init container to the pod template.
func AddDaemonSetInitContainer(ds *appsv1.DaemonSet, c *corev1.Container) {
	ds.Spec.Template.Spec.InitContainers = append(ds.Spec.Template.Spec.InitContainers, *c)
}

// AddDaemonSetVolume appends a volume to the pod template.
func AddDaemonSetVolume(ds *appsv1.DaemonSet, v *corev1.Volume) {
	ds.Spec.Template.Spec.Volumes = append(ds.Spec.Template.Spec.Volumes, *v)
}

// AddDaemonSetImagePullSecret appends an image pull secret to the pod template.
func AddDaemonSetImagePullSecret(ds *appsv1.DaemonSet, s *corev1.LocalObjectReference) {
	ds.Spec.Template.Spec.ImagePullSecrets = append(ds.Spec.Template.Spec.ImagePullSecrets, *s)
}

// AddDaemonSetToleration appends a toleration to the pod template.
func AddDaemonSetToleration(ds *appsv1.DaemonSet, t *corev1.Toleration) {
	ds.Spec.Template.Spec.Tolerations = append(ds.Spec.Template.Spec.Tolerations, *t)
}

// AddDaemonSetTopologySpreadConstraints appends a topology spread constraint if not nil.
func AddDaemonSetTopologySpreadConstraints(ds *appsv1.DaemonSet, c *corev1.TopologySpreadConstraint) {
	if c == nil {
		return
	}
	ds.Spec.Template.Spec.TopologySpreadConstraints = append(ds.Spec.Template.Spec.TopologySpreadConstraints, *c)
}

// SetDaemonSetServiceAccountName sets the service account name.
func SetDaemonSetServiceAccountName(ds *appsv1.DaemonSet, name string) {
	ds.Spec.Template.Spec.ServiceAccountName = name
}

// SetDaemonSetSecurityContext sets the pod security context.
func SetDaemonSetSecurityContext(ds *appsv1.DaemonSet, sc *corev1.PodSecurityContext) {
	ds.Spec.Template.Spec.SecurityContext = sc
}

// SetDaemonSetAffinity sets the pod affinity rules.
func SetDaemonSetAffinity(ds *appsv1.DaemonSet, aff *corev1.Affinity) {
	ds.Spec.Template.Spec.Affinity = aff
}

// SetDaemonSetNodeSelector sets the node selector.
func SetDaemonSetNodeSelector(ds *appsv1.DaemonSet, ns map[string]string) {
	ds.Spec.Template.Spec.NodeSelector = ns
}

// SetDaemonSetUpdateStrategy sets the update strategy.
func SetDaemonSetUpdateStrategy(ds *appsv1.DaemonSet, strat appsv1.DaemonSetUpdateStrategy) {
	ds.Spec.UpdateStrategy = strat
}
