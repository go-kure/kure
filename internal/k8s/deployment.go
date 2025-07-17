package k8s

import (
	"errors"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateDeployment(name string, namespace string) *appsv1.Deployment {

	obj := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
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
		},
	}
	return obj
}

func AddDeploymentContainer(deployment *appsv1.Deployment, container *corev1.Container) error {
	if deployment == nil || container == nil {
		return errors.New("nil deployment or container")
	}
	deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, *container)
	return nil
}

func AddDeploymentInitContainer(deployment *appsv1.Deployment, container *corev1.Container) error {
	if deployment == nil || container == nil {
		return errors.New("nil deployment or container")
	}
	deployment.Spec.Template.Spec.InitContainers = append(deployment.Spec.Template.Spec.InitContainers, *container)
	return nil
}

func AddDeploymentVolume(deployment *appsv1.Deployment, volume *corev1.Volume) error {
	if deployment == nil || volume == nil {
		return errors.New("nil deployment or volume")
	}
	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, *volume)
	return nil
}

func AddDeploymentImagePullSecret(deployment *appsv1.Deployment, imagePullSecret *corev1.LocalObjectReference) error {
	if deployment == nil || imagePullSecret == nil {
		return errors.New("nil deployment or imagePullSecret")
	}
	deployment.Spec.Template.Spec.ImagePullSecrets = append(deployment.Spec.Template.Spec.ImagePullSecrets, *imagePullSecret)
	return nil
}

func AddDeploymentToleration(deployment *appsv1.Deployment, toleration *corev1.Toleration) error {
	if deployment == nil || toleration == nil {
		return errors.New("nil deployment or toleration")
	}
	deployment.Spec.Template.Spec.Tolerations = append(deployment.Spec.Template.Spec.Tolerations, *toleration)
	return nil
}

func AddDeploymentTopologySpreadConstraints(deployment *appsv1.Deployment, topologySpreadConstraint *corev1.TopologySpreadConstraint) error {
	if deployment == nil {
		return errors.New("nil deployment")
	}
	if topologySpreadConstraint == nil {
		return nil
	}
	deployment.Spec.Template.Spec.TopologySpreadConstraints = append(deployment.Spec.Template.Spec.TopologySpreadConstraints, *topologySpreadConstraint)
	return nil
}

func SetDeploymentServiceAccountName(deployment *appsv1.Deployment, serviceAccountName string) error {
	if deployment == nil {
		return errors.New("nil deployment")
	}
	deployment.Spec.Template.Spec.ServiceAccountName = serviceAccountName
	return nil
}

func SetDeploymentSecurityContext(deployment *appsv1.Deployment, securityContext *corev1.PodSecurityContext) error {
	if deployment == nil {
		return errors.New("nil deployment")
	}
	deployment.Spec.Template.Spec.SecurityContext = securityContext
	return nil
}

func SetDeploymentAffinity(deployment *appsv1.Deployment, affinity *corev1.Affinity) error {
	if deployment == nil {
		return errors.New("nil deployment")
	}
	deployment.Spec.Template.Spec.Affinity = affinity
	return nil
}

func SetDeploymentNodeSelector(deployment *appsv1.Deployment, nodeSelector map[string]string) error {
	if deployment == nil {
		return errors.New("nil deployment")
	}
	deployment.Spec.Template.Spec.NodeSelector = nodeSelector
	return nil
}

// SetDeploymentReplicas sets the desired replica count.
func SetDeploymentReplicas(deployment *appsv1.Deployment, replicas int32) error {
	if deployment == nil {
		return errors.New("nil deployment")
	}
	if deployment.Spec.Replicas == nil {
		deployment.Spec.Replicas = new(int32)
	}
	*deployment.Spec.Replicas = replicas
	return nil
}

// SetDeploymentStrategy sets the deployment strategy.
func SetDeploymentStrategy(deployment *appsv1.Deployment, strategy appsv1.DeploymentStrategy) error {
	if deployment == nil {
		return errors.New("nil deployment")
	}
	deployment.Spec.Strategy = strategy
	return nil
}

// SetDeploymentRevisionHistoryLimit sets the revision history limit.
func SetDeploymentRevisionHistoryLimit(deployment *appsv1.Deployment, limit int32) error {
	if deployment == nil {
		return errors.New("nil deployment")
	}
	deployment.Spec.RevisionHistoryLimit = &limit
	return nil
}

// SetDeploymentMinReadySeconds sets the minimum ready seconds.
func SetDeploymentMinReadySeconds(deployment *appsv1.Deployment, secs int32) error {
	if deployment == nil {
		return errors.New("nil deployment")
	}
	deployment.Spec.MinReadySeconds = secs
	return nil
}

// SetDeploymentProgressDeadlineSeconds sets the progress deadline seconds.
func SetDeploymentProgressDeadlineSeconds(deployment *appsv1.Deployment, secs int32) error {
	if deployment == nil {
		return errors.New("nil deployment")
	}
	deployment.Spec.ProgressDeadlineSeconds = &secs
	return nil
}
