package kubernetes

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-kure/kure/pkg/errors"
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
				Spec:       corev1.PodSpec{},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{},
			ServiceName:          "",
			PodManagementPolicy:  appsv1.OrderedReadyPodManagement,
			UpdateStrategy:       appsv1.StatefulSetUpdateStrategy{},
		},
	}
	return obj
}

// SetStatefulSetPodSpec assigns a PodSpec to the StatefulSet template.
func SetStatefulSetPodSpec(sts *appsv1.StatefulSet, spec *corev1.PodSpec) error {
	if sts == nil {
		return errors.ErrNilStatefulSet
	}
	if spec == nil {
		return errors.ErrNilSpec
	}
	sts.Spec.Template.Spec = *spec
	return nil
}

// AddStatefulSetContainer appends a container to the StatefulSet pod template.
func AddStatefulSetContainer(sts *appsv1.StatefulSet, c *corev1.Container) error {
	if sts == nil {
		return errors.ErrNilStatefulSet
	}
	return AddPodSpecContainer(&sts.Spec.Template.Spec, c)
}

// AddStatefulSetInitContainer appends an init container to the pod template.
func AddStatefulSetInitContainer(sts *appsv1.StatefulSet, c *corev1.Container) error {
	if sts == nil {
		return errors.ErrNilStatefulSet
	}
	return AddPodSpecInitContainer(&sts.Spec.Template.Spec, c)
}

// AddStatefulSetVolume appends a volume to the pod template.
func AddStatefulSetVolume(sts *appsv1.StatefulSet, v *corev1.Volume) error {
	if sts == nil {
		return errors.ErrNilStatefulSet
	}
	return AddPodSpecVolume(&sts.Spec.Template.Spec, v)
}

// AddStatefulSetImagePullSecret appends an image pull secret to the pod template.
func AddStatefulSetImagePullSecret(sts *appsv1.StatefulSet, s *corev1.LocalObjectReference) error {
	if sts == nil {
		return errors.ErrNilStatefulSet
	}
	return AddPodSpecImagePullSecret(&sts.Spec.Template.Spec, s)
}

// AddStatefulSetToleration appends a toleration to the pod template.
func AddStatefulSetToleration(sts *appsv1.StatefulSet, t *corev1.Toleration) error {
	if sts == nil {
		return errors.ErrNilStatefulSet
	}
	return AddPodSpecToleration(&sts.Spec.Template.Spec, t)
}

// AddStatefulSetTopologySpreadConstraints appends a topology spread constraint if not nil.
func AddStatefulSetTopologySpreadConstraints(sts *appsv1.StatefulSet, c *corev1.TopologySpreadConstraint) error {
	if sts == nil {
		return errors.ErrNilStatefulSet
	}
	return AddPodSpecTopologySpreadConstraints(&sts.Spec.Template.Spec, c)
}

// AddStatefulSetVolumeClaimTemplate appends a PVC template to the StatefulSet.
func AddStatefulSetVolumeClaimTemplate(sts *appsv1.StatefulSet, pvc corev1.PersistentVolumeClaim) {
	if sts == nil {
		panic("AddStatefulSetVolumeClaimTemplate: sts must not be nil")
	}
	sts.Spec.VolumeClaimTemplates = append(sts.Spec.VolumeClaimTemplates, pvc)
}

// SetStatefulSetServiceAccountName sets the service account name for the pod template.
func SetStatefulSetServiceAccountName(sts *appsv1.StatefulSet, name string) {
	if sts == nil {
		panic("SetStatefulSetServiceAccountName: sts must not be nil")
	}
	SetPodSpecServiceAccountName(&sts.Spec.Template.Spec, name)
}

// SetStatefulSetSecurityContext sets the pod security context.
func SetStatefulSetSecurityContext(sts *appsv1.StatefulSet, sc *corev1.PodSecurityContext) {
	if sts == nil {
		panic("SetStatefulSetSecurityContext: sts must not be nil")
	}
	SetPodSpecSecurityContext(&sts.Spec.Template.Spec, sc)
}

// SetStatefulSetAffinity sets the pod affinity rules.
func SetStatefulSetAffinity(sts *appsv1.StatefulSet, aff *corev1.Affinity) {
	if sts == nil {
		panic("SetStatefulSetAffinity: sts must not be nil")
	}
	SetPodSpecAffinity(&sts.Spec.Template.Spec, aff)
}

// SetStatefulSetNodeSelector sets the node selector.
func SetStatefulSetNodeSelector(sts *appsv1.StatefulSet, ns map[string]string) {
	if sts == nil {
		panic("SetStatefulSetNodeSelector: sts must not be nil")
	}
	SetPodSpecNodeSelector(&sts.Spec.Template.Spec, ns)
}

// SetStatefulSetUpdateStrategy sets the update strategy for the StatefulSet.
func SetStatefulSetUpdateStrategy(sts *appsv1.StatefulSet, strategy appsv1.StatefulSetUpdateStrategy) {
	if sts == nil {
		panic("SetStatefulSetUpdateStrategy: sts must not be nil")
	}
	sts.Spec.UpdateStrategy = strategy
}

// SetStatefulSetReplicas sets the replica count.
func SetStatefulSetReplicas(sts *appsv1.StatefulSet, replicas int32) {
	if sts == nil {
		panic("SetStatefulSetReplicas: sts must not be nil")
	}
	if sts.Spec.Replicas == nil {
		sts.Spec.Replicas = new(int32)
	}
	*sts.Spec.Replicas = replicas
}

// SetStatefulSetServiceName sets the service name used by the StatefulSet.
func SetStatefulSetServiceName(sts *appsv1.StatefulSet, svc string) {
	if sts == nil {
		panic("SetStatefulSetServiceName: sts must not be nil")
	}
	sts.Spec.ServiceName = svc
}

// SetStatefulSetPodManagementPolicy sets the pod management policy.
func SetStatefulSetPodManagementPolicy(sts *appsv1.StatefulSet, policy appsv1.PodManagementPolicyType) {
	if sts == nil {
		panic("SetStatefulSetPodManagementPolicy: sts must not be nil")
	}
	sts.Spec.PodManagementPolicy = policy
}

// SetStatefulSetRevisionHistoryLimit sets the revision history limit.
func SetStatefulSetRevisionHistoryLimit(sts *appsv1.StatefulSet, limit *int32) {
	if sts == nil {
		panic("SetStatefulSetRevisionHistoryLimit: sts must not be nil")
	}
	sts.Spec.RevisionHistoryLimit = limit
}

// SetStatefulSetMinReadySeconds sets the minimum ready seconds.
func SetStatefulSetMinReadySeconds(sts *appsv1.StatefulSet, secs int32) {
	if sts == nil {
		panic("SetStatefulSetMinReadySeconds: sts must not be nil")
	}
	sts.Spec.MinReadySeconds = secs
}
