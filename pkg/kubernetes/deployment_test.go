package kubernetes

import (
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateDeployment(t *testing.T) {
	dep := CreateDeployment("my-app", "default")
	if dep.Name != "my-app" || dep.Namespace != "default" {
		t.Fatalf("metadata mismatch: %s/%s", dep.Namespace, dep.Name)
	}
	if dep.Kind != "Deployment" {
		t.Errorf("unexpected kind %q", dep.Kind)
	}
	if dep.Labels["app"] != "my-app" {
		t.Errorf("expected label app=my-app, got %v", dep.Labels)
	}
}

func TestDeploymentNilErrors(t *testing.T) {
	// Functions with secondary nil checks — still return error on nil receiver
	if err := SetDeploymentPodSpec(nil, &corev1.PodSpec{}); err == nil {
		t.Error("expected error for nil Deployment on SetDeploymentPodSpec")
	}
	if err := AddDeploymentContainer(nil, &corev1.Container{Name: "c"}); err == nil {
		t.Error("expected error for nil Deployment on AddDeploymentContainer")
	}
	if err := AddDeploymentInitContainer(nil, &corev1.Container{Name: "c"}); err == nil {
		t.Error("expected error for nil Deployment on AddDeploymentInitContainer")
	}
	if err := AddDeploymentVolume(nil, &corev1.Volume{Name: "v"}); err == nil {
		t.Error("expected error for nil Deployment on AddDeploymentVolume")
	}
	if err := AddDeploymentImagePullSecret(nil, &corev1.LocalObjectReference{Name: "s"}); err == nil {
		t.Error("expected error for nil Deployment on AddDeploymentImagePullSecret")
	}
	if err := AddDeploymentToleration(nil, &corev1.Toleration{Key: "k"}); err == nil {
		t.Error("expected error for nil Deployment on AddDeploymentToleration")
	}
	if err := AddDeploymentTopologySpreadConstraints(nil, &corev1.TopologySpreadConstraint{}); err == nil {
		t.Error("expected error for nil Deployment on AddDeploymentTopologySpreadConstraints")
	}

	// Functions that now panic on nil receiver
	assertPanics(t, func() { SetDeploymentServiceAccountName(nil, "sa") })
	assertPanics(t, func() { SetDeploymentSecurityContext(nil, &corev1.PodSecurityContext{}) })
	assertPanics(t, func() { SetDeploymentAffinity(nil, &corev1.Affinity{}) })
	assertPanics(t, func() { SetDeploymentNodeSelector(nil, map[string]string{}) })
	assertPanics(t, func() { SetDeploymentReplicas(nil, 3) })
	assertPanics(t, func() { SetDeploymentStrategy(nil, appsv1.DeploymentStrategy{}) })
	assertPanics(t, func() { SetDeploymentRevisionHistoryLimit(nil, 5) })
	assertPanics(t, func() { SetDeploymentMinReadySeconds(nil, 10) })
	assertPanics(t, func() { SetDeploymentProgressDeadlineSeconds(nil, 60) })
}

func TestDeploymentNilArgErrors(t *testing.T) {
	dep := CreateDeployment("test", "default")
	if err := SetDeploymentPodSpec(dep, nil); err == nil {
		t.Error("expected error for nil PodSpec")
	}
	if err := AddDeploymentContainer(dep, nil); err == nil {
		t.Error("expected error for nil Container")
	}
	if err := AddDeploymentInitContainer(dep, nil); err == nil {
		t.Error("expected error for nil InitContainer")
	}
	if err := AddDeploymentVolume(dep, nil); err == nil {
		t.Error("expected error for nil Volume")
	}
	if err := AddDeploymentImagePullSecret(dep, nil); err == nil {
		t.Error("expected error for nil ImagePullSecret")
	}
	if err := AddDeploymentToleration(dep, nil); err == nil {
		t.Error("expected error for nil Toleration")
	}
}

func TestDeploymentTopologySpreadConstraints(t *testing.T) {
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
			t.Errorf("first constraint mismatch")
		}
		if !reflect.DeepEqual(dep.Spec.Template.Spec.TopologySpreadConstraints[1], second) {
			t.Errorf("second constraint mismatch")
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

	SetDeploymentServiceAccountName(dep, "sa")
	if dep.Spec.Template.Spec.ServiceAccountName != "sa" {
		t.Errorf("service account name not set")
	}

	sc := &corev1.PodSecurityContext{RunAsUser: func(i int64) *int64 { return &i }(1)}
	SetDeploymentSecurityContext(dep, sc)
	if dep.Spec.Template.Spec.SecurityContext != sc {
		t.Errorf("security context not set")
	}

	aff := &corev1.Affinity{}
	SetDeploymentAffinity(dep, aff)
	if dep.Spec.Template.Spec.Affinity != aff {
		t.Errorf("affinity not set")
	}

	ns := map[string]string{"role": "db"}
	SetDeploymentNodeSelector(dep, ns)
	if !reflect.DeepEqual(dep.Spec.Template.Spec.NodeSelector, ns) {
		t.Errorf("node selector not set")
	}

	SetDeploymentReplicas(dep, 3)
	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 3 {
		t.Errorf("replicas not set")
	}

	strategy := appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType}
	SetDeploymentStrategy(dep, strategy)
	if dep.Spec.Strategy.Type != appsv1.RollingUpdateDeploymentStrategyType {
		t.Errorf("strategy not set")
	}

	SetDeploymentRevisionHistoryLimit(dep, 5)
	if dep.Spec.RevisionHistoryLimit == nil || *dep.Spec.RevisionHistoryLimit != 5 {
		t.Errorf("revision history limit not set")
	}

	SetDeploymentMinReadySeconds(dep, 10)
	if dep.Spec.MinReadySeconds != 10 {
		t.Errorf("min ready seconds not set")
	}

	SetDeploymentProgressDeadlineSeconds(dep, 60)
	if dep.Spec.ProgressDeadlineSeconds == nil || *dep.Spec.ProgressDeadlineSeconds != 60 {
		t.Errorf("progress deadline seconds not set")
	}
}

func TestSetDeploymentPodSpec(t *testing.T) {
	dep := CreateDeployment("test", "default")
	spec := &corev1.PodSpec{
		Containers: []corev1.Container{
			{Name: "test", Image: "nginx"},
		},
	}
	if err := SetDeploymentPodSpec(dep, spec); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(dep.Spec.Template.Spec.Containers) != 1 {
		t.Fatal("expected PodSpec to be assigned")
	}
}
