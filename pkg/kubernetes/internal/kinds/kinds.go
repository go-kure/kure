// Package kinds enumerates the object kinds registered in the pkg/kubernetes
// scheme and states each one's scope. It is the single source both the
// constructor generator (internal/gen) and the identity test read, so the two
// can never disagree about which kinds exist.
//
// The scope table below is hand-seeded from the upstream +kubebuilder:resource
// markers (CRDs) and the API server's built-in resource scopes (k8s.io/api). It
// is total: a registered kind missing from both sets is an error, not a
// default, so a scheme change that adds a kind fails generation until its scope
// is stated. Deriving the table from the upstream markers automatically is the
// job of the generated kinds/scope/maturity tables that follow this package.
package kinds

import (
	"reflect"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/kubernetes"
)

// Kind is one registered object kind.
type Kind struct {
	GVK        schema.GroupVersionKind
	Type       reflect.Type // the struct type (not the pointer)
	ImportPath string       // Go import path of the type's package
	TypeName   string       // Go type name
	Package    string       // kure package directory under pkg/kubernetes ("" for the root)
	Namespaced bool
}

// Key returns the group/kind key the scope table is indexed by ("apps/Deployment", "/Pod").
func (k Kind) Key() string { return k.GVK.Group + "/" + k.GVK.Kind }

// packageRoutes maps upstream import-path prefixes to the kure package that
// hosts the generated wrappers for their kinds.
var packageRoutes = []struct{ prefix, pkg string }{
	{"k8s.io/api/", ""},
	{"k8s.io/apiextensions-apiserver/", ""},
	{"sigs.k8s.io/gateway-api/", ""},
	{"github.com/cert-manager/", "certmanager"},
	{"github.com/cilium/", "cilium"},
	{"github.com/cloudnative-pg/", "cnpg"},
	{"github.com/external-secrets/", "externalsecrets"},
	{"github.com/fluxcd/", "fluxcd"},
	{"github.com/controlplaneio-fluxcd/", "fluxcd"},
	{"go.universe.tf/metallb/", "metallb"},
	{"github.com/prometheus-operator/", "prometheus"},
	{"github.com/backube/volsync/", "volsync"},
}

// Registered returns every object kind in the scheme, sorted by package, kind,
// group and version. A kind is an object when its pointer type implements
// client.Object; list kinds and apimachinery's meta types are excluded.
func Registered() ([]Kind, error) {
	if err := kubernetes.RegisterSchemes(); err != nil {
		return nil, err
	}
	return classify(kubernetes.Scheme.AllKnownTypes())
}

// classify filters and routes the scheme's known types into Kinds.
func classify(known map[schema.GroupVersionKind]reflect.Type) ([]Kind, error) {
	clientObject := reflect.TypeOf((*client.Object)(nil)).Elem()
	var out []Kind
	for gvk, typ := range known {
		if strings.HasSuffix(gvk.Kind, "List") {
			continue
		}
		if !reflect.PointerTo(typ).Implements(clientObject) {
			continue
		}
		if strings.HasPrefix(typ.PkgPath(), "k8s.io/apimachinery/") {
			continue
		}
		k := Kind{GVK: gvk, Type: typ, ImportPath: typ.PkgPath(), TypeName: typ.Name()}
		pkg, ok := routePackage(k.ImportPath)
		if !ok {
			return nil, errors.Errorf("kinds: no kure package routes import path %s (kind %s); add it to packageRoutes", k.ImportPath, gvk)
		}
		k.Package = pkg
		switch key := k.Key(); {
		case namespacedKinds[key]:
			k.Namespaced = true
		case clusterKinds[key]:
			k.Namespaced = false
		default:
			return nil, errors.Errorf("kinds: scope of %s (%s) is not stated; add it to namespacedKinds or clusterKinds", key, gvk)
		}
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].sortKey() < out[j].sortKey() })
	return out, nil
}

// sortKey orders kinds by package, kind, group, version.
func (k Kind) sortKey() string {
	return strings.Join([]string{k.Package, k.GVK.Kind, k.GVK.Group, k.GVK.Version}, "\x00")
}

func routePackage(importPath string) (string, bool) {
	for _, r := range packageRoutes {
		if strings.HasPrefix(importPath, r.prefix) {
			return r.pkg, true
		}
	}
	return "", false
}

// set builds a lookup set from group/kind keys.
func set(keys ...string) map[string]bool {
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k] = true
	}
	return m
}

// clusterKinds are the registered kinds whose objects carry no namespace.
// CRDs: from +kubebuilder:resource:scope=Cluster in the pinned module sources.
// Built-ins: the API server's resource scopes.
var clusterKinds = set(
	// k8s.io/api
	"/ComponentStatus",
	"/Namespace",
	"/Node",
	"/PersistentVolume",
	"/RangeAllocation",
	"networking.k8s.io/IPAddress",
	"networking.k8s.io/IngressClass",
	"networking.k8s.io/ServiceCIDR",
	"rbac.authorization.k8s.io/ClusterRole",
	"rbac.authorization.k8s.io/ClusterRoleBinding",
	"storage.k8s.io/CSIDriver",
	"storage.k8s.io/CSINode",
	"storage.k8s.io/StorageClass",
	"storage.k8s.io/VolumeAttachment",
	"storage.k8s.io/VolumeAttributesClass",
	"apiextensions.k8s.io/CustomResourceDefinition",
	// gateway-api
	"gateway.networking.k8s.io/GatewayClass",
	// cert-manager
	"cert-manager.io/ClusterIssuer",
	// cilium
	"cilium.io/CiliumBGPAdvertisement",
	"cilium.io/CiliumBGPClusterConfig",
	"cilium.io/CiliumBGPNodeConfig",
	"cilium.io/CiliumBGPNodeConfigOverride",
	"cilium.io/CiliumBGPPeerConfig",
	"cilium.io/CiliumCIDRGroup",
	"cilium.io/CiliumClusterwideEnvoyConfig",
	"cilium.io/CiliumClusterwideNetworkPolicy",
	"cilium.io/CiliumEgressGatewayPolicy",
	"cilium.io/CiliumIdentity",
	"cilium.io/CiliumLoadBalancerIPPool",
	"cilium.io/CiliumNode",
	// cloudnative-pg
	"postgresql.cnpg.io/ClusterImageCatalog",
	// external-secrets
	"external-secrets.io/ClusterExternalSecret",
	"external-secrets.io/ClusterSecretStore",
)

// namespacedKinds are the registered kinds whose objects live in a namespace.
// Absent +kubebuilder:resource:scope marker = Namespaced (the kubebuilder
// default); every module kure pins follows it.
var namespacedKinds = set(
	// k8s.io/api core
	"/Binding",
	"/ConfigMap",
	"/Endpoints",
	"/Event",
	"/LimitRange",
	"/PersistentVolumeClaim",
	"/Pod",
	"/PodTemplate",
	"/ReplicationController",
	"/ResourceQuota",
	"/Secret",
	"/Service",
	"/ServiceAccount",
	// k8s.io/api apps, batch, autoscaling, policy, networking, rbac, storage
	"apps/ControllerRevision",
	"apps/DaemonSet",
	"apps/Deployment",
	"apps/ReplicaSet",
	"apps/StatefulSet",
	"autoscaling/HorizontalPodAutoscaler",
	"batch/CronJob",
	"batch/Job",
	"policy/Eviction",
	"policy/PodDisruptionBudget",
	"networking.k8s.io/Ingress",
	"networking.k8s.io/NetworkPolicy",
	"rbac.authorization.k8s.io/Role",
	"rbac.authorization.k8s.io/RoleBinding",
	"storage.k8s.io/CSIStorageCapacity",
	// gateway-api
	"gateway.networking.k8s.io/BackendTLSPolicy",
	"gateway.networking.k8s.io/GRPCRoute",
	"gateway.networking.k8s.io/Gateway",
	"gateway.networking.k8s.io/HTTPRoute",
	"gateway.networking.k8s.io/ListenerSet",
	"gateway.networking.k8s.io/ReferenceGrant",
	"gateway.networking.k8s.io/TCPRoute",
	"gateway.networking.k8s.io/TLSRoute",
	"gateway.networking.k8s.io/UDPRoute",
	// cert-manager
	"acme.cert-manager.io/Challenge",
	"acme.cert-manager.io/Order",
	"cert-manager.io/Certificate",
	"cert-manager.io/CertificateRequest",
	"cert-manager.io/Issuer",
	// cilium
	"cilium.io/CiliumEndpoint",
	"cilium.io/CiliumEnvoyConfig",
	"cilium.io/CiliumLocalRedirectPolicy",
	"cilium.io/CiliumNetworkPolicy",
	"cilium.io/CiliumNodeConfig",
	// cloudnative-pg + barman-cloud plugin
	"postgresql.cnpg.io/Backup",
	"postgresql.cnpg.io/Cluster",
	"postgresql.cnpg.io/Database",
	"postgresql.cnpg.io/DatabaseRole",
	"postgresql.cnpg.io/FailoverQuorum",
	"postgresql.cnpg.io/ImageCatalog",
	"postgresql.cnpg.io/Pooler",
	"postgresql.cnpg.io/Publication",
	"postgresql.cnpg.io/ScheduledBackup",
	"postgresql.cnpg.io/Subscription",
	"barmancloud.cnpg.io/ObjectStore",
	// flux-operator
	"fluxcd.controlplane.io/FluxInstance",
	"fluxcd.controlplane.io/FluxReport",
	"fluxcd.controlplane.io/ResourceSet",
	"fluxcd.controlplane.io/ResourceSetInputProvider",
	// external-secrets
	"external-secrets.io/ExternalSecret",
	"external-secrets.io/SecretStore",
	// fluxcd controllers
	"helm.toolkit.fluxcd.io/HelmRelease",
	"image.toolkit.fluxcd.io/ImageUpdateAutomation",
	"kustomize.toolkit.fluxcd.io/Kustomization",
	"notification.toolkit.fluxcd.io/Alert",
	"notification.toolkit.fluxcd.io/Provider",
	"notification.toolkit.fluxcd.io/Receiver",
	"source.toolkit.fluxcd.io/Bucket",
	"source.toolkit.fluxcd.io/ExternalArtifact",
	"source.toolkit.fluxcd.io/GitRepository",
	"source.toolkit.fluxcd.io/HelmChart",
	"source.toolkit.fluxcd.io/HelmRepository",
	"source.toolkit.fluxcd.io/OCIRepository",
	"source.extensions.fluxcd.io/ArtifactGenerator",
	// prometheus-operator
	"monitoring.coreos.com/Alertmanager",
	"monitoring.coreos.com/PodMonitor",
	"monitoring.coreos.com/Probe",
	"monitoring.coreos.com/Prometheus",
	"monitoring.coreos.com/PrometheusRule",
	"monitoring.coreos.com/ServiceMonitor",
	"monitoring.coreos.com/ThanosRuler",
	// metallb
	"metallb.io/BFDProfile",
	"metallb.io/BGPAdvertisement",
	"metallb.io/BGPPeer",
	"metallb.io/Community",
	"metallb.io/ConfigurationState",
	"metallb.io/IPAddressPool",
	"metallb.io/L2Advertisement",
	"metallb.io/ServiceBGPStatus",
	"metallb.io/ServiceL2Status",
	// volsync
	"volsync.backube/ReplicationDestination",
	"volsync.backube/ReplicationSource",
)
