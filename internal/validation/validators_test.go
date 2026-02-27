package validation

import (
	stderrors "errors"
	"testing"

	"github.com/go-kure/kure/pkg/errors"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/kustomize/api/types"

	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
)

func TestNewValidator(t *testing.T) {
	validator := NewValidator()
	if validator == nil {
		t.Fatal("expected non-nil validator")
	}
}

func TestValidateNotNil(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name    string
		obj     interface{}
		errType error
		wantErr bool
	}{
		{
			name:    "nil object",
			obj:     nil,
			errType: errors.ErrNilPod,
			wantErr: true,
		},
		{
			name:    "nil pointer",
			obj:     (*corev1.Pod)(nil),
			errType: errors.ErrNilPod,
			wantErr: true,
		},
		{
			name:    "valid object",
			obj:     &corev1.Pod{},
			errType: errors.ErrNilPod,
			wantErr: false,
		},
		{
			name:    "non-pointer object",
			obj:     "test",
			errType: errors.ErrNilPod,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateNotNil(tt.obj, tt.errType)

			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.wantErr && !stderrors.Is(err, tt.errType) {
				t.Errorf("expected error %v, got %v", tt.errType, err)
			}
		})
	}
}

// Test Core Kubernetes Resources Validation

func TestValidatePod(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name    string
		pod     *corev1.Pod
		wantErr bool
	}{
		{
			name:    "nil pod",
			pod:     nil,
			wantErr: true,
		},
		{
			name:    "valid pod",
			pod:     &corev1.Pod{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidatePod(tt.pod)

			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateDeployment(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name       string
		deployment *appsv1.Deployment
		wantErr    bool
	}{
		{
			name:       "nil deployment",
			deployment: nil,
			wantErr:    true,
		},
		{
			name:       "valid deployment",
			deployment: &appsv1.Deployment{},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateDeployment(tt.deployment)

			if tt.wantErr && !stderrors.Is(err, errors.ErrNilDeployment) {
				t.Errorf("expected ErrNilDeployment, got %v", err)
			}

			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateService(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name    string
		service *corev1.Service
		wantErr bool
	}{
		{
			name:    "nil service",
			service: nil,
			wantErr: true,
		},
		{
			name:    "valid service",
			service: &corev1.Service{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateService(tt.service)

			if tt.wantErr && !stderrors.Is(err, errors.ErrNilService) {
				t.Errorf("expected ErrNilService, got %v", err)
			}

			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateConfigMap(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateConfigMap(nil)
	if !stderrors.Is(err, errors.ErrNilConfigMap) {
		t.Errorf("expected ErrNilConfigMap, got %v", err)
	}

	err = validator.ValidateConfigMap(&corev1.ConfigMap{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateSecret(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateSecret(nil)
	if !stderrors.Is(err, errors.ErrNilSecret) {
		t.Errorf("expected ErrNilSecret, got %v", err)
	}

	err = validator.ValidateSecret(&corev1.Secret{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateServiceAccount(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateServiceAccount(nil)
	if !stderrors.Is(err, errors.ErrNilServiceAccount) {
		t.Errorf("expected ErrNilServiceAccount, got %v", err)
	}

	err = validator.ValidateServiceAccount(&corev1.ServiceAccount{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateNamespace(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateNamespace(nil)
	if !stderrors.Is(err, errors.ErrNilNamespace) {
		t.Errorf("expected ErrNilNamespace, got %v", err)
	}

	err = validator.ValidateNamespace(&corev1.Namespace{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateIngress(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateIngress(nil)
	if !stderrors.Is(err, errors.ErrNilIngress) {
		t.Errorf("expected ErrNilIngress, got %v", err)
	}

	err = validator.ValidateIngress(&networkingv1.Ingress{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateStatefulSet(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateStatefulSet(nil)
	if !stderrors.Is(err, errors.ErrNilStatefulSet) {
		t.Errorf("expected ErrNilStatefulSet, got %v", err)
	}

	err = validator.ValidateStatefulSet(&appsv1.StatefulSet{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateDaemonSet(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateDaemonSet(nil)
	if !stderrors.Is(err, errors.ErrNilDaemonSet) {
		t.Errorf("expected ErrNilDaemonSet, got %v", err)
	}

	err = validator.ValidateDaemonSet(&appsv1.DaemonSet{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateJob(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateJob(nil)
	if !stderrors.Is(err, errors.ErrNilJob) {
		t.Errorf("expected ErrNilJob, got %v", err)
	}

	err = validator.ValidateJob(&batchv1.Job{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateCronJob(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateCronJob(nil)
	if !stderrors.Is(err, errors.ErrNilCronJob) {
		t.Errorf("expected ErrNilCronJob, got %v", err)
	}

	err = validator.ValidateCronJob(&batchv1.CronJob{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// Test RBAC Resources Validation

func TestValidateRole(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateRole(nil)
	if !stderrors.Is(err, errors.ErrNilRole) {
		t.Errorf("expected ErrNilRole, got %v", err)
	}

	err = validator.ValidateRole(&rbacv1.Role{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateClusterRole(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateClusterRole(nil)
	if !stderrors.Is(err, errors.ErrNilClusterRole) {
		t.Errorf("expected ErrNilClusterRole, got %v", err)
	}

	err = validator.ValidateClusterRole(&rbacv1.ClusterRole{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateRoleBinding(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateRoleBinding(nil)
	if !stderrors.Is(err, errors.ErrNilRoleBinding) {
		t.Errorf("expected ErrNilRoleBinding, got %v", err)
	}

	err = validator.ValidateRoleBinding(&rbacv1.RoleBinding{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateClusterRoleBinding(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateClusterRoleBinding(nil)
	if !stderrors.Is(err, errors.ErrNilClusterRoleBinding) {
		t.Errorf("expected ErrNilClusterRoleBinding, got %v", err)
	}

	err = validator.ValidateClusterRoleBinding(&rbacv1.ClusterRoleBinding{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// Test Component Validation

func TestValidatePodSpec(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidatePodSpec(nil)
	if !stderrors.Is(err, errors.ErrNilPodSpec) {
		t.Errorf("expected ErrNilPodSpec, got %v", err)
	}

	err = validator.ValidatePodSpec(&corev1.PodSpec{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateContainer(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateContainer(nil)
	if !stderrors.Is(err, errors.ErrNilContainer) {
		t.Errorf("expected ErrNilContainer, got %v", err)
	}

	err = validator.ValidateContainer(&corev1.Container{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateInitContainer(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateInitContainer(nil)
	if !stderrors.Is(err, errors.ErrNilInitContainer) {
		t.Errorf("expected ErrNilInitContainer, got %v", err)
	}

	err = validator.ValidateInitContainer(&corev1.Container{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateEphemeralContainer(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateEphemeralContainer(nil)
	if !stderrors.Is(err, errors.ErrNilEphemeralContainer) {
		t.Errorf("expected ErrNilEphemeralContainer, got %v", err)
	}

	err = validator.ValidateEphemeralContainer(&corev1.EphemeralContainer{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateVolume(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateVolume(nil)
	if !stderrors.Is(err, errors.ErrNilVolume) {
		t.Errorf("expected ErrNilVolume, got %v", err)
	}

	err = validator.ValidateVolume(&corev1.Volume{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateImagePullSecret(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateImagePullSecret(nil)
	if !stderrors.Is(err, errors.ErrNilImagePullSecret) {
		t.Errorf("expected ErrNilImagePullSecret, got %v", err)
	}

	err = validator.ValidateImagePullSecret(&corev1.LocalObjectReference{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateToleration(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateToleration(nil)
	if !stderrors.Is(err, errors.ErrNilToleration) {
		t.Errorf("expected ErrNilToleration, got %v", err)
	}

	err = validator.ValidateToleration(&corev1.Toleration{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateServicePort(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateServicePort(nil)
	if !stderrors.Is(err, errors.ErrNilServicePort) {
		t.Errorf("expected ErrNilServicePort, got %v", err)
	}

	err = validator.ValidateServicePort(&corev1.ServicePort{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidatePodDisruptionBudget(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidatePodDisruptionBudget(nil)
	if !stderrors.Is(err, errors.ErrNilPodDisruptionBudget) {
		t.Errorf("expected ErrNilPodDisruptionBudget, got %v", err)
	}

	err = validator.ValidatePodDisruptionBudget(&policyv1.PodDisruptionBudget{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateKustomization(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateKustomization(nil)
	if !stderrors.Is(err, errors.ErrNilKustomization) {
		t.Errorf("expected ErrNilKustomization, got %v", err)
	}

	err = validator.ValidateKustomization(&types.Kustomization{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateFluxInstance(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateFluxInstance(nil)
	if !stderrors.Is(err, errors.ErrNilFluxInstance) {
		t.Errorf("expected ErrNilFluxInstance, got %v", err)
	}

	err = validator.ValidateFluxInstance(&fluxv1.FluxInstance{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateIPAddressPool(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateIPAddressPool(nil)
	if !stderrors.Is(err, errors.ErrNilIPAddressPool) {
		t.Errorf("expected ErrNilIPAddressPool, got %v", err)
	}

	err = validator.ValidateIPAddressPool(&metallbv1beta1.IPAddressPool{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateBGPPeer(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateBGPPeer(nil)
	if !stderrors.Is(err, errors.ErrNilBGPPeer) {
		t.Errorf("expected ErrNilBGPPeer, got %v", err)
	}

	err = validator.ValidateBGPPeer(&metallbv1beta1.BGPPeer{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateBGPAdvertisement(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateBGPAdvertisement(nil)
	if !stderrors.Is(err, errors.ErrNilBGPAdvertisement) {
		t.Errorf("expected ErrNilBGPAdvertisement, got %v", err)
	}

	err = validator.ValidateBGPAdvertisement(&metallbv1beta1.BGPAdvertisement{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateL2Advertisement(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateL2Advertisement(nil)
	if !stderrors.Is(err, errors.ErrNilL2Advertisement) {
		t.Errorf("expected ErrNilL2Advertisement, got %v", err)
	}

	err = validator.ValidateL2Advertisement(&metallbv1beta1.L2Advertisement{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateBFDProfile(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateBFDProfile(nil)
	if !stderrors.Is(err, errors.ErrNilBFDProfile) {
		t.Errorf("expected ErrNilBFDProfile, got %v", err)
	}

	err = validator.ValidateBFDProfile(&metallbv1beta1.BFDProfile{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// Test Compound Validation Methods

func TestValidateDeploymentWithSpec(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name       string
		deployment *appsv1.Deployment
		spec       *corev1.PodSpec
		wantErr    bool
	}{
		{
			name:       "nil deployment",
			deployment: nil,
			spec:       &corev1.PodSpec{},
			wantErr:    true,
		},
		{
			name:       "valid deployment, nil spec",
			deployment: &appsv1.Deployment{},
			spec:       nil,
			wantErr:    false,
		},
		{
			name:       "valid deployment, valid spec",
			deployment: &appsv1.Deployment{},
			spec:       &corev1.PodSpec{},
			wantErr:    false,
		},
		{
			name:       "valid deployment, nil spec pointer",
			deployment: &appsv1.Deployment{},
			spec:       (*corev1.PodSpec)(nil),
			wantErr:    false, // Nil specs are typically allowed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateDeploymentWithSpec(tt.deployment, tt.spec)

			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidatePodSpecWithContainer(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name      string
		spec      *corev1.PodSpec
		container *corev1.Container
		wantErr   bool
	}{
		{
			name:      "nil spec",
			spec:      nil,
			container: &corev1.Container{},
			wantErr:   true,
		},
		{
			name:      "nil container",
			spec:      &corev1.PodSpec{},
			container: nil,
			wantErr:   true,
		},
		{
			name:      "valid spec and container",
			spec:      &corev1.PodSpec{},
			container: &corev1.Container{},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidatePodSpecWithContainer(tt.spec, tt.container)

			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateServiceWithPort(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name    string
		service *corev1.Service
		port    *corev1.ServicePort
		wantErr bool
	}{
		{
			name:    "nil service",
			service: nil,
			port:    &corev1.ServicePort{},
			wantErr: true,
		},
		{
			name:    "nil port",
			service: &corev1.Service{},
			port:    nil,
			wantErr: true,
		},
		{
			name:    "valid service and port",
			service: &corev1.Service{},
			port:    &corev1.ServicePort{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateServiceWithPort(tt.service, tt.port)

			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateHorizontalPodAutoscaler(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateHorizontalPodAutoscaler(nil)
	if !stderrors.Is(err, errors.ErrNilHorizontalPodAutoscaler) {
		t.Errorf("expected ErrNilHorizontalPodAutoscaler, got %v", err)
	}

	err = validator.ValidateHorizontalPodAutoscaler(&autoscalingv2.HorizontalPodAutoscaler{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateCertificate(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateCertificate(nil)
	if !stderrors.Is(err, errors.ErrNilCertificate) {
		t.Errorf("expected ErrNilCertificate, got %v", err)
	}
}

func TestValidateIssuer(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateIssuer(nil)
	if !stderrors.Is(err, errors.ErrNilIssuer) {
		t.Errorf("expected ErrNilIssuer, got %v", err)
	}
}

func TestValidateClusterIssuer(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateClusterIssuer(nil)
	if !stderrors.Is(err, errors.ErrNilClusterIssuer) {
		t.Errorf("expected ErrNilClusterIssuer, got %v", err)
	}
}

func TestValidateACMEIssuer(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateACMEIssuer(nil)
	if !stderrors.Is(err, errors.ErrNilACMEIssuer) {
		t.Errorf("expected ErrNilACMEIssuer, got %v", err)
	}
}

func TestValidateSecretStore(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateSecretStore(nil)
	if !stderrors.Is(err, errors.ErrNilSecretStore) {
		t.Errorf("expected ErrNilSecretStore, got %v", err)
	}
}

func TestValidateClusterSecretStore(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateClusterSecretStore(nil)
	if !stderrors.Is(err, errors.ErrNilClusterSecretStore) {
		t.Errorf("expected ErrNilClusterSecretStore, got %v", err)
	}
}

func TestValidateExternalSecret(t *testing.T) {
	validator := NewValidator()

	err := validator.ValidateExternalSecret(nil)
	if !stderrors.Is(err, errors.ErrNilExternalSecret) {
		t.Errorf("expected ErrNilExternalSecret, got %v", err)
	}
}

func TestValidatorMethods_Integration(t *testing.T) {
	validator := NewValidator()

	// Test a complete validation workflow
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "nginx:latest",
						},
					},
				},
			},
		},
	}

	podSpec := &deployment.Spec.Template.Spec
	container := &deployment.Spec.Template.Spec.Containers[0]

	// Test individual validations
	if err := validator.ValidateDeployment(deployment); err != nil {
		t.Errorf("deployment validation failed: %v", err)
	}

	if err := validator.ValidatePodSpec(podSpec); err != nil {
		t.Errorf("pod spec validation failed: %v", err)
	}

	if err := validator.ValidateContainer(container); err != nil {
		t.Errorf("container validation failed: %v", err)
	}

	// Test compound validation
	if err := validator.ValidateDeploymentWithSpec(deployment, podSpec); err != nil {
		t.Errorf("compound deployment validation failed: %v", err)
	}

	if err := validator.ValidatePodSpecWithContainer(podSpec, container); err != nil {
		t.Errorf("compound pod spec validation failed: %v", err)
	}
}
