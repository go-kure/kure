# Builder contract: release 2 migration notes

This page is the ledger for the second release of the builder contract (ADR-038,
the "thin core + admissible sugar" decision). Release 1 made the upstream Go struct
the construction API and removed the bare forwarders and injected defaults; its
ledger is the [release 1 migration notes](/concepts/builder-contract-release-1/).
Release 2 removes the one construction path that survived it: the `Kind(&Config)`
layer — a function per kind taking a kure-invented `Config` struct and translating
it into the upstream spec — in the six packages that still carried one, together
with the sealed-interface sum types and the values that layer used to inject. The
normative contract text is the [Kubernetes Builders](/api-reference/kubernetes-builders/)
page.

Every removed function, type and field is listed here once, with the upstream
expression that replaces it.

## Why the layer went

In four of the six packages the `Config` was lossy by construction: the vocabulary
reached 5 of a cert-manager `Certificate`'s 24 spec fields, 2 of an issuer's 5 arms
(no Vault, SelfSigned or Venafi), 15 of a CNPG `Cluster`'s 55, 6 of a
`ServiceMonitor`'s 19 — and every upstream release widened the gap. In the other two
(`cilium`, `volsync`) it was a verbatim copy of `obj.Spec = spec`. cert-manager's
`IssuerVariant`, `ACMESolver` and `DNS01Provider` and volsync's `SourceMover` and
`DestinationMover` were interfaces closed by an unexported marker method — the only
compile-time wall in kure: a caller could not add an Azure DNS solver or a Vault
issuer even by hand. The layer also held the last injected values the release 1
purge deferred to it (CNPG `enablePDB`, `primaryUpdateStrategy`, S3 key names, the
barman-cloud plugin entry, the pooler type coercion).

One idiom remains: the generated `Create<Kind>` constructor plus the upstream struct.

## The rule that replaces it

> No exported function under `pkg/kubernetes/...` takes a kure-defined type where
> an upstream spec type exists.

`TestAdmission_NoOwnParameterTypes` (`pkg/kubernetes/admission_params_test.go`)
walks every exported top-level function in the non-test, non-generated files and
fails on any parameter whose type is a struct or interface declared under
`pkg/kubernetes` — reached directly or through pointer, slice, array or map layers.
There is no exclusion list. A named scalar such as `kubernetes.PSALevel` (a string
enum with no upstream spec type) is not a spec type and passes; a defined type over
an upstream struct (volsync's former `SourceResticConfig`) does not.

## How a call site migrates

Every `Kind(&KindConfig{Name: n, Namespace: ns, ...})` becomes the generated
constructor plus assignments on the upstream struct:

```go
// before
cert := certmanager.Certificate(&certmanager.CertificateConfig{
    Name: "api-tls", Namespace: "default",
    SecretName: "api-tls-secret",
    DNSNames:   []string{"api.example.com"},
})

// after
cert := certmanager.CreateCertificate("api-tls", "default")
cert.Spec = certv1.CertificateSpec{
    SecretName: "api-tls-secret",
    DNSNames:   []string{"api.example.com"},
}
```

Cluster-scoped kinds take `(name)` only. Two behaviours of the layer have no
equivalent and need no replacement: a `nil` `*Config` returned a `nil` object
(constructors never return nil), and a typed-nil variant stored in a sealed
interface was treated as "no variant" (there is no interface to store it in — an
unset pointer arm is unset).

The golden fixtures under each package's `testdata/` were written by the layer as it
stood before this release; the tests that now produce them use `Create<Kind>` and
the upstream struct and reproduce that output byte for byte, including every value
the layer used to inject. Those values are the caller's own lines now, and each
package section below lists them.

## `pkg/kubernetes/certmanager`

### Removed functions (3)

| Removed | Replacement |
|---|---|
| `Certificate(&CertificateConfig{...})` | `CreateCertificate(name, namespace)` + `cert.Spec = certv1.CertificateSpec{...}` |
| `Issuer(&IssuerConfig{...})` | `CreateIssuer(name, namespace)` + `SetIssuerACME(issuer, acme)` / `SetIssuerCA(issuer, ca)` |
| `ClusterIssuer(&ClusterIssuerConfig{...})` | `CreateClusterIssuer(name)` + `SetClusterIssuerACME(ci, acme)` / `SetClusterIssuerCA(ci, ca)` |

### Removed types (14) and their fields

The sealed interfaces `IssuerVariant`, `ACMESolver` and `DNS01Provider` are gone
with the eleven structs that implemented or carried them. The upstream one-of is
`certv1.IssuerConfig`, embedded in `IssuerSpec` (the `Spec` type of both `Issuer` and `ClusterIssuer`), with a
pointer per arm: `ACME`, `CA`, `Vault`, `SelfSigned`, `Venafi`.

| Removed field | Upstream field |
|---|---|
| `CertificateConfig.Name`, `.Namespace` | `CreateCertificate(name, namespace)` |
| `CertificateConfig.SecretName` | `cert.Spec.SecretName` |
| `CertificateConfig.IssuerRef` | `cert.Spec.IssuerRef` |
| `CertificateConfig.DNSNames` | `cert.Spec.DNSNames`, or `AddCertificateDNSName(cert, dns)` per name |
| `CertificateConfig.Duration` | `cert.Spec.Duration`, or `SetCertificateDuration(cert, d)` |
| `CertificateConfig.RenewBefore` | `cert.Spec.RenewBefore`, or `SetCertificateRenewBefore(cert, d)` |
| `IssuerConfig.Name`, `.Namespace` | `CreateIssuer(name, namespace)` |
| `IssuerConfig.Variant` = `*ACMEConfig` | `SetIssuerACME(issuer, &cmacme.ACMEIssuer{...})` |
| `IssuerConfig.Variant` = `*CAConfig` | `SetIssuerCA(issuer, &certv1.CAIssuer{...})` |
| `ClusterIssuerConfig.Name` | `CreateClusterIssuer(name)` |
| `ClusterIssuerConfig.Variant` | `SetClusterIssuerACME(ci, acme)` / `SetClusterIssuerCA(ci, ca)` |
| `ACMEConfig.Server` | `cmacme.ACMEIssuer.Server` |
| `ACMEConfig.Email` | `cmacme.ACMEIssuer.Email` |
| `ACMEConfig.PrivateKey` | `cmacme.ACMEIssuer.PrivateKey` |
| `ACMEConfig.Solvers []ACMESolverConfig` | `cmacme.ACMEIssuer.Solvers []cmacme.ACMEChallengeSolver`, or `AddACMEIssuerSolver(acme, solver)` per solver |
| `CAConfig.SecretName` | `certv1.CAIssuer.SecretName` |
| `ACMESolverConfig.Solver` = `*HTTP01SolverConfig` | `cmacme.ACMEChallengeSolver.HTTP01 = &cmacme.ACMEChallengeSolverHTTP01{Ingress: &cmacme.ACMEChallengeSolverHTTP01Ingress{...}}` |
| `ACMESolverConfig.Solver` = `*DNS01SolverConfig` | `cmacme.ACMEChallengeSolver.DNS01 = &cmacme.ACMEChallengeSolverDNS01{...}` |
| `HTTP01SolverConfig.ServiceType` | `cmacme.ACMEChallengeSolverHTTP01Ingress.ServiceType` |
| `HTTP01SolverConfig.IngressClass string` | `cmacme.ACMEChallengeSolverHTTP01Ingress.IngressClassName *string` — `ptr.To(class)`; leave nil to omit |
| `DNS01SolverConfig.Provider` = `*CloudflareProviderConfig` | `cmacme.ACMEChallengeSolverDNS01.Cloudflare = &cmacme.ACMEIssuerDNS01ProviderCloudflare{...}` |
| `DNS01SolverConfig.Provider` = `*Route53ProviderConfig` | `cmacme.ACMEChallengeSolverDNS01.Route53 = &cmacme.ACMEIssuerDNS01ProviderRoute53{...}` |
| `DNS01SolverConfig.Provider` = `*GoogleProviderConfig` | `cmacme.ACMEChallengeSolverDNS01.CloudDNS = &cmacme.ACMEIssuerDNS01ProviderCloudDNS{...}` |
| `CloudflareProviderConfig.Email` | `cmacme.ACMEIssuerDNS01ProviderCloudflare.Email` |
| `CloudflareProviderConfig.APIToken` | `cmacme.ACMEIssuerDNS01ProviderCloudflare.APIToken` |
| `Route53ProviderConfig.Region` | `cmacme.ACMEIssuerDNS01ProviderRoute53.Region` |
| `Route53ProviderConfig.SecretAccessKey *cmmeta.SecretKeySelector` | `cmacme.ACMEIssuerDNS01ProviderRoute53.SecretAccessKey cmmeta.SecretKeySelector` (a value; dereference) |
| `GoogleProviderConfig.Project` | `cmacme.ACMEIssuerDNS01ProviderCloudDNS.Project` |
| `GoogleProviderConfig.ServiceAccount` | `cmacme.ACMEIssuerDNS01ProviderCloudDNS.ServiceAccount` |

### Behaviour of the layer that a struct literal does not have

None of these was documented as a feature; each is listed so a caller relying on
it can see what to write instead.

| The layer used to | Now |
|---|---|
| Leave `ingressClassName` out when `IngressClass` was `""` | `IngressClassName` is a `*string`; nil omits it, `ptr.To("")` writes an empty class |
| Drop a solver with neither `HTTP01` nor `DNS01` set | Nothing is dropped; an empty `ACMEChallengeSolver{}` serialises as `{}` |
| Drop a Cloudflare provider whose `APIToken` was nil, and a Route 53 provider whose `SecretAccessKey` was nil | Nothing is dropped; cert-manager validates the reference at apply time |
| Emit `privateKeySecretRef: {name: ""}` for an ACME issuer with no `PrivateKey` | Unchanged — `cmacme.ACMEIssuer.PrivateKey` is a value field with no `omitempty` upstream, so the literal emits the same |

Golden deltas: none. Every fixture in `pkg/kubernetes/certmanager/testdata` is
reproduced byte for byte.

## `pkg/kubernetes/cnpg`

### Removed functions (5)

| Removed | Replacement |
|---|---|
| `Cluster(&ClusterConfig{...}) (*Cluster, error)` | `CreateCluster(name, namespace)` + `cluster.Spec = cnpgv1.ClusterSpec{...}`; no error return — the two things it could fail on (quantity parsing, the object-store map round-trip) are the caller's now |
| `Database(&DatabaseConfig{...})` | `CreateDatabase(name, namespace)` + `db.Spec = cnpgv1.DatabaseSpec{...}` |
| `ObjectStore(&ObjectStoreConfig{...})` | `CreateObjectStore(name, namespace)` + `store.Spec = barmanv1.ObjectStoreSpec{...}` |
| `ScheduledBackup(&ScheduledBackupConfig{...})` | `CreateScheduledBackup(name, namespace)` + `backup.Spec = spec` |
| `Pooler(&PoolerConfig{...})` | `CreatePooler(name, namespace)` + `pooler.Spec = cnpgv1.PoolerSpec{...}` |

### Removed types (21) and their fields

| Removed field | Upstream field |
|---|---|
| `ClusterConfig.Name`, `.Namespace` | `CreateCluster(name, namespace)` |
| `ClusterConfig.Options *ClusterOptions` | `cluster.Spec` (`cnpgv1.ClusterSpec`) |
| `ClusterOptions.Instances int32` | `Spec.Instances int` |
| `ClusterOptions.ImageName` | `Spec.ImageName` |
| `ClusterOptions.StorageSize` | `Spec.StorageConfiguration.Size` |
| `ClusterOptions.InheritedLabels`, `.InheritedAnnotations` | `Spec.InheritedMetadata = &cnpgv1.EmbeddedObjectMetadata{Labels, Annotations}` |
| `ClusterOptions.Resources *ResourceOptions` | `Spec.Resources` (`corev1.ResourceRequirements`) |
| `ResourceOptions.RequestsCPU`, `.RequestsMemory` (strings) | `Spec.Resources.Requests[corev1.ResourceCPU / ResourceMemory] = q`, with `q, err := resource.ParseQuantity(s)` and the error returned when `s` comes from configuration; `resource.MustParse` only for a literal |
| `ResourceOptions.LimitsCPU`, `.LimitsMemory` (strings) | `Spec.Resources.Limits[corev1.ResourceCPU / ResourceMemory] = q`, with `q, err := resource.ParseQuantity(s)` and the error returned when `s` comes from configuration; `resource.MustParse` only for a literal |
| `ClusterOptions.Backup *BackupOptions` | `Spec.Backup = &cnpgv1.BackupConfiguration{...}` |
| `BackupOptions.DestinationPath`, `.EndpointURL` | `Spec.Backup.BarmanObjectStore = &barmanapi.BarmanObjectStoreConfiguration{DestinationPath, EndpointURL}` |
| `BackupOptions.RetentionPolicy` | `Spec.Backup.RetentionPolicy` |
| `BackupOptions.S3Credentials *S3CredentialOptions` | `Spec.Backup.BarmanObjectStore.AWS = &barmanapi.S3Credentials{...}` (`AWS` is promoted from the embedded `barmanapi.BarmanCredentials`; in a literal it is `BarmanCredentials: barmanapi.BarmanCredentials{AWS: ...}`) |
| `S3CredentialOptions.SecretName`, `.AccessKeyIDKey` | `barmanapi.S3Credentials.AccessKeyIDReference = &machineryapi.SecretKeySelector{LocalObjectReference: {Name: secret}, Key: key}` |
| `S3CredentialOptions.SecretName`, `.SecretAccessKeyKey` | `barmanapi.S3Credentials.SecretAccessKeyReference = &machineryapi.SecretKeySelector{LocalObjectReference: {Name: secret}, Key: key}` |
| `ClusterOptions.Monitoring *MonitoringOptions` | `Spec.Monitoring = &cnpgv1.MonitoringConfiguration{...}` |
| `MonitoringOptions.EnablePodMonitor` | `Spec.Monitoring.EnablePodMonitor` (deprecated upstream, still the only opt-in) |
| `MonitoringOptions.CustomQueriesConfigMap []ConfigMapKeyRefOptions` | `Spec.Monitoring.CustomQueriesConfigMap []cnpgv1.ConfigMapKeySelector` |
| `ConfigMapKeyRefOptions.Name`, `.Key` | `cnpgv1.ConfigMapKeySelector{LocalObjectReference: machineryapi.LocalObjectReference{Name}, Key}` |
| `ClusterOptions.Bootstrap *BootstrapOptions` | `Spec.Bootstrap = &cnpgv1.BootstrapConfiguration{...}` |
| `BootstrapOptions.RecoverySource` | `Spec.Bootstrap.Recovery = &cnpgv1.BootstrapRecovery{Source}` |
| `BootstrapOptions.PgBasebackupSource` | `Spec.Bootstrap.PgBaseBackup = &cnpgv1.BootstrapPgBaseBackup{Source}` |
| `ClusterOptions.ExternalClusters []ExternalClusterOptions` | `Spec.ExternalClusters []cnpgv1.ExternalCluster` |
| `ExternalClusterOptions.Name`, `.ConnectionParameters` | `cnpgv1.ExternalCluster.Name`, `.ConnectionParameters` |
| `ExternalClusterOptions.BarmanObjectStore map[string]any` | `cnpgv1.ExternalCluster.BarmanObjectStore *barmanapi.BarmanObjectStoreConfiguration` — a typed literal; a caller holding a map does its own `json.Marshal` / `json.Unmarshal` into the upstream type |
| `ClusterOptions.PostgresParams` | `Spec.PostgresConfiguration.Parameters` |
| `ClusterOptions.Synchronous *SynchronousOptions` | `Spec.PostgresConfiguration.Synchronous = &cnpgv1.SynchronousReplicaConfiguration{...}` |
| `SynchronousOptions.Method string` | `.Method cnpgv1.SynchronousReplicaConfigurationMethod` (`SynchronousReplicaConfigurationMethodAny` / `First`) |
| `SynchronousOptions.Number int32` | `.Number int` |
| `SynchronousOptions.DataDurability string` | `.DataDurability cnpgv1.DataDurabilityLevel` (`DataDurabilityLevelRequired` / `Preferred`) |
| `SynchronousOptions.MaxStandbyDelay int32` | nothing — the layer never wrote it, and `SynchronousReplicaConfiguration` has no such field (`MaxStandbyNamesFromCluster` is the nearest) |
| `ClusterOptions.ObjectStoreName` | `Spec.Plugins = []cnpgv1.PluginConfiguration{{Name: "barman-cloud.barmancloud.cnpg.io", IsWALArchiver: ptr.To(true), Parameters: map[string]string{"objectStoreName": name}}}` |
| `ClusterOptions.Affinity *AffinityOptions` | `Spec.Affinity` (`cnpgv1.AffinityConfiguration`, a value) |
| `AffinityOptions.EnablePodAntiAffinity bool` | `Spec.Affinity.EnablePodAntiAffinity *bool` — `ptr.To(b)` |
| `AffinityOptions.TopologyKey`, `.PodAntiAffinityType`, `.NodeSelector` | `Spec.Affinity.TopologyKey`, `.PodAntiAffinityType` (`cnpgv1.PodAntiAffinityTypePreferred` / `Required`), `.NodeSelector` |
| `ClusterOptions.ManagedRoles []ManagedRoleOptions` | `Spec.Managed = &cnpgv1.ManagedConfiguration{Roles: []cnpgv1.RoleConfiguration{...}}`, or `AddClusterManagedRole(cluster, role)` per role |
| `ManagedRoleOptions.Name`, `.Comment`, `.Login`, `.Superuser`, `.CreateDB`, `.CreateRole`, `.Replication`, `.Inherit *bool`, `.InRoles` | the same-named fields of `cnpgv1.RoleConfiguration` |
| `ManagedRoleOptions.ConnectionLimit *int64` | `cnpgv1.RoleConfiguration.ConnectionLimit int64` (a value; `-1` is the upstream default) |
| `ManagedRoleOptions.PasswordSecret string` | `cnpgv1.RoleConfiguration.PasswordSecret = &cnpgv1.LocalObjectReference{Name}` |
| `ManagedRoleOptions.Ensure string` | `cnpgv1.RoleConfiguration.Ensure cnpgv1.EnsureOption` (`EnsurePresent` / `EnsureAbsent`) |
| `DatabaseConfig.Name`, `.Namespace` | `CreateDatabase(name, namespace)` |
| `DatabaseConfig.Options *DatabaseOptions` | `db.Spec` (`cnpgv1.DatabaseSpec`) |
| `DatabaseOptions.ClusterName` | `Spec.ClusterRef = corev1.LocalObjectReference{Name}` |
| `DatabaseOptions.DBName` | `Spec.Name` |
| `DatabaseOptions.Owner` | `Spec.Owner` |
| `DatabaseOptions.ReclaimPolicy string` (`"delete"` or `""`) | `Spec.ReclaimPolicy cnpgv1.DatabaseReclaimPolicy` (`DatabaseReclaimDelete` / `DatabaseReclaimRetain`) |
| `DatabaseOptions.Ensure string` (`"absent"` or `""`) | `Spec.Ensure cnpgv1.EnsureOption` |
| `DatabaseOptions.Extensions []ExtensionOptions` | `Spec.Extensions []cnpgv1.ExtensionSpec`, or `AddDatabaseExtension(db, ext)` per extension |
| `ExtensionOptions.Name`, `.Ensure` | `cnpgv1.ExtensionSpec{DatabaseObjectSpec: cnpgv1.DatabaseObjectSpec{Name, Ensure}}` |
| `ObjectStoreConfig.Name`, `.Namespace` | `CreateObjectStore(name, namespace)` |
| `ObjectStoreConfig.Options *ObjectStoreOptions` | `store.Spec` (`barmanv1.ObjectStoreSpec`) |
| `ObjectStoreOptions.DestinationPath`, `.EndpointURL`, `.ServerName` | `Spec.Configuration.DestinationPath`, `.EndpointURL`, `.ServerName` |
| `ObjectStoreOptions.SecretName`, `.AccessKeyIDKey`, `.SecretAccessKeyKey` | `SetObjectStoreS3Credentials(store, &barmanapi.S3Credentials{...})` with the two `machineryapi.SecretKeySelector` references spelled out |
| `ObjectStoreOptions.RetentionPolicy` | `Spec.RetentionPolicy` |
| `ScheduledBackupConfig.Name`, `.Namespace`, `.Spec` | `CreateScheduledBackup(name, namespace)`; `backup.Spec = spec` |
| `PoolerConfig.Name`, `.Namespace` | `CreatePooler(name, namespace)` |
| `PoolerConfig.Options *PoolerOptions` | `pooler.Spec` (`cnpgv1.PoolerSpec`) |
| `PoolerOptions.ClusterName` | `Spec.Cluster = cnpgv1.LocalObjectReference{Name}` |
| `PoolerOptions.Instances int32` (0 = omit) | `Spec.Instances *int32` — `ptr.To[int32](n)`; nil omits |
| `PoolerOptions.Type string` (`"ro"`, anything else `rw`) | `Spec.Type cnpgv1.PoolerType` (`PoolerTypeRW` / `PoolerTypeRO`) |
| `PoolerOptions.PgBouncer *PgBouncerOptions` | `Spec.PgBouncer = &cnpgv1.PgBouncerSpec{...}` (required upstream: no `omitempty`, so always set it, empty if nothing else) |
| `PgBouncerOptions.PoolMode string` | `cnpgv1.PgBouncerSpec.PoolMode cnpgv1.PgBouncerPoolMode` (`PgBouncerPoolModeSession` / `Transaction`) |
| `PgBouncerOptions.Parameters` | `cnpgv1.PgBouncerSpec.Parameters` |

### Values the layer injected

Each of these was a line in emitted YAML the caller never wrote. The release 1
defaults purge deferred them to this release (they lived in the layer, not in a
constructor or a sugar helper). They are all expressible — the re-authored goldens
write every one of them — but nothing writes them for you.

| Injected value | Old trigger | Write it yourself as |
|---|---|---|
| `spec.enablePDB: true` / `false` | derived from `Instances > 1` | `Spec.EnablePDB = ptr.To(instances > 1)` — or the value you actually want |
| `spec.primaryUpdateStrategy: unsupervised` | always | `Spec.PrimaryUpdateStrategy = cnpgv1.PrimaryUpdateStrategyUnsupervised` |
| S3 key names `ACCESS_KEY_ID` / `SECRET_ACCESS_KEY` | a `SecretName` with empty key names, on `ObjectStoreOptions` and `S3CredentialOptions` | the `Key` of each `machineryapi.SecretKeySelector` |
| `spec.plugins[0]` = `barman-cloud.barmancloud.cnpg.io`, `isWALArchiver: true` | `ObjectStoreName != ""` | the `cnpgv1.PluginConfiguration` literal above |
| Pooler `spec.type: rw` | any `Type` other than `"ro"`, including `""` | `Spec.Type = cnpgv1.PoolerTypeRW` |
| Pooler `spec.pgbouncer: {}` | always | `Spec.PgBouncer = &cnpgv1.PgBouncerSpec{}` |
| Extension `ensure: present` | every extension without `Ensure: "absent"` | `DatabaseObjectSpec.Ensure = cnpgv1.EnsurePresent` |
| `Database` `ensure` / `databaseReclaimPolicy` dropped for any string other than `"absent"` / `"delete"` | string normalisation | the upstream constant; there is no string to normalise |
| `spec.bootstrap` chose `recovery` over `pg_basebackup` when both sources were set | precedence in the layer | set the one arm you mean |
| `spec.backup` omitted when both `DestinationPath` and `RetentionPolicy` were empty | a guard in the layer | leave `Spec.Backup` nil |
| `spec.monitoring` omitted, `CustomQueriesConfigMap` included, unless `EnablePodMonitor` was true | a guard in the layer | set `Spec.Monitoring` whenever you want custom queries, with or without `EnablePodMonitor` |
| `postgresql.synchronous` omitted, `Number` and `DataDurability` included, unless `Method` was non-empty | a guard in the layer | leave `Spec.PostgresConfiguration.Synchronous` nil, or set it with a `Method` |

Golden deltas: none. Every fixture in `pkg/kubernetes/cnpg/testdata` is reproduced
byte for byte, the injected values above included.

## `pkg/kubernetes/externalsecrets`

### Removed functions (3)

| Removed | Replacement |
|---|---|
| `ExternalSecret(&ExternalSecretConfig{...})` | `CreateExternalSecret(name, namespace)` + `es.Spec.SecretStoreRef = ref` + `AddExternalSecretData(es, d)` per entry |
| `SecretStore(&SecretStoreConfig{...})` | `CreateSecretStore(name, namespace)` + `SetSecretStoreProvider(ss, p)` + `ss.Spec.Controller = c` |
| `ClusterSecretStore(&ClusterSecretStoreConfig{...})` | `CreateClusterSecretStore(name)` + `SetClusterSecretStoreProvider(css, p)` + `css.Spec.Controller = c` |

### Removed types (3) and their fields

| Removed field | Upstream field |
|---|---|
| `ExternalSecretConfig.Name`, `.Namespace` | `CreateExternalSecret(name, namespace)` |
| `ExternalSecretConfig.SecretStoreRef` | `es.Spec.SecretStoreRef` |
| `ExternalSecretConfig.Data []esv1.ExternalSecretData` | `es.Spec.Data`, or `AddExternalSecretData(es, d)` per entry |
| `SecretStoreConfig.Name`, `.Namespace` | `CreateSecretStore(name, namespace)` |
| `SecretStoreConfig.Provider *esv1.SecretStoreProvider` | `ss.Spec.Provider`, or `SetSecretStoreProvider(ss, p)` |
| `SecretStoreConfig.Controller` | `ss.Spec.Controller` |
| `ClusterSecretStoreConfig.Name` | `CreateClusterSecretStore(name)` |
| `ClusterSecretStoreConfig.Provider *esv1.SecretStoreProvider` | `css.Spec.Provider`, or `SetClusterSecretStoreProvider(css, p)` |
| `ClusterSecretStoreConfig.Controller` | `css.Spec.Controller` |

The layer skipped `Provider` when nil and `Controller` when empty; a nil pointer and
an empty string serialise to nothing either way, so there is no behaviour to
replace. Golden deltas: none. Every fixture in
`pkg/kubernetes/externalsecrets/testdata` is reproduced byte for byte.

## `pkg/kubernetes/prometheus`

### Removed functions (3)

| Removed | Replacement |
|---|---|
| `ServiceMonitor(&ServiceMonitorConfig{...})` | `CreateServiceMonitor(name, namespace)` + `sm.Spec = monitoringv1.ServiceMonitorSpec{...}` |
| `PodMonitor(&PodMonitorConfig{...})` | `CreatePodMonitor(name, namespace)` + `pm.Spec = monitoringv1.PodMonitorSpec{...}` |
| `PrometheusRule(&PrometheusRuleConfig{...})` | `CreatePrometheusRule(name, namespace)` + `AddPrometheusRuleGroup(rule, g)` per group |

### Removed types (3) and their fields

| Removed field | Upstream field |
|---|---|
| `ServiceMonitorConfig.Name`, `.Namespace` | `CreateServiceMonitor(name, namespace)` |
| `ServiceMonitorConfig.Selector` | `sm.Spec.Selector` |
| `ServiceMonitorConfig.Endpoints` | `sm.Spec.Endpoints`, or `AddServiceMonitorEndpoint(sm, ep)` per endpoint |
| `ServiceMonitorConfig.JobLabel` | `sm.Spec.JobLabel` |
| `ServiceMonitorConfig.TargetLabels` | `sm.Spec.TargetLabels`, or `AddServiceMonitorTargetLabel(sm, l)` per label |
| `ServiceMonitorConfig.NamespaceSelector *monitoringv1.NamespaceSelector` | `sm.Spec.NamespaceSelector monitoringv1.NamespaceSelector` (a value; dereference) |
| `ServiceMonitorConfig.SampleLimit *int64` | `SetServiceMonitorSampleLimit(sm, n)` |
| `ServiceMonitorConfig.Labels map[string]string` | `kubernetes.SetLabels(sm, labels)` — it was `metadata.labels` |
| `PodMonitorConfig.Name`, `.Namespace` | `CreatePodMonitor(name, namespace)` |
| `PodMonitorConfig.Selector` | `pm.Spec.Selector` |
| `PodMonitorConfig.PodMetricsEndpoints` | `pm.Spec.PodMetricsEndpoints`, or `AddPodMonitorEndpoint(pm, ep)` per endpoint |
| `PodMonitorConfig.JobLabel` | `pm.Spec.JobLabel` |
| `PodMonitorConfig.PodTargetLabels` | `pm.Spec.PodTargetLabels`, or `AddPodMonitorPodTargetLabel(pm, l)` per label |
| `PodMonitorConfig.NamespaceSelector *monitoringv1.NamespaceSelector` | `pm.Spec.NamespaceSelector monitoringv1.NamespaceSelector` (a value; dereference) |
| `PodMonitorConfig.SampleLimit *int64` | `SetPodMonitorSampleLimit(pm, n)` |
| `PodMonitorConfig.Labels map[string]string` | `kubernetes.SetLabels(pm, labels)` |
| `PrometheusRuleConfig.Name`, `.Namespace` | `CreatePrometheusRule(name, namespace)` |
| `PrometheusRuleConfig.Groups` | `rule.Spec.Groups`, or `AddPrometheusRuleGroup(rule, g)` per group |
| `PrometheusRuleConfig.Labels map[string]string` | `kubernetes.SetLabels(rule, labels)` |

The layer skipped `JobLabel` when empty, `NamespaceSelector` and `SampleLimit` when
nil and `Labels` when nil; an empty string, a nil pointer and a nil map serialise to
nothing either way, so there is no behaviour to replace. `spec.endpoints` and
`spec.podMetricsEndpoints` render `null` when unset — release 1 recorded that
change when the layer stopped seeding empty slices, and a struct literal is the
same. Golden deltas: none. Every fixture in `pkg/kubernetes/prometheus/testdata` is
reproduced byte for byte.

## `pkg/kubernetes/cilium`

### Removed functions (13)

Every one of these was `obj.Spec = cfg.Spec` (or the `Spec` / `Specs` pair on the two
policy kinds, or the `ExternalCIDRs` list on the CIDR group) behind a `Name` and, for
the namespaced kinds, a `Namespace`. The replacement is the generated constructor and
the same assignment.

| Removed | Replacement |
|---|---|
| `CiliumNetworkPolicy(&CiliumNetworkPolicyConfig{...})` | `CreateCiliumNetworkPolicy(name, namespace)` + `SetCiliumNetworkPolicySpec(obj, rule)` / `AddCiliumNetworkPolicySpec(obj, rule)` |
| `CiliumClusterwideNetworkPolicy(&CiliumClusterwideNetworkPolicyConfig{...})` | `CreateCiliumClusterwideNetworkPolicy(name)` + `SetCiliumClusterwideNetworkPolicySpec(obj, rule)` / `AddCiliumClusterwideNetworkPolicySpec(obj, rule)` |
| `CiliumCIDRGroup(&CiliumCIDRGroupConfig{...})` | `CreateCiliumCIDRGroup(name)` + `obj.Spec.ExternalCIDRs = cidrs` |
| `CiliumEgressGatewayPolicy(&CiliumEgressGatewayPolicyConfig{...})` | `CreateCiliumEgressGatewayPolicy(name)` + `obj.Spec = spec` |
| `CiliumLocalRedirectPolicy(&CiliumLocalRedirectPolicyConfig{...})` | `CreateCiliumLocalRedirectPolicy(name, namespace)` + `obj.Spec = spec` |
| `CiliumLoadBalancerIPPool(&CiliumLoadBalancerIPPoolConfig{...})` | `CreateCiliumLoadBalancerIPPool(name)` + `obj.Spec = spec` |
| `CiliumEnvoyConfig(&CiliumEnvoyConfigConfig{...})` | `CreateCiliumEnvoyConfig(name, namespace)` + `obj.Spec = spec` |
| `CiliumClusterwideEnvoyConfig(&CiliumClusterwideEnvoyConfigConfig{...})` | `CreateCiliumClusterwideEnvoyConfig(name)` + `obj.Spec = spec` |
| `CiliumBGPClusterConfig(&CiliumBGPClusterConfigConfig{...})` | `CreateCiliumBGPClusterConfig(name)` + `obj.Spec = spec` |
| `CiliumBGPPeerConfig(&CiliumBGPPeerConfigConfig{...})` | `CreateCiliumBGPPeerConfig(name)` + `obj.Spec = spec` |
| `CiliumBGPAdvertisement(&CiliumBGPAdvertisementConfig{...})` | `CreateCiliumBGPAdvertisement(name)` + `obj.Spec = spec` |
| `CiliumBGPNodeConfig(&CiliumBGPNodeConfigConfig{...})` | `CreateCiliumBGPNodeConfig(name)` + `obj.Spec = spec` |
| `CiliumBGPNodeConfigOverride(&CiliumBGPNodeConfigOverrideConfig{...})` | `CreateCiliumBGPNodeConfigOverride(name)` + `obj.Spec = spec` |

### Removed types (13) and their fields

| Removed field | Upstream field |
|---|---|
| `<Kind>Config.Name`, `.Namespace` | `Create<Kind>(name, namespace)`, or `Create<Kind>(name)` for the cluster-scoped kinds |
| `CiliumNetworkPolicyConfig.Spec *api.Rule` | `obj.Spec` (`*api.Rule`), or `SetCiliumNetworkPolicySpec(obj, rule)` |
| `CiliumNetworkPolicyConfig.Specs api.Rules` | `obj.Specs`, or `AddCiliumNetworkPolicySpec(obj, rule)` per rule |
| `CiliumClusterwideNetworkPolicyConfig.Spec`, `.Specs` | `obj.Spec` / `obj.Specs`, or the `SetCiliumClusterwideNetworkPolicySpec` / `AddCiliumClusterwideNetworkPolicySpec` pair |
| `CiliumCIDRGroupConfig.ExternalCIDRs []api.CIDR` | `obj.Spec.ExternalCIDRs`, or `AddCiliumCIDRGroupCIDR(obj, cidr)` per CIDR |
| `CiliumEgressGatewayPolicyConfig.Spec` | `obj.Spec` (`ciliumv2.CiliumEgressGatewayPolicySpec`) |
| `CiliumLocalRedirectPolicyConfig.Spec` | `obj.Spec` (`ciliumv2.CiliumLocalRedirectPolicySpec`) |
| `CiliumLoadBalancerIPPoolConfig.Spec` | `obj.Spec` (`ciliumv2.CiliumLoadBalancerIPPoolSpec`) |
| `CiliumEnvoyConfigConfig.Spec`, `CiliumClusterwideEnvoyConfigConfig.Spec` | `obj.Spec` (`ciliumv2.CiliumEnvoyConfigSpec`) |
| `CiliumBGPClusterConfigConfig.Spec` | `obj.Spec` (`ciliumv2.CiliumBGPClusterConfigSpec`) |
| `CiliumBGPPeerConfigConfig.Spec` | `obj.Spec` (`ciliumv2.CiliumBGPPeerConfigSpec`) |
| `CiliumBGPAdvertisementConfig.Spec` | `obj.Spec` (`ciliumv2.CiliumBGPAdvertisementSpec`) |
| `CiliumBGPNodeConfigConfig.Spec` | `obj.Spec` (`ciliumv2.CiliumBGPNodeSpec`) |
| `CiliumBGPNodeConfigOverrideConfig.Spec` | `obj.Spec` (`ciliumv2.CiliumBGPNodeConfigOverrideSpec`) |

The two policy builders skipped a nil `Spec`; a nil pointer serialises to nothing
either way. Golden deltas: none. Every fixture in `pkg/kubernetes/cilium/testdata`
is reproduced byte for byte.
