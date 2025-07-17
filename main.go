package main

import (
	"fmt"
	"os"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

	"github.com/go-kure/kure/internal/externalsecrets"
	"github.com/go-kure/kure/internal/fluxcd"
	"github.com/go-kure/kure/internal/k8s"
	"github.com/go-kure/kure/internal/metallb"

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
		fmt.Fprintf(os.Stderr, "%s: %v\n", msg, err)
	}
}

func main() {
	y := printers.YAMLPrinter{}

	// Namespace example
	ns := k8s.CreateNamespace("demo")
	k8s.AddNamespaceLabel(ns, "env", "demo")
	k8s.AddNamespaceAnnotation(ns, "owner", "example")

	// Service account
	sa := k8s.CreateServiceAccount("demo-sa", "demo")
	logError("add serviceaccount secret", k8s.AddServiceAccountSecret(sa, apiv1.ObjectReference{Name: "sa-secret"}))
	logError("add serviceaccount image pull secret", k8s.AddServiceAccountImagePullSecret(sa, apiv1.LocalObjectReference{Name: "sa-pull"}))
	logError("set serviceaccount automount token", k8s.SetServiceAccountAutomountToken(sa, true))

	// Secret example
	secret := k8s.CreateSecret("demo-secret", "demo")
	logError("add secret data", k8s.AddSecretData(secret, "cert", []byte("data")))
	logError("add secret string data", k8s.AddSecretStringData(secret, "token", "abcd"))
	logError("set secret type", k8s.SetSecretType(secret, apiv1.SecretTypeOpaque))
	logError("set secret immutable", k8s.SetSecretImmutable(secret, true))

	// ConfigMap example
	cm := k8s.CreateConfigMap("demo-config", "demo")
	logError("add configmap data", k8s.AddConfigMapData(cm, "foo", "bar"))
	logError("add configmap data map", k8s.AddConfigMapDataMap(cm, map[string]string{"extra": "value"}))
	logError("add configmap binary data", k8s.AddConfigMapBinaryData(cm, "bin", []byte{0x1}))
	logError("add configmap binary data map", k8s.AddConfigMapBinaryDataMap(cm, map[string][]byte{"more": {0x2}}))
	logError("set configmap data", k8s.SetConfigMapData(cm, map[string]string{"hello": "world"}))
	logError("set configmap binary data", k8s.SetConfigMapBinaryData(cm, map[string][]byte{"bye": {0x0}}))
	logError("set configmap immutable", k8s.SetConfigMapImmutable(cm, false))

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

	// StorageClass example
	sc := k8s.CreateStorageClass("demo-sc", "kubernetes.io/no-provisioner")
	k8s.AddStorageClassParameter(sc, "type", "local")
	k8s.SetStorageClassAllowVolumeExpansion(sc, true)
	k8s.SetPVCStorageClass(pvc, sc)

	// Pod example
	pod := k8s.CreatePod("demo-pod", "demo")
	mainCtr := k8s.CreateContainer("app", "nginx", nil, nil)
	logError("add container port", k8s.AddContainerPort(mainCtr, apiv1.ContainerPort{Name: "http", ContainerPort: 80}))
	logError("add container env", k8s.AddContainerEnv(mainCtr, apiv1.EnvVar{Name: "ENV", Value: "prod"}))
	logError("add container env from", k8s.AddContainerEnvFrom(mainCtr, apiv1.EnvFromSource{ConfigMapRef: &apiv1.ConfigMapEnvSource{LocalObjectReference: apiv1.LocalObjectReference{Name: cm.Name}}}))
	logError("add container volume mount", k8s.AddContainerVolumeMount(mainCtr, apiv1.VolumeMount{Name: "data", MountPath: "/data"}))
	logError("add container volume device", k8s.AddContainerVolumeDevice(mainCtr, apiv1.VolumeDevice{Name: "block", DevicePath: "/dev/block"}))
	logError("set container liveness probe", k8s.SetContainerLivenessProbe(mainCtr, apiv1.Probe{InitialDelaySeconds: 5}))
	logError("set container readiness probe", k8s.SetContainerReadinessProbe(mainCtr, apiv1.Probe{InitialDelaySeconds: 5}))
	logError("set container startup probe", k8s.SetContainerStartupProbe(mainCtr, apiv1.Probe{InitialDelaySeconds: 1}))
	logError("set container resources", k8s.SetContainerResources(mainCtr, apiv1.ResourceRequirements{
		Limits: apiv1.ResourceList{"memory": resource.MustParse("128Mi")},
		Requests: apiv1.ResourceList{
			"cpu":    resource.MustParse("50m"),
			"memory": resource.MustParse("64Mi"),
		},
	}))
	logError("set container image pull policy", k8s.SetContainerImagePullPolicy(mainCtr, apiv1.PullIfNotPresent))
	logError("set container security context", k8s.SetContainerSecurityContext(mainCtr, apiv1.SecurityContext{RunAsUser: ptr[int64](1000)}))

	initCtr := k8s.CreateContainer("init", "busybox", []string{"sh", "-c"}, []string{"echo init"})

	logError("add pod container", k8s.AddPodContainer(pod, mainCtr))
	logError("add pod init container", k8s.AddPodInitContainer(pod, initCtr))
	logError("add pod volume", k8s.AddPodVolume(pod, &apiv1.Volume{Name: "data"}))
	logError("add pod image pull secret", k8s.AddPodImagePullSecret(pod, &apiv1.LocalObjectReference{Name: "pullsecret"}))
	logError("add pod toleration", k8s.AddPodToleration(pod, &apiv1.Toleration{Key: "role"}))
	tsc := apiv1.TopologySpreadConstraint{MaxSkew: 1, TopologyKey: "zone", WhenUnsatisfiable: apiv1.DoNotSchedule, LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}}}
	logError("add pod topology spread constraints", k8s.AddPodTopologySpreadConstraints(pod, &tsc))
	logError("set pod serviceaccount name", k8s.SetPodServiceAccountName(pod, sa.Name))
	logError("set pod security context", k8s.SetPodSecurityContext(pod, &apiv1.PodSecurityContext{}))
	logError("set pod affinity", k8s.SetPodAffinity(pod, &apiv1.Affinity{}))
	logError("set pod node selector", k8s.SetPodNodeSelector(pod, map[string]string{"type": "worker"}))

	// Deployment example
	dep := k8s.CreateDeployment("demo-deployment", "demo")
	logError("add deployment container", k8s.AddDeploymentContainer(dep, mainCtr))
	logError("add deployment init container", k8s.AddDeploymentInitContainer(dep, initCtr))
	logError("add deployment volume", k8s.AddDeploymentVolume(dep, &apiv1.Volume{Name: "data"}))
	logError("add deployment image pull secret", k8s.AddDeploymentImagePullSecret(dep, &apiv1.LocalObjectReference{Name: "pullsecret"}))
	logError("add deployment toleration", k8s.AddDeploymentToleration(dep, &apiv1.Toleration{Key: "role"}))
	logError("add deployment topology spread constraints", k8s.AddDeploymentTopologySpreadConstraints(dep, &tsc))
	logError("set deployment serviceaccount name", k8s.SetDeploymentServiceAccountName(dep, sa.Name))
	logError("set deployment security context", k8s.SetDeploymentSecurityContext(dep, &apiv1.PodSecurityContext{}))
	logError("set deployment affinity", k8s.SetDeploymentAffinity(dep, &apiv1.Affinity{}))
	logError("set deployment node selector", k8s.SetDeploymentNodeSelector(dep, map[string]string{"role": "web"}))

	// StatefulSet example
	sts := k8s.CreateStatefulSet("demo-sts", "demo")
	logError("add statefulset container", k8s.AddStatefulSetContainer(sts, mainCtr))
	logError("add statefulset init container", k8s.AddStatefulSetInitContainer(sts, initCtr))
	logError("add statefulset volume", k8s.AddStatefulSetVolume(sts, &apiv1.Volume{Name: "data"}))
	logError("add statefulset volumeclaim template", k8s.AddStatefulSetVolumeClaimTemplate(sts, *k8s.CreatePersistentVolumeClaim("data", "demo")))
	logError("add statefulset toleration", k8s.AddStatefulSetToleration(sts, &apiv1.Toleration{Key: "role"}))
	logError("set statefulset serviceaccount name", k8s.SetStatefulSetServiceAccountName(sts, sa.Name))
	logError("set statefulset service name", k8s.SetStatefulSetServiceName(sts, "demo-svc"))
	logError("set statefulset replicas", k8s.SetStatefulSetReplicas(sts, 3))

	// DaemonSet example
	ds := k8s.CreateDaemonSet("demo-ds", "demo")
	logError("add daemonset container", k8s.AddDaemonSetContainer(ds, mainCtr))
	logError("add daemonset init container", k8s.AddDaemonSetInitContainer(ds, initCtr))
	logError("add daemonset volume", k8s.AddDaemonSetVolume(ds, &apiv1.Volume{Name: "data"}))
	logError("add daemonset toleration", k8s.AddDaemonSetToleration(ds, &apiv1.Toleration{Key: "role"}))
	logError("set daemonset serviceaccount name", k8s.SetDaemonSetServiceAccountName(ds, sa.Name))
	logError("set daemonset node selector", k8s.SetDaemonSetNodeSelector(ds, map[string]string{"type": "worker"}))

	// Job example
	job := k8s.CreateJob("demo-job", "demo")
	logError("add job container", k8s.AddJobContainer(job, mainCtr))
	logError("set job backoff limit", k8s.SetJobBackoffLimit(job, 3))
	logError("set job completions", k8s.SetJobCompletions(job, 1))
	logError("set job parallelism", k8s.SetJobParallelism(job, 1))

	// CronJob example
	cron := k8s.CreateCronJob("demo-cron", "demo", "*/5 * * * *")
	logError("add cronjob container", k8s.AddCronJobContainer(cron, mainCtr))
	logError("set cronjob concurrency policy", k8s.SetCronJobConcurrencyPolicy(cron, batchv1.ForbidConcurrent))

	// Service and ingress example
	svc := k8s.CreateService("demo-svc", "demo")
	logError("set service selector", k8s.SetServiceSelector(svc, map[string]string{"app": "demo"}))
	logError("add service port", k8s.AddServicePort(svc, apiv1.ServicePort{Name: "http", Port: 80, TargetPort: intstr.FromString("http")}))
	logError("set service type", k8s.SetServiceType(svc, apiv1.ServiceTypeClusterIP))
	logError("set service external traffic policy", k8s.SetServiceExternalTrafficPolicy(svc, apiv1.ServiceExternalTrafficPolicyCluster))

	ing := k8s.CreateIngress("demo-ing", "demo", "nginx")
	rule := k8s.CreateIngressRule("example.com")
	pathtype := netv1.PathTypePrefix
	path := k8s.CreateIngressPath("/", &pathtype, svc.Name, "http")
	k8s.AddIngressRulePath(rule, path)
	k8s.AddIngressRule(ing, rule)
	k8s.AddIngressTLS(ing, netv1.IngressTLS{Hosts: []string{"example.com"}, SecretName: secret.Name})

	// RBAC examples
	role := k8s.CreateRole("demo-role", "demo")
	k8s.AddRoleRule(role, rbacv1.PolicyRule{Verbs: []string{"get"}, Resources: []string{"pods"}})

	clusterRole := k8s.CreateClusterRole("demo-cr")
	k8s.AddClusterRoleRule(clusterRole, rbacv1.PolicyRule{Verbs: []string{"list"}, Resources: []string{"nodes"}})

	roleBind := k8s.CreateRoleBinding("demo-rb", "demo", rbacv1.RoleRef{Kind: "Role", Name: role.Name})
	k8s.AddRoleBindingSubject(roleBind, rbacv1.Subject{Kind: "ServiceAccount", Name: sa.Name, Namespace: sa.Namespace})

	clusterRoleBind := k8s.CreateClusterRoleBinding("demo-crb", rbacv1.RoleRef{Kind: "ClusterRole", Name: clusterRole.Name})
	k8s.AddClusterRoleBindingSubject(clusterRoleBind, rbacv1.Subject{Kind: "User", Name: "admin"})

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
	np := k8s.CreateNetworkPolicy("demo-netpol", "demo")
	npRule := netv1.NetworkPolicyIngressRule{}
	peer := netv1.NetworkPolicyPeer{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}}}
	k8s.AddNetworkPolicyIngressPeer(&npRule, peer)
	k8s.AddNetworkPolicyIngressRule(np, npRule)
	k8s.AddNetworkPolicyPolicyType(np, netv1.PolicyTypeIngress)

	// Resource quota example
	rq := k8s.CreateResourceQuota("demo-quota", "demo")
	k8s.AddResourceQuotaHard(rq, apiv1.ResourceRequestsStorage, resource.MustParse("1Gi"))

	// Limit range example
	lr := k8s.CreateLimitRange("demo-limits", "demo")
	lrItem := apiv1.LimitRangeItem{Type: apiv1.LimitTypeContainer}
	k8s.AddLimitRangeItemDefaultRequest(&lrItem, apiv1.ResourceCPU, resource.MustParse("100m"))
	k8s.AddLimitRangeItem(lr, lrItem)

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
