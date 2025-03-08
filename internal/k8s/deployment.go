package k8s

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func CreateDeployment(name string) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app": name,
			},
			Annotations: map[string]string{
				"app": name,
			},
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: appsv1.SchemeGroupVersion.String(),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken: pointer.Bool(false),
				},
			},
		},
	}
	return deployment
}

func AddDeploymentContainer(deployment *appsv1.Deployment, container *corev1.Container) {
	deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, *container)
}

func AddDeploymentInitContainer(deployment *appsv1.Deployment, container *corev1.Container) {
	deployment.Spec.Template.Spec.InitContainers = append(deployment.Spec.Template.Spec.InitContainers, *container)
}

func AddDeploymentVolume(deployment *appsv1.Deployment, volume *corev1.Volume) {
	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, *volume)
}

func AddDeploymentImagePullSecret(deployment *appsv1.Deployment, imagePullSecret *corev1.LocalObjectReference) {
	deployment.Spec.Template.Spec.ImagePullSecrets = append(deployment.Spec.Template.Spec.ImagePullSecrets, *imagePullSecret)
}

func AddDeploymentToleration(deployment *appsv1.Deployment, toleration *corev1.Toleration) {
	deployment.Spec.Template.Spec.Tolerations = append(deployment.Spec.Template.Spec.Tolerations, *toleration)
}

func AddDeploymentTopologySpreadConstraints(deployment *appsv1.Deployment, topologySpreadConstraint *corev1.TopologySpreadConstraint) {
	deployment.Spec.Template.Spec.TopologySpreadConstraints = append(deployment.Spec.Template.Spec.TopologySpreadConstraints)
}

func SetDeploymentServiceAccountName(deployment *appsv1.Deployment, serviceAccountName string) {
	deployment.Spec.Template.Spec.ServiceAccountName = serviceAccountName
}

func SetDeploymentSecurityContext(deployment *appsv1.Deployment, securityContext *corev1.PodSecurityContext) {
	deployment.Spec.Template.Spec.SecurityContext = securityContext
}

func SetDeploymentAffinity(deployment *appsv1.Deployment, affinity *corev1.Affinity) {
	deployment.Spec.Template.Spec.Affinity = affinity
}

func SetDeploymentNodeSelector(deployment *appsv1.Deployment, nodeSelector map[string]string) {
	deployment.Spec.Template.Spec.NodeSelector = nodeSelector
}
