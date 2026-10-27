# pkg/kubernetes/fluxcd

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes/fluxcd.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/fluxcd)

Low-level builder functions for FluxCD Kubernetes resources. Each resource type follows the `Create*(name, namespace)` + `Set*()/Add*()` pattern.

Each block on this page, except the one marked as an excerpt, is the body of an `Example` function
in `example_test.go`, which `go test` runs: it imports this package as `fluxcd`, the Flux APIs as
`sourcev1` (`github.com/fluxcd/source-controller/api/v1`), `kustv1`
(`github.com/fluxcd/kustomize-controller/api/v1`), `helmv2`
(`github.com/fluxcd/helm-controller/api/v2`), `meta` and `kustomize`
(`github.com/fluxcd/pkg/apis/meta`, `github.com/fluxcd/pkg/apis/kustomize`),
`metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"`, `time`, and `fmt` for the line that prints what
the example built.

## Constructors

Every kind this package registers has a generated `Create<Kind>` wrapper in `zz_generated_create.go`, produced from the scheme by `pkg/kubernetes/internal/gen` (`make gen-builders`, checked by `make check-builders` in CI). A wrapper delegates to `kubernetes.Create[T]` and emits **TypeMeta and identity only**: no default, no label, no spec value. Namespaced kinds take `(name, namespace)`, cluster-scoped kinds take `(name)`. The upstream struct is the construction API; set spec fields directly or through the admissible `Set*`/`Add*` sugar below.

<!-- doc-example: pkg/kubernetes/fluxcd ExampleCreateGitRepository -->
```go
obj := fluxcd.CreateGitRepository("my-repo", "flux-system")
fmt.Println(obj.Kind, obj.Namespace+"/"+obj.Name)
```
<!-- doc-example:end -->

Twenty-four hand-written constructors for spec fragments remain (`CreateGitSpec`, `CreatePostBuild`, `CreateInstallRemediation`, `CreateCrossNamespaceSourceReference` and the rest). Their sub-types are not `client.Object`, so they get no generated wrapper, and Flux's spec fragments are deep enough that assembling one by literal at every call site is the less readable option. They are not identity constructors and are not covered by the identity test — for anything they do not cover, a struct literal is the idiom.

The kinds this package registers, their scope, and what stated that scope are rows in the generated [Supported kinds and field maturity](/api-reference/api-tables/) tables. The sections below are worked examples, not the coverage list.

See the [Kubernetes Builders](/api-reference/kubernetes-builders/) page for the full builder contract: construction, sugar admission classes, purity and the release-1 migration ledger.


## Source Controllers

### GitRepository

<!-- doc-example: pkg/kubernetes/fluxcd ExampleSetGitRepositoryReference -->
```go
gr := fluxcd.CreateGitRepository("my-repo", "flux-system")
gr.Spec.URL = "https://github.com/org/repo"
fluxcd.SetGitRepositoryReference(gr, &sourcev1.GitRepositoryRef{Branch: "main"})
gr.Spec.Interval = metav1.Duration{Duration: 5 * time.Minute}
fluxcd.SetGitRepositorySecretRef(gr, &meta.LocalObjectReference{Name: "git-credentials"})
fmt.Println(gr.Spec.Reference.Branch, gr.Spec.Interval.Duration, gr.Spec.SecretRef.Name)
```
<!-- doc-example:end -->

Additional setters: `SetGitRepositoryTimeout`, `SetGitRepositoryVerification`,
`SetGitRepositoryProxySecretRef`, `SetGitRepositoryIgnore`, `AddGitRepositoryInclude`,
`AddGitRepositorySparseCheckoutPath`.

### OCIRepository

<!-- doc-example: pkg/kubernetes/fluxcd ExampleSetOCIRepositoryReference -->
```go
oci := fluxcd.CreateOCIRepository("my-manifests", "flux-system")
oci.Spec.URL = "oci://registry.example.com/manifests"
fluxcd.SetOCIRepositoryReference(oci, &sourcev1.OCIRepositoryRef{Tag: "latest"})
oci.Spec.Interval = &metav1.Duration{Duration: 10 * time.Minute}
fluxcd.SetOCIRepositorySecretRef(oci, &meta.LocalObjectReference{Name: "registry-credentials"})
fmt.Println(oci.Spec.Reference.Tag, oci.Spec.SecretRef.Name)
```
<!-- doc-example:end -->

Additional setters: `SetOCIRepositoryLayerSelector`, `SetOCIRepositoryVerify`,
`SetOCIRepositoryCertSecretRef`, `SetOCIRepositoryProxySecretRef`, `SetOCIRepositoryTimeout`,
`SetOCIRepositoryIgnore`.

The OIDC provider and plain-HTTP access are plain fields with no setter: assign them,
`oci.Spec.Provider = "aws"` and `oci.Spec.Insecure = true`. A setter for a single plain field
is outside the [builder contract](/api-reference/kubernetes-builders/).

### HelmRepository

**HTTP/HTTPS repository:**

<!-- doc-example: pkg/kubernetes/fluxcd ExampleSetHelmRepositorySecretRef -->
```go
hr := fluxcd.CreateHelmRepository("bitnami", "flux-system")
hr.Spec.URL = "https://charts.bitnami.com/bitnami"
hr.Spec.Type = "default"
hr.Spec.Interval = metav1.Duration{Duration: 10 * time.Minute}
fluxcd.SetHelmRepositoryTimeout(hr, &metav1.Duration{Duration: 60 * time.Second})
hr.Spec.PassCredentials = true
fluxcd.SetHelmRepositorySecretRef(hr, &meta.LocalObjectReference{Name: "bitnami-auth"})
fmt.Println(hr.Spec.Timeout.Duration, hr.Spec.SecretRef.Name)
```
<!-- doc-example:end -->

**OCI registry:**

<!-- doc-example: pkg/kubernetes/fluxcd ExampleCreateHelmRepository_oci -->
```go
hr := fluxcd.CreateHelmRepository("ghcr-charts", "flux-system")
hr.Spec.URL = "oci://ghcr.io/example/charts"
hr.Spec.Type = "oci"
hr.Spec.Provider = "generic" // OCI-only: generic, aws, azure, gcp
hr.Spec.Interval = metav1.Duration{Duration: 5 * time.Minute}
fluxcd.SetHelmRepositorySecretRef(hr, &meta.LocalObjectReference{Name: "ghcr-auth"})
fmt.Println(hr.Spec.Type, hr.Spec.Provider)
```
<!-- doc-example:end -->

Additional setters: `SetHelmRepositoryCertSecretRef`, `SetHelmRepositoryAccessFrom`.

### HelmChart

<!-- doc-example: pkg/kubernetes/fluxcd ExampleCreateHelmChart -->
```go
hc := fluxcd.CreateHelmChart("redis", "flux-system")
hc.Spec.Chart = "redis"
hc.Spec.Version = "19.0.0"
hc.Spec.SourceRef = sourcev1.LocalHelmChartSourceReference{
    Kind: "HelmRepository",
    Name: "bitnami",
}
hc.Spec.Interval = metav1.Duration{Duration: 10 * time.Minute}
fmt.Println(hc.Spec.Chart, hc.Spec.Version, hc.Spec.SourceRef.Kind)
```
<!-- doc-example:end -->

Additional setters: `AddHelmChartValuesFile`, `SetHelmChartVerify`.

> **Note (Flux 2.9):** `source-controller/api` v1.9 split the verification types.
> `SetHelmChartVerify` now takes `*sourcev1.HelmChartVerification` (previously
> `*sourcev1.OCIRepositoryVerification`); `SetOCIRepositoryVerify` still takes
> `*sourcev1.OCIRepositoryVerification`. The API version is unchanged (both v1).

### Bucket

<!-- doc-example: pkg/kubernetes/fluxcd ExampleSetBucketSecretRef -->
```go
b := fluxcd.CreateBucket("my-bucket", "flux-system")
b.Spec.Endpoint = "minio.example.com"
b.Spec.BucketName = "manifests"
b.Spec.Interval = metav1.Duration{Duration: 10 * time.Minute}
fluxcd.SetBucketSecretRef(b, &meta.LocalObjectReference{Name: "minio-credentials"})
fmt.Println(b.Spec.BucketName, b.Spec.SecretRef.Name)
```
<!-- doc-example:end -->

Additional setters: `SetBucketSTS`, `SetBucketCertSecretRef`, `SetBucketProxySecretRef`,
`SetBucketTimeout`, `SetBucketIgnore`.

## Deployment Controllers

### Kustomization

<!-- doc-example: pkg/kubernetes/fluxcd ExampleAddKustomizationDependsOn -->
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
fmt.Println(k.Spec.Path, k.Spec.DependsOn[0].Name)
```
<!-- doc-example:end -->

Additional setters: `SetKustomizationRetryInterval`, `SetKustomizationKubeConfig`,
`AddKustomizationHealthCheck`, `AddKustomizationHealthCheckExpr`, `AddKustomizationComponent`,
`SetKustomizationTimeout`, `AddKustomizationImage`, `AddKustomizationPatch`,
`SetKustomizationCommonMetadata`, `SetKustomizationDecryption`, `SetKustomizationPostBuild`.

### HelmRelease

**Chart template (chart + version + source reference):**

<!-- doc-example: pkg/kubernetes/fluxcd ExampleSetHelmReleaseValuesFromMap -->
```go
hr := fluxcd.CreateHelmRelease("redis", "apps")
hr.Spec.ReleaseName = "redis-prod"
hr.Spec.TargetNamespace = "apps"
hr.Spec.Interval = metav1.Duration{Duration: 10 * time.Minute}
fluxcd.SetHelmReleaseChart(hr, &helmv2.HelmChartTemplate{
    Spec: helmv2.HelmChartTemplateSpec{
        Chart:   "redis",
        Version: "19.0.0",
        SourceRef: &helmv2.CrossNamespaceObjectReference{
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
fmt.Println(hr.Spec.Chart.Spec.Chart, string(hr.Spec.Values.Raw), hr.Spec.ValuesFrom[0].Name)
```
<!-- doc-example:end -->

`SetHelmReleaseValuesFromMap` panics rather than returning an error, because a
sugar helper cannot return one under the builder contract. Only a value that
`encoding/json` refuses outright — a channel, a function, a NaN or `+Inf`
float, a cyclic structure — reaches that panic. Decoded YAML can carry one:
`gopkg.in/yaml.v3` decodes the legal YAML scalars `.nan` and `.inf` into a
`float64` NaN or `+Inf` without error. `sigs.k8s.io/yaml` converts through JSON
and returns that error at decode time instead, so a map it produced always
marshals. When values come from a decoder that could produce such a value,
marshal them yourself and hand the result to `SetHelmReleaseValues`:

<!-- doc-example:excerpt the body of a caller's own function that returns an error -->
```go
import "github.com/go-kure/kure/pkg/errors"

raw, err := json.Marshal(values)
if err != nil {
    return errors.Wrap(err, "helm values")
}
fluxcd.SetHelmReleaseValues(hr, &apiextensionsv1.JSON{Raw: raw})
```

**ChartRef mode (existing OCIRepository or HelmChart):**

<!-- doc-example: pkg/kubernetes/fluxcd ExampleSetHelmReleaseChartRef -->
```go
hr := fluxcd.CreateHelmRelease("my-app", "apps")
fluxcd.SetHelmReleaseChartRef(hr, &helmv2.CrossNamespaceSourceReference{
    Kind:      "OCIRepository",
    Name:      "my-oci-source",
    Namespace: "flux-system",
})
fmt.Println(hr.Spec.ChartRef.Kind, hr.Spec.ChartRef.Name)
```
<!-- doc-example:end -->

**Drift detection and remediation:**

<!-- doc-example: pkg/kubernetes/fluxcd ExampleSetHelmReleaseDriftDetection -->
```go
hr := fluxcd.CreateHelmRelease("my-app", "apps")

fluxcd.SetHelmReleaseDriftDetection(hr, fluxcd.CreateDriftDetection(helmv2.DriftDetectionEnabled))
fluxcd.SetHelmReleaseInstallCRDs(hr, helmv2.CreateReplace)
fluxcd.SetHelmReleaseInstallRemediation(hr, fluxcd.CreateInstallRemediation(3))
fluxcd.SetHelmReleaseUpgradeCRDs(hr, helmv2.CreateReplace)
fluxcd.SetHelmReleaseUpgradeRemediation(hr, fluxcd.CreateUpgradeRemediation(3))
fmt.Println(hr.Spec.DriftDetection.Mode, hr.Spec.Install.CRDs, hr.Spec.Upgrade.Remediation.Retries)
```
<!-- doc-example:end -->

**Post-render:**

<!-- doc-example: pkg/kubernetes/fluxcd ExampleAddHelmReleasePostRenderer -->
```go
hr := fluxcd.CreateHelmRelease("my-app", "apps")

k := fluxcd.CreatePostRendererKustomize()
fluxcd.AddPostRendererKustomizeImage(k, kustomize.Image{Name: "redis", NewTag: "7.0"})
fluxcd.AddHelmReleasePostRenderer(hr, helmv2.PostRenderer{Kustomize: k})
fmt.Println(hr.Spec.PostRenderers[0].Kustomize.Images[0].NewTag)
```
<!-- doc-example:end -->

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

`CreateProvider`, `CreateAlert` and `CreateReceiver` all emit
`notification.toolkit.fluxcd.io/v1`. For Provider and Alert that requires **Flux 2.10 or
newer** on the target cluster: older releases serve them only at `v1beta3` and reject a `v1`
object. The supported Flux range and its per-kind floors are in the
[compatibility matrix](/api-reference/compatibility/#infrastructure-dependencies).

### Migrating Provider and Alert to v1

Flux 2.10 promoted Provider and Alert to `v1`, the stored version. Kure follows:

1. `CreateProvider` and `CreateAlert` return `*notificationv1.Provider` and
   `*notificationv1.Alert`, and every Provider and Alert setter takes the `v1` type. A
   variable declared as the `v1beta3` type no longer compiles. A cluster on Flux older than
   2.10 needs the `v1beta3` object, which kure still registers (so `v1beta3` manifests keep
   parsing) but no longer wraps. Build it with the generic constructor:
   `kubernetes.Create[notificationv1beta3.Provider](name, namespace)`, with
   `notificationv1beta3` importing `github.com/fluxcd/notification-controller/api/v1beta3`.
   Then assign its fields directly.
2. `SetProviderInterval` is removed. The `v1` Provider has no `spec.interval`, and `v1beta3`
   already ignored it. Drop the call.
3. `v1` drops fields, and the result is a compile error in one case and a runtime failure in
   the other:
   - A Go assignment to `Alert.Spec.Summary` or `Provider.Spec.Proxy` no longer compiles. Use
     `AddAlertEventMetadata(alert, "summary", …)` and `Provider.Spec.ProxySecretRef`.
   - Secret content compiles but fails at runtime. notification-controller v1.10 rejects a
     `proxy` key in the `spec.secretRef` Secret: put the proxy URL under the key `address` in a
     separate Secret, reference that Secret from `spec.proxySecretRef`, and delete `proxy` from
     the `spec.secretRef` Secret. A commit-status Provider (`github`, `gitlab`, `gitea`,
     `bitbucket`, `bitbucketserver`, `azuredevops`) rejects TLS data (`ca.crt`, `tls.crt`,
     `tls.key`, or the legacy `caFile`, `certFile`, `keyFile`) in the `spec.secretRef` Secret,
     and every other Provider type ignores it there: move it to a Secret referenced from
     `spec.certSecretRef`. An Azure Event Hub Provider's static JWT `token` is no longer
     accepted: remove it and authenticate with `spec.serviceAccountName` (object identity),
     or another method the upstream Provider documentation lists.
4. Changed Flux Go types around the source and chart template:
   - `OCIRepositorySpec.Interval` is now `*metav1.Duration` and
     `HelmChartTemplateSpec.SourceRef` is now `*helmv2.CrossNamespaceObjectReference`: take the
     address when assigning.
   - `HelmChartTemplateSpec` now embeds `sourcev1.OCIRepositorySpec`, which owns `Interval` and
     `Verify`. A composite literal naming `Interval` or `Verify` directly no longer compiles:
     nest them as `OCIRepositorySpec: sourcev1.OCIRepositorySpec{Interval: &d, Verify: …}`, or
     assign the promoted field (`chart.Spec.Interval = &d`) after construction.
   - `helmv2.HelmChartTemplateVerification` is removed: `Verify` takes
     `*sourcev1.OCIRepositoryVerification`.
   - Removed methods and their replacements:
     - `HelmChartTemplate.GetInterval`: use `HelmRelease.GetTemplateInterval`.
     - `HelmRelease.GetHelmChartName` and `HelmChartTemplate.GetNamespace`: use the `Name` and
       `Namespace` of `HelmRelease.GetHelmChartTemplateReference()`. It returns nil when the
       release sets no `spec.chart`, so check it first. `HelmRelease.GetHelmChartReference`
       is not a replacement: it reads `spec.chartRef` only.
     - `HelmReleaseStatus.GetHelmChart`: use `HelmReleaseStatus.GetHelmChartReference`. It
       returns nil when the status names no chart.
5. A `fluxv1.Dependency` passed to `AddResourceSetDependency` with `ReadyExpr` set must also set
   `Ready: true`. flux-operator v0.62's CRD rejects `readyExpr` without `ready`; before, the
   expression was simply not evaluated. Alternatively, drop `ReadyExpr`.
6. A `meta.KubeConfigReference` with `ConfigMapRef`, passed to `SetKustomizationKubeConfig` or
   `SetHelmReleaseKubeConfig`, now authenticates through object identity. Rename the
   ConfigMap's `audiences` key to `credential.auth.fluxcd.io/audiences`.

### Example

<!-- doc-example: pkg/kubernetes/fluxcd ExampleSetProviderSecretRef -->
```go
provider := fluxcd.CreateProvider("slack", "flux-system")
provider.Spec.Type = "slack" // plain fields: assigned, not set
provider.Spec.Channel = "#alerts"
fluxcd.SetProviderSecretRef(provider, &meta.LocalObjectReference{Name: "slack-webhook"})

alert := fluxcd.CreateAlert("slack-alert", "flux-system")
alert.Spec.ProviderRef = meta.LocalObjectReference{Name: "slack"}
alert.Spec.EventSeverity = "error" // no Alert setters remain: all were bare assignments

receiver := fluxcd.CreateReceiver("github-receiver", "flux-system")
receiver.Spec.Type = "github"
receiver.Spec.Events = []string{"push"}
fluxcd.SetReceiverSecretRef(receiver, meta.LocalObjectReference{Name: "webhook-token"})
fmt.Println(provider.Spec.SecretRef.Name, alert.Spec.EventSeverity, receiver.Spec.SecretRef.Name)
```
<!-- doc-example:end -->

## Image Automation

The three kinds of `image.toolkit.fluxcd.io/v1` each have a constructor:
`CreateImageRepository` and `CreateImagePolicy` (types from
`github.com/fluxcd/image-reflector-controller/api`) and `CreateImageUpdateAutomation`
(type from `github.com/fluxcd/image-automation-controller/api`). All three are
namespaced and carry identity only.

`ImageRepository` and `ImagePolicy` have no setters: every field of their specs is
assigned directly on the upstream type. `ImageUpdateAutomation` keeps the pointer
setters `SetImageUpdateAutomationGitSpec`, `SetImageUpdateAutomationPolicySelector`
and `SetImageUpdateAutomationUpdateStrategy`.

## Flux Operator

<!-- doc-example: pkg/kubernetes/fluxcd ExampleCreateFluxInstance -->
```go
instance := fluxcd.CreateFluxInstance("flux", "flux-system")
instance.Spec.Distribution.Variant = "upstream-alpine" // Distribution is a plain field
// Pointer setters remain for the optional blocks: SetFluxInstanceCluster,
// SetFluxInstanceSharding, SetFluxInstanceStorage, SetFluxInstanceKustomize,
// SetFluxInstanceSync, SetFluxInstanceWait, SetFluxInstanceCommonMetadata and
// SetFluxInstanceMigrateResources. SetFluxInstanceSyncPullSecret and
// SetFluxInstanceSyncProvider write one field of spec.sync, creating it if nil.
fmt.Println(instance.Spec.Distribution.Variant)
```
<!-- doc-example:end -->

## Extended Resource Types

### ExternalArtifact

Allows a Flux source artifact produced outside the cluster to be referenced by other Flux resources.

<!-- doc-example: pkg/kubernetes/fluxcd ExampleSetExternalArtifactSourceRef -->
```go
ea := fluxcd.CreateExternalArtifact("my-artifact", "flux-system")
fluxcd.SetExternalArtifactSourceRef(ea, &meta.NamespacedObjectKindReference{
    APIVersion: "source.toolkit.fluxcd.io/v1",
    Kind:       "OCIRepository",
    Name:       "my-oci-source",
    Namespace:  "flux-system",
})
fmt.Println(ea.Spec.SourceRef.Kind, ea.Spec.SourceRef.Name)
```
<!-- doc-example:end -->

### ArtifactGenerator

Provided by the optional **source-watcher** component. Assembles a new artifact by copying files from one or more source artifacts.

<!-- doc-example: pkg/kubernetes/fluxcd ExampleAddArtifactGeneratorSource -->
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
fmt.Println(ag.Spec.Sources[0].Alias, ag.Spec.OutputArtifacts[0].Name, ag.Spec.OutputArtifacts[0].Copy[0].From)
```
<!-- doc-example:end -->

## Related Packages

- [stack/fluxcd](/api-reference/flux-engine/) — high-level Flux workflow engine
- [stack](/api-reference/stack/) — domain model that produces Flux resources
