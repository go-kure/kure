package kubernetes

import (
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddDeploymentTopologySpreadConstraints(t *testing.T) {
	t.Run("nil constraint", func(t *testing.T) {
		dep := CreateDeployment("test", "default")
		if err := AddDeploymentTopologySpreadConstraints(dep, nil); err != nil {
			t.Fatalf("AddDeploymentTopologySpreadConstraints returned error: %v", err)
		}
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
		if err := AddDeploymentTopologySpreadConstraints(dep, &c); err != nil {
			t.Fatalf("AddDeploymentTopologySpreadConstraints returned error: %v", err)
		}
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
		if err := AddDeploymentTopologySpreadConstraints(dep, &first); err != nil {
			t.Fatalf("AddDeploymentTopologySpreadConstraints returned error: %v", err)
		}
		if err := AddDeploymentTopologySpreadConstraints(dep, &second); err != nil {
			t.Fatalf("AddDeploymentTopologySpreadConstraints returned error: %v", err)
		}
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
func TestDeploymentFunctions(t *testing.T) {
	dep := CreateDeployment("app", "ns")
	if dep.Name != "app" || dep.Namespace != "ns" {
		t.Fatalf("metadata mismatch: %s/%s", dep.Namespace, dep.Name)
	}
	if dep.Kind != "Deployment" {
		t.Errorf("unexpected kind %q", dep.Kind)
	}

	c := corev1.Container{Name: "c"}
	if err := AddDeploymentContainer(dep, &c); err != nil {
		t.Fatalf("AddDeploymentContainer returned error: %v", err)
	}
	if len(dep.Spec.Template.Spec.Containers) != 1 || dep.Spec.Template.Spec.Containers[0].Name != "c" {
		t.Errorf("container not added")
	}

	ic := corev1.Container{Name: "init"}
	if err := AddDeploymentInitContainer(dep, &ic); err != nil {
		t.Fatalf("AddDeploymentInitContainer returned error: %v", err)
	}
	if len(dep.Spec.Template.Spec.InitContainers) != 1 {
		t.Errorf("init container not added")
	}

	v := corev1.Volume{Name: "vol"}
	if err := AddDeploymentVolume(dep, &v); err != nil {
		t.Fatalf("AddDeploymentVolume returned error: %v", err)
	}
	if len(dep.Spec.Template.Spec.Volumes) != 1 {
		t.Errorf("volume not added")
	}

	secret := corev1.LocalObjectReference{Name: "secret"}
	if err := AddDeploymentImagePullSecret(dep, &secret); err != nil {
		t.Fatalf("AddDeploymentImagePullSecret returned error: %v", err)
	}
	if len(dep.Spec.Template.Spec.ImagePullSecrets) != 1 {
		t.Errorf("image pull secret not added")
	}

	tol := corev1.Toleration{Key: "k"}
	if err := AddDeploymentToleration(dep, &tol); err != nil {
		t.Fatalf("AddDeploymentToleration returned error: %v", err)
	}
	if len(dep.Spec.Template.Spec.Tolerations) != 1 {
		t.Errorf("toleration not added")
	}

	tsc := corev1.TopologySpreadConstraint{MaxSkew: 1, TopologyKey: "zone", WhenUnsatisfiable: corev1.ScheduleAnyway, LabelSelector: &metav1.LabelSelector{}}
	if err := AddDeploymentTopologySpreadConstraints(dep, &tsc); err != nil {
		t.Fatalf("AddDeploymentTopologySpreadConstraints returned error: %v", err)
	}
	if len(dep.Spec.Template.Spec.TopologySpreadConstraints) != 1 {
		t.Errorf("topology constraint not added")
	}

	if err := SetDeploymentServiceAccountName(dep, "sa"); err != nil {
		t.Fatalf("SetDeploymentServiceAccountName returned error: %v", err)
	}
	if dep.Spec.Template.Spec.ServiceAccountName != "sa" {
		t.Errorf("service account name not set")
	}

	sc := &corev1.PodSecurityContext{RunAsUser: func(i int64) *int64 { return &i }(1)}
	if err := SetDeploymentSecurityContext(dep, sc); err != nil {
		t.Fatalf("SetDeploymentSecurityContext returned error: %v", err)
	}
	if dep.Spec.Template.Spec.SecurityContext != sc {
		t.Errorf("security context not set")
	}

	aff := &corev1.Affinity{}
	if err := SetDeploymentAffinity(dep, aff); err != nil {
		t.Fatalf("SetDeploymentAffinity returned error: %v", err)
	}
	if dep.Spec.Template.Spec.Affinity != aff {
		t.Errorf("affinity not set")
	}

	ns := map[string]string{"role": "db"}
	if err := SetDeploymentNodeSelector(dep, ns); err != nil {
		t.Fatalf("SetDeploymentNodeSelector returned error: %v", err)
	}
	if !reflect.DeepEqual(dep.Spec.Template.Spec.NodeSelector, ns) {
		t.Errorf("node selector not set")
	}

	if err := SetDeploymentReplicas(dep, 3); err != nil {
		t.Fatalf("SetDeploymentReplicas returned error: %v", err)
	}
	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 3 {
		t.Errorf("replicas not set")
	}

	strat := appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType}
	if err := SetDeploymentStrategy(dep, strat); err != nil {
		t.Fatalf("SetDeploymentStrategy returned error: %v", err)
	}
	if dep.Spec.Strategy.Type != appsv1.RollingUpdateDeploymentStrategyType {
		t.Errorf("strategy not set")
	}

	if err := SetDeploymentRevisionHistoryLimit(dep, 5); err != nil {
		t.Fatalf("SetDeploymentRevisionHistoryLimit returned error: %v", err)
	}
	if dep.Spec.RevisionHistoryLimit == nil || *dep.Spec.RevisionHistoryLimit != 5 {
		t.Errorf("revision history limit not set")
	}

	if err := SetDeploymentMinReadySeconds(dep, 10); err != nil {
		t.Fatalf("SetDeploymentMinReadySeconds returned error: %v", err)
	}
	if dep.Spec.MinReadySeconds != 10 {
		t.Errorf("min ready seconds not set")
	}

	if err := SetDeploymentProgressDeadlineSeconds(dep, 60); err != nil {
		t.Fatalf("SetDeploymentProgressDeadlineSeconds returned error: %v", err)
	}
	if dep.Spec.ProgressDeadlineSeconds == nil || *dep.Spec.ProgressDeadlineSeconds != 60 {
		t.Errorf("progress deadline seconds not set")
	}
}
