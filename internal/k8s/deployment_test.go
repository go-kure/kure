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
func TestDeploymentFunctions(t *testing.T) {
    dep := CreateDeployment("app", "ns")
    if dep.Name != "app" || dep.Namespace != "ns" {
        t.Fatalf("metadata mismatch: %s/%s", dep.Namespace, dep.Name)
    }
    if dep.Kind != "Deployment" {
        t.Errorf("unexpected kind %q", dep.Kind)
    }

    c := corev1.Container{Name: "c"}
    AddDeploymentContainer(dep, &c)
    if len(dep.Spec.Template.Spec.Containers) != 1 || dep.Spec.Template.Spec.Containers[0].Name != "c" {
        t.Errorf("container not added")
    }

    ic := corev1.Container{Name: "init"}
    AddDeploymentInitContainer(dep, &ic)
    if len(dep.Spec.Template.Spec.InitContainers) != 1 {
        t.Errorf("init container not added")
    }

    v := corev1.Volume{Name: "vol"}
    AddDeploymentVolume(dep, &v)
    if len(dep.Spec.Template.Spec.Volumes) != 1 {
        t.Errorf("volume not added")
    }

    secret := corev1.LocalObjectReference{Name: "secret"}
    AddDeploymentImagePullSecret(dep, &secret)
    if len(dep.Spec.Template.Spec.ImagePullSecrets) != 1 {
        t.Errorf("image pull secret not added")
    }

    tol := corev1.Toleration{Key: "k"}
    AddDeploymentToleration(dep, &tol)
    if len(dep.Spec.Template.Spec.Tolerations) != 1 {
        t.Errorf("toleration not added")
    }

    tsc := corev1.TopologySpreadConstraint{MaxSkew: 1, TopologyKey: "zone", WhenUnsatisfiable: corev1.ScheduleAnyway, LabelSelector: &metav1.LabelSelector{}}
    AddDeploymentTopologySpreadConstraints(dep, &tsc)
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
}
