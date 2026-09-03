package kubernetes

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/go-kure/kure/pkg/errors"
)

// SetDeploymentPodSpec assigns a PodSpec to the Deployment template.
func SetDeploymentPodSpec(deployment *appsv1.Deployment, spec *corev1.PodSpec) error {
	if deployment == nil {
		return errors.ErrNilDeployment
	}
	if spec == nil {
		return errors.ErrNilPodSpec
	}
	deployment.Spec.Template.Spec = *spec
	return nil
}

// AddDeploymentContainer appends a container to the Deployment's pod template.
func AddDeploymentContainer(deployment *appsv1.Deployment, container *corev1.Container) error {
	if deployment == nil {
		return errors.ErrNilDeployment
	}
	if container == nil {
		return errors.ErrNilContainer
	}
	deployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, *container)
	return nil
}

// AddDeploymentInitContainer appends an init container to the Deployment's pod template.
func AddDeploymentInitContainer(deployment *appsv1.Deployment, container *corev1.Container) error {
	if deployment == nil {
		return errors.ErrNilDeployment
	}
	if container == nil {
		return errors.ErrNilInitContainer
	}
	deployment.Spec.Template.Spec.InitContainers = append(deployment.Spec.Template.Spec.InitContainers, *container)
	return nil
}

// AddDeploymentVolume appends a volume to the Deployment's pod template.
func AddDeploymentVolume(deployment *appsv1.Deployment, volume *corev1.Volume) error {
	if deployment == nil {
		return errors.ErrNilDeployment
	}
	if volume == nil {
		return errors.ErrNilVolume
	}
	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, *volume)
	return nil
}

// AddDeploymentImagePullSecret appends an image pull secret to the Deployment's pod template.
func AddDeploymentImagePullSecret(deployment *appsv1.Deployment, imagePullSecret *corev1.LocalObjectReference) error {
	if deployment == nil {
		return errors.ErrNilDeployment
	}
	if imagePullSecret == nil {
		return errors.ErrNilImagePullSecret
	}
	deployment.Spec.Template.Spec.ImagePullSecrets = append(deployment.Spec.Template.Spec.ImagePullSecrets, *imagePullSecret)
	return nil
}

// AddDeploymentToleration appends a toleration to the Deployment's pod template.
func AddDeploymentToleration(deployment *appsv1.Deployment, toleration *corev1.Toleration) error {
	if deployment == nil {
		return errors.ErrNilDeployment
	}
	if toleration == nil {
		return errors.ErrNilToleration
	}
	deployment.Spec.Template.Spec.Tolerations = append(deployment.Spec.Template.Spec.Tolerations, *toleration)
	return nil
}

// AddDeploymentTopologySpreadConstraints appends a topology spread constraint
// to the Deployment's pod template. If the constraint is nil the call is a
// no-op.
func AddDeploymentTopologySpreadConstraints(deployment *appsv1.Deployment, topologySpreadConstraint *corev1.TopologySpreadConstraint) error {
	if deployment == nil {
		return errors.ErrNilDeployment
	}
	if topologySpreadConstraint == nil {
		return nil
	}
	deployment.Spec.Template.Spec.TopologySpreadConstraints = append(deployment.Spec.Template.Spec.TopologySpreadConstraints, *topologySpreadConstraint)
	return nil
}

// SetDeploymentSecurityContext sets the pod-level security context on the
// Deployment's pod template.
func SetDeploymentSecurityContext(deployment *appsv1.Deployment, securityContext *corev1.PodSecurityContext) {
	if deployment == nil {
		panic("SetDeploymentSecurityContext: deployment must not be nil")
	}
	deployment.Spec.Template.Spec.SecurityContext = securityContext
}

// SetDeploymentAffinity assigns affinity rules to the Deployment's pod template.
func SetDeploymentAffinity(deployment *appsv1.Deployment, affinity *corev1.Affinity) {
	if deployment == nil {
		panic("SetDeploymentAffinity: deployment must not be nil")
	}
	deployment.Spec.Template.Spec.Affinity = affinity
}

// SetDeploymentReplicas sets the desired replica count.
func SetDeploymentReplicas(deployment *appsv1.Deployment, replicas int32) {
	if deployment == nil {
		panic("SetDeploymentReplicas: deployment must not be nil")
	}
	if deployment.Spec.Replicas == nil {
		deployment.Spec.Replicas = new(int32)
	}
	*deployment.Spec.Replicas = replicas
}

// SetDeploymentRevisionHistoryLimit sets the revision history limit.
func SetDeploymentRevisionHistoryLimit(deployment *appsv1.Deployment, limit int32) {
	if deployment == nil {
		panic("SetDeploymentRevisionHistoryLimit: deployment must not be nil")
	}
	deployment.Spec.RevisionHistoryLimit = &limit
}

// SetDeploymentProgressDeadlineSeconds sets the progress deadline seconds.
func SetDeploymentProgressDeadlineSeconds(deployment *appsv1.Deployment, secs int32) {
	if deployment == nil {
		panic("SetDeploymentProgressDeadlineSeconds: deployment must not be nil")
	}
	deployment.Spec.ProgressDeadlineSeconds = &secs
}
