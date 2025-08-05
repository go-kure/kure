package validation

import (
	"reflect"

	"github.com/go-kure/kure/pkg/errors"
	
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/kustomize/api/types"
	
	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
)

// Validator provides centralized validation for Kubernetes resources and their components.
type Validator struct{}

// NewValidator creates a new validator instance.
func NewValidator() *Validator {
	return &Validator{}
}

// validateNotNil is a generic helper for nil validation
func (v *Validator) validateNotNil(obj interface{}, errType error) error {
	if obj == nil || (reflect.ValueOf(obj).Kind() == reflect.Ptr && reflect.ValueOf(obj).IsNil()) {
		return errType
	}
	return nil
}

// Core Kubernetes Resources Validation

func (v *Validator) ValidatePod(pod *corev1.Pod) error {
	return v.validateNotNil(pod, errors.ErrNilPod)
}

func (v *Validator) ValidateDeployment(deployment *appsv1.Deployment) error {
	return v.validateNotNil(deployment, errors.ErrNilDeployment)
}

func (v *Validator) ValidateService(service *corev1.Service) error {
	return v.validateNotNil(service, errors.ErrNilService)
}

func (v *Validator) ValidateConfigMap(cm *corev1.ConfigMap) error {
	return v.validateNotNil(cm, errors.ErrNilConfigMap)
}

func (v *Validator) ValidateSecret(secret *corev1.Secret) error {
	return v.validateNotNil(secret, errors.ErrNilSecret)
}

func (v *Validator) ValidateServiceAccount(sa *corev1.ServiceAccount) error {
	return v.validateNotNil(sa, errors.ErrNilServiceAccount)
}

func (v *Validator) ValidateNamespace(ns *corev1.Namespace) error {
	return v.validateNotNil(ns, errors.ErrNilNamespace)
}

func (v *Validator) ValidateIngress(ingress *networkingv1.Ingress) error {
	return v.validateNotNil(ingress, errors.ErrNilIngress)
}

func (v *Validator) ValidateStatefulSet(sts *appsv1.StatefulSet) error {
	return v.validateNotNil(sts, errors.ErrNilStatefulSet)
}

func (v *Validator) ValidateDaemonSet(ds *appsv1.DaemonSet) error {
	return v.validateNotNil(ds, errors.ErrNilDaemonSet)
}

func (v *Validator) ValidateJob(job *batchv1.Job) error {
	return v.validateNotNil(job, errors.ErrNilJob)
}

func (v *Validator) ValidateCronJob(cronJob *batchv1.CronJob) error {
	return v.validateNotNil(cronJob, errors.ErrNilCronJob)
}

// RBAC Resources Validation

func (v *Validator) ValidateRole(role *rbacv1.Role) error {
	return v.validateNotNil(role, errors.ErrNilRole)
}

func (v *Validator) ValidateClusterRole(clusterRole *rbacv1.ClusterRole) error {
	return v.validateNotNil(clusterRole, errors.ErrNilClusterRole)
}

func (v *Validator) ValidateRoleBinding(roleBinding *rbacv1.RoleBinding) error {
	return v.validateNotNil(roleBinding, errors.ErrNilRoleBinding)
}

func (v *Validator) ValidateClusterRoleBinding(clusterRoleBinding *rbacv1.ClusterRoleBinding) error {
	return v.validateNotNil(clusterRoleBinding, errors.ErrNilClusterRoleBinding)
}

// Component Validation (specs, containers, etc.)

func (v *Validator) ValidatePodSpec(spec *corev1.PodSpec) error {
	return v.validateNotNil(spec, errors.ErrNilPodSpec)
}

func (v *Validator) ValidateContainer(container *corev1.Container) error {
	return v.validateNotNil(container, errors.ErrNilContainer)
}

func (v *Validator) ValidateInitContainer(container *corev1.Container) error {
	return v.validateNotNil(container, errors.ErrNilInitContainer)
}

func (v *Validator) ValidateEphemeralContainer(container *corev1.EphemeralContainer) error {
	return v.validateNotNil(container, errors.ErrNilEphemeralContainer)
}

func (v *Validator) ValidateVolume(volume *corev1.Volume) error {
	return v.validateNotNil(volume, errors.ErrNilVolume)
}

func (v *Validator) ValidateImagePullSecret(secret *corev1.LocalObjectReference) error {
	return v.validateNotNil(secret, errors.ErrNilImagePullSecret)
}

func (v *Validator) ValidateToleration(toleration *corev1.Toleration) error {
	return v.validateNotNil(toleration, errors.ErrNilToleration)
}

func (v *Validator) ValidateServicePort(port *corev1.ServicePort) error {
	return v.validateNotNil(port, errors.ErrNilServicePort)
}

func (v *Validator) ValidatePodDisruptionBudget(pdb *policyv1.PodDisruptionBudget) error {
	return v.validateNotNil(pdb, errors.ErrNilPodDisruptionBudget)
}

// Other Resources Validation

func (v *Validator) ValidateKustomization(k *types.Kustomization) error {
	return v.validateNotNil(k, errors.ErrNilKustomization)
}

// Flux Resources Validation

func (v *Validator) ValidateFluxInstance(fi *fluxv1.FluxInstance) error {
	return v.validateNotNil(fi, errors.ErrNilFluxInstance)
}

// MetalLB Resources Validation

func (v *Validator) ValidateIPAddressPool(pool *metallbv1beta1.IPAddressPool) error {
	return v.validateNotNil(pool, errors.ErrNilIPAddressPool)
}

func (v *Validator) ValidateBGPPeer(peer *metallbv1beta1.BGPPeer) error {
	return v.validateNotNil(peer, errors.ErrNilBGPPeer)
}

func (v *Validator) ValidateBGPAdvertisement(adv *metallbv1beta1.BGPAdvertisement) error {
	return v.validateNotNil(adv, errors.ErrNilBGPAdvertisement)
}

func (v *Validator) ValidateL2Advertisement(adv *metallbv1beta1.L2Advertisement) error {
	return v.validateNotNil(adv, errors.ErrNilL2Advertisement)
}

func (v *Validator) ValidateBFDProfile(profile *metallbv1beta1.BFDProfile) error {
	return v.validateNotNil(profile, errors.ErrNilBFDProfile)
}

// Compound validation methods for common patterns

// ValidateDeploymentWithSpec validates both deployment and its core spec components
func (v *Validator) ValidateDeploymentWithSpec(deployment *appsv1.Deployment, spec *corev1.PodSpec) error {
	if err := v.ValidateDeployment(deployment); err != nil {
		return err
	}
	if spec != nil {
		return v.ValidatePodSpec(spec)
	}
	return nil
}

// ValidatePodSpecWithContainer validates PodSpec and Container together
func (v *Validator) ValidatePodSpecWithContainer(spec *corev1.PodSpec, container *corev1.Container) error {
	if err := v.ValidatePodSpec(spec); err != nil {
		return err
	}
	return v.ValidateContainer(container)
}

// ValidateServiceWithPort validates Service and ServicePort together
func (v *Validator) ValidateServiceWithPort(service *corev1.Service, port *corev1.ServicePort) error {
	if err := v.ValidateService(service); err != nil {
		return err
	}
	return v.ValidateServicePort(port)
}