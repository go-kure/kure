package layout

import (
	"testing"
)

func TestLayoutRulesForPreset_CentralizedControlPlane(t *testing.T) {
	rules, err := LayoutRulesForPreset(PresetCentralizedControlPlane)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rules.NodeGrouping != GroupFlat {
		t.Errorf("expected NodeGrouping=%s, got %s", GroupFlat, rules.NodeGrouping)
	}
	if rules.BundleGrouping != GroupFlat {
		t.Errorf("expected BundleGrouping=%s, got %s", GroupFlat, rules.BundleGrouping)
	}
	if rules.ApplicationGrouping != GroupFlat {
		t.Errorf("expected ApplicationGrouping=%s, got %s", GroupFlat, rules.ApplicationGrouping)
	}
	if rules.FilePer != FilePerResource {
		t.Errorf("expected FilePer=%s, got %s", FilePerResource, rules.FilePer)
	}
	if rules.FluxPlacement != FluxSeparate {
		t.Errorf("expected FluxPlacement=%s, got %s", FluxSeparate, rules.FluxPlacement)
	}

	if err := rules.Validate(); err != nil {
		t.Errorf("preset rules should validate: %v", err)
	}
}

func TestLayoutRulesForPreset_SiblingControlPlane(t *testing.T) {
	rules, err := LayoutRulesForPreset(PresetSiblingControlPlane)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rules.NodeGrouping != GroupByName {
		t.Errorf("expected NodeGrouping=%s, got %s", GroupByName, rules.NodeGrouping)
	}
	if rules.FluxPlacement != FluxSeparate {
		t.Errorf("expected FluxPlacement=%s, got %s", FluxSeparate, rules.FluxPlacement)
	}

	if err := rules.Validate(); err != nil {
		t.Errorf("preset rules should validate: %v", err)
	}
}

func TestLayoutRulesForPreset_ParentDeployedControl(t *testing.T) {
	rules, err := LayoutRulesForPreset(PresetParentDeployedControl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rules.FluxPlacement != FluxIntegrated {
		t.Errorf("expected FluxPlacement=%s, got %s", FluxIntegrated, rules.FluxPlacement)
	}

	if err := rules.Validate(); err != nil {
		t.Errorf("preset rules should validate: %v", err)
	}
}

func TestLayoutRulesForPreset_Unknown(t *testing.T) {
	_, err := LayoutRulesForPreset(LayoutPreset("unknown"))
	if err == nil {
		t.Error("expected error for unknown preset")
	}
}

func TestConfigForPreset_CentralizedControlPlane(t *testing.T) {
	cfg, err := ConfigForPreset(PresetCentralizedControlPlane)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ManifestsDir != "clusters" {
		t.Errorf("expected ManifestsDir=clusters, got %s", cfg.ManifestsDir)
	}
	if cfg.KustomizationMode != KustomizationExplicit {
		t.Errorf("expected KustomizationMode=%s, got %s", KustomizationExplicit, cfg.KustomizationMode)
	}
	if cfg.ManifestFileName == nil {
		t.Fatal("ManifestFileName should not be nil")
	}

	// Pattern A uses {kind}-{name}.yaml naming
	fname := cfg.ManifestFileName("default", "deployment", "myapp", FilePerResource)
	if fname != "deployment-myapp.yaml" {
		t.Errorf("expected deployment-myapp.yaml, got %s", fname)
	}

	// FilePerKind mode
	fname = cfg.ManifestFileName("default", "deployment", "myapp", FilePerKind)
	if fname != "deployment.yaml" {
		t.Errorf("expected deployment.yaml, got %s", fname)
	}
}

func TestConfigForPreset_Unknown(t *testing.T) {
	_, err := ConfigForPreset(LayoutPreset("unknown"))
	if err == nil {
		t.Error("expected error for unknown preset")
	}
}

func TestKindNameManifestFileName(t *testing.T) {
	tests := []struct {
		namespace string
		kind      string
		name      string
		mode      FileExportMode
		expected  string
	}{
		{"default", "deployment", "myapp", FilePerResource, "deployment-myapp.yaml"},
		{"kube-system", "service", "dns", FilePerResource, "service-dns.yaml"},
		{"default", "configmap", "cfg", FilePerKind, "configmap.yaml"},
		{"ns", "secret", "db-creds", FilePerResource, "secret-db-creds.yaml"},
	}

	for _, tc := range tests {
		result := KindNameManifestFileName(tc.namespace, tc.kind, tc.name, tc.mode)
		if result != tc.expected {
			t.Errorf("KindNameManifestFileName(%q, %q, %q, %q) = %q, want %q",
				tc.namespace, tc.kind, tc.name, tc.mode, result, tc.expected)
		}
	}
}
