package fluxcd

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	imagev1 "github.com/fluxcd/image-automation-controller/api/v1beta2"
	meta "github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
)

// CreateImageUpdateAutomation returns a new ImageUpdateAutomation object.
func CreateImageUpdateAutomation(name, namespace string, spec imagev1.ImageUpdateAutomationSpec) *imagev1.ImageUpdateAutomation {
	obj := &imagev1.ImageUpdateAutomation{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ImageUpdateAutomation",
			APIVersion: imagev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	return obj
}

// SetImageUpdateAutomationSourceRef sets the source reference for the automation.
func SetImageUpdateAutomationSourceRef(auto *imagev1.ImageUpdateAutomation, ref imagev1.CrossNamespaceSourceReference) {
	auto.Spec.SourceRef = ref
}

// SetImageUpdateAutomationGitSpec sets the git specification for the automation.
func SetImageUpdateAutomationGitSpec(auto *imagev1.ImageUpdateAutomation, spec *imagev1.GitSpec) {
	auto.Spec.GitSpec = spec
}

// SetImageUpdateAutomationInterval sets the reconcile interval.
func SetImageUpdateAutomationInterval(auto *imagev1.ImageUpdateAutomation, interval metav1.Duration) {
	auto.Spec.Interval = interval
}

// SetImageUpdateAutomationPolicySelector sets the policy selector.
func SetImageUpdateAutomationPolicySelector(auto *imagev1.ImageUpdateAutomation, selector *metav1.LabelSelector) {
	auto.Spec.PolicySelector = selector
}

// SetImageUpdateAutomationUpdateStrategy sets the update strategy.
func SetImageUpdateAutomationUpdateStrategy(auto *imagev1.ImageUpdateAutomation, strategy *imagev1.UpdateStrategy) {
	auto.Spec.Update = strategy
}

// SetImageUpdateAutomationSuspend sets the suspend flag.
func SetImageUpdateAutomationSuspend(auto *imagev1.ImageUpdateAutomation, suspend bool) {
	auto.Spec.Suspend = suspend
}

// CreateCrossNamespaceSourceReference creates a new cross namespace source reference.
func CreateCrossNamespaceSourceReference(apiVersion, kind, name, namespace string) imagev1.CrossNamespaceSourceReference {
	return imagev1.CrossNamespaceSourceReference{
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       name,
		Namespace:  namespace,
	}
}

// CreateGitCheckoutSpec creates a new GitCheckoutSpec.
func CreateGitCheckoutSpec(ref sourcev1.GitRepositoryRef) *imagev1.GitCheckoutSpec {
	return &imagev1.GitCheckoutSpec{Reference: ref}
}

// SetGitCheckoutReference sets the reference of the checkout spec.
func SetGitCheckoutReference(spec *imagev1.GitCheckoutSpec, ref sourcev1.GitRepositoryRef) {
	spec.Reference = ref
}

// CreateCommitUser returns a CommitUser struct.
func CreateCommitUser(name, email string) imagev1.CommitUser {
	return imagev1.CommitUser{Name: name, Email: email}
}

// CreateSigningKey returns a SigningKey with the secret reference populated.
func CreateSigningKey(secretName string) *imagev1.SigningKey {
	return &imagev1.SigningKey{SecretRef: meta.LocalObjectReference{Name: secretName}}
}

// CreateCommitSpec creates a CommitSpec with the given author.
func CreateCommitSpec(author imagev1.CommitUser) imagev1.CommitSpec {
	return imagev1.CommitSpec{Author: author}
}

// SetCommitSigningKey sets the signing key for a CommitSpec.
func SetCommitSigningKey(spec *imagev1.CommitSpec, key *imagev1.SigningKey) {
	spec.SigningKey = key
}

// SetCommitMessageTemplate sets the message template for a CommitSpec.
func SetCommitMessageTemplate(spec *imagev1.CommitSpec, tpl string) {
	spec.MessageTemplate = tpl
}

// SetCommitMessageTemplateValues replaces the message template values map.
func SetCommitMessageTemplateValues(spec *imagev1.CommitSpec, values map[string]string) {
	spec.MessageTemplateValues = values
}

// AddCommitMessageTemplateValue adds a single key/value pair to the template values map.
func AddCommitMessageTemplateValue(spec *imagev1.CommitSpec, key, value string) {
	if spec.MessageTemplateValues == nil {
		spec.MessageTemplateValues = make(map[string]string)
	}
	spec.MessageTemplateValues[key] = value
}

// SetCommitAuthor sets the author of the commit spec.
func SetCommitAuthor(spec *imagev1.CommitSpec, author imagev1.CommitUser) {
	spec.Author = author
}

// CreatePushSpec returns a PushSpec.
func CreatePushSpec(branch, refspec string, options map[string]string) *imagev1.PushSpec {
	return &imagev1.PushSpec{Branch: branch, Refspec: refspec, Options: options}
}

// SetPushBranch sets the branch for the push spec.
func SetPushBranch(spec *imagev1.PushSpec, branch string) { spec.Branch = branch }

// SetPushRefspec sets the refspec for the push spec.
func SetPushRefspec(spec *imagev1.PushSpec, refspec string) { spec.Refspec = refspec }

// SetPushOptions replaces the options map for the push spec.
func SetPushOptions(spec *imagev1.PushSpec, opts map[string]string) { spec.Options = opts }

// AddPushOption adds a single option to the push spec.
func AddPushOption(spec *imagev1.PushSpec, key, value string) {
	if spec.Options == nil {
		spec.Options = make(map[string]string)
	}
	spec.Options[key] = value
}

// CreateGitSpec creates a GitSpec struct.
func CreateGitSpec(commit imagev1.CommitSpec, checkout *imagev1.GitCheckoutSpec, push *imagev1.PushSpec) *imagev1.GitSpec {
	return &imagev1.GitSpec{Checkout: checkout, Commit: commit, Push: push}
}

// SetGitSpecCheckout sets the checkout spec.
func SetGitSpecCheckout(spec *imagev1.GitSpec, checkout *imagev1.GitCheckoutSpec) {
	spec.Checkout = checkout
}

// SetGitSpecCommit sets the commit spec.
func SetGitSpecCommit(spec *imagev1.GitSpec, commit imagev1.CommitSpec) { spec.Commit = commit }

// SetGitSpecPush sets the push spec.
func SetGitSpecPush(spec *imagev1.GitSpec, push *imagev1.PushSpec) { spec.Push = push }

// CreateUpdateStrategy creates an UpdateStrategy struct.
func CreateUpdateStrategy(strategy imagev1.UpdateStrategyName, path string) *imagev1.UpdateStrategy {
	return &imagev1.UpdateStrategy{Strategy: strategy, Path: path}
}

// SetUpdateStrategyName sets the strategy name.
func SetUpdateStrategyName(spec *imagev1.UpdateStrategy, name imagev1.UpdateStrategyName) {
	spec.Strategy = name
}

// SetUpdateStrategyPath sets the update path.
func SetUpdateStrategyPath(spec *imagev1.UpdateStrategy, path string) { spec.Path = path }

// CreateImageRef constructs an ImageRef.
func CreateImageRef(name, tag, digest string) imagev1.ImageRef {
	return imagev1.ImageRef{Name: name, Tag: tag, Digest: digest}
}

// SetImageRefDigest sets the digest on an ImageRef.
func SetImageRefDigest(ref *imagev1.ImageRef, digest string) { ref.Digest = digest }

// SetImageRefTag sets the tag on an ImageRef.
func SetImageRefTag(ref *imagev1.ImageRef, tag string) { ref.Tag = tag }

// SetImageRefName sets the name on an ImageRef.
func SetImageRefName(ref *imagev1.ImageRef, name string) { ref.Name = name }

// AddObservedPolicy records an observed policy in the automation status.
func AddObservedPolicy(auto *imagev1.ImageUpdateAutomation, name string, ref imagev1.ImageRef) {
	if auto.Status.ObservedPolicies == nil {
		auto.Status.ObservedPolicies = make(imagev1.ObservedPolicies)
	}
	auto.Status.ObservedPolicies[name] = ref
}

// SetObservedPolicies sets the observed policies map.
func SetObservedPolicies(auto *imagev1.ImageUpdateAutomation, policies imagev1.ObservedPolicies) {
	auto.Status.ObservedPolicies = policies
}
