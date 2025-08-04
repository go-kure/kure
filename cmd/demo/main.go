package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"

	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/cli-runtime/pkg/printers"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	imagev1 "github.com/fluxcd/image-automation-controller/api/v1beta2"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"

	certmanagerapi "github.com/cert-manager/cert-manager/pkg/apis/certmanager"
	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"

	"github.com/go-kure/kure/internal/certmanager"
	"github.com/go-kure/kure/internal/kubernetes"
	"github.com/go-kure/kure/pkg/patch"
	"github.com/go-kure/kure/pkg/stack/layout"
	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/generators"

	"github.com/go-kure/kure/internal/externalsecrets"
	"github.com/go-kure/kure/internal/fluxcd"
	"github.com/go-kure/kure/internal/metallb"
	kio "github.com/go-kure/kure/pkg/io"

	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	"github.com/fluxcd/notification-controller/api/v1"
	notificationv1beta2 "github.com/fluxcd/notification-controller/api/v1beta2"
	"github.com/fluxcd/pkg/apis/meta"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
)

func ptr[T any](v T) *T { return &v }

func logError(msg string, err error) {
	if err != nil {
		log.Printf("%s: %v", msg, err)
	}
}

func main() {
	var (
		internals   bool
		appWorkload bool
		clusterDemo bool
		multiOCI    bool
		patchDemo   bool
		format      string
	)

	flag.BoolVar(&internals, "internals", false, "run internal demos")
	flag.BoolVar(&internals, "i", false, "run internal demos")
	flag.BoolVar(&appWorkload, "app-workload", false, "run AppWorkload example")
	flag.BoolVar(&appWorkload, "a", false, "run AppWorkload example")
	flag.BoolVar(&clusterDemo, "cluster", false, "run cluster example")
	flag.BoolVar(&clusterDemo, "c", false, "run cluster example")
	flag.BoolVar(&multiOCI, "multi-oci", false, "run multi-OCI package example")
	flag.BoolVar(&multiOCI, "m", false, "run multi-OCI package example")
	flag.BoolVar(&patchDemo, "patches", false, "run patch module demo")
	flag.BoolVar(&patchDemo, "p", false, "run patch module demo")
	flag.StringVar(&format, "format", "json", "output format: json|yaml|toml")
	flag.StringVar(&format, "f", "json", "output format: json|yaml|toml")
	flag.Parse()

	switch {
	case internals:
		runInternals()
	case appWorkload:
		if err := runAppWorkload(); err != nil {
			log.Printf("app-workload error: %v", err)
		}
	case clusterDemo:
		if err := runClusterExample(); err != nil {
			log.Printf("cluster error: %v", err)
		}
	case multiOCI:
		if err := runMultiOCIExample(); err != nil {
			log.Printf("multi-oci error: %v", err)
		}
	case patchDemo:
		if err := runPatchDemo(); err != nil {
			log.Printf("patch demo error: %v", err)
		}
	default:
		flag.Usage()
	}
}

func runInternals() {
	y := printers.YAMLPrinter{}

	// Namespace example
	ns := kubernetes.CreateNamespace("demo")
	kubernetes.AddNamespaceLabel(ns, "env", "demo")
	kubernetes.AddNamespaceAnnotation(ns, "owner", "example")

	// Service account
	sa := kubernetes.CreateServiceAccount("demo-sa", "demo")
	logError("add serviceaccount secret", kubernetes.AddServiceAccountSecret(sa, apiv1.ObjectReference{Name: "sa-secret"}))
	logError("add serviceaccount image pull secret", kubernetes.AddServiceAccountImagePullSecret(sa, apiv1.LocalObjectReference{Name: "sa-pull"}))
	logError("set serviceaccount automount token", kubernetes.SetServiceAccountAutomountToken(sa, true))

	// Secret example
	secret := kubernetes.CreateSecret("demo-secret", "demo")
	logError("add secret data", kubernetes.AddSecretData(secret, "cert", []byte("data")))
	logError("add secret string data", kubernetes.AddSecretStringData(secret, "token", "abcd"))
	logError("set secret type", kubernetes.SetSecretType(secret, apiv1.SecretTypeOpaque))
	logError("set secret immutable", kubernetes.SetSecretImmutable(secret, true))

	// ConfigMap example
	cm := kubernetes.CreateConfigMap("demo-config", "demo")
	logError("add configmap data", kubernetes.AddConfigMapData(cm, "foo", "bar"))
	logError("add configmap data map", kubernetes.AddConfigMapDataMap(cm, map[string]string{"extra": "value"}))
	logError("add configmap binary data", kubernetes.AddConfigMapBinaryData(cm, "bin", []byte{0x1}))
	logError("add configmap binary data map", kubernetes.AddConfigMapBinaryDataMap(cm, map[string][]byte{"more": {0x2}}))
	logError("set configmap data", kubernetes.SetConfigMapData(cm, map[string]string{"hello": "world"}))
	logError("set configmap binary data", kubernetes.SetConfigMapBinaryData(cm, map[string][]byte{"bye": {0x0}}))
	logError("set configmap immutable", kubernetes.SetConfigMapImmutable(cm, false))

	// PersistentVolumeClaim example
	pvc := kubernetes.CreatePersistentVolumeClaim("demo-pvc", "demo")
	kubernetes.AddPVCAccessMode(pvc, apiv1.ReadWriteOnce)
	kubernetes.SetPVCStorageClassName(pvc, "standard")
	kubernetes.SetPVCVolumeMode(pvc, apiv1.PersistentVolumeFilesystem)
	kubernetes.SetPVCResources(pvc, apiv1.VolumeResourceRequirements{
		Requests: apiv1.ResourceList{
			apiv1.ResourceStorage: resource.MustParse("2Gi"),
		},
	})
	kubernetes.SetPVCSelector(pvc, &metav1.LabelSelector{MatchLabels: map[string]string{"disk": "fast"}})
	kubernetes.SetPVCVolumeName(pvc, "pv1")
	kubernetes.SetPVCDataSource(pvc, &apiv1.TypedLocalObjectReference{Kind: "PersistentVolumeClaim", Name: "source"})
	kubernetes.SetPVCDataSourceRef(pvc, &apiv1.TypedObjectReference{Kind: "PersistentVolumeClaim", Name: "source"})

	// StorageClass example
	sc := kubernetes.CreateStorageClass("demo-sc", "kubernetes.io/no-provisioner")
	kubernetes.AddStorageClassParameter(sc, "type", "local")
	kubernetes.SetStorageClassAllowVolumeExpansion(sc, true)
	kubernetes.SetPVCStorageClass(pvc, sc)

	// Pod example
	pod := kubernetes.CreatePod("demo-pod", "demo")
	mainCtr := kubernetes.CreateContainer("app", "nginx", nil, nil)
	logError("add container port", kubernetes.AddContainerPort(mainCtr, apiv1.ContainerPort{Name: "http", ContainerPort: 80}))
	logError("add container env", kubernetes.AddContainerEnv(mainCtr, apiv1.EnvVar{Name: "ENV", Value: "prod"}))
	logError("add container env from", kubernetes.AddContainerEnvFrom(mainCtr, apiv1.EnvFromSource{ConfigMapRef: &apiv1.ConfigMapEnvSource{LocalObjectReference: apiv1.LocalObjectReference{Name: cm.Name}}}))
	logError("add container volume mount", kubernetes.AddContainerVolumeMount(mainCtr, apiv1.VolumeMount{Name: "data", MountPath: "/data"}))
	logError("add container volume device", kubernetes.AddContainerVolumeDevice(mainCtr, apiv1.VolumeDevice{Name: "block", DevicePath: "/dev/block"}))
	logError("set container liveness probe", kubernetes.SetContainerLivenessProbe(mainCtr, apiv1.Probe{InitialDelaySeconds: 5}))
	logError("set container readiness probe", kubernetes.SetContainerReadinessProbe(mainCtr, apiv1.Probe{InitialDelaySeconds: 5}))
	logError("set container startup probe", kubernetes.SetContainerStartupProbe(mainCtr, apiv1.Probe{InitialDelaySeconds: 1}))
	logError("set container resources", kubernetes.SetContainerResources(mainCtr, apiv1.ResourceRequirements{
		Limits: apiv1.ResourceList{"memory": resource.MustParse("128Mi")},
		Requests: apiv1.ResourceList{
			"cpu":    resource.MustParse("50m"),
			"memory": resource.MustParse("64Mi"),
		},
	}))
	logError("set container image pull policy", kubernetes.SetContainerImagePullPolicy(mainCtr, apiv1.PullIfNotPresent))
	logError("set container security context", kubernetes.SetContainerSecurityContext(mainCtr, apiv1.SecurityContext{RunAsUser: ptr[int64](1000)}))

	initCtr := kubernetes.CreateContainer("init", "busybox", []string{"sh", "-c"}, []string{"echo init"})

	logError("add pod container", kubernetes.AddPodContainer(pod, mainCtr))
	logError("add pod init container", kubernetes.AddPodInitContainer(pod, initCtr))
	logError("add pod volume", kubernetes.AddPodVolume(pod, &apiv1.Volume{Name: "data"}))
	logError("add pod image pull secret", kubernetes.AddPodImagePullSecret(pod, &apiv1.LocalObjectReference{Name: "pullsecret"}))
	logError("add pod toleration", kubernetes.AddPodToleration(pod, &apiv1.Toleration{Key: "role"}))
	tsc := apiv1.TopologySpreadConstraint{MaxSkew: 1, TopologyKey: "zone", WhenUnsatisfiable: apiv1.DoNotSchedule, LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}}}
	logError("add pod topology spread constraints", kubernetes.AddPodTopologySpreadConstraints(pod, &tsc))
	logError("set pod serviceaccount name", kubernetes.SetPodServiceAccountName(pod, sa.Name))
	logError("set pod security context", kubernetes.SetPodSecurityContext(pod, &apiv1.PodSecurityContext{}))
	logError("set pod affinity", kubernetes.SetPodAffinity(pod, &apiv1.Affinity{}))
	logError("set pod node selector", kubernetes.SetPodNodeSelector(pod, map[string]string{"type": "worker"}))

	// Deployment example
	dep := kubernetes.CreateDeployment("demo-deployment", "demo")
	logError("add deployment container", kubernetes.AddDeploymentContainer(dep, mainCtr))
	logError("add deployment init container", kubernetes.AddDeploymentInitContainer(dep, initCtr))
	logError("add deployment volume", kubernetes.AddDeploymentVolume(dep, &apiv1.Volume{Name: "data"}))
	logError("add deployment image pull secret", kubernetes.AddDeploymentImagePullSecret(dep, &apiv1.LocalObjectReference{Name: "pullsecret"}))
	logError("add deployment toleration", kubernetes.AddDeploymentToleration(dep, &apiv1.Toleration{Key: "role"}))
	logError("add deployment topology spread constraints", kubernetes.AddDeploymentTopologySpreadConstraints(dep, &tsc))
	logError("set deployment serviceaccount name", kubernetes.SetDeploymentServiceAccountName(dep, sa.Name))
	logError("set deployment security context", kubernetes.SetDeploymentSecurityContext(dep, &apiv1.PodSecurityContext{}))
	logError("set deployment affinity", kubernetes.SetDeploymentAffinity(dep, &apiv1.Affinity{}))
	logError("set deployment node selector", kubernetes.SetDeploymentNodeSelector(dep, map[string]string{"role": "web"}))

	// StatefulSet example
	sts := kubernetes.CreateStatefulSet("demo-sts", "demo")
	logError("add statefulset container", kubernetes.AddStatefulSetContainer(sts, mainCtr))
	logError("add statefulset init container", kubernetes.AddStatefulSetInitContainer(sts, initCtr))
	logError("add statefulset volume", kubernetes.AddStatefulSetVolume(sts, &apiv1.Volume{Name: "data"}))
	logError("add statefulset volumeclaim template", kubernetes.AddStatefulSetVolumeClaimTemplate(sts, *kubernetes.CreatePersistentVolumeClaim("data", "demo")))
	logError("add statefulset toleration", kubernetes.AddStatefulSetToleration(sts, &apiv1.Toleration{Key: "role"}))
	logError("set statefulset serviceaccount name", kubernetes.SetStatefulSetServiceAccountName(sts, sa.Name))
	logError("set statefulset service name", kubernetes.SetStatefulSetServiceName(sts, "demo-svc"))
	logError("set statefulset replicas", kubernetes.SetStatefulSetReplicas(sts, 3))

	// DaemonSet example
	ds := kubernetes.CreateDaemonSet("demo-ds", "demo")
	logError("add daemonset container", kubernetes.AddDaemonSetContainer(ds, mainCtr))
	logError("add daemonset init container", kubernetes.AddDaemonSetInitContainer(ds, initCtr))
	logError("add daemonset volume", kubernetes.AddDaemonSetVolume(ds, &apiv1.Volume{Name: "data"}))
	logError("add daemonset toleration", kubernetes.AddDaemonSetToleration(ds, &apiv1.Toleration{Key: "role"}))
	logError("set daemonset serviceaccount name", kubernetes.SetDaemonSetServiceAccountName(ds, sa.Name))
	logError("set daemonset node selector", kubernetes.SetDaemonSetNodeSelector(ds, map[string]string{"type": "worker"}))

	// Job example
	job := kubernetes.CreateJob("demo-job", "demo")
	logError("add job container", kubernetes.AddJobContainer(job, mainCtr))
	logError("set job backoff limit", kubernetes.SetJobBackoffLimit(job, 3))
	logError("set job completions", kubernetes.SetJobCompletions(job, 1))
	logError("set job parallelism", kubernetes.SetJobParallelism(job, 1))

	// CronJob example
	cron := kubernetes.CreateCronJob("demo-cron", "demo", "*/5 * * * *")
	logError("add cronjob container", kubernetes.AddCronJobContainer(cron, mainCtr))
	logError("set cronjob concurrency policy", kubernetes.SetCronJobConcurrencyPolicy(cron, batchv1.ForbidConcurrent))

	// Service and ingress example
	svc := kubernetes.CreateService("demo-svc", "demo")
	logError("set service selector", kubernetes.SetServiceSelector(svc, map[string]string{"app": "demo"}))
	logError("add service port", kubernetes.AddServicePort(svc, apiv1.ServicePort{Name: "http", Port: 80, TargetPort: intstr.FromString("http")}))
	logError("set service type", kubernetes.SetServiceType(svc, apiv1.ServiceTypeClusterIP))
	logError("set service external traffic policy", kubernetes.SetServiceExternalTrafficPolicy(svc, apiv1.ServiceExternalTrafficPolicyCluster))

	ing := kubernetes.CreateIngress("demo-ing", "demo", "nginx")
	rule := kubernetes.CreateIngressRule("example.com")
	pathtype := netv1.PathTypePrefix
	path := kubernetes.CreateIngressPath("/", &pathtype, svc.Name, "http")
	kubernetes.AddIngressRulePath(rule, path)
	kubernetes.AddIngressRule(ing, rule)
	kubernetes.AddIngressTLS(ing, netv1.IngressTLS{Hosts: []string{"example.com"}, SecretName: secret.Name})

	// RBAC examples
	role := kubernetes.CreateRole("demo-role", "demo")
	kubernetes.AddRoleRule(role, rbacv1.PolicyRule{Verbs: []string{"get"}, Resources: []string{"pods"}})

	clusterRole := kubernetes.CreateClusterRole("demo-cr")
	kubernetes.AddClusterRoleRule(clusterRole, rbacv1.PolicyRule{Verbs: []string{"list"}, Resources: []string{"nodes"}})

	roleBind := kubernetes.CreateRoleBinding("demo-rb", "demo", rbacv1.RoleRef{Kind: "Role", Name: role.Name})
	kubernetes.AddRoleBindingSubject(roleBind, rbacv1.Subject{Kind: "ServiceAccount", Name: sa.Name, Namespace: sa.Namespace})

	clusterRoleBind := kubernetes.CreateClusterRoleBinding("demo-crb", rbacv1.RoleRef{Kind: "ClusterRole", Name: clusterRole.Name})
	kubernetes.AddClusterRoleBindingSubject(clusterRoleBind, rbacv1.Subject{Kind: "User", Name: "admin"})

	// HelmRelease example
	hr := fluxcd.CreateHelmRelease("demo-hr", "demo", helmv2.HelmReleaseSpec{})
	fluxcd.SetHelmReleaseReleaseName(hr, "demo")
	fluxcd.SetHelmReleaseInterval(hr, metav1.Duration{Duration: time.Minute})
	fluxcd.AddHelmReleaseLabel(hr, "app", "demo")
	fluxcd.SetHelmReleaseChart(hr, &helmv2.HelmChartTemplate{
		Spec: helmv2.HelmChartTemplateSpec{
			Chart:   "demo",
			Version: "1.0.0",
			SourceRef: helmv2.CrossNamespaceObjectReference{
				Kind:      "HelmRepository",
				Name:      "demo",
				Namespace: "demo",
			},
		},
	})

	// Kustomization example
	ks := fluxcd.CreateKustomization("demo-ks", "demo", kustv1.KustomizationSpec{Path: "./manifests", Prune: true})
	fluxcd.SetKustomizationInterval(ks, metav1.Duration{Duration: time.Minute})
	fluxcd.SetKustomizationSourceRef(ks, kustv1.CrossNamespaceSourceReference{Kind: "GitRepository", Name: "demo-repo"})
	fluxcd.AddKustomizationImage(ks, kustomize.Image{Name: "nginx", NewTag: "latest"})

	// Flux source-controller examples
	gitRepo := fluxcd.CreateGitRepository("demo-git", "demo", sourcev1.GitRepositorySpec{})
	fluxcd.SetGitRepositoryURL(gitRepo, "https://github.com/example/repo.git")
	fluxcd.SetGitRepositoryInterval(gitRepo, metav1.Duration{Duration: time.Minute})

	helmRepo := fluxcd.CreateHelmRepository("demo-helm", "demo", sourcev1.HelmRepositorySpec{})
	fluxcd.SetHelmRepositoryURL(helmRepo, "https://charts.example.com")
	fluxcd.SetHelmRepositoryInterval(helmRepo, metav1.Duration{Duration: time.Hour})

	bucket := fluxcd.CreateBucket("demo-bucket", "demo", sourcev1.BucketSpec{})
	fluxcd.SetBucketName(bucket, "artifacts")
	fluxcd.SetBucketEndpoint(bucket, "https://s3.example.com")

	chart := fluxcd.CreateHelmChart("demo-chart", "demo", sourcev1.HelmChartSpec{})
	fluxcd.SetHelmChartChart(chart, "app")

	ociRepo := fluxcd.CreateOCIRepository("demo-oci", "demo", sourcev1beta2.OCIRepositorySpec{})
	fluxcd.SetOCIRepositoryURL(ociRepo, "oci://registry/app")

	// Network policy example
	np := kubernetes.CreateNetworkPolicy("demo-netpol", "demo")
	npRule := netv1.NetworkPolicyIngressRule{}
	peer := netv1.NetworkPolicyPeer{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}}}
	kubernetes.AddNetworkPolicyIngressPeer(&npRule, peer)
	kubernetes.AddNetworkPolicyIngressRule(np, npRule)
	kubernetes.AddNetworkPolicyPolicyType(np, netv1.PolicyTypeIngress)

	// Resource quota example
	rq := kubernetes.CreateResourceQuota("demo-quota", "demo")
	kubernetes.AddResourceQuotaHard(rq, apiv1.ResourceRequestsStorage, resource.MustParse("1Gi"))

	// Limit range example
	lr := kubernetes.CreateLimitRange("demo-limits", "demo")
	lrItem := apiv1.LimitRangeItem{Type: apiv1.LimitTypeContainer}
	kubernetes.AddLimitRangeItemDefaultRequest(&lrItem, apiv1.ResourceCPU, resource.MustParse("100m"))
	kubernetes.AddLimitRangeItem(lr, lrItem)

	// Notification controller examples
	provider := fluxcd.CreateProvider("demo-provider", "demo", notificationv1beta2.ProviderSpec{Type: notificationv1beta2.SlackProvider})
	alert := fluxcd.CreateAlert("demo-alert", "demo", notificationv1beta2.AlertSpec{
		ProviderRef:  meta.LocalObjectReference{Name: provider.Name},
		EventSources: []v1.CrossNamespaceObjectReference{{Kind: "Kustomization", Name: dep.Name, Namespace: dep.Namespace}},
	})
	receiver := fluxcd.CreateReceiver("demo-receiver", "demo", notificationv1beta2.ReceiverSpec{
		Type:      notificationv1beta2.GitHubReceiver,
		Resources: []v1.CrossNamespaceObjectReference{{Kind: "Kustomization", Name: dep.Name, Namespace: dep.Namespace}},
		SecretRef: meta.LocalObjectReference{Name: "webhook-secret"},
	})

	// Image automation example
	author := fluxcd.CreateCommitUser("Flux Bot", "bot@example.com")
	commit := fluxcd.CreateCommitSpec(author)
	gitSpec := fluxcd.CreateGitSpec(commit, nil, nil)
	autoSpec := imagev1.ImageUpdateAutomationSpec{
		SourceRef: fluxcd.CreateCrossNamespaceSourceReference("", "GitRepository", "demo-repo", "demo"),
		Interval:  metav1.Duration{Duration: time.Minute},
		GitSpec:   gitSpec,
	}
	auto := fluxcd.CreateImageUpdateAutomation("demo-auto", "demo", autoSpec)
	fluxcd.SetImageUpdateAutomationSuspend(auto, false)

	// cert-manager examples
	issuer := certmanager.CreateIssuer("demo-issuer", "demo", certv1.IssuerSpec{})
	acme := certmanager.CreateACMEIssuer("https://acme.example.com", "ops@example.com",
		cmmeta.SecretKeySelector{LocalObjectReference: cmmeta.LocalObjectReference{Name: "acme-key"}})
	certmanager.AddACMEIssuerSolver(acme, certmanager.CreateACMEHTTP01Solver(apiv1.ServiceTypeNodePort, "nginx"))
	certmanager.SetIssuerACME(issuer, acme)

	certmanager.SetIssuerCA(issuer, &certv1.CAIssuer{SecretName: "ca-key"})
	cert := certmanager.CreateCertificate("demo-cert", "demo", certv1.CertificateSpec{})
	certmanager.AddCertificateDNSName(cert, "example.com")
	certmanager.SetCertificateIssuerRef(cert, cmmeta.ObjectReference{Name: issuer.Name, Kind: "Issuer", Group: certmanagerapi.GroupName})
	clusterIssuer := certmanager.CreateClusterIssuer("demo-clusterissuer", certv1.IssuerSpec{})

	// metallb examples
	pool := metallb.CreateIPAddressPool("demo-pool", "demo", metallbv1beta1.IPAddressPoolSpec{Addresses: []string{"172.18.0.0/24"}})
	logError("add ipaddresspool address", metallb.AddIPAddressPoolAddress(pool, "172.19.0.0/24"))
	l2adv := metallb.CreateL2Advertisement("demo-l2adv", "demo", metallbv1beta1.L2AdvertisementSpec{})
	logError("add l2advertisement ipaddresspool", metallb.AddL2AdvertisementIPAddressPool(l2adv, pool.Name))
	bgpadv := metallb.CreateBGPAdvertisement("demo-bgpadv", "demo", metallbv1beta1.BGPAdvertisementSpec{})
	logError("set bgpadvertisement localpref", metallb.SetBGPAdvertisementLocalPref(bgpadv, 100))
	bgpPeer := metallb.CreateBGPPeer("demo-peer", "demo", metallbv1beta1.BGPPeerSpec{MyASN: 64512, ASN: 64512, Address: "10.0.0.2"})
	bfd := metallb.CreateBFDProfile("demo-bfd", "demo", metallbv1beta1.BFDProfileSpec{})

	// external-secrets examples
	ss := externalsecrets.CreateSecretStore("demo-store", "demo", esv1.SecretStoreSpec{})
	ssProvider := &esv1.SecretStoreProvider{AWS: &esv1.AWSProvider{Service: esv1.AWSServiceSecretsManager, Region: "us-east-1"}}
	externalsecrets.SetSecretStoreProvider(ss, ssProvider)
	css := externalsecrets.CreateClusterSecretStore("demo-css", esv1.SecretStoreSpec{})
	externalsecrets.SetClusterSecretStoreProvider(css, ssProvider)
	es := externalsecrets.CreateExternalSecret("demo-external", "demo", esv1.ExternalSecretSpec{})
	externalsecrets.AddExternalSecretData(es, esv1.ExternalSecretData{SecretKey: "password", RemoteRef: esv1.ExternalSecretDataRemoteRef{Key: "db/password"}})
	externalsecrets.SetExternalSecretSecretStoreRef(es, esv1.SecretStoreRef{Name: ss.Name})

	// flux-operator examples
	fi := fluxcd.CreateFluxInstance("flux", "flux-system", fluxv1.FluxInstanceSpec{
		Distribution: fluxv1.Distribution{Version: "2.x", Registry: "ghcr.io/fluxcd"},
	})
	logError("add flux instance component", fluxcd.AddFluxInstanceComponent(fi, "source-controller"))

	fr := fluxcd.CreateFluxReport("flux", "flux-system", fluxv1.FluxReportSpec{
		Distribution: fluxv1.FluxDistributionStatus{Entitlement: "oss", Status: "Running"},
	})

	rs := fluxcd.CreateResourceSet("demo-rs", "demo", fluxv1.ResourceSetSpec{})
	logError("add resource set resource", fluxcd.AddResourceSetResource(rs, &apiextensionsv1.JSON{Raw: []byte("{}")}))

	prov := fluxcd.CreateResourceSetInputProvider("demo-rsip", "demo", fluxv1.ResourceSetInputProviderSpec{Type: fluxv1.InputProviderStatic})
	logError("add resource set input provider schedule", fluxcd.AddResourceSetInputProviderSchedule(prov, fluxcd.CreateSchedule("@daily")))

	// Print objects as YAML
	objects := []runtime.Object{
		sa, ns, secret, cm, pvc, pod, dep, sts, ds, job, cron, svc, ing,
		role, roleBind, clusterRole, clusterRoleBind, hr, ks, gitRepo,
		helmRepo, bucket, chart, ociRepo, pool, l2adv, bgpadv, bgpPeer,
		bfd, ss, css, es, fi, fr, rs, prov, np, rq, lr, provider, alert,
		receiver, auto, issuer, clusterIssuer, cert,
	}

	for _, obj := range objects {
		logError("failed to print YAML", y.PrintObj(obj, os.Stdout))
	}
}

func runAppWorkload() error {
	file, err := os.Open("examples/app-workload.yaml")
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {

		}
	}(file)

	dec := yaml.NewDecoder(file)
	var apps []*stack.Application
	for {
		var cfg generators.AppWorkloadConfig
		if err := dec.Decode(&cfg); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		app := stack.NewApplication(cfg.Name, cfg.Namespace, &cfg)
		apps = append(apps, app)
	}

	bundle, err := stack.NewBundle("example", apps, nil)
	if err != nil {
		return err
	}
	err = bundle.Validate()
	if err != nil {
		return err
	}
	resources, err := bundle.Generate()
	if err != nil {
		return err
	}

	out, err := kio.EncodeObjectsToYAML(resources)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(out)
	if err == nil {
		_, err = os.Stdout.Write([]byte("\n"))
	}
	return err
}

func runClusterExample() error {
	file, err := os.Open("examples/cluster/cluster.yaml")
	if err != nil {
		return err
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(file)

	dec := yaml.NewDecoder(file)
	var cl stack.Cluster
	if err := dec.Decode(&cl); err != nil {
		return err
	}
	if cl.Node == nil {
		return nil
	}

	rootBundle, err := stack.NewBundle(cl.Node.Name, nil, nil)
	if err != nil {
		return err
	}
	cl.Node.Bundle = rootBundle

	baseDir := "examples/cluster"
	for _, child := range cl.Node.Children {
		child.Parent = cl.Node
		childBundle, err := stack.NewBundle(child.Name, nil, nil)
		if err != nil {
			return err
		}
		child.Bundle = childBundle
		childBundle.Parent = rootBundle
		dir := filepath.Join(baseDir, child.Name)
		entries, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			ext := filepath.Ext(entry.Name())
			if ext != ".yaml" && ext != ".yml" {
				continue
			}
			fp := filepath.Join(dir, entry.Name())
			f, err := os.Open(fp)
			if err != nil {
				return err
			}
			dec := yaml.NewDecoder(f)
			for {
				var cfg generators.AppWorkloadConfig
				if err := dec.Decode(&cfg); err != nil {
					if err == io.EOF {
						break
					}
					_ = f.Close()
					return err
				}
				app := stack.NewApplication(cfg.Name, cfg.Namespace, &cfg)
				bundle, err := stack.NewBundle(cfg.Name, []*stack.Application{app}, nil)
				if err != nil {
					_ = f.Close()
					return err
				}
				bundle.Parent = child.Bundle
				node := &stack.Node{Name: cfg.Name, Parent: child, Bundle: bundle}
				child.Children = append(child.Children, node)
			}
			_ = f.Close()
		}
	}

	repoDir := filepath.Join("out", "cluster-repo")
	if err := os.RemoveAll(repoDir); err != nil {
		return err
	}
	
	// Configure layout for proper GitOps structure
	cfg := layout.Config{ManifestsDir: "clusters"}
	rules := layout.LayoutRules{
		NodeGrouping:        layout.GroupByName,
		BundleGrouping:      layout.GroupFlat,       // Flatten bundles to avoid bundle/app/app/app nesting
		ApplicationGrouping: layout.GroupByName,     // Group applications by name for individual directories
		ClusterName:         cl.Name,         // Use cluster name from cluster.yaml
		FluxPlacement:       layout.FluxIntegrated, // Use integrated Flux placement
	}
	
	// Generate integrated layout with Flux Kustomizations
	wf := fluxstack.NewWorkflow()
	ml, err := wf.ClusterWithLayout(&cl, rules)
	if err != nil {
		return err
	}
	
	// Write the complete integrated layout
	if err := layout.WriteManifest(repoDir, cfg, ml); err != nil {
		return err
	}
	log.Printf("manifests written to %s", repoDir)
	return nil
}

// runMultiOCIExample demonstrates the new multi-OCI package functionality
func runMultiOCIExample() error {
	log.Println("Running multi-OCI package demo...")

	// Define different package references
	ociPackageRef := &schema.GroupVersionKind{
		Group:   "source.toolkit.fluxcd.io",
		Version: "v1beta2",
		Kind:    "OCIRepository",
	}
	gitPackageRef := &schema.GroupVersionKind{
		Group:   "source.toolkit.fluxcd.io",
		Version: "v1",
		Kind:    "GitRepository",
	}

	// Create applications for different packages
	appConfig1 := &generators.AppWorkloadConfig{
		Name:      "web-app",
		Namespace: "production",
		Workload:  generators.DeploymentWorkload,
		Replicas:  3,
		Containers: []generators.ContainerConfig{
			{
				Name:  "nginx",
				Image: "nginx:latest",
				Ports: []generators.ContainerPort{
					{ContainerPort: 80, Protocol: "TCP"},
				},
			},
		},
	}
	app1 := stack.NewApplication(appConfig1.Name, appConfig1.Namespace, appConfig1)
	bundle1, err := stack.NewBundle("web-apps", []*stack.Application{app1}, nil)
	if err != nil {
		return err
	}

	appConfig2 := &generators.AppWorkloadConfig{
		Name:      "database",
		Namespace: "production",
		Workload:  generators.StatefulSetWorkload,
		Replicas:  1,
		Containers: []generators.ContainerConfig{
			{
				Name:  "postgres",
				Image: "postgres:13",
				Ports: []generators.ContainerPort{
					{ContainerPort: 5432, Protocol: "TCP"},
				},
			},
		},
	}
	app2 := stack.NewApplication(appConfig2.Name, appConfig2.Namespace, appConfig2)
	bundle2, err := stack.NewBundle("databases", []*stack.Application{app2}, nil)
	if err != nil {
		return err
	}

	appConfig3 := &generators.AppWorkloadConfig{
		Name:      "monitoring",
		Namespace: "monitoring",
		Workload:  generators.DeploymentWorkload,
		Replicas:  1,
		Containers: []generators.ContainerConfig{
			{
				Name:  "prometheus",
				Image: "prometheus:latest",
				Ports: []generators.ContainerPort{
					{ContainerPort: 9090, Protocol: "TCP"},
				},
			},
		},
	}
	app3 := stack.NewApplication(appConfig3.Name, appConfig3.Namespace, appConfig3)
	bundle3, err := stack.NewBundle("monitoring", []*stack.Application{app3}, nil)
	if err != nil {
		return err
	}

	// Create nodes with different package references
	// OCI package will contain web apps and databases
	webNode := &stack.Node{
		Name:       "web",
		Bundle:     bundle1,
		PackageRef: ociPackageRef,
	}
	dbNode := &stack.Node{
		Name:       "database",
		Bundle:     bundle2,
		PackageRef: ociPackageRef, // Same package as web apps
	}

	// Git package will contain monitoring
	monitoringNode := &stack.Node{
		Name:       "monitoring",
		Bundle:     bundle3,
		PackageRef: gitPackageRef, // Different package
	}

	// Create root cluster
	root := &stack.Node{
		Name:     "cluster",
		Children: []*stack.Node{webNode, dbNode, monitoringNode},
	}
	webNode.Parent = root
	dbNode.Parent = root
	monitoringNode.Parent = root

	cluster := &stack.Cluster{Name: "multi-oci-demo", Node: root}

	// Demonstrate package separation
	packages, err := layout.WalkClusterByPackage(cluster, layout.LayoutRules{})
	if err != nil {
		return err
	}

	log.Printf("Found %d packages:", len(packages))
	for key, layout := range packages {
		log.Printf("  Package: %s", key)
		if layout != nil {
			log.Printf("    Children: %d", len(layout.Children))
			for _, child := range layout.Children {
				log.Printf("      - %s (namespace: %s)", child.Name, child.Namespace)
			}
		}
	}

	// Write each package to separate directories
	baseDir := "examples/multi-oci-demo"
	if err := os.RemoveAll(baseDir); err != nil {
		return err
	}
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return err
	}

	if err := layout.WritePackagesToDisk(packages, baseDir); err != nil {
		return err
	}
	log.Printf("Package manifests written to %s", baseDir)

	// Demonstrate Flux workflow by package
	wf := fluxstack.NewWorkflow()
	fluxPackages, err := wf.ClusterByPackage(cluster)
	if err != nil {
		return err
	}

	log.Printf("Found %d Flux packages:", len(fluxPackages))
	for key, objs := range fluxPackages {
		log.Printf("  Flux Package: %s (%d objects)", key, len(objs))
		for _, obj := range objs {
			log.Printf("    - %s/%s", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
		}
	}

	// Write Flux configurations for each package
	fluxDir := filepath.Join("out", "multi-oci-flux")
	if err := os.MkdirAll(fluxDir, 0755); err != nil {
		return err
	}

	cfg := layout.Config{ManifestsDir: ""}
	for packageKey, objs := range fluxPackages {
		fluxLayout := &layout.ManifestLayout{
			Name:      packageKey,
			Namespace: ".",
			Resources: objs,
		}
		packageDir := filepath.Join(fluxDir, sanitizePackageName(packageKey))
		if err := layout.WriteManifest(packageDir, cfg, fluxLayout); err != nil {
			return err
		}
	}
	log.Printf("Flux manifests written to %s", fluxDir)

	return nil
}

// sanitizePackageName converts package reference strings to valid directory names
func sanitizePackageName(packageKey string) string {
	if packageKey == "default" {
		return "default"
	}
	// Replace problematic characters with dashes
	name := packageKey
	name = filepath.Base(name) // Remove any path separators
	return name
}

func runPatchDemo() error {
	fmt.Println("=== Kure Patch Module Demo (TOML Format) ===")
	fmt.Println()

	// Define paths to our example files
	examplesDir := "examples/patches"
	baseYAML := filepath.Join(examplesDir, "cert-manager-simple.yaml")
	patchFiles := []string{
		filepath.Join(examplesDir, "resources.patch"),
		filepath.Join(examplesDir, "ingress.patch"),
		filepath.Join(examplesDir, "security.patch"),
		filepath.Join(examplesDir, "advanced.patch"),
	}

	// Check if files exist
	if _, err := os.Stat(baseYAML); os.IsNotExist(err) {
		return fmt.Errorf("base YAML file not found: %s", baseYAML)
	}

	fmt.Printf("Loading base cert-manager resources from: %s\n", baseYAML)
	
	// Load base resources with structure preservation
	baseFile, err := os.Open(baseYAML)
	if err != nil {
		return fmt.Errorf("failed to open base YAML: %w", err)
	}
	defer baseFile.Close()

	documentSet, err := patch.LoadResourcesWithStructure(baseFile)
	if err != nil {
		return fmt.Errorf("failed to load resources with structure: %w", err)
	}

	fmt.Printf("Loaded %d base resources with preserved structure\n", len(documentSet.Documents))
	fmt.Println()

	// Create patchable set with structure preservation (will be updated for each patch file)
	// This is just a placeholder - actual patches will be loaded per file in WritePatchedFiles

	// Write patched files to disk
	fmt.Println("=== Writing Patched Files to Disk ===")
	fmt.Printf("Using naming pattern: <originalname>-patch-<patchname>.yaml\n")
	fmt.Println()

	// Create a minimal patchable set just for the WritePatchedFiles functionality
	patchableSet := &patch.PatchableAppSet{
		Resources:   documentSet.GetResources(),
		DocumentSet: documentSet,
		Patches:     make([]struct{Target string; Patch patch.PatchOp}, 0),
	}

	outputDir := "out"
	if err := patchableSet.WritePatchedFiles(baseYAML, patchFiles, outputDir); err != nil {
		return fmt.Errorf("failed to write patched files: %w", err)
	}

	fmt.Println("=== TOML Patch Demo Complete ===")
	fmt.Println()
	fmt.Println("Key Features Demonstrated:")
	fmt.Println("- TOML-style header format: [kind.name.section.selector]")
	fmt.Println("- Complex selectors: containers.name=main, ports.0, containers[image=nginx]")
	fmt.Println("- Automatic Kubernetes path mapping (deployment → spec.template.spec)")
	fmt.Println("- Variable substitution: ${values.key} and ${features.flag}")
	fmt.Println("- Context-aware field resolution based on resource kind")
	fmt.Println("- YAML structure preservation with comments and order")
	fmt.Println("- File output with naming pattern: <originalname>-patch-<patchname>.yaml")
	fmt.Println()
	fmt.Println("Generated Files:")
	for _, patchFile := range patchFiles {
		outputFile := patch.GenerateOutputFilename(baseYAML, patchFile, outputDir)
		fmt.Printf("  - %s\n", outputFile)
	}
	fmt.Println()

	return nil
}
