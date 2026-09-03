package kubernetes

import (
	appsv1 "k8s.io/api/apps/v1"
)

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
