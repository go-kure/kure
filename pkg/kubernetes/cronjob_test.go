package kubernetes

import (
	"reflect"
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateCronJob(t *testing.T) {
	cj := CreateCronJob("my-app", "default", "*/5 * * * *")
	if cj.Name != "my-app" || cj.Namespace != "default" {
		t.Fatalf("metadata mismatch: %s/%s", cj.Namespace, cj.Name)
	}
	if cj.Kind != "CronJob" {
		t.Errorf("unexpected kind %q", cj.Kind)
	}
	if cj.Labels["app"] != "my-app" {
		t.Errorf("expected label app=my-app, got %v", cj.Labels)
	}
	if cj.Spec.Schedule != "*/5 * * * *" {
		t.Errorf("expected schedule */5 * * * *, got %q", cj.Spec.Schedule)
	}
	if cj.Spec.JobTemplate.Spec.Template.Spec.RestartPolicy != corev1.RestartPolicyNever {
		t.Errorf("expected restart policy Never, got %q", cj.Spec.JobTemplate.Spec.Template.Spec.RestartPolicy)
	}
}

func TestCronJobNilErrors(t *testing.T) {
	// Functions with secondary nil checks — still return errors
	if err := SetCronJobPodSpec(nil, &corev1.PodSpec{}); err == nil {
		t.Error("expected error for nil CronJob on SetCronJobPodSpec")
	}
	if err := AddCronJobContainer(nil, &corev1.Container{Name: "c"}); err == nil {
		t.Error("expected error for nil CronJob on AddCronJobContainer")
	}
	if err := AddCronJobInitContainer(nil, &corev1.Container{Name: "c"}); err == nil {
		t.Error("expected error for nil CronJob on AddCronJobInitContainer")
	}
	if err := AddCronJobVolume(nil, &corev1.Volume{Name: "v"}); err == nil {
		t.Error("expected error for nil CronJob on AddCronJobVolume")
	}
	if err := AddCronJobImagePullSecret(nil, &corev1.LocalObjectReference{Name: "s"}); err == nil {
		t.Error("expected error for nil CronJob on AddCronJobImagePullSecret")
	}
	if err := AddCronJobToleration(nil, &corev1.Toleration{Key: "k"}); err == nil {
		t.Error("expected error for nil CronJob on AddCronJobToleration")
	}
	if err := AddCronJobTopologySpreadConstraint(nil, &corev1.TopologySpreadConstraint{}); err == nil {
		t.Error("expected error for nil CronJob on AddCronJobTopologySpreadConstraint")
	}

	// Functions that now panic on nil receiver
	assertPanics(t, func() { SetCronJobServiceAccountName(nil, "sa") })
	assertPanics(t, func() { SetCronJobSecurityContext(nil, nil) })
	assertPanics(t, func() { SetCronJobAffinity(nil, nil) })
	assertPanics(t, func() { SetCronJobNodeSelector(nil, nil) })
	assertPanics(t, func() { SetCronJobSchedule(nil, "* * * * *") })
	assertPanics(t, func() { SetCronJobConcurrencyPolicy(nil, batchv1.ForbidConcurrent) })
	assertPanics(t, func() { SetCronJobSuspend(nil, true) })
	assertPanics(t, func() { SetCronJobSuccessfulJobsHistoryLimit(nil, 3) })
	assertPanics(t, func() { SetCronJobFailedJobsHistoryLimit(nil, 1) })
	assertPanics(t, func() { SetCronJobStartingDeadlineSeconds(nil, 60) })
	tz := "UTC"
	assertPanics(t, func() { SetCronJobTimeZone(nil, &tz) })
}

func TestCronJobNilArgErrors(t *testing.T) {
	cj := CreateCronJob("test", "default", "* * * * *")
	if err := SetCronJobPodSpec(cj, nil); err == nil {
		t.Error("expected error for nil PodSpec")
	}
	if err := AddCronJobContainer(cj, nil); err == nil {
		t.Error("expected error for nil Container")
	}
	if err := AddCronJobInitContainer(cj, nil); err == nil {
		t.Error("expected error for nil InitContainer")
	}
	if err := AddCronJobVolume(cj, nil); err == nil {
		t.Error("expected error for nil Volume")
	}
	if err := AddCronJobImagePullSecret(cj, nil); err == nil {
		t.Error("expected error for nil ImagePullSecret")
	}
	if err := AddCronJobToleration(cj, nil); err == nil {
		t.Error("expected error for nil Toleration")
	}
}

func TestCronJobTopologySpreadConstraints(t *testing.T) {
	t.Run("nil constraint", func(t *testing.T) {
		cj := CreateCronJob("test", "default", "* * * * *")
		if err := AddCronJobTopologySpreadConstraint(cj, nil); err != nil {
			t.Fatalf("AddCronJobTopologySpreadConstraint returned error: %v", err)
		}
		if len(cj.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints) != 0 {
			t.Errorf("expected no constraints, got %d", len(cj.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints))
		}
	})

	t.Run("append single constraint", func(t *testing.T) {
		cj := CreateCronJob("test", "default", "* * * * *")
		c := corev1.TopologySpreadConstraint{
			MaxSkew:           1,
			TopologyKey:       "topology.kubernetes.io/zone",
			WhenUnsatisfiable: corev1.DoNotSchedule,
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test"},
			},
		}
		if err := AddCronJobTopologySpreadConstraint(cj, &c); err != nil {
			t.Fatalf("AddCronJobTopologySpreadConstraint returned error: %v", err)
		}
		if len(cj.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints) != 1 {
			t.Fatalf("expected 1 constraint, got %d", len(cj.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints))
		}
		if !reflect.DeepEqual(cj.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints[0], c) {
			t.Errorf("constraint mismatch: got %+v, want %+v", cj.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints[0], c)
		}
	})

	t.Run("append additional constraint", func(t *testing.T) {
		cj := CreateCronJob("test", "default", "* * * * *")
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
		if err := AddCronJobTopologySpreadConstraint(cj, &first); err != nil {
			t.Fatalf("AddCronJobTopologySpreadConstraint returned error: %v", err)
		}
		if err := AddCronJobTopologySpreadConstraint(cj, &second); err != nil {
			t.Fatalf("AddCronJobTopologySpreadConstraint returned error: %v", err)
		}
		if len(cj.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints) != 2 {
			t.Fatalf("expected 2 constraints, got %d", len(cj.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints))
		}
		if !reflect.DeepEqual(cj.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints[0], first) {
			t.Errorf("first constraint mismatch")
		}
		if !reflect.DeepEqual(cj.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints[1], second) {
			t.Errorf("second constraint mismatch")
		}
	})
}

func TestCronJobFunctions(t *testing.T) {
	cj := CreateCronJob("app", "ns", "* * * * *")
	if cj.Name != "app" || cj.Namespace != "ns" {
		t.Fatalf("metadata mismatch: %s/%s", cj.Namespace, cj.Name)
	}
	if cj.Kind != "CronJob" {
		t.Errorf("unexpected kind %q", cj.Kind)
	}

	c := corev1.Container{Name: "c"}
	if err := AddCronJobContainer(cj, &c); err != nil {
		t.Fatalf("AddCronJobContainer returned error: %v", err)
	}
	if len(cj.Spec.JobTemplate.Spec.Template.Spec.Containers) != 1 || cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Name != "c" {
		t.Errorf("container not added")
	}

	ic := corev1.Container{Name: "init"}
	if err := AddCronJobInitContainer(cj, &ic); err != nil {
		t.Fatalf("AddCronJobInitContainer returned error: %v", err)
	}
	if len(cj.Spec.JobTemplate.Spec.Template.Spec.InitContainers) != 1 {
		t.Errorf("init container not added")
	}

	v := corev1.Volume{Name: "vol"}
	if err := AddCronJobVolume(cj, &v); err != nil {
		t.Fatalf("AddCronJobVolume returned error: %v", err)
	}
	if len(cj.Spec.JobTemplate.Spec.Template.Spec.Volumes) != 1 {
		t.Errorf("volume not added")
	}

	secret := corev1.LocalObjectReference{Name: "secret"}
	if err := AddCronJobImagePullSecret(cj, &secret); err != nil {
		t.Fatalf("AddCronJobImagePullSecret returned error: %v", err)
	}
	if len(cj.Spec.JobTemplate.Spec.Template.Spec.ImagePullSecrets) != 1 {
		t.Errorf("image pull secret not added")
	}

	tol := corev1.Toleration{Key: "k"}
	if err := AddCronJobToleration(cj, &tol); err != nil {
		t.Fatalf("AddCronJobToleration returned error: %v", err)
	}
	if len(cj.Spec.JobTemplate.Spec.Template.Spec.Tolerations) != 1 {
		t.Errorf("toleration not added")
	}

	tsc := corev1.TopologySpreadConstraint{MaxSkew: 1, TopologyKey: "zone", WhenUnsatisfiable: corev1.ScheduleAnyway, LabelSelector: &metav1.LabelSelector{}}
	if err := AddCronJobTopologySpreadConstraint(cj, &tsc); err != nil {
		t.Fatalf("AddCronJobTopologySpreadConstraint returned error: %v", err)
	}
	if len(cj.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints) != 1 {
		t.Errorf("topology constraint not added")
	}

	SetCronJobServiceAccountName(cj, "sa")
	if cj.Spec.JobTemplate.Spec.Template.Spec.ServiceAccountName != "sa" {
		t.Errorf("service account name not set")
	}

	sc := &corev1.PodSecurityContext{RunAsUser: func(i int64) *int64 { return &i }(1)}
	SetCronJobSecurityContext(cj, sc)
	if cj.Spec.JobTemplate.Spec.Template.Spec.SecurityContext != sc {
		t.Errorf("security context not set")
	}

	aff := &corev1.Affinity{}
	SetCronJobAffinity(cj, aff)
	if cj.Spec.JobTemplate.Spec.Template.Spec.Affinity != aff {
		t.Errorf("affinity not set")
	}

	ns := map[string]string{"role": "db"}
	SetCronJobNodeSelector(cj, ns)
	if !reflect.DeepEqual(cj.Spec.JobTemplate.Spec.Template.Spec.NodeSelector, ns) {
		t.Errorf("node selector not set")
	}

	SetCronJobSchedule(cj, "*/5 * * * *")
	if cj.Spec.Schedule != "*/5 * * * *" {
		t.Errorf("schedule not updated")
	}

	SetCronJobConcurrencyPolicy(cj, batchv1.ForbidConcurrent)
	if cj.Spec.ConcurrencyPolicy != batchv1.ForbidConcurrent {
		t.Errorf("concurrency policy not set")
	}

	SetCronJobSuspend(cj, true)
	if cj.Spec.Suspend == nil || !*cj.Spec.Suspend {
		t.Errorf("suspend not set")
	}

	SetCronJobSuccessfulJobsHistoryLimit(cj, 1)
	if cj.Spec.SuccessfulJobsHistoryLimit == nil || *cj.Spec.SuccessfulJobsHistoryLimit != 1 {
		t.Errorf("successful jobs history limit not set")
	}

	SetCronJobFailedJobsHistoryLimit(cj, 2)
	if cj.Spec.FailedJobsHistoryLimit == nil || *cj.Spec.FailedJobsHistoryLimit != 2 {
		t.Errorf("failed jobs history limit not set")
	}

	SetCronJobStartingDeadlineSeconds(cj, 60)
	if cj.Spec.StartingDeadlineSeconds == nil || *cj.Spec.StartingDeadlineSeconds != 60 {
		t.Errorf("starting deadline seconds not set")
	}

	tz := "UTC"
	SetCronJobTimeZone(cj, &tz)
	if cj.Spec.TimeZone == nil || *cj.Spec.TimeZone != "UTC" {
		t.Errorf("timezone not set")
	}
}

func TestSetCronJobPodSpec(t *testing.T) {
	cj := CreateCronJob("test", "default", "* * * * *")
	spec := &corev1.PodSpec{
		Containers: []corev1.Container{
			{Name: "test", Image: "nginx"},
		},
	}
	if err := SetCronJobPodSpec(cj, spec); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(cj.Spec.JobTemplate.Spec.Template.Spec.Containers) != 1 {
		t.Fatal("expected PodSpec to be assigned")
	}
}
