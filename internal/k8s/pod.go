package k8s

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreatePod(name string, namespace string) *corev1.Pod {
	obj := corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
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
	}
	return &obj
}

func AddPodContainer(pod *corev1.Pod, container *corev1.Container) {
	pod.Spec.Containers = append(pod.Spec.Containers, *container)
}

func AddPodInitContainer(pod *corev1.Pod, container *corev1.Container) {
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, *container)
}

func AddPodVolume(pod *corev1.Pod, volume *corev1.Volume) {
	pod.Spec.Volumes = append(pod.Spec.Volumes, *volume)
}

func AddPodImagePullSecret(pod *corev1.Pod, imagePullSecret *corev1.LocalObjectReference) {
	pod.Spec.ImagePullSecrets = append(pod.Spec.ImagePullSecrets, *imagePullSecret)
}

func AddPodToleration(pod *corev1.Pod, toleration *corev1.Toleration) {
	pod.Spec.Tolerations = append(pod.Spec.Tolerations, *toleration)
}

func AddPodTopologySpreadConstraints(pod *corev1.Pod, topologySpreadConstraint *corev1.TopologySpreadConstraint) {
	if topologySpreadConstraint == nil {
		return
	}
	pod.Spec.TopologySpreadConstraints = append(pod.Spec.TopologySpreadConstraints, *topologySpreadConstraint)
}

func SetPodServiceAccountName(pod *corev1.Pod, serviceAccountName string) {
	pod.Spec.ServiceAccountName = serviceAccountName
}

func SetPodSecurityContext(pod *corev1.Pod, securityContext *corev1.PodSecurityContext) {
	pod.Spec.SecurityContext = securityContext
}

func SetPodAffinity(pod *corev1.Pod, affinity *corev1.Affinity) {
	pod.Spec.Affinity = affinity
}

func SetPodNodeSelector(pod *corev1.Pod, nodeSelector map[string]string) {
	pod.Spec.NodeSelector = nodeSelector
}
