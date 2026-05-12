package fluxcd

import (
	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	imagev1 "github.com/fluxcd/image-automation-controller/api/v1"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	notificationv1 "github.com/fluxcd/notification-controller/api/v1"
	notificationv1beta3 "github.com/fluxcd/notification-controller/api/v1beta3"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	sourceWatcherv1beta1 "github.com/fluxcd/source-watcher/api/v2/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateGitRepository returns a new GitRepository with TypeMeta and ObjectMeta set.
func CreateGitRepository(name, namespace string) *sourcev1.GitRepository {
	return &sourcev1.GitRepository{
		TypeMeta: metav1.TypeMeta{
			Kind:       "GitRepository",
			APIVersion: sourcev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateHelmRepository returns a new HelmRepository with TypeMeta and ObjectMeta set.
func CreateHelmRepository(name, namespace string) *sourcev1.HelmRepository {
	return &sourcev1.HelmRepository{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HelmRepository",
			APIVersion: sourcev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateOCIRepository returns a new OCIRepository with TypeMeta and ObjectMeta set.
func CreateOCIRepository(name, namespace string) *sourcev1.OCIRepository {
	return &sourcev1.OCIRepository{
		TypeMeta: metav1.TypeMeta{
			Kind:       "OCIRepository",
			APIVersion: sourcev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateBucket returns a new Bucket with TypeMeta and ObjectMeta set.
func CreateBucket(name, namespace string) *sourcev1.Bucket {
	return &sourcev1.Bucket{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Bucket",
			APIVersion: sourcev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateHelmChart returns a new HelmChart with TypeMeta and ObjectMeta set.
func CreateHelmChart(name, namespace string) *sourcev1.HelmChart {
	return &sourcev1.HelmChart{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HelmChart",
			APIVersion: sourcev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateKustomization returns a new Kustomization with TypeMeta and ObjectMeta set.
func CreateKustomization(name, namespace string) *kustv1.Kustomization {
	return &kustv1.Kustomization{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Kustomization",
			APIVersion: kustv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateHelmRelease returns a new HelmRelease with TypeMeta and ObjectMeta set.
func CreateHelmRelease(name, namespace string) *helmv2.HelmRelease {
	return &helmv2.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HelmRelease",
			APIVersion: helmv2.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateProvider returns a new notification Provider with TypeMeta and ObjectMeta set.
func CreateProvider(name, namespace string) *notificationv1beta3.Provider {
	return &notificationv1beta3.Provider{
		TypeMeta: metav1.TypeMeta{
			Kind:       notificationv1beta3.ProviderKind,
			APIVersion: notificationv1beta3.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateAlert returns a new Alert with TypeMeta and ObjectMeta set.
func CreateAlert(name, namespace string) *notificationv1beta3.Alert {
	return &notificationv1beta3.Alert{
		TypeMeta: metav1.TypeMeta{
			Kind:       notificationv1beta3.AlertKind,
			APIVersion: notificationv1beta3.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateReceiver returns a new Receiver with TypeMeta and ObjectMeta set.
func CreateReceiver(name, namespace string) *notificationv1.Receiver {
	return &notificationv1.Receiver{
		TypeMeta: metav1.TypeMeta{
			Kind:       notificationv1.ReceiverKind,
			APIVersion: notificationv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateImageUpdateAutomation returns a new ImageUpdateAutomation with TypeMeta and ObjectMeta set.
func CreateImageUpdateAutomation(name, namespace string) *imagev1.ImageUpdateAutomation {
	return &imagev1.ImageUpdateAutomation{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ImageUpdateAutomation",
			APIVersion: imagev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateResourceSet returns a new ResourceSet with TypeMeta and ObjectMeta set.
func CreateResourceSet(name, namespace string) *fluxv1.ResourceSet {
	return &fluxv1.ResourceSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       fluxv1.ResourceSetKind,
			APIVersion: fluxv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateResourceSetInputProvider returns a new ResourceSetInputProvider with TypeMeta and ObjectMeta set.
func CreateResourceSetInputProvider(name, namespace string) *fluxv1.ResourceSetInputProvider {
	return &fluxv1.ResourceSetInputProvider{
		TypeMeta: metav1.TypeMeta{
			Kind:       fluxv1.ResourceSetInputProviderKind,
			APIVersion: fluxv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateFluxInstance returns a new FluxInstance with TypeMeta and ObjectMeta set.
func CreateFluxInstance(name, namespace string) *fluxv1.FluxInstance {
	return &fluxv1.FluxInstance{
		TypeMeta: metav1.TypeMeta{
			Kind:       fluxv1.FluxInstanceKind,
			APIVersion: fluxv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateFluxReport returns a new FluxReport with TypeMeta and ObjectMeta set.
func CreateFluxReport(name, namespace string) *fluxv1.FluxReport {
	return &fluxv1.FluxReport{
		TypeMeta: metav1.TypeMeta{
			Kind:       fluxv1.FluxReportKind,
			APIVersion: fluxv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateExternalArtifact returns a new ExternalArtifact with TypeMeta and ObjectMeta set.
func CreateExternalArtifact(name, namespace string) *sourcev1.ExternalArtifact {
	return &sourcev1.ExternalArtifact{
		TypeMeta: metav1.TypeMeta{
			Kind:       sourcev1.ExternalArtifactKind,
			APIVersion: sourcev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateArtifactGenerator returns a new ArtifactGenerator with TypeMeta and ObjectMeta set.
func CreateArtifactGenerator(name, namespace string) *sourceWatcherv1beta1.ArtifactGenerator {
	return &sourceWatcherv1beta1.ArtifactGenerator{
		TypeMeta: metav1.TypeMeta{
			Kind:       sourceWatcherv1beta1.ArtifactGeneratorKind,
			APIVersion: sourceWatcherv1beta1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}
