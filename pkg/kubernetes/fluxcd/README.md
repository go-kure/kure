# pkg/kubernetes/fluxcd

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes/fluxcd.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/fluxcd)

Low-level builder functions for FluxCD Kubernetes resources. Each resource type follows the `Create*(name, namespace)` + `Set*()/Add*()` pattern.

## Constructors

Every kind this package registers has a generated `Create<Kind>` wrapper in `zz_generated_create.go`, produced from the scheme by `pkg/kubernetes/internal/gen` (`make gen-builders`, checked by `make check-builders` in CI). A wrapper delegates to `kubernetes.Create[T]` and emits **TypeMeta and identity only**: no default, no label, no spec value. Namespaced kinds take `(name, namespace)`, cluster-scoped kinds take `(name)`. The upstream struct is the construction API; set spec fields directly or through the admissible `Set*`/`Add*` sugar below.

```go
obj := fluxcd.CreateGitRepository("my-repo", "flux-system")
```

Twenty-four hand-written constructors for spec fragments remain (`CreateGitSpec`, `CreatePostBuild`, `CreateInstallRemediation`, `CreateCrossNamespaceSourceReference` and the rest). Their sub-types are not `client.Object`, so they get no generated wrapper, and Flux's spec fragments are deep enough that assembling one by literal at every call site is the less readable option. They are not identity constructors and are not covered by the identity test — for anything they do not cover, a struct literal is the idiom.

The kinds this package registers, their scope, and what stated that scope are rows in the generated [Supported kinds and field maturity](/api-reference/api-tables/) tables. The sections below are worked examples, not the coverage list.

See the [Kubernetes Builders](/api-reference/kubernetes-builders/) page for the full builder contract: construction, sugar admission classes, purity and the release-1 migration ledger.


## Source Controllers

### GitRepository

```go
gr := fluxcd.CreateGitRepository("my-repo", "flux-system")
gr.Spec.URL = "https://github.com/org/repo"
fluxcd.SetGitRepositoryReference(gr, &sourcev1.GitRepositoryRef{Branch: "main"})
gr.Spec.Interval = metav1.Duration{Duration: 5 * time.Minute}
fluxcd.SetGitRepositorySecretRef(gr, &meta.LocalObjectReference{Name: "git-credentials"})
```

Additional setters: `SetGitRepositoryTimeout`, `SetGitRepositoryVerification`,
`SetGitRepositoryProxySecretRef`, `SetGitRepositoryIgnore`, `AddGitRepositoryInclude`,
`AddGitRepositorySparseCheckoutPath`.

### OCIRepository

```go
oci := fluxcd.CreateOCIRepository("my-manifests", "flux-system")
oci.Spec.URL = "oci://registry.example.com/manifests"
fluxcd.SetOCIRepositoryReference(oci, &sourcev1.OCIRepositoryRef{Tag: "latest"})
oci.Spec.Interval = metav1.Duration{Duration: 10 * time.Minute}
fluxcd.SetOCIRepositorySecretRef(oci, &meta.LocalObjectReference{Name: "registry-credentials"})
```

Additional setters: `SetOCIRepositoryLayerSelector`, `SetOCIRepositoryVerify`,
`SetOCIRepositoryCertSecretRef`, `SetOCIRepositoryProxySecretRef`, `SetOCIRepositoryTimeout`,
`SetOCIRepositoryIgnore`.

### HelmRepository

**HTTP/HTTPS repository:**

```go
hr := fluxcd.CreateHelmRepository("bitnami", "flux-system")
hr.Spec.URL = "https://charts.bitnami.com/bitnami"
hr.Spec.Type = "default"
hr.Spec.Interval = metav1.Duration{Duration: 10 * time.Minute}
fluxcd.SetHelmRepositoryTimeout(hr, &metav1.Duration{Duration: 60 * time.Second})
hr.Spec.PassCredentials = true
fluxcd.SetHelmRepositorySecretRef(hr, &meta.LocalObjectReference{Name: "bitnami-auth"})
```

**OCI registry:**

```go
hr := fluxcd.CreateHelmRepository("ghcr-charts", "flux-system")
hr.Spec.URL = "oci://ghcr.io/example/charts"
hr.Spec.Type = "oci"
hr.Spec.Provider = "generic" // OCI-only: generic, aws, azure, gcp
hr.Spec.Interval = metav1.Duration{Duration: 5 * time.Minute}
fluxcd.SetHelmRepositorySecretRef(hr, &meta.LocalObjectReference{Name: "ghcr-auth"})
```

Additional setters: `SetHelmRepositoryCertSecretRef`, `SetHelmRepositoryAccessFrom`.

### HelmChart

```go
hc := fluxcd.CreateHelmChart("redis", "flux-system")
hc.Spec.Chart = "redis"
hc.Spec.Version = "19.0.0"
hc.Spec.SourceRef = sourcev1.LocalHelmChartSourceReference{
    Kind: "HelmRepository",
    Name: "bitnami",
}
hc.Spec.Interval = metav1.Duration{Duration: 10 * time.Minute}
```

Additional setters: `AddHelmChartValuesFile`, `SetHelmChartVerify`.

> **Note (Flux 2.9):** `source-controller/api` v1.9 split the verification types.
> `SetHelmChartVerify` now takes `*sourcev1.HelmChartVerification` (previously
> `*sourcev1.OCIRepositoryVerification`); `SetOCIRepositoryVerify` still takes
> `*sourcev1.OCIRepositoryVerification`. The API version is unchanged (both v1).

### Bucket

```go
b := fluxcd.CreateBucket("my-bucket", "flux-system")
b.Spec.Endpoint = "minio.example.com"
b.Spec.BucketName = "manifests"
b.Spec.Interval = metav1.Duration{Duration: 10 * time.Minute}
fluxcd.SetBucketSecretRef(b, &meta.LocalObjectReference{Name: "minio-credentials"})
```

Additional setters: `SetBucketSTS`, `SetBucketCertSecretRef`, `SetBucketProxySecretRef`,
`SetBucketTimeout`, `SetBucketIgnore`.

## Deployment Controllers

### Kustomization

```go
k := fluxcd.CreateKustomization("my-app", "flux-system")
k.Spec.SourceRef = kustv1.CrossNamespaceSourceReference{
    Kind: "GitRepository",
    Name: "my-repo",
}
k.Spec.Path = "./clusters/production/apps"
k.Spec.Interval = metav1.Duration{Duration: 10 * time.Minute}
k.Spec.Prune = true
k.Spec.TargetNamespace = "production"
k.Spec.Wait = true
fluxcd.AddKustomizationDependsOn(k, kustv1.DependencyReference{Name: "cert-manager"})
```

Additional setters: `SetKustomizationRetryInterval`, `SetKustomizationKubeConfig`,
`AddKustomizationHealthCheck`, `AddKustomizationHealthCheckExpr`, `AddKustomizationComponent`,
`SetKustomizationTimeout`, `AddKustomizationImage`, `AddKustomizationPatch`,
`SetKustomizationCommonMetadata`, `SetKustomizationDecryption`, `SetKustomizationPostBuild`.

### HelmRelease

**Chart template (chart + version + source reference):**

```go
hr := fluxcd.CreateHelmRelease("redis", "apps")
hr.Spec.ReleaseName = "redis-prod"
hr.Spec.TargetNamespace = "apps"
hr.Spec.Interval = metav1.Duration{Duration: 10 * time.Minute}
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
// Panics if the map does not marshal (a channel, a function, a NaN).
fluxcd.SetHelmReleaseValuesFromMap(hr, map[string]any{"replicaCount": 3})
// Alternative — pre-marshalled JSON:
// fluxcd.SetHelmReleaseValues(hr, &apiextensionsv1.JSON{Raw: []byte(`{"replicaCount":3}`)})
fluxcd.AddHelmReleaseValuesFrom(hr, helmv2.ValuesReference{
    Kind: "ConfigMap",
    Name: "redis-defaults",
})
```

`SetHelmReleaseValuesFromMap` panics rather than returning an error, because a
sugar helper cannot return one under the builder contract. Only a value that
`encoding/json` refuses outright — a channel, a function, a NaN or `+Inf`
float, a cyclic structure — reaches that panic; ordinary user-supplied YAML or
JSON decoded into `map[string]any` always marshals. When values come from
somewhere that could produce such a value, marshal them yourself and hand the
result to `SetHelmReleaseValues`:

```go
import "github.com/go-kure/kure/pkg/errors"

raw, err := json.Marshal(values)
if err != nil {
    return errors.Wrap(err, "helm values")
}
fluxcd.SetHelmReleaseValues(hr, &apiextensionsv1.JSON{Raw: raw})
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

Additional setters: `SetHelmReleaseKubeConfig`, `AddHelmReleaseDependsOn`, `SetHelmReleaseTimeout`,
`SetHelmReleaseMaxHistory`, `SetHelmReleasePersistentClient`, `SetHelmReleaseInstall`,
`SetHelmReleaseUpgrade`, `SetHelmReleaseRollback`, `SetHelmReleaseUninstall`, `SetHelmReleaseTest`,
`SetHelmReleaseValues`, `SetHelmReleaseValuesFromMap`, `SetHelmReleaseCommonMetadata`,
`AddHelmReleaseHealthCheckExpr`, `SetHelmReleaseWaitStrategy`.

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
provider.Spec.Type = "slack"          // plain fields: assigned, not set
provider.Spec.Channel = "#alerts"
fluxcd.SetProviderSecretRef(provider, &meta.LocalObjectReference{Name: "slack-webhook"})

alert := fluxcd.CreateAlert("slack-alert", "flux-system")
alert.Spec.ProviderRef = meta.LocalObjectReference{Name: "slack"}
alert.Spec.EventSeverity = "error"     // no Alert setters remain: all were bare assignments

receiver := fluxcd.CreateReceiver("github-receiver", "flux-system")
receiver.Spec.Type = "github"
receiver.Spec.Events = []string{"push"}
fluxcd.SetReceiverSecretRef(receiver, meta.LocalObjectReference{Name: "webhook-token"})
```

## Flux Operator

```go
instance := fluxcd.CreateFluxInstance("flux", "flux-system")
instance.Spec.Distribution.Variant = "upstream-alpine"   // Distribution is a plain field
// Pointer setters remain for the optional blocks: SetFluxInstanceCluster,
// SetFluxInstanceSharding, SetFluxInstanceStorage, SetFluxInstanceKustomize,
// SetFluxInstanceSync, SetFluxInstanceWait, SetFluxInstanceCommonMetadata and
// SetFluxInstanceMigrateResources.
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
src.Namespace = "flux-system"
fluxcd.AddArtifactGeneratorSource(ag, src)

out := fluxcd.CreateOutputArtifact("combined")
out.Revision = "@app"
cp := fluxcd.CreateCopyOperation("@app/manifests/**", "@artifact/manifests")
fluxcd.AddOutputArtifactCopyOperation(&out, cp)
fluxcd.AddArtifactGeneratorOutputArtifact(ag, out)
```

## Related Packages

- [stack/fluxcd](/api-reference/flux-engine/) — high-level Flux workflow engine
- [stack](/api-reference/stack/) — domain model that produces Flux resources
