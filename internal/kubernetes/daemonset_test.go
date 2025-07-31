package kubernetes

import (
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddDaemonSetTopologySpreadConstraints(t *testing.T) {
	t.Run("nil constraint", func(t *testing.T) {
		ds := CreateDaemonSet("test", "default")
		if err := AddDaemonSetTopologySpreadConstraints(ds, nil); err != nil {
			t.Fatalf("AddDaemonSetTopologySpreadConstraints returned error: %v", err)
		}
		if len(ds.Spec.Template.Spec.TopologySpreadConstraints) != 0 {
			t.Errorf("expected no constraints, got %d", len(ds.Spec.Template.Spec.TopologySpreadConstraints))
		}
	})

	t.Run("append single constraint", func(t *testing.T) {
		ds := CreateDaemonSet("test", "default")
		c := corev1.TopologySpreadConstraint{
			MaxSkew:           1,
			TopologyKey:       "zone",
			WhenUnsatisfiable: corev1.DoNotSchedule,
			LabelSelector:     &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
		}
		if err := AddDaemonSetTopologySpreadConstraints(ds, &c); err != nil {
			t.Fatalf("AddDaemonSetTopologySpreadConstraints returned error: %v", err)
		}
		if len(ds.Spec.Template.Spec.TopologySpreadConstraints) != 1 {
			t.Fatalf("expected 1 constraint, got %d", len(ds.Spec.Template.Spec.TopologySpreadConstraints))
		}
		if !reflect.DeepEqual(ds.Spec.Template.Spec.TopologySpreadConstraints[0], c) {
			t.Errorf("constraint mismatch: got %+v, want %+v", ds.Spec.Template.Spec.TopologySpreadConstraints[0], c)
		}
	})

	t.Run("append additional constraint", func(t *testing.T) {
		ds := CreateDaemonSet("test", "default")
		first := corev1.TopologySpreadConstraint{
			MaxSkew:           1,
			TopologyKey:       "zone",
			WhenUnsatisfiable: corev1.DoNotSchedule,
			LabelSelector:     &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
		}
		second := corev1.TopologySpreadConstraint{
			MaxSkew:           2,
			TopologyKey:       "hostname",
			WhenUnsatisfiable: corev1.DoNotSchedule,
			LabelSelector:     &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
		}
		if err := AddDaemonSetTopologySpreadConstraints(ds, &first); err != nil {
			t.Fatalf("AddDaemonSetTopologySpreadConstraints returned error: %v", err)
		}
		if err := AddDaemonSetTopologySpreadConstraints(ds, &second); err != nil {
			t.Fatalf("AddDaemonSetTopologySpreadConstraints returned error: %v", err)
		}
		if len(ds.Spec.Template.Spec.TopologySpreadConstraints) != 2 {
			t.Fatalf("expected 2 constraints, got %d", len(ds.Spec.Template.Spec.TopologySpreadConstraints))
		}
		if !reflect.DeepEqual(ds.Spec.Template.Spec.TopologySpreadConstraints[0], first) {
			t.Errorf("first constraint mismatch")
		}
		if !reflect.DeepEqual(ds.Spec.Template.Spec.TopologySpreadConstraints[1], second) {
			t.Errorf("second constraint mismatch")
		}
	})
}

func TestDaemonSetFunctions(t *testing.T) {
	ds := CreateDaemonSet("app", "ns")
	if ds.Name != "app" || ds.Namespace != "ns" {
		t.Fatalf("metadata mismatch: %s/%s", ds.Namespace, ds.Name)
	}
	if ds.Kind != "DaemonSet" {
		t.Errorf("unexpected kind %q", ds.Kind)
	}

	c := corev1.Container{Name: "c"}
	if err := AddDaemonSetContainer(ds, &c); err != nil {
		t.Fatalf("AddDaemonSetContainer returned error: %v", err)
	}
	if len(ds.Spec.Template.Spec.Containers) != 1 || ds.Spec.Template.Spec.Containers[0].Name != "c" {
		t.Errorf("container not added")
	}

	ic := corev1.Container{Name: "init"}
	if err := AddDaemonSetInitContainer(ds, &ic); err != nil {
		t.Fatalf("AddDaemonSetInitContainer returned error: %v", err)
	}
	if len(ds.Spec.Template.Spec.InitContainers) != 1 {
		t.Errorf("init container not added")
	}

	v := corev1.Volume{Name: "vol"}
	if err := AddDaemonSetVolume(ds, &v); err != nil {
		t.Fatalf("AddDaemonSetVolume returned error: %v", err)
	}
	if len(ds.Spec.Template.Spec.Volumes) != 1 {
		t.Errorf("volume not added")
	}

	secret := corev1.LocalObjectReference{Name: "secret"}
	if err := AddDaemonSetImagePullSecret(ds, &secret); err != nil {
		t.Fatalf("AddDaemonSetImagePullSecret returned error: %v", err)
	}
	if len(ds.Spec.Template.Spec.ImagePullSecrets) != 1 {
		t.Errorf("image pull secret not added")
	}

	tol := corev1.Toleration{Key: "k"}
	if err := AddDaemonSetToleration(ds, &tol); err != nil {
		t.Fatalf("AddDaemonSetToleration returned error: %v", err)
	}
	if len(ds.Spec.Template.Spec.Tolerations) != 1 {
		t.Errorf("toleration not added")
	}

	tsc := corev1.TopologySpreadConstraint{MaxSkew: 1, TopologyKey: "zone", WhenUnsatisfiable: corev1.ScheduleAnyway, LabelSelector: &metav1.LabelSelector{}}
	if err := AddDaemonSetTopologySpreadConstraints(ds, &tsc); err != nil {
		t.Fatalf("AddDaemonSetTopologySpreadConstraints returned error: %v", err)
	}
	if len(ds.Spec.Template.Spec.TopologySpreadConstraints) != 1 {
		t.Errorf("topology constraint not added")
	}

	if err := SetDaemonSetServiceAccountName(ds, "sa"); err != nil {
		t.Fatalf("SetDaemonSetServiceAccountName returned error: %v", err)
	}
	if ds.Spec.Template.Spec.ServiceAccountName != "sa" {
		t.Errorf("service account name not set")
	}

	sc := &corev1.PodSecurityContext{}
	if err := SetDaemonSetSecurityContext(ds, sc); err != nil {
		t.Fatalf("SetDaemonSetSecurityContext returned error: %v", err)
	}
	if ds.Spec.Template.Spec.SecurityContext != sc {
		t.Errorf("security context not set")
	}

	aff := &corev1.Affinity{}
	if err := SetDaemonSetAffinity(ds, aff); err != nil {
		t.Fatalf("SetDaemonSetAffinity returned error: %v", err)
	}
	if ds.Spec.Template.Spec.Affinity != aff {
		t.Errorf("affinity not set")
	}

	ns := map[string]string{"role": "db"}
	if err := SetDaemonSetNodeSelector(ds, ns); err != nil {
		t.Fatalf("SetDaemonSetNodeSelector returned error: %v", err)
	}
	if !reflect.DeepEqual(ds.Spec.Template.Spec.NodeSelector, ns) {
		t.Errorf("node selector not set")
	}

	strat := appsv1.DaemonSetUpdateStrategy{Type: appsv1.RollingUpdateDaemonSetStrategyType}
	if err := SetDaemonSetUpdateStrategy(ds, strat); err != nil {
		t.Fatalf("SetDaemonSetUpdateStrategy returned error: %v", err)
	}
	if ds.Spec.UpdateStrategy.Type != appsv1.RollingUpdateDaemonSetStrategyType {
		t.Errorf("update strategy not set")
	}

	rhl := int32(3)
	if err := SetDaemonSetRevisionHistoryLimit(ds, &rhl); err != nil {
		t.Fatalf("SetDaemonSetRevisionHistoryLimit returned error: %v", err)
	}
	if ds.Spec.RevisionHistoryLimit == nil || *ds.Spec.RevisionHistoryLimit != 3 {
		t.Errorf("revision history limit not set")
	}
}
