package patch

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func makeResource(kind, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name": name,
			},
		},
	}
}

func TestNewPatchableAppSetStrategicTarget(t *testing.T) {
	resources := []*unstructured.Unstructured{
		makeResource("Deployment", "my-app"),
		makeResource("Service", "my-app"),
		makeResource("ConfigMap", "config"),
	}

	strategicPatch := &StrategicPatch{
		Patch: map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"env": "prod",
				},
			},
		},
	}

	tests := []struct {
		name       string
		target     string
		wantErr    string
		wantTarget string
	}{
		{
			name:    "ambiguous short name is rejected",
			target:  "my-app",
			wantErr: "ambiguous",
		},
		{
			name:       "kind-qualified target resolves correctly",
			target:     "deployment.my-app",
			wantTarget: "deployment.my-app",
		},
		{
			name:       "kind-qualified service target resolves correctly",
			target:     "service.my-app",
			wantTarget: "service.my-app",
		},
		{
			name:       "unique short name resolves to canonical key",
			target:     "config",
			wantTarget: "configmap.config",
		},
		{
			name:    "non-existent target is rejected",
			target:  "does-not-exist",
			wantErr: "not found",
		},
		{
			name:       "mixed-case kind-qualified target resolves correctly",
			target:     "Deployment.my-app",
			wantTarget: "deployment.my-app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patches := []PatchSpec{{
				Target:    tt.target,
				Strategic: strategicPatch,
			}}
			appSet, err := NewPatchableAppSet(resources, patches)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(appSet.Patches) != 1 {
				t.Fatalf("expected 1 patch, got %d", len(appSet.Patches))
			}
			if appSet.Patches[0].Target != tt.wantTarget {
				t.Errorf("got target %q, want %q", appSet.Patches[0].Target, tt.wantTarget)
			}
		})
	}
}

func TestResolveTargetKey(t *testing.T) {
	resources := []*unstructured.Unstructured{
		makeResource("Deployment", "my-app"),
		makeResource("Service", "my-app"),
		makeResource("ConfigMap", "config"),
	}

	tests := []struct {
		name    string
		target  string
		want    string
		wantErr string
	}{
		{
			name:   "kind-qualified target matches",
			target: "deployment.my-app",
			want:   "deployment.my-app",
		},
		{
			name:   "kind-qualified service target matches",
			target: "service.my-app",
			want:   "service.my-app",
		},
		{
			name:   "short name with single match",
			target: "config",
			want:   "configmap.config",
		},
		{
			name:    "short name with multiple matches is ambiguous",
			target:  "my-app",
			wantErr: "ambiguous",
		},
		{
			name:    "non-existent target returns error",
			target:  "does-not-exist",
			wantErr: "not found",
		},
		{
			name:   "mixed-case kind-qualified target matches",
			target: "Deployment.my-app",
			want:   "deployment.my-app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveTargetKey(resources, tt.target)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func makeNamespacedResource(kind, name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       kind,
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
		},
	}
}

func TestResolveTargetKeyDuplicateKindName(t *testing.T) {
	// Two Deployments with the same kind and name (no namespace set)
	resources := []*unstructured.Unstructured{
		makeResource("Deployment", "my-app"),
		makeResource("Deployment", "my-app"),
	}

	_, err := ResolveTargetKey(resources, "deployment.my-app")
	if err == nil {
		t.Fatal("expected error for duplicate kind+name, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected error containing %q, got: %v", "ambiguous", err)
	}
}

func TestResolveTargetKeyNamespaceDisambiguation(t *testing.T) {
	resources := []*unstructured.Unstructured{
		makeNamespacedResource("Deployment", "my-app", "staging"),
		makeNamespacedResource("Deployment", "my-app", "production"),
	}

	tests := []struct {
		name    string
		target  string
		want    string
		wantErr string
	}{
		{
			name:    "kind.name is ambiguous across namespaces",
			target:  "deployment.my-app",
			wantErr: "ambiguous",
		},
		{
			name:   "namespace/kind.name resolves staging",
			target: "staging/deployment.my-app",
			want:   "staging/deployment.my-app",
		},
		{
			name:   "namespace/kind.name resolves production",
			target: "production/deployment.my-app",
			want:   "production/deployment.my-app",
		},
		{
			name:   "namespace/kind.name is case-insensitive for kind",
			target: "staging/Deployment.my-app",
			want:   "staging/deployment.my-app",
		},
		{
			name:    "wrong namespace not found",
			target:  "dev/deployment.my-app",
			wantErr: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveTargetKey(resources, tt.target)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestNewPatchableAppSet_AmbiguousFieldTargetErrors verifies that an ambiguous
// field-level target propagates as an error containing "ambiguous" rather than
// being masked as "explicit target not found" (which WritePatchedFiles would skip).
func TestNewPatchableAppSet_AmbiguousFieldTargetErrors(t *testing.T) {
	resources := []*unstructured.Unstructured{
		makeNamespacedResource("Deployment", "my-app", "staging"),
		makeNamespacedResource("Deployment", "my-app", "production"),
	}

	patches := []PatchSpec{{
		Target: "deployment.my-app",
		Patch:  PatchOp{Op: "set", Path: "spec.replicas", Value: int64(3)},
	}}

	_, err := NewPatchableAppSet(resources, patches)
	if err == nil {
		t.Fatal("expected error for ambiguous field-level target, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected error containing %q, got: %v", "ambiguous", err)
	}
	if strings.Contains(err.Error(), "explicit target not found") {
		t.Fatalf("ambiguity error must not be masked as 'not found', got: %v", err)
	}
}

func TestNewPatchableAppSetStrategicNamespaceTarget(t *testing.T) {
	resources := []*unstructured.Unstructured{
		makeNamespacedResource("Deployment", "my-app", "staging"),
		makeNamespacedResource("Deployment", "my-app", "production"),
	}

	strategicPatch := &StrategicPatch{
		Patch: map[string]interface{}{
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"env": "staging",
				},
			},
		},
	}

	patches := []PatchSpec{{
		Target:    "staging/deployment.my-app",
		Strategic: strategicPatch,
	}}
	appSet, err := NewPatchableAppSet(resources, patches)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(appSet.Patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(appSet.Patches))
	}
	if appSet.Patches[0].Target != "staging/deployment.my-app" {
		t.Errorf("got target %q, want %q", appSet.Patches[0].Target, "staging/deployment.my-app")
	}

	// Verify the patch resolves to the correct resource
	resolved, err := appSet.Resolve()
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(resolved) != 1 {
		t.Fatalf("expected 1 resolved, got %d", len(resolved))
	}
	if resolved[0].Base.GetNamespace() != "staging" {
		t.Errorf("expected namespace staging, got %q", resolved[0].Base.GetNamespace())
	}
}
