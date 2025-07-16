package main

import (
	"os"

	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/cli-runtime/pkg/printers"

	"github.com/go-kure/kure/internal/k8s"
)

func ptr[T any](v T) *T { return &v }

func main() {
	y := printers.YAMLPrinter{}

	// Namespace example
	ns := k8s.CreateNamespace("demo")
	k8s.AddNamespaceLabel(ns, "env", "demo")
	k8s.AddNamespaceAnnotation(ns, "owner", "example")

	// Service account
	sa := k8s.CreateServiceAccount("demo-sa", "demo")
	k8s.AddServiceAccountSecret(sa, apiv1.ObjectReference{Name: "sa-secret"})
	k8s.AddServiceAccountImagePullSecret(sa, apiv1.LocalObjectReference{Name: "sa-pull"})
	k8s.SetServiceAccountAutomountToken(sa, true)

	// Secret example
	secret := k8s.CreateSecret("demo-secret", "demo")
	k8s.AddSecretData(secret, "cert", []byte("data"))
	k8s.AddSecretStringData(secret, "token", "abcd")
	k8s.SetSecretType(secret, apiv1.SecretTypeOpaque)
	k8s.SetSecretImmutable(secret, true)

	// ConfigMap example
	cm := k8s.CreateConfigMap("demo-config", "demo")
	k8s.AddConfigMapData(cm, "foo", "bar")
	k8s.AddConfigMapDataMap(cm, map[string]string{"extra": "value"})
	k8s.AddConfigMapBinaryData(cm, "bin", []byte{0x1})
	k8s.AddConfigMapBinaryDataMap(cm, map[string][]byte{"more": {0x2}})
	k8s.SetConfigMapData(cm, map[string]string{"hello": "world"})
	k8s.SetConfigMapBinaryData(cm, map[string][]byte{"bye": {0x0}})
	k8s.SetConfigMapImmutable(cm, false)

	// PersistentVolumeClaim example
	pvc := k8s.CreatePersistentVolumeClaim("demo-pvc", "demo")
	k8s.AddPVCAccessMode(pvc, apiv1.ReadWriteOnce)
	k8s.SetPVCStorageClassName(pvc, "standard")
	k8s.SetPVCVolumeMode(pvc, apiv1.PersistentVolumeFilesystem)
	k8s.SetPVCResources(pvc, apiv1.VolumeResourceRequirements{
		Requests: apiv1.ResourceList{
			apiv1.ResourceStorage: resource.MustParse("2Gi"),
		},
	})
	k8s.SetPVCSelector(pvc, &metav1.LabelSelector{MatchLabels: map[string]string{"disk": "fast"}})
	k8s.SetPVCVolumeName(pvc, "pv1")
	k8s.SetPVCDataSource(pvc, &apiv1.TypedLocalObjectReference{Kind: "PersistentVolumeClaim", Name: "source"})
	k8s.SetPVCDataSourceRef(pvc, &apiv1.TypedObjectReference{Kind: "PersistentVolumeClaim", Name: "source"})

	// Pod example
	pod := k8s.CreatePod("demo-pod", "demo")
	mainCtr := k8s.CreateContainer("app", "nginx", nil, nil)
	k8s.AddContainerPort(mainCtr, apiv1.ContainerPort{Name: "http", ContainerPort: 80})
	k8s.AddContainerEnv(mainCtr, apiv1.EnvVar{Name: "ENV", Value: "prod"})
	k8s.AddContainerEnvFrom(mainCtr, apiv1.EnvFromSource{ConfigMapRef: &apiv1.ConfigMapEnvSource{LocalObjectReference: apiv1.LocalObjectReference{Name: cm.Name}}})
	k8s.AddContainerVolumeMount(mainCtr, apiv1.VolumeMount{Name: "data", MountPath: "/data"})
	k8s.AddContainerVolumeDevice(mainCtr, apiv1.VolumeDevice{Name: "block", DevicePath: "/dev/block"})
	k8s.SetContainerLivenessProbe(mainCtr, apiv1.Probe{InitialDelaySeconds: 5})
	k8s.SetContainerReadinessProbe(mainCtr, apiv1.Probe{InitialDelaySeconds: 5})
	k8s.SetContainerStartupProbe(mainCtr, apiv1.Probe{InitialDelaySeconds: 1})
	k8s.SetContainerResources(mainCtr, apiv1.ResourceRequirements{
		Limits: apiv1.ResourceList{"memory": resource.MustParse("128Mi")},
		Requests: apiv1.ResourceList{
			"cpu":    resource.MustParse("50m"),
			"memory": resource.MustParse("64Mi"),
		},
	})
	k8s.SetContainerImagePullPolicy(mainCtr, apiv1.PullIfNotPresent)
	k8s.SetContainerSecurityContext(mainCtr, apiv1.SecurityContext{RunAsUser: ptr[int64](1000)})

	initCtr := k8s.CreateContainer("init", "busybox", []string{"sh", "-c"}, []string{"echo init"})

	k8s.AddPodContainer(pod, mainCtr)
	k8s.AddPodInitContainer(pod, initCtr)
	k8s.AddPodVolume(pod, &apiv1.Volume{Name: "data"})
	k8s.AddPodImagePullSecret(pod, &apiv1.LocalObjectReference{Name: "pullsecret"})
	k8s.AddPodToleration(pod, &apiv1.Toleration{Key: "role"})
	tsc := apiv1.TopologySpreadConstraint{MaxSkew: 1, TopologyKey: "zone", WhenUnsatisfiable: apiv1.DoNotSchedule, LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}}}
	k8s.AddPodTopologySpreadConstraints(pod, &tsc)
	k8s.SetPodServiceAccountName(pod, sa.Name)
	k8s.SetPodSecurityContext(pod, &apiv1.PodSecurityContext{})
	k8s.SetPodAffinity(pod, &apiv1.Affinity{})
	k8s.SetPodNodeSelector(pod, map[string]string{"type": "worker"})

	// Deployment example
	dep := k8s.CreateDeployment("demo-deployment", "demo")
	k8s.AddDeploymentContainer(dep, mainCtr)
	k8s.AddDeploymentInitContainer(dep, initCtr)
	k8s.AddDeploymentVolume(dep, &apiv1.Volume{Name: "data"})
	k8s.AddDeploymentImagePullSecret(dep, &apiv1.LocalObjectReference{Name: "pullsecret"})
	k8s.AddDeploymentToleration(dep, &apiv1.Toleration{Key: "role"})
	k8s.AddDeploymentTopologySpreadConstraints(dep, &tsc)
	k8s.SetDeploymentServiceAccountName(dep, sa.Name)
	k8s.SetDeploymentSecurityContext(dep, &apiv1.PodSecurityContext{})
	k8s.SetDeploymentAffinity(dep, &apiv1.Affinity{})
	k8s.SetDeploymentNodeSelector(dep, map[string]string{"role": "web"})

	// StatefulSet example
	sts := k8s.CreateStatefulSet("demo-sts", "demo")
	k8s.AddStatefulSetContainer(sts, mainCtr)
	k8s.AddStatefulSetInitContainer(sts, initCtr)
	k8s.AddStatefulSetVolume(sts, &apiv1.Volume{Name: "data"})
	k8s.AddStatefulSetVolumeClaimTemplate(sts, *k8s.CreatePersistentVolumeClaim("data", "demo"))
	k8s.AddStatefulSetToleration(sts, &apiv1.Toleration{Key: "role"})
	k8s.SetStatefulSetServiceAccountName(sts, sa.Name)
	k8s.SetStatefulSetServiceName(sts, "demo-svc")
	k8s.SetStatefulSetReplicas(sts, 3)

	// DaemonSet example
	ds := k8s.CreateDaemonSet("demo-ds", "demo")
	k8s.AddDaemonSetContainer(ds, mainCtr)
	k8s.AddDaemonSetInitContainer(ds, initCtr)
	k8s.AddDaemonSetVolume(ds, &apiv1.Volume{Name: "data"})
	k8s.AddDaemonSetToleration(ds, &apiv1.Toleration{Key: "role"})
	k8s.SetDaemonSetServiceAccountName(ds, sa.Name)
	k8s.SetDaemonSetNodeSelector(ds, map[string]string{"type": "worker"})

	// Service and ingress example
	svc := k8s.CreateService("demo-svc", "demo")
	k8s.SetServiceSelector(svc, map[string]string{"app": "demo"})
	k8s.AddServicePort(svc, apiv1.ServicePort{Name: "http", Port: 80, TargetPort: intstr.FromString("http")})
	k8s.SetServiceType(svc, apiv1.ServiceTypeClusterIP)
	k8s.SetServiceExternalTrafficPolicy(svc, apiv1.ServiceExternalTrafficPolicyCluster)

	ing := k8s.CreateIngress("demo-ing", "demo", "nginx")
	rule := k8s.CreateIngressRule("example.com")
	pathtype := netv1.PathTypePrefix
	path := k8s.CreateIngressPath("/", &pathtype, svc.Name, "http")
	k8s.AddIngressRulePath(rule, path)
	k8s.AddIngressRule(ing, rule)
	k8s.AddIngressTLS(ing, netv1.IngressTLS{Hosts: []string{"example.com"}, SecretName: secret.Name})

	// Print objects as YAML
	y.PrintObj(sa, os.Stdout)
	y.PrintObj(ns, os.Stdout)
	y.PrintObj(secret, os.Stdout)
	y.PrintObj(cm, os.Stdout)
	y.PrintObj(pvc, os.Stdout)
	y.PrintObj(pod, os.Stdout)
	y.PrintObj(dep, os.Stdout)
	y.PrintObj(sts, os.Stdout)
	y.PrintObj(ds, os.Stdout)
	y.PrintObj(svc, os.Stdout)
	y.PrintObj(ing, os.Stdout)
}
