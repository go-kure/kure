package fluxcd

import (
	"testing"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/fluxcd/pkg/apis/kustomize"
	"github.com/fluxcd/pkg/apis/meta"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateHelmRelease(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		spec      helmv2.HelmReleaseSpec
		expected  *helmv2.HelmRelease
	}{
		{
			name:      "ValidInput",
			namespace: "default",
			spec: helmv2.HelmReleaseSpec{
				Chart: &helmv2.HelmChartTemplate{
					ObjectMeta: &helmv2.HelmChartTemplateObjectMeta{
						Labels:      nil,
						Annotations: nil,
					},
					Spec: helmv2.HelmChartTemplateSpec{
						Chart:   "example-chart",
						Version: "1.0.0",
					},
				},
			},
			expected: &helmv2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HelmRelease",
					APIVersion: helmv2.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ValidInput",
					Namespace: "default",
				},
				Spec: helmv2.HelmReleaseSpec{
					Chart: &helmv2.HelmChartTemplate{
						Spec: helmv2.HelmChartTemplateSpec{
							Chart:   "example-chart",
							Version: "1.0.0",
						},
					},
				},
			},
		},
		{
			name:      "CustomNamespace",
			namespace: "custom-namespace",
			spec: helmv2.HelmReleaseSpec{
				Chart: &helmv2.HelmChartTemplate{
					Spec: helmv2.HelmChartTemplateSpec{
						Chart:   "custom-chart",
						Version: "2.1.0",
					},
				},
			},
			expected: &helmv2.HelmRelease{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HelmRelease",
					APIVersion: helmv2.GroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "CustomNamespace",
					Namespace: "custom-namespace",
				},
				Spec: helmv2.HelmReleaseSpec{
					Chart: &helmv2.HelmChartTemplate{
						Spec: helmv2.HelmChartTemplateSpec{
							Chart:   "custom-chart",
							Version: "2.1.0",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CreateHelmRelease(tt.name, tt.namespace, tt.spec)
			if result.TypeMeta != tt.expected.TypeMeta {
				t.Errorf("TypeMeta mismatch: got %v, want %v", result.TypeMeta, tt.expected.TypeMeta)
			}
			if result.ObjectMeta.Name != tt.expected.ObjectMeta.Name {
				t.Errorf("Name mismatch: got %v, want %v", result.ObjectMeta.Name, tt.expected.ObjectMeta.Name)
			}
			if result.ObjectMeta.Namespace != tt.expected.ObjectMeta.Namespace {
				t.Errorf("Namespace mismatch: got %v, want %v", result.ObjectMeta.Namespace, tt.expected.ObjectMeta.Namespace)
			}
			if result.Spec.Chart.Spec.Chart != tt.expected.Spec.Chart.Spec.Chart ||
				result.Spec.Chart.Spec.Version != tt.expected.Spec.Chart.Spec.Version {
				t.Errorf("Spec mismatch: got %+v, want %+v", result.Spec, tt.expected.Spec)
			}
		})
	}
}

func TestHelmReleaseHelpers(t *testing.T) {
	hr := CreateHelmRelease("demo", "ns", helmv2.HelmReleaseSpec{})
	AddHelmReleaseLabel(hr, "app", "demo")
	AddHelmReleaseAnnotation(hr, "team", "dev")
	SetHelmReleaseReleaseName(hr, "demo")
	SetHelmReleaseTargetNamespace(hr, "target")
	SetHelmReleaseStorageNamespace(hr, "storage")
	SetHelmReleaseInterval(hr, metav1.Duration{Duration: time.Minute})
	SetHelmReleaseTimeout(hr, metav1.Duration{Duration: time.Minute})
	SetHelmReleaseMaxHistory(hr, 2)
	SetHelmReleaseServiceAccountName(hr, "sa")
	SetHelmReleasePersistentClient(hr, true)
	SetHelmReleaseSuspend(hr, true)
	SetHelmReleaseKubeConfig(hr, &meta.KubeConfigReference{SecretRef: &meta.SecretKeyReference{Name: "k"}})
	AddHelmReleaseDependsOn(hr, helmv2.DependencyReference{Name: "dep"})
	SetHelmReleaseValues(hr, &apiextensionsv1.JSON{Raw: []byte("{}")})
	AddHelmReleaseValuesFrom(hr, helmv2.ValuesReference{Kind: "ConfigMap", Name: "vals"})
	AddHelmReleasePostRenderer(hr, helmv2.PostRenderer{})

	if hr.Labels["app"] != "demo" {
		t.Errorf("label not set")
	}
	if hr.Annotations["team"] != "dev" {
		t.Errorf("annotation not set")
	}
	if hr.Spec.ReleaseName != "demo" {
		t.Errorf("release name not set")
	}
	if hr.Spec.TargetNamespace != "target" {
		t.Errorf("target namespace not set")
	}
	if hr.Spec.StorageNamespace != "storage" {
		t.Errorf("storage namespace not set")
	}
	if hr.Spec.Interval.Duration != time.Minute {
		t.Errorf("interval not set")
	}
	if hr.Spec.Timeout == nil || hr.Spec.Timeout.Duration != time.Minute {
		t.Errorf("timeout not set")
	}
	if hr.Spec.MaxHistory == nil || *hr.Spec.MaxHistory != 2 {
		t.Errorf("maxHistory not set")
	}
	if hr.Spec.ServiceAccountName != "sa" {
		t.Errorf("service account not set")
	}
	if hr.Spec.PersistentClient == nil || !*hr.Spec.PersistentClient {
		t.Errorf("persistent client not set")
	}
	if !hr.Spec.Suspend {
		t.Errorf("suspend not set")
	}
	if hr.Spec.KubeConfig == nil || hr.Spec.KubeConfig.SecretRef.Name != "k" {
		t.Errorf("kubeconfig not set")
	}
	if len(hr.Spec.DependsOn) != 1 || hr.Spec.DependsOn[0].Name != "dep" {
		t.Errorf("dependsOn not added")
	}
	if hr.Spec.Values == nil {
		t.Errorf("values not set")
	}
	if len(hr.Spec.ValuesFrom) != 1 || hr.Spec.ValuesFrom[0].Name != "vals" {
		t.Errorf("valuesFrom not added")
	}
	if len(hr.Spec.PostRenderers) != 1 {
		t.Errorf("postRenderer not added")
	}
}

func TestCreatePostRendererKustomize(t *testing.T) {
	k := CreatePostRendererKustomize()
	if k == nil {
		t.Fatal("expected non-nil Kustomize")
	}
}

func TestAddPostRendererKustomizePatch(t *testing.T) {
	k := CreatePostRendererKustomize()
	patch1 := kustomize.Patch{Patch: `{"op":"add","path":"/metadata/labels/env","value":"test"}`}
	patch2 := kustomize.Patch{
		Patch: "- op: replace\n  path: /spec/replicas\n  value: 3",
		Target: &kustomize.Selector{
			Kind: "Deployment",
			Name: "my-app",
		},
	}
	AddPostRendererKustomizePatch(k, patch1)
	AddPostRendererKustomizePatch(k, patch2)

	if len(k.Patches) != 2 {
		t.Fatalf("expected 2 patches, got %d", len(k.Patches))
	}
	if k.Patches[0].Patch != patch1.Patch {
		t.Errorf("first patch content mismatch")
	}
	if k.Patches[1].Target == nil || k.Patches[1].Target.Kind != "Deployment" {
		t.Errorf("second patch target mismatch")
	}
}

func TestAddPostRendererKustomizeImage(t *testing.T) {
	k := CreatePostRendererKustomize()
	img1 := kustomize.Image{Name: "nginx", NewName: "my-registry/nginx", NewTag: "1.25"}
	img2 := kustomize.Image{Name: "redis", Digest: "sha256:abc123"}
	AddPostRendererKustomizeImage(k, img1)
	AddPostRendererKustomizeImage(k, img2)

	if len(k.Images) != 2 {
		t.Fatalf("expected 2 images, got %d", len(k.Images))
	}
	if k.Images[0].NewName != "my-registry/nginx" {
		t.Errorf("first image NewName mismatch")
	}
	if k.Images[0].NewTag != "1.25" {
		t.Errorf("first image NewTag mismatch")
	}
	if k.Images[1].Digest != "sha256:abc123" {
		t.Errorf("second image Digest mismatch")
	}
}

func TestHelmReleasePostRendererIntegration(t *testing.T) {
	hr := CreateHelmRelease("my-release", "default", helmv2.HelmReleaseSpec{})

	k := CreatePostRendererKustomize()
	AddPostRendererKustomizePatch(k, kustomize.Patch{
		Patch: `{"op":"add","path":"/metadata/labels/env","value":"prod"}`,
	})
	AddPostRendererKustomizeImage(k, kustomize.Image{
		Name:    "nginx",
		NewName: "my-registry/nginx",
		NewTag:  "stable",
	})

	AddHelmReleasePostRenderer(hr, helmv2.PostRenderer{Kustomize: k})

	if len(hr.Spec.PostRenderers) != 1 {
		t.Fatalf("expected 1 post renderer, got %d", len(hr.Spec.PostRenderers))
	}
	pr := hr.Spec.PostRenderers[0]
	if pr.Kustomize == nil {
		t.Fatal("expected Kustomize post renderer to be set")
	}
	if len(pr.Kustomize.Patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(pr.Kustomize.Patches))
	}
	if len(pr.Kustomize.Images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(pr.Kustomize.Images))
	}
	if pr.Kustomize.Images[0].NewTag != "stable" {
		t.Errorf("image NewTag mismatch: got %s, want stable", pr.Kustomize.Images[0].NewTag)
	}
}

func TestCreateDriftDetection(t *testing.T) {
	modes := []helmv2.DriftDetectionMode{
		helmv2.DriftDetectionEnabled,
		helmv2.DriftDetectionWarn,
		helmv2.DriftDetectionDisabled,
	}
	for _, mode := range modes {
		t.Run(string(mode), func(t *testing.T) {
			dd := CreateDriftDetection(mode)
			if dd == nil {
				t.Fatal("expected non-nil DriftDetection")
			}
			if dd.Mode != mode {
				t.Errorf("mode mismatch: got %s, want %s", dd.Mode, mode)
			}
		})
	}
}

func TestAddDriftDetectionIgnoreRule(t *testing.T) {
	dd := CreateDriftDetection(helmv2.DriftDetectionEnabled)

	rule1 := CreateIgnoreRule([]string{"/spec/replicas"}, nil)
	AddDriftDetectionIgnoreRule(dd, rule1)
	if len(dd.Ignore) != 1 {
		t.Fatalf("expected 1 ignore rule, got %d", len(dd.Ignore))
	}

	rule2 := CreateIgnoreRule(
		[]string{"/metadata/annotations", "/metadata/labels"},
		&kustomize.Selector{Kind: "Deployment", Name: "my-app"},
	)
	AddDriftDetectionIgnoreRule(dd, rule2)
	if len(dd.Ignore) != 2 {
		t.Fatalf("expected 2 ignore rules, got %d", len(dd.Ignore))
	}
	if dd.Ignore[1].Target == nil || dd.Ignore[1].Target.Kind != "Deployment" {
		t.Errorf("second rule target mismatch")
	}
	if len(dd.Ignore[1].Paths) != 2 {
		t.Errorf("expected 2 paths in second rule, got %d", len(dd.Ignore[1].Paths))
	}
}

func TestCreateIgnoreRule(t *testing.T) {
	t.Run("without target", func(t *testing.T) {
		rule := CreateIgnoreRule([]string{"/spec/replicas"}, nil)
		if len(rule.Paths) != 1 || rule.Paths[0] != "/spec/replicas" {
			t.Errorf("paths mismatch: got %v", rule.Paths)
		}
		if rule.Target != nil {
			t.Errorf("expected nil target")
		}
	})
	t.Run("with target", func(t *testing.T) {
		target := &kustomize.Selector{Kind: "ConfigMap", Name: "my-config"}
		rule := CreateIgnoreRule([]string{"/data"}, target)
		if rule.Target == nil {
			t.Fatal("expected non-nil target")
		}
		if rule.Target.Kind != "ConfigMap" {
			t.Errorf("target kind mismatch: got %s", rule.Target.Kind)
		}
		if rule.Target.Name != "my-config" {
			t.Errorf("target name mismatch: got %s", rule.Target.Name)
		}
	})
}

func TestHelmReleaseDriftDetectionIntegration(t *testing.T) {
	hr := CreateHelmRelease("my-release", "default", helmv2.HelmReleaseSpec{})

	dd := CreateDriftDetection(helmv2.DriftDetectionWarn)

	AddDriftDetectionIgnoreRule(dd, CreateIgnoreRule([]string{"/spec/replicas"}, nil))
	AddDriftDetectionIgnoreRule(dd, CreateIgnoreRule(
		[]string{"/metadata/annotations", "/metadata/labels"},
		&kustomize.Selector{Kind: "Deployment", Name: "my-app"},
	))

	SetHelmReleaseDriftDetection(hr, dd)

	if hr.Spec.DriftDetection == nil {
		t.Fatal("expected DriftDetection to be set")
	}
	if hr.Spec.DriftDetection.Mode != helmv2.DriftDetectionWarn {
		t.Errorf("mode mismatch: got %s, want warn", hr.Spec.DriftDetection.Mode)
	}
	if len(hr.Spec.DriftDetection.Ignore) != 2 {
		t.Fatalf("expected 2 ignore rules, got %d", len(hr.Spec.DriftDetection.Ignore))
	}
	if hr.Spec.DriftDetection.Ignore[0].Target != nil {
		t.Errorf("first rule should have nil target")
	}
	if hr.Spec.DriftDetection.Ignore[0].Paths[0] != "/spec/replicas" {
		t.Errorf("first rule path mismatch")
	}
	if hr.Spec.DriftDetection.Ignore[1].Target == nil {
		t.Fatal("second rule should have a target")
	}
	if hr.Spec.DriftDetection.Ignore[1].Target.Kind != "Deployment" {
		t.Errorf("second rule target kind mismatch")
	}
}

func TestCreateInstallRemediation(t *testing.T) {
	tests := []struct {
		name    string
		retries int
	}{
		{"zero retries", 0},
		{"positive retries", 3},
		{"unlimited retries", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := CreateInstallRemediation(tt.retries)
			if r == nil {
				t.Fatal("expected non-nil InstallRemediation")
			}
			if r.Retries != tt.retries {
				t.Errorf("Retries = %d, want %d", r.Retries, tt.retries)
			}
		})
	}
}

func TestCreateUpgradeRemediation(t *testing.T) {
	tests := []struct {
		name    string
		retries int
	}{
		{"zero retries", 0},
		{"positive retries", 5},
		{"unlimited retries", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := CreateUpgradeRemediation(tt.retries)
			if r == nil {
				t.Fatal("expected non-nil UpgradeRemediation")
			}
			if r.Retries != tt.retries {
				t.Errorf("Retries = %d, want %d", r.Retries, tt.retries)
			}
		})
	}
}

func TestSetHelmReleaseInstallRemediation(t *testing.T) {
	hr := CreateHelmRelease("test", "default", helmv2.HelmReleaseSpec{})

	// Install is nil initially — should be created
	r := CreateInstallRemediation(3)
	SetInstallRemediationIgnoreTestFailures(r, true)
	SetInstallRemediationRemediateLastFailure(r, true)
	SetHelmReleaseInstallRemediation(hr, r)

	if hr.Spec.Install == nil {
		t.Fatal("expected Install to be created")
	}
	if hr.Spec.Install.Remediation == nil {
		t.Fatal("expected Install.Remediation to be set")
	}
	if hr.Spec.Install.Remediation.Retries != 3 {
		t.Errorf("Retries = %d, want 3", hr.Spec.Install.Remediation.Retries)
	}
	if hr.Spec.Install.Remediation.IgnoreTestFailures == nil || !*hr.Spec.Install.Remediation.IgnoreTestFailures {
		t.Error("IgnoreTestFailures should be true")
	}
	if hr.Spec.Install.Remediation.RemediateLastFailure == nil || !*hr.Spec.Install.Remediation.RemediateLastFailure {
		t.Error("RemediateLastFailure should be true")
	}
}

func TestSetHelmReleaseInstallRemediationExistingInstall(t *testing.T) {
	hr := CreateHelmRelease("test", "default", helmv2.HelmReleaseSpec{})
	hr.Spec.Install = &helmv2.Install{CreateNamespace: true}

	r := CreateInstallRemediation(2)
	SetHelmReleaseInstallRemediation(hr, r)

	// Existing Install config should be preserved
	if !hr.Spec.Install.CreateNamespace {
		t.Error("CreateNamespace should still be true")
	}
	if hr.Spec.Install.Remediation.Retries != 2 {
		t.Errorf("Retries = %d, want 2", hr.Spec.Install.Remediation.Retries)
	}
}

func TestSetHelmReleaseUpgradeRemediation(t *testing.T) {
	hr := CreateHelmRelease("test", "default", helmv2.HelmReleaseSpec{})

	r := CreateUpgradeRemediation(5)
	SetUpgradeRemediationIgnoreTestFailures(r, false)
	SetUpgradeRemediationRemediateLastFailure(r, true)
	SetUpgradeRemediationStrategy(r, helmv2.RollbackRemediationStrategy)
	SetHelmReleaseUpgradeRemediation(hr, r)

	if hr.Spec.Upgrade == nil {
		t.Fatal("expected Upgrade to be created")
	}
	if hr.Spec.Upgrade.Remediation == nil {
		t.Fatal("expected Upgrade.Remediation to be set")
	}
	if hr.Spec.Upgrade.Remediation.Retries != 5 {
		t.Errorf("Retries = %d, want 5", hr.Spec.Upgrade.Remediation.Retries)
	}
	if hr.Spec.Upgrade.Remediation.IgnoreTestFailures == nil || *hr.Spec.Upgrade.Remediation.IgnoreTestFailures {
		t.Error("IgnoreTestFailures should be false")
	}
	if hr.Spec.Upgrade.Remediation.RemediateLastFailure == nil || !*hr.Spec.Upgrade.Remediation.RemediateLastFailure {
		t.Error("RemediateLastFailure should be true")
	}
	if hr.Spec.Upgrade.Remediation.Strategy == nil || *hr.Spec.Upgrade.Remediation.Strategy != helmv2.RollbackRemediationStrategy {
		t.Errorf("Strategy = %v, want rollback", hr.Spec.Upgrade.Remediation.Strategy)
	}
}

func TestSetUpgradeRemediationStrategyUninstall(t *testing.T) {
	r := CreateUpgradeRemediation(1)
	SetUpgradeRemediationStrategy(r, helmv2.UninstallRemediationStrategy)

	if r.Strategy == nil {
		t.Fatal("expected Strategy to be set")
	}
	if *r.Strategy != helmv2.UninstallRemediationStrategy {
		t.Errorf("Strategy = %v, want uninstall", *r.Strategy)
	}
}

func TestSetHelmReleaseUpgradeRemediationExistingUpgrade(t *testing.T) {
	hr := CreateHelmRelease("test", "default", helmv2.HelmReleaseSpec{})
	hr.Spec.Upgrade = &helmv2.Upgrade{Force: true}

	r := CreateUpgradeRemediation(1)
	SetHelmReleaseUpgradeRemediation(hr, r)

	// Existing Upgrade config should be preserved
	if !hr.Spec.Upgrade.Force {
		t.Error("Force should still be true")
	}
	if hr.Spec.Upgrade.Remediation.Retries != 1 {
		t.Errorf("Retries = %d, want 1", hr.Spec.Upgrade.Remediation.Retries)
	}
}

func TestCreateWaitStrategy(t *testing.T) {
	tests := []struct {
		name     string
		strategy helmv2.WaitStrategyName
	}{
		{"poller", helmv2.WaitStrategyPoller},
		{"legacy", helmv2.WaitStrategyLegacy},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := CreateWaitStrategy(tt.strategy)
			if ws == nil {
				t.Fatal("expected non-nil WaitStrategy")
			}
			if ws.Name != tt.strategy {
				t.Errorf("Name = %q, want %q", ws.Name, tt.strategy)
			}
		})
	}
}

func TestSetHelmReleaseWaitStrategy(t *testing.T) {
	hr := CreateHelmRelease("test", "default", helmv2.HelmReleaseSpec{})

	ws := CreateWaitStrategy(helmv2.WaitStrategyPoller)
	SetHelmReleaseWaitStrategy(hr, ws)

	if hr.Spec.WaitStrategy == nil {
		t.Fatal("expected WaitStrategy to be set")
	}
	if hr.Spec.WaitStrategy.Name != helmv2.WaitStrategyPoller {
		t.Errorf("WaitStrategy.Name = %q, want %q", hr.Spec.WaitStrategy.Name, helmv2.WaitStrategyPoller)
	}
}

func TestRemediationIntegration(t *testing.T) {
	hr := CreateHelmRelease("my-app", "production", helmv2.HelmReleaseSpec{})

	// Configure install remediation
	installRemediation := CreateInstallRemediation(3)
	SetInstallRemediationRemediateLastFailure(installRemediation, true)
	SetHelmReleaseInstallRemediation(hr, installRemediation)

	// Configure upgrade remediation with rollback strategy
	upgradeRemediation := CreateUpgradeRemediation(5)
	SetUpgradeRemediationStrategy(upgradeRemediation, helmv2.RollbackRemediationStrategy)
	SetUpgradeRemediationRemediateLastFailure(upgradeRemediation, true)
	SetUpgradeRemediationIgnoreTestFailures(upgradeRemediation, true)
	SetHelmReleaseUpgradeRemediation(hr, upgradeRemediation)

	// Verify complete configuration
	if hr.Spec.Install.Remediation.Retries != 3 {
		t.Errorf("Install retries = %d, want 3", hr.Spec.Install.Remediation.Retries)
	}
	if hr.Spec.Upgrade.Remediation.Retries != 5 {
		t.Errorf("Upgrade retries = %d, want 5", hr.Spec.Upgrade.Remediation.Retries)
	}
	if *hr.Spec.Upgrade.Remediation.Strategy != helmv2.RollbackRemediationStrategy {
		t.Errorf("Upgrade strategy = %v, want rollback", *hr.Spec.Upgrade.Remediation.Strategy)
	}
}
