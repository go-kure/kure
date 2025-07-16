package main

import (
	"os"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	v1 "github.com/fluxcd/notification-controller/api/v1"
	notificationv1beta2 "github.com/fluxcd/notification-controller/api/v1beta2"
	meta "github.com/fluxcd/pkg/apis/meta"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
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

	// StorageClass example
	sc := k8s.CreateStorageClass("demo-sc", "kubernetes.io/no-provisioner")
	k8s.AddStorageClassParameter(sc, "type", "local")
	k8s.SetStorageClassAllowVolumeExpansion(sc, true)
	k8s.SetPVCStorageClass(pvc, sc)

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

	// Job example
	job := k8s.CreateJob("demo-job", "demo")
	k8s.AddJobContainer(job, mainCtr)
	k8s.SetJobBackoffLimit(job, 3)
	k8s.SetJobCompletions(job, 1)
	k8s.SetJobParallelism(job, 1)

	// CronJob example
	cron := k8s.CreateCronJob("demo-cron", "demo", "*/5 * * * *")
	k8s.AddCronJobContainer(cron, mainCtr)
	k8s.SetCronJobConcurrencyPolicy(cron, batchv1.ForbidConcurrent)

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
	metallb.AddIPAddressPoolAddress(pool, "172.19.0.0/24")
	l2adv := metallb.CreateL2Advertisement("demo-l2adv", "demo", metallbv1beta1.L2AdvertisementSpec{})
	metallb.AddL2AdvertisementIPAddressPool(l2adv, pool.Name)
	bgpadv := metallb.CreateBGPAdvertisement("demo-bgpadv", "demo", metallbv1beta1.BGPAdvertisementSpec{})
	metallb.SetBGPAdvertisementLocalPref(bgpadv, 100)
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
	y.PrintObj(job, os.Stdout)
	y.PrintObj(cron, os.Stdout)
	y.PrintObj(svc, os.Stdout)
	y.PrintObj(ing, os.Stdout)
	y.PrintObj(role, os.Stdout)
	y.PrintObj(roleBind, os.Stdout)
	y.PrintObj(clusterRole, os.Stdout)
	y.PrintObj(clusterRoleBind, os.Stdout)
	y.PrintObj(hr, os.Stdout)
	y.PrintObj(ks, os.Stdout)
	y.PrintObj(gitRepo, os.Stdout)
	y.PrintObj(helmRepo, os.Stdout)
	y.PrintObj(bucket, os.Stdout)
	y.PrintObj(chart, os.Stdout)
	y.PrintObj(ociRepo, os.Stdout)
	y.PrintObj(pool, os.Stdout)
	y.PrintObj(l2adv, os.Stdout)
	y.PrintObj(bgpadv, os.Stdout)
	y.PrintObj(bgpPeer, os.Stdout)
	y.PrintObj(bfd, os.Stdout)
	y.PrintObj(ss, os.Stdout)
	y.PrintObj(css, os.Stdout)
	y.PrintObj(es, os.Stdout)
	y.PrintObj(np, os.Stdout)
	y.PrintObj(rq, os.Stdout)
	y.PrintObj(lr, os.Stdout)
	y.PrintObj(provider, os.Stdout)
	y.PrintObj(alert, os.Stdout)
	y.PrintObj(receiver, os.Stdout)
	y.PrintObj(auto, os.Stdout)
	y.PrintObj(issuer, os.Stdout)
	y.PrintObj(clusterIssuer, os.Stdout)
	y.PrintObj(cert, os.Stdout)
}
