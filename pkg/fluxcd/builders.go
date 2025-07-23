package fluxcd

import internal "github.com/go-kure/kure/internal/fluxcd"

var (
	// NewKustomization returns a Kustomization object.
	NewKustomization = internal.CreateKustomization

	// NewGitRepository returns a GitRepository object.
	NewGitRepository  = internal.CreateGitRepository
	NewHelmRepository = internal.CreateHelmRepository
	NewOCIRepository  = internal.CreateOCIRepository
	NewBucket         = internal.CreateBucket
	NewHelmChart      = internal.CreateHelmChart

	// NewHelmRelease returns a HelmRelease object.
	NewHelmRelease = internal.CreateHelmRelease

	// NewImageUpdateAutomation returns an ImageUpdateAutomation object.
	NewImageUpdateAutomation = internal.CreateImageUpdateAutomation

	// NewCrossNamespaceSourceReference creates a cross-namespace source reference.
	NewCrossNamespaceSourceReference = internal.CreateCrossNamespaceSourceReference
	NewGitCheckoutSpec               = internal.CreateGitCheckoutSpec
	NewCommitUser                    = internal.CreateCommitUser
	NewSigningKey                    = internal.CreateSigningKey
	NewCommitSpec                    = internal.CreateCommitSpec
	NewPushSpec                      = internal.CreatePushSpec
	NewGitSpec                       = internal.CreateGitSpec
	NewUpdateStrategy                = internal.CreateUpdateStrategy
	NewImageRef                      = internal.CreateImageRef

	// NewPostBuild constructs an empty PostBuild.
	NewPostBuild           = internal.CreatePostBuild
	NewSubstituteReference = internal.CreateSubstituteReference
	NewDecryption          = internal.CreateDecryption
	NewCommonMetadata      = internal.CreateCommonMetadata

	// Notification objects
	NewProvider = internal.CreateProvider
	NewAlert    = internal.CreateAlert
	NewReceiver = internal.CreateReceiver

	// Other Flux resources
	NewResourceSet              = internal.CreateResourceSet
	NewResourceSetInputProvider = internal.CreateResourceSetInputProvider
	NewSchedule                 = internal.CreateSchedule
	NewFluxInstance             = internal.CreateFluxInstance
	NewFluxReport               = internal.CreateFluxReport
)
