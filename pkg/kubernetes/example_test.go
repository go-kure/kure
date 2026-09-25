package kubernetes_test

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	"github.com/go-kure/kure/pkg/kubernetes"
)

// The README's Go blocks are generated from these functions
// (scripts/gen-doc-examples.sh); each prints what it built so go test checks
// the example as well as compiling it.

func ExampleCreateDeployment() {
	d := kubernetes.CreateDeployment("web", "default")
	d.Spec.Replicas = ptr.To[int32](3)
	d.Spec.Template.Spec.ServiceAccountName = "web"
	fmt.Println(d.Kind, *d.Spec.Replicas, d.Spec.Template.Spec.ServiceAccountName)
	// Output: Deployment 3 web
}

func ExampleCreate() {
	d := kubernetes.Create[appsv1.Deployment]("web", "default")
	ns := kubernetes.Create[corev1.Namespace]("platform", "") // cluster-scoped: pass ""
	fmt.Println(d.APIVersion, d.Kind, ns.APIVersion, ns.Kind)
	// Output: apps/v1 Deployment v1 Namespace
}

func ExampleAddLabel() {
	obj := kubernetes.CreateDeployment("web", "default")

	kubernetes.SetLabels(obj, map[string]string{"app": "web"})
	kubernetes.AddLabel(obj, "tier", "frontend") // initialises a nil map
	kubernetes.SetAnnotations(obj, map[string]string{"owner": "platform"})
	kubernetes.AddAnnotation(obj, "note", "rotated 2026-09")
	fmt.Println(obj.Labels["app"], obj.Labels["tier"], obj.Annotations["note"])
	// Output: web frontend rotated 2026-09
}

func ExampleGetGroupVersionKind() {
	myDeployment := kubernetes.CreateDeployment("web", "default")
	allowedGVKs := []schema.GroupVersionKind{appsv1.SchemeGroupVersion.WithKind("Deployment")}

	// Lazily registers every supported API group (core K8s, FluxCD, cert-manager, ...)
	err := kubernetes.RegisterSchemes()
	if err != nil {
		panic(err)
	}

	// Resolve the GVK of any registered runtime.Object
	gvk, err := kubernetes.GetGroupVersionKind(myDeployment)
	if err != nil {
		panic(err)
	}

	// Check if a GVK is in an allow list
	ok := kubernetes.IsGVKAllowed(gvk, allowedGVKs)
	fmt.Println(gvk, ok)
	// Output: apps/v1, Kind=Deployment true
}

func ExampleSetDeploymentReplicas() {
	dep := kubernetes.CreateDeployment("my-app", "default")
	kubernetes.AddLabel(dep, "app", "my-app")
	dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"app": "my-app"}}
	dep.Spec.Template.Labels = map[string]string{"app": "my-app"}

	podSpec := &dep.Spec.Template.Spec
	kubernetes.AddPodSpecContainer(podSpec, &corev1.Container{Name: "app", Image: "nginx:1.25"})
	kubernetes.AddPodSpecToleration(podSpec, &corev1.Toleration{Key: "dedicated", Value: "web"})
	kubernetes.SetDeploymentReplicas(dep, 3)
	fmt.Println(podSpec.Containers[0].Image, podSpec.Tolerations[0].Key, *dep.Spec.Replicas)
	// Output: nginx:1.25 dedicated 3
}

func ExampleCreateJob() {
	job := kubernetes.CreateJob("migrate", "default")
	job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever

	kubernetes.AddPodSpecContainer(&job.Spec.Template.Spec,
		&corev1.Container{Name: "migrate", Image: "busybox:1.36"})
	fmt.Println(job.Spec.Template.Spec.RestartPolicy, job.Spec.Template.Spec.Containers[0].Name)
	// Output: Never migrate
}

func ExampleCreateCronJob() {
	cj := kubernetes.CreateCronJob("my-job", "default")
	cj.Spec.Schedule = "*/5 * * * *"
	cj.Spec.JobTemplate.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever

	kubernetes.AddPodSpecContainer(&cj.Spec.JobTemplate.Spec.Template.Spec,
		&corev1.Container{Name: "worker", Image: "busybox:1.36"})
	cj.Spec.ConcurrencyPolicy = batchv1.ForbidConcurrent
	fmt.Println(cj.Spec.Schedule, cj.Spec.ConcurrencyPolicy)
	// Output: */5 * * * * Forbid
}

func ExampleAddServicePort() {
	svc := kubernetes.CreateService("my-app", "default")
	svc.Spec.Selector = map[string]string{"app": "my-app"}
	kubernetes.AddServicePort(svc, corev1.ServicePort{Name: "http", Port: 80, TargetPort: intstr.FromInt32(8080)})
	svc.Spec.Type = corev1.ServiceTypeLoadBalancer
	kubernetes.AddAnnotation(svc, "external-dns.alpha.kubernetes.io/hostname", "app.example.com")
	fmt.Println(svc.Spec.Ports[0].Port, svc.Spec.Ports[0].TargetPort.IntValue(), svc.Spec.Type)
	// Output: 80 8080 LoadBalancer
}

func ExampleAddIngressRule() {
	ing := kubernetes.CreateIngress("my-app", "default")
	kubernetes.SetIngressClassName(ing, "nginx")

	rule := &netv1.IngressRule{Host: "app.example.com"}
	pt := netv1.PathTypePrefix
	path := kubernetes.CreateIngressPath("/", &pt, "my-app", "http")
	kubernetes.AddIngressRulePath(rule, path)
	kubernetes.AddIngressRule(ing, rule)
	kubernetes.AddIngressTLS(ing, netv1.IngressTLS{Hosts: []string{"app.example.com"}, SecretName: "my-app-tls"})
	fmt.Println(*ing.Spec.IngressClassName, ing.Spec.Rules[0].Host, ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name)
	// Output: nginx app.example.com my-app
}

func ExampleAddHPACPUMetric() {
	hpa := kubernetes.CreateHorizontalPodAutoscaler("my-app", "default")
	kubernetes.SetHPAScaleTargetRef(hpa, "apps/v1", "Deployment", "my-app")
	kubernetes.SetHPAMinMaxReplicas(hpa, 2, 10)
	kubernetes.AddHPACPUMetric(hpa, 80)

	pdb := kubernetes.CreatePodDisruptionBudget("my-app", "default")
	kubernetes.SetPDBMinAvailable(pdb, intstr.FromInt32(2))
	kubernetes.SetPDBSelector(pdb, &metav1.LabelSelector{MatchLabels: map[string]string{"app": "my-app"}})
	fmt.Println(*hpa.Spec.MinReplicas, hpa.Spec.MaxReplicas, *hpa.Spec.Metrics[0].Resource.Target.AverageUtilization, pdb.Spec.MinAvailable.IntValue())
	// Output: 2 10 80 2
}

func ExampleSetPDBMaxUnavailable() {
	pdb := kubernetes.CreatePodDisruptionBudget("my-app", "default")
	kubernetes.SetPDBMinAvailable(pdb, intstr.FromInt32(2))

	pdb.Spec.MinAvailable = nil
	kubernetes.SetPDBMaxUnavailable(pdb, intstr.FromString("25%"))
	fmt.Println(pdb.Spec.MinAvailable == nil, pdb.Spec.MaxUnavailable.String())
	// Output: true 25%
}

func ExamplePSALabels() {
	ns := kubernetes.CreateNamespace("my-app")
	kubernetes.AddLabel(ns, "env", "prod")

	// enforce, warn, audit; "" skips a mode, version "" omits the version labels
	for k, v := range kubernetes.PSALabels(kubernetes.PSARestricted, kubernetes.PSARestricted, kubernetes.PSARestricted, "v1.28") {
		kubernetes.AddLabel(ns, k, v)
	}
	fmt.Println(len(ns.Labels), ns.Labels["pod-security.kubernetes.io/enforce"])
	// Output: 7 restricted
}

func ExampleAddConfigMapData() {
	certBytes := []byte("-----BEGIN CERTIFICATE-----")
	defaults := map[string]string{"log-level": "info"}

	cm := kubernetes.CreateConfigMap("my-config", "default")
	kubernetes.AddConfigMapData(cm, "key", "value")
	kubernetes.AddConfigMapBinaryData(cm, "cert", certBytes)
	kubernetes.SetConfigMapImmutable(cm, true)

	// Replacing a map wholesale is an assignment, not a helper
	cm.Data = map[string]string{"key": "value"}

	// Merging one is a loop over the single-key helper
	for k, v := range defaults {
		kubernetes.AddConfigMapData(cm, k, v)
	}
	fmt.Println(len(cm.Data), cm.Data["log-level"], *cm.Immutable)
	// Output: 2 info true
}

func ExampleValidatePodSpecPSA() {
	container := &corev1.Container{Name: "app", Image: "nginx:1.25", SecurityContext: kubernetes.RestrictedSecurityContext()}
	podSpec := &corev1.PodSpec{Containers: []corev1.Container{*container}}

	psc, err := kubernetes.PodSecurityContextForLevel(kubernetes.PSARestricted)
	if err != nil {
		panic(err)
	}
	podSpec.SecurityContext = psc

	err = kubernetes.ValidateContainerPSA(container, kubernetes.PSARestricted)
	fmt.Println(*container.SecurityContext.RunAsNonRoot, err)
	err = kubernetes.ValidatePodSpecPSA(podSpec, kubernetes.PSARestricted)
	fmt.Println(err)
	// Output:
	// true <nil>
	// <nil>
}

func ExampleSetResourceRequest() {
	reqs := kubernetes.CreateResourceRequirements()
	kubernetes.SetResourceRequest(reqs, corev1.ResourceCPU, resource.MustParse("100m"))
	kubernetes.SetResourceLimit(reqs, corev1.ResourceMemory, resource.MustParse("512Mi"))
	fmt.Println(reqs.Requests.Cpu(), reqs.Limits.Memory())
	// Output: 100m 512Mi
}
