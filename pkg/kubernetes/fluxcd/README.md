# pkg/kubernetes/fluxcd

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes/fluxcd.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/fluxcd)

Low-level builder functions for FluxCD Kubernetes resources. Each resource type follows the `Create*(name, namespace)` + `Set*()/Add*()` pattern.

## Constructors

Every kind this package registers has a generated `Create<Kind>` wrapper in `zz_generated_create.go`, produced from the scheme by `pkg/kubernetes/internal/gen` (`make gen-builders`, checked by `make check-builders` in CI). A wrapper delegates to `kubernetes.Create[T]` and emits **TypeMeta and identity only**: no default, no label, no spec value. Namespaced kinds take `(name, namespace)`, cluster-scoped kinds take `(name)`. The upstream struct is the construction API; set spec fields directly or through the admissible `Set*`/`Add*` sugar below.

```go
obj := fluxcd.CreateGitRepository("my-repo", "flux-system")
```

The hand-written `Create*` helpers for spec fragments (sub-types that are not `client.Object`) that remain in this package are legacy and are removed by the prune work item of the builder-contract epic; a struct literal is the idiom.

See the [Kubernetes Builders](/api-reference/kubernetes-builders/) page for the full builder contract: construction, sugar admission classes, purity and the release-1 migration ledger.


## Source Controllers

### GitRepository

```go
gr := fluxcd.CreateGitRepository("my-repo", "flux-system")
fluxcd.SetGitRepositoryURL(gr, "https://github.com/org/repo")
fluxcd.SetGitRepositoryReference(gr, &sourcev1.GitRepositoryRef{Branch: "main"})
fluxcd.SetGitRepositoryInterval(gr, metav1.Duration{Duration: 5 * time.Minute})
fluxcd.SetGitRepositorySecretRef(gr, &meta.LocalObjectReference{Name: "git-credentials"})
```

Additional setters: `SetGitRepositoryProvider`, `SetGitRepositoryTimeout`, `SetGitRepositoryVerification`,
`SetGitRepositoryProxySecretRef`, `SetGitRepositoryIgnore`, `SetGitRepositorySuspend`,
`SetGitRepositoryRecurseSubmodules`, `AddGitRepositoryInclude`,
`SetGitRepositorySparseCheckout`, `AddGitRepositorySparseCheckoutPath`,
`SetGitRepositoryServiceAccountName`.

### OCIRepository

```go
oci := fluxcd.CreateOCIRepository("my-manifests", "flux-system")
fluxcd.SetOCIRepositoryURL(oci, "oci://registry.example.com/manifests")
fluxcd.SetOCIRepositoryReference(oci, &sourcev1.OCIRepositoryRef{Tag: "latest"})
fluxcd.SetOCIRepositoryInterval(oci, metav1.Duration{Duration: 10 * time.Minute})
fluxcd.SetOCIRepositorySecretRef(oci, &meta.LocalObjectReference{Name: "registry-credentials"})
```

Additional setters: `SetOCIRepositoryProvider`, `SetOCIRepositoryLayerSelector`,
`SetOCIRepositoryVerify`, `SetOCIRepositoryServiceAccountName`, `SetOCIRepositoryCertSecretRef`,
`SetOCIRepositoryProxySecretRef`, `SetOCIRepositoryTimeout`, `SetOCIRepositoryIgnore`,
`SetOCIRepositoryInsecure`, `SetOCIRepositorySuspend`.

### HelmRepository

**HTTP/HTTPS repository:**

```go
hr := fluxcd.CreateHelmRepository("bitnami", "flux-system")
fluxcd.SetHelmRepositoryURL(hr, "https://charts.bitnami.com/bitnami")
fluxcd.SetHelmRepositoryType(hr, "default")
fluxcd.SetHelmRepositoryInterval(hr, metav1.Duration{Duration: 10 * time.Minute})
fluxcd.SetHelmRepositoryTimeout(hr, &metav1.Duration{Duration: 60 * time.Second})
fluxcd.SetHelmRepositoryPassCredentials(hr, true)
fluxcd.SetHelmRepositorySecretRef(hr, &meta.LocalObjectReference{Name: "bitnami-auth"})
```

**OCI registry:**

```go
hr := fluxcd.CreateHelmRepository("ghcr-charts", "flux-system")
fluxcd.SetHelmRepositoryURL(hr, "oci://ghcr.io/example/charts")
fluxcd.SetHelmRepositoryType(hr, "oci")
fluxcd.SetHelmRepositoryProvider(hr, "generic") // OCI-only: generic, aws, azure, gcp
fluxcd.SetHelmRepositoryInterval(hr, metav1.Duration{Duration: 5 * time.Minute})
fluxcd.SetHelmRepositorySecretRef(hr, &meta.LocalObjectReference{Name: "ghcr-auth"})
```

Additional setters: `SetHelmRepositoryCertSecretRef`, `SetHelmRepositoryInsecure` (OCI only),
`SetHelmRepositorySuspend`, `SetHelmRepositoryAccessFrom`.

### HelmChart

```go
hc := fluxcd.CreateHelmChart("redis", "flux-system")
fluxcd.SetHelmChartChart(hc, "redis")
fluxcd.SetHelmChartVersion(hc, "19.0.0")
fluxcd.SetHelmChartSourceRef(hc, sourcev1.LocalHelmChartSourceReference{
    Kind: "HelmRepository",
    Name: "bitnami",
})
fluxcd.SetHelmChartInterval(hc, metav1.Duration{Duration: 10 * time.Minute})
```

Additional setters: `SetHelmChartReconcileStrategy`, `AddHelmChartValuesFile`,
`SetHelmChartValuesFiles`, `SetHelmChartIgnoreMissingValuesFiles`,
`SetHelmChartSuspend`, `SetHelmChartVerify`.

> **Note (Flux 2.9):** `source-controller/api` v1.9 split the verification types.
> `SetHelmChartVerify` now takes `*sourcev1.HelmChartVerification` (previously
> `*sourcev1.OCIRepositoryVerification`); `SetOCIRepositoryVerify` still takes
> `*sourcev1.OCIRepositoryVerification`. The API version is unchanged (both v1).

### Bucket

```go
b := fluxcd.CreateBucket("my-bucket", "flux-system")
fluxcd.SetBucketEndpoint(b, "minio.example.com")
fluxcd.SetBucketName(b, "manifests")
fluxcd.SetBucketInterval(b, metav1.Duration{Duration: 10 * time.Minute})
fluxcd.SetBucketSecretRef(b, &meta.LocalObjectReference{Name: "minio-credentials"})
```

Additional setters: `SetBucketProvider`, `SetBucketSTS`, `SetBucketInsecure`, `SetBucketRegion`,
`SetBucketPrefix`, `SetBucketCertSecretRef`, `SetBucketProxySecretRef`,
`SetBucketTimeout`, `SetBucketIgnore`, `SetBucketSuspend`.

## Deployment Controllers

### Kustomization

```go
k := fluxcd.CreateKustomization("my-app", "flux-system")
fluxcd.SetKustomizationSourceRef(k, kustv1.CrossNamespaceSourceReference{
    Kind: "GitRepository",
    Name: "my-repo",
})
fluxcd.SetKustomizationPath(k, "./clusters/production/apps")
fluxcd.SetKustomizationInterval(k, metav1.Duration{Duration: 10 * time.Minute})
fluxcd.SetKustomizationPrune(k, true)
fluxcd.SetKustomizationTargetNamespace(k, "production")
fluxcd.SetKustomizationWait(k, true)
fluxcd.AddKustomizationDependsOn(k, kustv1.DependencyReference{Name: "cert-manager"})
```

Additional setters: `SetKustomizationRetryInterval`, `SetKustomizationKubeConfig`,
`SetKustomizationDeletionPolicy`, `AddKustomizationHealthCheck`,
`AddKustomizationHealthCheckExpr`, `AddKustomizationComponent`,
`SetKustomizationServiceAccountName`, `SetKustomizationSuspend`,
`SetKustomizationTimeout`, `SetKustomizationForce`,
`SetKustomizationIgnoreMissingComponents`, `AddKustomizationImage`,
`AddKustomizationPatch`, `SetKustomizationNamePrefix`, `SetKustomizationNameSuffix`,
`SetKustomizationCommonMetadata`, `SetKustomizationDecryption`, `SetKustomizationPostBuild`.

### HelmRelease

**Chart template (chart + version + source reference):**

```go
hr := fluxcd.CreateHelmRelease("redis", "apps")
fluxcd.SetHelmReleaseReleaseName(hr, "redis-prod")
fluxcd.SetHelmReleaseTargetNamespace(hr, "apps")
fluxcd.SetHelmReleaseInterval(hr, metav1.Duration{Duration: 10 * time.Minute})
fluxcd.SetHelmReleaseChart(hr, &helmv2.HelmChartTemplate{
    Spec: helmv2.HelmChartTemplateSpec{
        Chart:   "redis",
        Version: "19.0.0",
        SourceRef: helmv2.CrossNamespaceObjectReference{
            Kind:      "HelmRepository",
            Name:      "bitnami",
            Namespace: "flux-system",
        },
    },
})
_ = fluxcd.SetHelmReleaseValuesFromMap(hr, map[string]any{"replicaCount": 3}) // handle err
// Alternative — pre-marshalled JSON:
// fluxcd.SetHelmReleaseValues(hr, &apiextensionsv1.JSON{Raw: []byte(`{"replicaCount":3}`)})
fluxcd.AddHelmReleaseValuesFrom(hr, helmv2.ValuesReference{
    Kind: "ConfigMap",
    Name: "redis-defaults",
})
```

**ChartRef mode (existing OCIRepository or HelmChart):**

```go
hr := fluxcd.CreateHelmRelease("my-app", "apps")
fluxcd.SetHelmReleaseChartRef(hr, &helmv2.CrossNamespaceSourceReference{
    Kind:      "OCIRepository",
    Name:      "my-oci-source",
    Namespace: "flux-system",
})
```

**Drift detection and remediation:**

```go
fluxcd.SetHelmReleaseDriftDetection(hr, fluxcd.CreateDriftDetection(helmv2.DriftDetectionEnabled))
fluxcd.SetHelmReleaseInstallCRDs(hr, helmv2.CreateReplace)
fluxcd.SetHelmReleaseInstallRemediation(hr, fluxcd.CreateInstallRemediation(3))
fluxcd.SetHelmReleaseUpgradeCRDs(hr, helmv2.CreateReplace)
fluxcd.SetHelmReleaseUpgradeRemediation(hr, fluxcd.CreateUpgradeRemediation(3))
```

**Post-render:**

```go
k := fluxcd.CreatePostRendererKustomize()
fluxcd.AddPostRendererKustomizeImage(k, kustomize.Image{Name: "redis", NewTag: "7.0"})
fluxcd.AddHelmReleasePostRenderer(hr, helmv2.PostRenderer{Kustomize: k})
```

Additional setters: `SetHelmReleaseKubeConfig`, `SetHelmReleaseSuspend`,
`SetHelmReleaseStorageNamespace`, `AddHelmReleaseDependsOn`, `SetHelmReleaseTimeout`,
`SetHelmReleaseMaxHistory`, `SetHelmReleaseServiceAccountName`, `SetHelmReleasePersistentClient`,
`SetHelmReleaseInstall`, `SetHelmReleaseUpgrade`, `SetHelmReleaseRollback`,
`SetHelmReleaseUninstall`, `SetHelmReleaseTest`, `SetHelmReleaseValues`,
`SetHelmReleaseValuesFromMap`, `SetHelmReleaseCommonMetadata`, `AddHelmReleaseHealthCheckExpr`,
`SetHelmReleaseWaitStrategy`.

Install flag setters: `SetHelmReleaseInstallTimeout`, `SetHelmReleaseInstallCRDs`,
`SetHelmReleaseInstallCreateNamespace`, `SetHelmReleaseInstallDisableSchemaValidation`,
`SetHelmReleaseInstallDisableOpenAPIValidation`, `SetHelmReleaseInstallDisableHooks`,
`SetHelmReleaseInstallDisableWait`, `SetHelmReleaseInstallDisableWaitForJobs`,
`SetHelmReleaseInstallDisableTakeOwnership`, `SetHelmReleaseInstallReplace`,
`SetHelmReleaseInstallRemediation`.

Upgrade flag setters: `SetHelmReleaseUpgradeTimeout`, `SetHelmReleaseUpgradeCRDs`,
`SetHelmReleaseUpgradeDisableSchemaValidation`, `SetHelmReleaseUpgradeDisableOpenAPIValidation`,
`SetHelmReleaseUpgradeDisableHooks`, `SetHelmReleaseUpgradeDisableWait`,
`SetHelmReleaseUpgradeDisableWaitForJobs`, `SetHelmReleaseUpgradeDisableTakeOwnership`,
`SetHelmReleaseUpgradeForce`, `SetHelmReleaseUpgradePreserveValues`,
`SetHelmReleaseUpgradeCleanupOnFail`, `SetHelmReleaseUpgradeRemediation`.

## Notification Controllers

> **Note:** Provider and Alert use `notification.toolkit.fluxcd.io/v1beta3`. Receiver is on v1.
> See [compatibility](/api-reference/compatibility/#notification-controller-provider-and-alert-on-v1beta3)
> for details and tracking issue [#250](https://github.com/go-kure/kure/issues/250).

```go
provider := fluxcd.CreateProvider("slack", "flux-system")
// SetProvider* setters configure type, channel, secretRef, etc.

alert := fluxcd.CreateAlert("slack-alert", "flux-system")
// SetAlert* setters configure providerRef, eventSeverity, summary, etc.

receiver := fluxcd.CreateReceiver("github-receiver", "flux-system")
// SetReceiver* setters configure type, events, resources, secretRef, etc.
```

## Flux Operator

```go
instance := fluxcd.CreateFluxInstance("flux", "flux-system")
fluxcd.SetFluxInstanceDistributionVariant(instance, "upstream-alpine")
// Additional: SetFluxInstance* for distribution, cluster, sharding, storage, kustomize, sync, wait.
```

## Extended Resource Types

### ExternalArtifact

Allows a Flux source artifact produced outside the cluster to be referenced by other Flux resources.

```go
ea := fluxcd.CreateExternalArtifact("my-artifact", "flux-system")
fluxcd.SetExternalArtifactSourceRef(ea, &meta.NamespacedObjectKindReference{
    APIVersion: "source.toolkit.fluxcd.io/v1",
    Kind:       "OCIRepository",
    Name:       "my-oci-source",
    Namespace:  "flux-system",
})
```

### ArtifactGenerator

Provided by the optional **source-watcher** component. Assembles a new artifact by copying files from one or more source artifacts.

```go
ag := fluxcd.CreateArtifactGenerator("my-gen", "flux-system")

src := fluxcd.CreateSourceReference("app", "my-oci-source", "OCIRepository")
fluxcd.SetSourceReferenceNamespace(&src, "flux-system")
fluxcd.AddArtifactGeneratorSource(ag, src)

out := fluxcd.CreateOutputArtifact("combined")
fluxcd.SetOutputArtifactRevision(&out, "@app")
cp := fluxcd.CreateCopyOperation("@app/manifests/**", "@artifact/manifests")
fluxcd.AddOutputArtifactCopyOperation(&out, cp)
fluxcd.AddArtifactGeneratorOutputArtifact(ag, out)
```

## Related Packages

- [stack/fluxcd](/api-reference/flux-engine/) — high-level Flux workflow engine
- [stack](/api-reference/stack/) — domain model that produces Flux resources
