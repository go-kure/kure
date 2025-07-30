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
				Spec: corev1.PodSpec{},
			},
		},
	}
	return obj
}

// SetDeploymentPodSpec assigns a PodSpec to the Deployment template.
func SetDeploymentPodSpec(dep *appsv1.Deployment, spec *corev1.PodSpec) error {
	if dep == nil || spec == nil {
		return errors.New("nil deployment or spec")
	}
	dep.Spec.Template.Spec = *spec
	return nil
}

func AddDeploymentContainer(deployment *appsv1.Deployment, container *corev1.Container) error {
	if deployment == nil {
		return errors.New("nil deployment")
	}
	return AddPodSpecContainer(&deployment.Spec.Template.Spec, container)
}

func AddDeploymentInitContainer(deployment *appsv1.Deployment, container *corev1.Container) error {
	if deployment == nil {
		return errors.New("nil deployment")
	}
	return AddPodSpecInitContainer(&deployment.Spec.Template.Spec, container)
}

func AddDeploymentVolume(deployment *appsv1.Deployment, volume *corev1.Volume) error {
	if deployment == nil {
		return errors.New("nil deployment")
	}
	return AddPodSpecVolume(&deployment.Spec.Template.Spec, volume)
}

func AddDeploymentImagePullSecret(deployment *appsv1.Deployment, imagePullSecret *corev1.LocalObjectReference) error {
	if deployment == nil {
		return errors.New("nil deployment")
	}
	return AddPodSpecImagePullSecret(&deployment.Spec.Template.Spec, imagePullSecret)
}

func AddDeploymentToleration(deployment *appsv1.Deployment, toleration *corev1.Toleration) error {
	if deployment == nil {
		return errors.New("nil deployment")
	}
	return AddPodSpecToleration(&deployment.Spec.Template.Spec, toleration)
}

func AddDeploymentTopologySpreadConstraints(deployment *appsv1.Deployment, topologySpreadConstraint *corev1.TopologySpreadConstraint) error {
	if deployment == nil {
		return errors.New("nil deployment")
	}
	return AddPodSpecTopologySpreadConstraints(&deployment.Spec.Template.Spec, topologySpreadConstraint)
}

func SetDeploymentServiceAccountName(deployment *appsv1.Deployment, serviceAccountName string) error {
	if deployment == nil {
		return errors.New("nil deployment")
	}
	return SetPodSpecServiceAccountName(&deployment.Spec.Template.Spec, serviceAccountName)
}

func SetDeploymentSecurityContext(deployment *appsv1.Deployment, securityContext *corev1.PodSecurityContext) error {
	if deployment == nil {
		return errors.New("nil deployment")
	}
	return SetPodSpecSecurityContext(&deployment.Spec.Template.Spec, securityContext)
}

func SetDeploymentAffinity(deployment *appsv1.Deployment, affinity *corev1.Affinity) error {
	if deployment == nil {
		return errors.New("nil deployment")
	}
	return SetPodSpecAffinity(&deployment.Spec.Template.Spec, affinity)
}

func SetDeploymentNodeSelector(deployment *appsv1.Deployment, nodeSelector map[string]string) error {
	if deployment == nil {
		return errors.New("nil deployment")
	}
	return SetPodSpecNodeSelector(&deployment.Spec.Template.Spec, nodeSelector)
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
