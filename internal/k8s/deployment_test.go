package k8s

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddDeploymentTopologySpreadConstraints(t *testing.T) {
	t.Run("nil constraint", func(t *testing.T) {
		dep := CreateDeployment("test", "default")
		AddDeploymentTopologySpreadConstraints(dep, nil)
		if len(dep.Spec.Template.Spec.TopologySpreadConstraints) != 0 {
			t.Errorf("expected no constraints, got %d", len(dep.Spec.Template.Spec.TopologySpreadConstraints))
		}
	})

	t.Run("append single constraint", func(t *testing.T) {
		dep := CreateDeployment("test", "default")
		c := corev1.TopologySpreadConstraint{
			MaxSkew:           1,
			TopologyKey:       "topology.kubernetes.io/zone",
			WhenUnsatisfiable: corev1.DoNotSchedule,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
		}
		AddDeploymentTopologySpreadConstraints(dep, &c)
		if len(dep.Spec.Template.Spec.TopologySpreadConstraints) != 1 {
			t.Fatalf("expected 1 constraint, got %d", len(dep.Spec.Template.Spec.TopologySpreadConstraints))
		}
		if !reflect.DeepEqual(dep.Spec.Template.Spec.TopologySpreadConstraints[0], c) {
			t.Errorf("constraint mismatch: got %+v, want %+v", dep.Spec.Template.Spec.TopologySpreadConstraints[0], c)
		}
	})

	t.Run("append additional constraint", func(t *testing.T) {
		dep := CreateDeployment("test", "default")
		first := corev1.TopologySpreadConstraint{
			MaxSkew:           1,
			TopologyKey:       "zone",
			WhenUnsatisfiable: corev1.DoNotSchedule,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
		}
		second := corev1.TopologySpreadConstraint{
			MaxSkew:           2,
			TopologyKey:       "hostname",
			WhenUnsatisfiable: corev1.DoNotSchedule,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
		}
		AddDeploymentTopologySpreadConstraints(dep, &first)
		AddDeploymentTopologySpreadConstraints(dep, &second)
		if len(dep.Spec.Template.Spec.TopologySpreadConstraints) != 2 {
			t.Fatalf("expected 2 constraints, got %d", len(dep.Spec.Template.Spec.TopologySpreadConstraints))
		}
		if !reflect.DeepEqual(dep.Spec.Template.Spec.TopologySpreadConstraints[0], first) {
			t.Errorf("first constraint mismatch: got %+v, want %+v", dep.Spec.Template.Spec.TopologySpreadConstraints[0], first)
		}
		if !reflect.DeepEqual(dep.Spec.Template.Spec.TopologySpreadConstraints[1], second) {
			t.Errorf("second constraint mismatch: got %+v, want %+v", dep.Spec.Template.Spec.TopologySpreadConstraints[1], second)
		}
	})
}
