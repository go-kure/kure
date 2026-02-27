package kubernetes

import (
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddStatefulSetTopologySpreadConstraints(t *testing.T) {
	t.Run("nil constraint", func(t *testing.T) {
		sts := CreateStatefulSet("test", "default")
		if err := AddStatefulSetTopologySpreadConstraints(sts, nil); err != nil {
			t.Fatalf("AddStatefulSetTopologySpreadConstraints returned error: %v", err)
		}
		if len(sts.Spec.Template.Spec.TopologySpreadConstraints) != 0 {
			t.Errorf("expected no constraints, got %d", len(sts.Spec.Template.Spec.TopologySpreadConstraints))
		}
	})

	t.Run("append single constraint", func(t *testing.T) {
		sts := CreateStatefulSet("test", "default")
		c := corev1.TopologySpreadConstraint{
			MaxSkew:           1,
			TopologyKey:       "zone",
			WhenUnsatisfiable: corev1.DoNotSchedule,
			LabelSelector:     &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
		}
		if err := AddStatefulSetTopologySpreadConstraints(sts, &c); err != nil {
			t.Fatalf("AddStatefulSetTopologySpreadConstraints returned error: %v", err)
		}
		if len(sts.Spec.Template.Spec.TopologySpreadConstraints) != 1 {
			t.Fatalf("expected 1 constraint, got %d", len(sts.Spec.Template.Spec.TopologySpreadConstraints))
		}
		if !reflect.DeepEqual(sts.Spec.Template.Spec.TopologySpreadConstraints[0], c) {
			t.Errorf("constraint mismatch: got %+v, want %+v", sts.Spec.Template.Spec.TopologySpreadConstraints[0], c)
		}
	})

	t.Run("append additional constraint", func(t *testing.T) {
		sts := CreateStatefulSet("test", "default")
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
		if err := AddStatefulSetTopologySpreadConstraints(sts, &first); err != nil {
			t.Fatalf("AddStatefulSetTopologySpreadConstraints returned error: %v", err)
		}
		if err := AddStatefulSetTopologySpreadConstraints(sts, &second); err != nil {
			t.Fatalf("AddStatefulSetTopologySpreadConstraints returned error: %v", err)
		}
		if len(sts.Spec.Template.Spec.TopologySpreadConstraints) != 2 {
			t.Fatalf("expected 2 constraints, got %d", len(sts.Spec.Template.Spec.TopologySpreadConstraints))
		}
		if !reflect.DeepEqual(sts.Spec.Template.Spec.TopologySpreadConstraints[0], first) {
			t.Errorf("first constraint mismatch")
		}
		if !reflect.DeepEqual(sts.Spec.Template.Spec.TopologySpreadConstraints[1], second) {
			t.Errorf("second constraint mismatch")
		}
	})
}

func TestStatefulSetFunctions(t *testing.T) {
	sts := CreateStatefulSet("app", "ns")
	if sts.Name != "app" || sts.Namespace != "ns" {
		t.Fatalf("metadata mismatch: %s/%s", sts.Namespace, sts.Name)
	}
	if sts.Kind != "StatefulSet" {
		t.Errorf("unexpected kind %q", sts.Kind)
	}

	c := corev1.Container{Name: "c"}
	if err := AddStatefulSetContainer(sts, &c); err != nil {
		t.Fatalf("AddStatefulSetContainer returned error: %v", err)
	}
	if len(sts.Spec.Template.Spec.Containers) != 1 || sts.Spec.Template.Spec.Containers[0].Name != "c" {
		t.Errorf("container not added")
	}

	ic := corev1.Container{Name: "init"}
	if err := AddStatefulSetInitContainer(sts, &ic); err != nil {
		t.Fatalf("AddStatefulSetInitContainer returned error: %v", err)
	}
	if len(sts.Spec.Template.Spec.InitContainers) != 1 {
		t.Errorf("init container not added")
	}

	v := corev1.Volume{Name: "vol"}
	if err := AddStatefulSetVolume(sts, &v); err != nil {
		t.Fatalf("AddStatefulSetVolume returned error: %v", err)
	}
	if len(sts.Spec.Template.Spec.Volumes) != 1 {
		t.Errorf("volume not added")
	}

	secret := corev1.LocalObjectReference{Name: "secret"}
	if err := AddStatefulSetImagePullSecret(sts, &secret); err != nil {
		t.Fatalf("AddStatefulSetImagePullSecret returned error: %v", err)
	}
	if len(sts.Spec.Template.Spec.ImagePullSecrets) != 1 {
		t.Errorf("image pull secret not added")
	}

	tol := corev1.Toleration{Key: "k"}
	if err := AddStatefulSetToleration(sts, &tol); err != nil {
		t.Fatalf("AddStatefulSetToleration returned error: %v", err)
	}
	if len(sts.Spec.Template.Spec.Tolerations) != 1 {
		t.Errorf("toleration not added")
	}

	tsc := corev1.TopologySpreadConstraint{MaxSkew: 1, TopologyKey: "zone", WhenUnsatisfiable: corev1.ScheduleAnyway, LabelSelector: &metav1.LabelSelector{}}
	if err := AddStatefulSetTopologySpreadConstraints(sts, &tsc); err != nil {
		t.Fatalf("AddStatefulSetTopologySpreadConstraints returned error: %v", err)
	}
	if len(sts.Spec.Template.Spec.TopologySpreadConstraints) != 1 {
		t.Errorf("topology constraint not added")
	}

	pvc := corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data"}}
	if err := AddStatefulSetVolumeClaimTemplate(sts, pvc); err != nil {
		t.Fatalf("AddStatefulSetVolumeClaimTemplate returned error: %v", err)
	}
	if len(sts.Spec.VolumeClaimTemplates) != 1 {
		t.Errorf("volume claim template not added")
	}

	if err := SetStatefulSetServiceAccountName(sts, "sa"); err != nil {
		t.Fatalf("SetStatefulSetServiceAccountName returned error: %v", err)
	}
	if sts.Spec.Template.Spec.ServiceAccountName != "sa" {
		t.Errorf("service account name not set")
	}

	sc := &corev1.PodSecurityContext{}
	if err := SetStatefulSetSecurityContext(sts, sc); err != nil {
		t.Fatalf("SetStatefulSetSecurityContext returned error: %v", err)
	}
	if sts.Spec.Template.Spec.SecurityContext != sc {
		t.Errorf("security context not set")
	}

	aff := &corev1.Affinity{}
	if err := SetStatefulSetAffinity(sts, aff); err != nil {
		t.Fatalf("SetStatefulSetAffinity returned error: %v", err)
	}
	if sts.Spec.Template.Spec.Affinity != aff {
		t.Errorf("affinity not set")
	}

	ns := map[string]string{"role": "db"}
	if err := SetStatefulSetNodeSelector(sts, ns); err != nil {
		t.Fatalf("SetStatefulSetNodeSelector returned error: %v", err)
	}
	if !reflect.DeepEqual(sts.Spec.Template.Spec.NodeSelector, ns) {
		t.Errorf("node selector not set")
	}

	strategy := appsv1.StatefulSetUpdateStrategy{Type: appsv1.RollingUpdateStatefulSetStrategyType}
	if err := SetStatefulSetUpdateStrategy(sts, strategy); err != nil {
		t.Fatalf("SetStatefulSetUpdateStrategy returned error: %v", err)
	}
	if sts.Spec.UpdateStrategy.Type != appsv1.RollingUpdateStatefulSetStrategyType {
		t.Errorf("update strategy not set")
	}

	if err := SetStatefulSetReplicas(sts, 3); err != nil {
		t.Fatalf("SetStatefulSetReplicas returned error: %v", err)
	}
	if sts.Spec.Replicas == nil || *sts.Spec.Replicas != 3 {
		t.Errorf("replicas not set")
	}

	if err := SetStatefulSetServiceName(sts, "svc"); err != nil {
		t.Fatalf("SetStatefulSetServiceName returned error: %v", err)
	}
	if sts.Spec.ServiceName != "svc" {
		t.Errorf("service name not set")
	}

	if err := SetStatefulSetPodManagementPolicy(sts, appsv1.ParallelPodManagement); err != nil {
		t.Fatalf("SetStatefulSetPodManagementPolicy returned error: %v", err)
	}
	if sts.Spec.PodManagementPolicy != appsv1.ParallelPodManagement {
		t.Errorf("pod management policy not set")
	}

	rhl := int32(4)
	if err := SetStatefulSetRevisionHistoryLimit(sts, &rhl); err != nil {
		t.Fatalf("SetStatefulSetRevisionHistoryLimit returned error: %v", err)
	}
	if sts.Spec.RevisionHistoryLimit == nil || *sts.Spec.RevisionHistoryLimit != 4 {
		t.Errorf("revision history limit not set")
	}

	if err := SetStatefulSetMinReadySeconds(sts, 5); err != nil {
		t.Fatalf("SetStatefulSetMinReadySeconds returned error: %v", err)
	}
	if sts.Spec.MinReadySeconds != 5 {
		t.Errorf("min ready seconds not set")
	}
}
