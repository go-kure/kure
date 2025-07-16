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
