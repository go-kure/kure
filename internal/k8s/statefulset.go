package k8s

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateStatefulSet returns a StatefulSet with sensible defaults set.
func CreateStatefulSet(name, namespace string) *appsv1.StatefulSet {
	obj := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
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
		Spec: appsv1.StatefulSetSpec{
			Replicas: new(int32),
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
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{},
			ServiceName:          "",
			PodManagementPolicy:  appsv1.OrderedReadyPodManagement,
			UpdateStrategy:       appsv1.StatefulSetUpdateStrategy{},
		},
	}
	return obj
}

// AddStatefulSetContainer appends a container to the StatefulSet pod template.
func AddStatefulSetContainer(sts *appsv1.StatefulSet, c *corev1.Container) {
	sts.Spec.Template.Spec.Containers = append(sts.Spec.Template.Spec.Containers, *c)
}

// AddStatefulSetInitContainer appends an init container to the pod template.
func AddStatefulSetInitContainer(sts *appsv1.StatefulSet, c *corev1.Container) {
	sts.Spec.Template.Spec.InitContainers = append(sts.Spec.Template.Spec.InitContainers, *c)
}

// AddStatefulSetVolume appends a volume to the pod template.
func AddStatefulSetVolume(sts *appsv1.StatefulSet, v *corev1.Volume) {
	sts.Spec.Template.Spec.Volumes = append(sts.Spec.Template.Spec.Volumes, *v)
}

// AddStatefulSetImagePullSecret appends an image pull secret to the pod template.
func AddStatefulSetImagePullSecret(sts *appsv1.StatefulSet, s *corev1.LocalObjectReference) {
	sts.Spec.Template.Spec.ImagePullSecrets = append(sts.Spec.Template.Spec.ImagePullSecrets, *s)
}

// AddStatefulSetToleration appends a toleration to the pod template.
func AddStatefulSetToleration(sts *appsv1.StatefulSet, t *corev1.Toleration) {
	sts.Spec.Template.Spec.Tolerations = append(sts.Spec.Template.Spec.Tolerations, *t)
}

// AddStatefulSetTopologySpreadConstraints appends a topology spread constraint if not nil.
func AddStatefulSetTopologySpreadConstraints(sts *appsv1.StatefulSet, c *corev1.TopologySpreadConstraint) {
	if c == nil {
		return
	}
	sts.Spec.Template.Spec.TopologySpreadConstraints = append(sts.Spec.Template.Spec.TopologySpreadConstraints, *c)
}

// AddStatefulSetVolumeClaimTemplate appends a PVC template to the StatefulSet.
func AddStatefulSetVolumeClaimTemplate(sts *appsv1.StatefulSet, pvc corev1.PersistentVolumeClaim) {
	sts.Spec.VolumeClaimTemplates = append(sts.Spec.VolumeClaimTemplates, pvc)
}

// SetStatefulSetServiceAccountName sets the service account name for the pod template.
func SetStatefulSetServiceAccountName(sts *appsv1.StatefulSet, name string) {
	sts.Spec.Template.Spec.ServiceAccountName = name
}

// SetStatefulSetSecurityContext sets the pod security context.
func SetStatefulSetSecurityContext(sts *appsv1.StatefulSet, sc *corev1.PodSecurityContext) {
	sts.Spec.Template.Spec.SecurityContext = sc
}

// SetStatefulSetAffinity sets the pod affinity rules.
func SetStatefulSetAffinity(sts *appsv1.StatefulSet, aff *corev1.Affinity) {
	sts.Spec.Template.Spec.Affinity = aff
}

// SetStatefulSetNodeSelector sets the node selector.
func SetStatefulSetNodeSelector(sts *appsv1.StatefulSet, ns map[string]string) {
	sts.Spec.Template.Spec.NodeSelector = ns
}

// SetStatefulSetUpdateStrategy sets the update strategy for the StatefulSet.
func SetStatefulSetUpdateStrategy(sts *appsv1.StatefulSet, strat appsv1.StatefulSetUpdateStrategy) {
	sts.Spec.UpdateStrategy = strat
}

// SetStatefulSetReplicas sets the replica count.
func SetStatefulSetReplicas(sts *appsv1.StatefulSet, replicas int32) {
	if sts.Spec.Replicas == nil {
		sts.Spec.Replicas = new(int32)
	}
	*sts.Spec.Replicas = replicas
}

// SetStatefulSetServiceName sets the service name used by the StatefulSet.
func SetStatefulSetServiceName(sts *appsv1.StatefulSet, svc string) {
	sts.Spec.ServiceName = svc
}

// SetStatefulSetPodManagementPolicy sets the pod management policy.
func SetStatefulSetPodManagementPolicy(sts *appsv1.StatefulSet, policy appsv1.PodManagementPolicyType) {
	sts.Spec.PodManagementPolicy = policy
}
