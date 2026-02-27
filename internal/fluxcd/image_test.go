package fluxcd

import (
	"reflect"
	"testing"
	"time"

	imagev1 "github.com/fluxcd/image-automation-controller/api/v1beta2"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateImageUpdateAutomation(t *testing.T) {
	spec := imagev1.ImageUpdateAutomationSpec{
		SourceRef: CreateCrossNamespaceSourceReference("", "GitRepository", "repo", "default"),
		Interval:  metav1.Duration{Duration: time.Minute},
	}
	got := CreateImageUpdateAutomation("auto", "default", spec)

	if got.TypeMeta.Kind != "ImageUpdateAutomation" {
		t.Fatalf("unexpected kind %s", got.TypeMeta.Kind)
	}
	if got.TypeMeta.APIVersion != imagev1.GroupVersion.String() {
		t.Errorf("unexpected apiVersion %s", got.TypeMeta.APIVersion)
	}
	if got.Name != "auto" || got.Namespace != "default" {
		t.Errorf("metadata mismatch")
	}
	if !reflect.DeepEqual(got.Spec, spec) {
		t.Errorf("spec mismatch: %#v != %#v", got.Spec, spec)
	}
}

func TestAutomationHelperFunctions(t *testing.T) {
	auto := CreateImageUpdateAutomation("auto", "ns", imagev1.ImageUpdateAutomationSpec{})
	ref := CreateCrossNamespaceSourceReference("", "GitRepository", "repo", "ns")
	SetImageUpdateAutomationSourceRef(auto, ref)
	if auto.Spec.SourceRef.Name != "repo" {
		t.Fatalf("source ref not set")
	}

	interval := metav1.Duration{Duration: 2 * time.Minute}
	SetImageUpdateAutomationInterval(auto, interval)
	if auto.Spec.Interval != interval {
		t.Errorf("interval not set")
	}

	SetImageUpdateAutomationSuspend(auto, true)
	if !auto.Spec.Suspend {
		t.Errorf("suspend not set")
	}

	sel := &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}}
	SetImageUpdateAutomationPolicySelector(auto, sel)
	if !reflect.DeepEqual(auto.Spec.PolicySelector, sel) {
		t.Errorf("selector not set")
	}

	strategy := CreateUpdateStrategy(imagev1.UpdateStrategySetters, "./manifests")
	SetImageUpdateAutomationUpdateStrategy(auto, strategy)
	if auto.Spec.Update == nil || auto.Spec.Update.Path != "./manifests" {
		t.Errorf("update strategy not set")
	}

	AddObservedPolicy(auto, "pol", CreateImageRef("img", "tag", ""))
	if auto.Status.ObservedPolicies["pol"].Name != "img" {
		t.Errorf("observed policy not added")
	}
}

func TestGitSpecHelperFunctions(t *testing.T) {
	author := CreateCommitUser("Bot", "bot@example.com")
	commit := CreateCommitSpec(author)
	AddCommitMessageTemplateValue(&commit, "key", "val")
	sign := CreateSigningKey("sec")
	SetCommitSigningKey(&commit, sign)

	checkout := CreateGitCheckoutSpec(sourcev1.GitRepositoryRef{Branch: "main"})
	push := CreatePushSpec("feature", "", nil)
	AddPushOption(push, "--force", "true")

	gitSpec := CreateGitSpec(commit, checkout, push)
	if gitSpec.Commit.Author.Email != "bot@example.com" {
		t.Errorf("commit author not set")
	}
	if gitSpec.Checkout.Reference.Branch != "main" {
		t.Errorf("checkout not set")
	}
	if gitSpec.Push.Options["--force"] != "true" {
		t.Errorf("push option not added")
	}
}

func TestUpdateStrategyHelpers(t *testing.T) {
	us := CreateUpdateStrategy(imagev1.UpdateStrategySetters, "")
	SetUpdateStrategyPath(us, "./path")
	SetUpdateStrategyName(us, imagev1.UpdateStrategySetters)
	if us.Path != "./path" || us.Strategy != imagev1.UpdateStrategySetters {
		t.Errorf("update strategy helpers failed")
	}
}
