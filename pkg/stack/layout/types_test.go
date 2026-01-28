package layout_test

import (
	"testing"

	"github.com/go-kure/kure/pkg/stack/layout"
)

func TestLayoutRules_Validate(t *testing.T) {
	tests := []struct {
		name    string
		rules   layout.LayoutRules
		wantErr bool
	}{
		{
			name: "valid default rules",
			rules: layout.LayoutRules{
				NodeGrouping:        layout.GroupByName,
				BundleGrouping:      layout.GroupFlat,
				ApplicationGrouping: layout.GroupFlat,
				ApplicationFileMode: layout.AppFilePerResource,
				FilePer:             layout.FilePerResource,
				FluxPlacement:       layout.FluxSeparate,
			},
			wantErr: false,
		},
		{
			name: "valid with unset values",
			rules: layout.LayoutRules{
				NodeGrouping:        layout.GroupUnset,
				BundleGrouping:      layout.GroupUnset,
				ApplicationGrouping: layout.GroupUnset,
				ApplicationFileMode: layout.AppFileUnset,
				FilePer:             layout.FilePerUnset,
				FluxPlacement:       layout.FluxUnset,
			},
			wantErr: false,
		},
		{
			name: "valid alternative values",
			rules: layout.LayoutRules{
				NodeGrouping:        layout.GroupFlat,
				BundleGrouping:      layout.GroupByName,
				ApplicationGrouping: layout.GroupByName,
				ApplicationFileMode: layout.AppFileSingle,
				FilePer:             layout.FilePerKind,
				FluxPlacement:       layout.FluxIntegrated,
			},
			wantErr: false,
		},
		{
			name: "invalid node grouping",
			rules: layout.LayoutRules{
				NodeGrouping:        layout.GroupingMode("invalid"),
				BundleGrouping:      layout.GroupFlat,
				ApplicationGrouping: layout.GroupFlat,
			},
			wantErr: true,
		},
		{
			name: "invalid bundle grouping",
			rules: layout.LayoutRules{
				NodeGrouping:        layout.GroupByName,
				BundleGrouping:      layout.GroupingMode("invalid"),
				ApplicationGrouping: layout.GroupFlat,
			},
			wantErr: true,
		},
		{
			name: "invalid application grouping",
			rules: layout.LayoutRules{
				NodeGrouping:        layout.GroupByName,
				BundleGrouping:      layout.GroupFlat,
				ApplicationGrouping: layout.GroupingMode("invalid"),
			},
			wantErr: true,
		},
		{
			name: "invalid application file mode",
			rules: layout.LayoutRules{
				NodeGrouping:        layout.GroupByName,
				BundleGrouping:      layout.GroupFlat,
				ApplicationGrouping: layout.GroupFlat,
				ApplicationFileMode: layout.ApplicationFileMode("invalid"),
			},
			wantErr: true,
		},
		{
			name: "invalid file per",
			rules: layout.LayoutRules{
				NodeGrouping:        layout.GroupByName,
				BundleGrouping:      layout.GroupFlat,
				ApplicationGrouping: layout.GroupFlat,
				ApplicationFileMode: layout.AppFilePerResource,
				FilePer:             layout.FileExportMode("invalid"),
			},
			wantErr: true,
		},
		{
			name: "invalid flux placement",
			rules: layout.LayoutRules{
				NodeGrouping:        layout.GroupByName,
				BundleGrouping:      layout.GroupFlat,
				ApplicationGrouping: layout.GroupFlat,
				ApplicationFileMode: layout.AppFilePerResource,
				FilePer:             layout.FilePerResource,
				FluxPlacement:       layout.FluxPlacement("invalid"),
			},
			wantErr: true,
		},
		{
			name: "valid with cluster name",
			rules: layout.LayoutRules{
				NodeGrouping:        layout.GroupByName,
				BundleGrouping:      layout.GroupFlat,
				ApplicationGrouping: layout.GroupFlat,
				ApplicationFileMode: layout.AppFilePerResource,
				FilePer:             layout.FilePerResource,
				FluxPlacement:       layout.FluxSeparate,
				ClusterName:         "my-cluster",
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.rules.Validate()
			if test.wantErr && err == nil {
				t.Error("expected validation error, got nil")
			}
			if !test.wantErr && err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestDefaultLayoutRules(t *testing.T) {
	rules := layout.DefaultLayoutRules()

	// Test that defaults validate
	if err := rules.Validate(); err != nil {
		t.Errorf("default rules should validate: %v", err)
	}

	// Test expected default values
	if rules.NodeGrouping != layout.GroupByName {
		t.Errorf("expected NodeGrouping=%s, got %s", layout.GroupByName, rules.NodeGrouping)
	}
	if rules.BundleGrouping != layout.GroupFlat {
		t.Errorf("expected BundleGrouping=%s, got %s", layout.GroupFlat, rules.BundleGrouping)
	}
	if rules.ApplicationGrouping != layout.GroupFlat {
		t.Errorf("expected ApplicationGrouping=%s, got %s", layout.GroupFlat, rules.ApplicationGrouping)
	}
	if rules.ApplicationFileMode != layout.AppFilePerResource {
		t.Errorf("expected ApplicationFileMode=%s, got %s", layout.AppFilePerResource, rules.ApplicationFileMode)
	}
	if rules.FilePer != layout.FilePerResource {
		t.Errorf("expected FilePer=%s, got %s", layout.FilePerResource, rules.FilePer)
	}
	if rules.FluxPlacement != layout.FluxSeparate {
		t.Errorf("expected FluxPlacement=%s, got %s", layout.FluxSeparate, rules.FluxPlacement)
	}
}
