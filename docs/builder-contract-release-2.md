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
fails on any parameter whose type names a struct or interface this tree defines —
declared under `pkg/kubernetes`, written as a literal in the signature, or reached
through pointer, slice, array, map or channel layers, a callback's parameters and
results, an alias or a named container (`type Config = *struct{...}`), or a type
argument of an instantiated generic or generic alias, or a type parameter's
constraint (its embedded types and union terms included). Upstream
named types are not otherwise entered. There is no exclusion list. A named scalar such as `kubernetes.PSALevel` (a string
enum with no upstream spec type) is not a spec type and passes; a defined type over
an upstream struct (volsync's former `SourceResticConfig`) does not.

## How a call site migrates

Every `Kind(&KindConfig{Name: n, Namespace: ns, ...})` becomes the generated
constructor plus assignments on the upstream struct:

<!-- doc-example:excerpt migration ledger: a before/after pair whose before half calls a builder this release removed -->
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

Most of the remaining differences share one shape. Where the layer guarded a field,
it set a pointer only when the input behind it was non-empty or positive, and it
filled a list by appending in a loop, so an empty input left the field nil. A struct
literal writes exactly what it is given, so for those fields an assigned empty value
can change the manifest: an empty slice renders `[]` where nil renders `null` on a
field without `omitempty`, and a pointer to an empty struct renders `{}`. The guard
is a property of each field, not a general default — the rows below that carry one
state its condition, and only those fields should be left nil for an empty input.
A pointer the layer always wrote, whatever its value, has no such guard: CNPG's
`EnablePodAntiAffinity` was written as `false` when asked for `false`, and leaving it
nil instead lets the operator apply anti-affinity.

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
| `ClusterOptions.InheritedLabels`, `.InheritedAnnotations` | `Spec.InheritedMetadata = &cnpgv1.EmbeddedObjectMetadata{Labels, Annotations}` — only when either map is non-empty; the layer left it nil otherwise, and a non-nil empty one renders `inheritedMetadata: {}` |
| `ClusterOptions.Resources *ResourceOptions` | `Spec.Resources` (`corev1.ResourceRequirements`) |
| `ResourceOptions.RequestsCPU`, `.RequestsMemory` (strings) | `Spec.Resources.Requests[corev1.ResourceCPU / ResourceMemory] = q`, with `q, err := resource.ParseQuantity(s)` and the error returned when `s` comes from configuration; `resource.MustParse` only for a literal; one entry per non-empty string — the layer skipped an empty one, and allocated the map only when at least one was set |
| `ResourceOptions.LimitsCPU`, `.LimitsMemory` (strings) | `Spec.Resources.Limits[corev1.ResourceCPU / ResourceMemory] = q`, with `q, err := resource.ParseQuantity(s)` and the error returned when `s` comes from configuration; `resource.MustParse` only for a literal; one entry per non-empty string — the layer skipped an empty one, and allocated the map only when at least one was set |
| `ClusterOptions.Backup *BackupOptions` | `Spec.Backup = &cnpgv1.BackupConfiguration{...}` |
| `BackupOptions.DestinationPath`, `.EndpointURL` | `Spec.Backup.BarmanObjectStore = &barmanapi.BarmanObjectStoreConfiguration{DestinationPath, EndpointURL}` |
| `BackupOptions.RetentionPolicy` | `Spec.Backup.RetentionPolicy` |
| `BackupOptions.S3Credentials *S3CredentialOptions` | `Spec.Backup.BarmanObjectStore.AWS = &barmanapi.S3Credentials{...}` (`AWS` is promoted from the embedded `barmanapi.BarmanCredentials`; in a literal it is `BarmanCredentials: barmanapi.BarmanCredentials{AWS: ...}`) — only when `SecretName` is non-empty; the layer left `AWS` nil otherwise, and selectors with an empty secret name are not the same manifest |
| `S3CredentialOptions.SecretName`, `.AccessKeyIDKey` | `barmanapi.S3Credentials.AccessKeyIDReference = &machineryapi.SecretKeySelector{LocalObjectReference: {Name: secret}, Key: key}` |
| `S3CredentialOptions.SecretName`, `.SecretAccessKeyKey` | `barmanapi.S3Credentials.SecretAccessKeyReference = &machineryapi.SecretKeySelector{LocalObjectReference: {Name: secret}, Key: key}` |
| `ClusterOptions.Monitoring *MonitoringOptions` | `Spec.Monitoring = &cnpgv1.MonitoringConfiguration{...}` |
| `MonitoringOptions.EnablePodMonitor` | `Spec.Monitoring.EnablePodMonitor` (deprecated upstream, still the only opt-in) |
| `MonitoringOptions.CustomQueriesConfigMap []ConfigMapKeyRefOptions` | `Spec.Monitoring.CustomQueriesConfigMap []cnpgv1.ConfigMapKeySelector` |
| `ConfigMapKeyRefOptions.Name`, `.Key` | `cnpgv1.ConfigMapKeySelector{LocalObjectReference: machineryapi.LocalObjectReference{Name}, Key}` |
| `ClusterOptions.Bootstrap *BootstrapOptions` | `Spec.Bootstrap = &cnpgv1.BootstrapConfiguration{...}` — only when one of the two sources is non-empty; with both empty the layer left `Spec.Bootstrap` nil, and CNPG then defaults to `initdb`. A bootstrap block with an empty `recovery` or `pg_basebackup` arm selects that mode instead |
| `BootstrapOptions.RecoverySource` | `Spec.Bootstrap.Recovery = &cnpgv1.BootstrapRecovery{Source}` — only for a non-empty source; it took precedence over `PgBasebackupSource` when both were set |
| `BootstrapOptions.PgBasebackupSource` | `Spec.Bootstrap.PgBaseBackup = &cnpgv1.BootstrapPgBaseBackup{Source}` — only for a non-empty source, and only when `RecoverySource` was empty |
| `ClusterOptions.ExternalClusters []ExternalClusterOptions` | `Spec.ExternalClusters []cnpgv1.ExternalCluster` (the layer assigned it only when non-empty; the field is `omitempty`, so an empty slice renders the same) |
| `ExternalClusterOptions.Name`, `.ConnectionParameters` | `cnpgv1.ExternalCluster.Name`, `.ConnectionParameters` |
| `ExternalClusterOptions.BarmanObjectStore map[string]any` | `cnpgv1.ExternalCluster.BarmanObjectStore *barmanapi.BarmanObjectStoreConfiguration` — a typed literal; a caller holding a map does its own `json.Marshal` / `json.Unmarshal` into the upstream type |
| `ClusterOptions.PostgresParams` | `Spec.PostgresConfiguration.Parameters` |
| `ClusterOptions.Synchronous *SynchronousOptions` | `Spec.PostgresConfiguration.Synchronous = &cnpgv1.SynchronousReplicaConfiguration{...}` |
| `SynchronousOptions.Method string` | `.Method cnpgv1.SynchronousReplicaConfigurationMethod` (`SynchronousReplicaConfigurationMethodAny` / `First`) |
| `SynchronousOptions.Number int32` | `.Number int` |
| `SynchronousOptions.DataDurability string` | `.DataDurability cnpgv1.DataDurabilityLevel` (`DataDurabilityLevelRequired` / `Preferred`) — only for a non-empty former value; `""` left it empty (`omitempty`) |
| `SynchronousOptions.MaxStandbyDelay int32` | nothing — the layer never wrote it, and `SynchronousReplicaConfiguration` has no such field (`MaxStandbyNamesFromCluster` is the nearest) |
| `ClusterOptions.ObjectStoreName` | `Spec.Plugins = []cnpgv1.PluginConfiguration{{Name: "barman-cloud.barmancloud.cnpg.io", IsWALArchiver: ptr.To(true), Parameters: map[string]string{"objectStoreName": name}}}` |
| `ClusterOptions.Affinity *AffinityOptions` | `Spec.Affinity` (`cnpgv1.AffinityConfiguration`, a value) |
| `AffinityOptions.EnablePodAntiAffinity bool` | `Spec.Affinity.EnablePodAntiAffinity *bool` — `ptr.To(b)`, including `ptr.To(false)`: the layer always wrote it when `Affinity` was supplied, and CNPG treats nil as enabled (pinned by `cluster-affinity-disabled.yaml`) |
| `AffinityOptions.TopologyKey`, `.PodAntiAffinityType`, `.NodeSelector` | `Spec.Affinity.TopologyKey`, `.PodAntiAffinityType` (`cnpgv1.PodAntiAffinityTypePreferred` / `Required`), `.NodeSelector` — copied as given; an empty type stayed empty for the operator to default |
| `ClusterOptions.ManagedRoles []ManagedRoleOptions` | `Spec.Managed = &cnpgv1.ManagedConfiguration{Roles: []cnpgv1.RoleConfiguration{...}}`, or `AddClusterManagedRole(cluster, role)` per role — only when there is at least one role; the layer left `Managed` nil otherwise, and a non-nil empty one renders `managed: {}` |
| `ManagedRoleOptions.Name`, `.Comment`, `.Login`, `.Superuser`, `.CreateDB`, `.CreateRole`, `.Replication`, `.Inherit *bool`, `.InRoles` | the same-named fields of `cnpgv1.RoleConfiguration` <!-- doc-api-refs:ignore CreateDB and CreateRole are upstream struct fields, not builders --> |
| `ManagedRoleOptions.ConnectionLimit *int64` | `cnpgv1.RoleConfiguration.ConnectionLimit int64` — `*p` only for a non-nil pointer; a nil one left the field `0`, which is `omitempty` and lets the operator apply its own default. Do not translate nil to an explicit `-1` |
| `ManagedRoleOptions.PasswordSecret string` | `cnpgv1.RoleConfiguration.PasswordSecret = &cnpgv1.LocalObjectReference{Name}` — only when the name is non-empty; the layer left it nil for `""`, and a reference with an empty name is not the same manifest |
| `ManagedRoleOptions.Ensure string` | `cnpgv1.RoleConfiguration.Ensure cnpgv1.EnsureOption` — `cnpgv1.EnsureAbsent` only for the former `"absent"`; every other value, `""` included, left the field empty (`omitempty`, and the operator defaults to present), so do not write `EnsurePresent` for it |
| `DatabaseConfig.Name`, `.Namespace` | `CreateDatabase(name, namespace)` |
| `DatabaseConfig.Options *DatabaseOptions` | `db.Spec` (`cnpgv1.DatabaseSpec`) |
| `DatabaseOptions.ClusterName` | `Spec.ClusterRef = corev1.LocalObjectReference{Name}` |
| `DatabaseOptions.DBName` | `Spec.Name` |
| `DatabaseOptions.Owner` | `Spec.Owner` |
| `DatabaseOptions.ReclaimPolicy string` (`"delete"` or `""`) | `Spec.ReclaimPolicy cnpgv1.DatabaseReclaimPolicy` — `DatabaseReclaimDelete` only for the former `"delete"`; anything else left it empty (`omitempty`, operator default retain), so do not write `DatabaseReclaimRetain` for it |
| `DatabaseOptions.Ensure string` (`"absent"` or `""`) | `Spec.Ensure cnpgv1.EnsureOption` — `EnsureAbsent` only for the former `"absent"`; anything else left it empty (`omitempty`, operator default present), so do not write `EnsurePresent` for it |
| `DatabaseOptions.Extensions []ExtensionOptions` | `Spec.Extensions []cnpgv1.ExtensionSpec`, or `AddDatabaseExtension(db, ext)` per extension |
| `ExtensionOptions.Name`, `.Ensure` | `cnpgv1.ExtensionSpec{DatabaseObjectSpec: cnpgv1.DatabaseObjectSpec{Name, Ensure}}` |
| `ObjectStoreConfig.Name`, `.Namespace` | `CreateObjectStore(name, namespace)` |
| `ObjectStoreConfig.Options *ObjectStoreOptions` | `store.Spec` (`barmanv1.ObjectStoreSpec`) |
| `ObjectStoreOptions.DestinationPath`, `.EndpointURL`, `.ServerName` | `Spec.Configuration.DestinationPath`, `.EndpointURL`, `.ServerName` |
| `ObjectStoreOptions.SecretName`, `.AccessKeyIDKey`, `.SecretAccessKeyKey` | `SetObjectStoreS3Credentials(store, &barmanapi.S3Credentials{...})` with the two `machineryapi.SecretKeySelector` references spelled out — only when `SecretName` is non-empty; the layer left `AWS` nil otherwise |
| `ObjectStoreOptions.RetentionPolicy` | `Spec.RetentionPolicy` |
| `ScheduledBackupConfig.Name`, `.Namespace`, `.Spec` | `CreateScheduledBackup(name, namespace)`; `backup.Spec = spec` |
| `PoolerConfig.Name`, `.Namespace` | `CreatePooler(name, namespace)` |
| `PoolerConfig.Options *PoolerOptions` | `pooler.Spec` (`cnpgv1.PoolerSpec`) |
| `PoolerOptions.ClusterName` | `Spec.Cluster = cnpgv1.LocalObjectReference{Name}` |
| `PoolerOptions.Instances int32` (≤ 0 = omit) | `Spec.Instances *int32` — `ptr.To[int32](n)` only for `n > 0`; the layer omitted it for zero and for a negative count, leaving the operator default |
| `PoolerOptions.Type string` (`"ro"`, anything else `rw`) | `Spec.Type cnpgv1.PoolerType` (`PoolerTypeRW` / `PoolerTypeRO`) |
| `PoolerOptions.PgBouncer *PgBouncerOptions` | `Spec.PgBouncer = &cnpgv1.PgBouncerSpec{...}` (required upstream: no `omitempty`, so always set it, empty if nothing else) |
| `PgBouncerOptions.PoolMode string` | `cnpgv1.PgBouncerSpec.PoolMode cnpgv1.PgBouncerPoolMode` (`PgBouncerPoolModeSession` / `Transaction`) — only for a non-empty former value; `""` left the field empty (`omitempty`) for CNPG to default, so do not write `PgBouncerPoolModeSession` for it |
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

Every conditional in the retired CNPG builders (`pkg/kubernetes/cnpg/create.go` and
`pooler.go` as of `4729da0`) is accounted for above: its row or this table states the
guard, including the ones where the only risk is writing a value the layer left out
(a connection limit, a role `ensure`, a pool mode), or the guarded field is
`omitempty` and the layer wrote nothing a literal would write differently.

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
`spec.podMetricsEndpoints` carry no `omitempty`: the layer filled them by appending,
so an empty (even explicitly allocated) input list left them nil and rendered
`null`, which release 1 recorded when the layer stopped seeding empty slices. A
literal that assigns an empty slice renders `[]` instead; leave the field nil, or use
`AddServiceMonitorEndpoint` / `AddPodMonitorEndpoint`, to keep `null`. Golden deltas: none. Every fixture in `pkg/kubernetes/prometheus/testdata` is
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
either way. `CiliumCIDRGroup` filled `spec.externalCIDRs` by appending, so an empty
input list left it nil and rendered `externalCIDRs: null` (the field has no
`omitempty`); assigning an empty slice renders `[]` — leave it nil, or use
`AddCiliumCIDRGroupCIDR`, to keep `null`. Golden deltas: none. Every fixture in `pkg/kubernetes/cilium/testdata`
is reproduced byte for byte.

## `pkg/kubernetes/volsync`

### Removed functions (2) and one changed signature

| Removed | Replacement |
|---|---|
| `ReplicationSource(&ReplicationSourceConfig{...})` | `CreateReplicationSource(name, namespace)` + `rs.Spec = volsyncv1alpha1.ReplicationSourceSpec{...}` |
| `ReplicationDestination(&ReplicationDestinationConfig{...})` | `CreateReplicationDestination(name, namespace)` + `rd.Spec = volsyncv1alpha1.ReplicationDestinationSpec{...}` |

| Before | After |
|---|---|
| `AddSyncthingPeer(cfg *SourceSyncthingConfig, address, id string, introducer bool)` | `AddSyncthingPeer(spec *volsyncv1alpha1.ReplicationSourceSyncthingSpec, address, id string, introducer bool)` — the value assigned to `rs.Spec.Syncthing`; same body, same nil panic |

### Removed types (15) and their fields

The sealed interfaces `SourceMover` and `DestinationMover` are gone with the nine
mover types defined over the upstream specs, `ExternalConfig`, `TriggerConfig` and
the two parent `Config` structs. The upstream one-of is a pointer per arm on the
spec; VolSync validates "exactly one" at apply time. `CopyMethod` and its four
constants stay: they re-export the upstream type and were never part of the layer.

| Removed field | Upstream field |
|---|---|
| `ReplicationSourceConfig.Name`, `.Namespace` | `CreateReplicationSource(name, namespace)` |
| `ReplicationSourceConfig.SourcePVC` | `rs.Spec.SourcePVC` |
| `ReplicationSourceConfig.Paused` | `rs.Spec.Paused` |
| `ReplicationSourceConfig.Trigger *TriggerConfig` | `rs.Spec.Trigger = &volsyncv1alpha1.ReplicationSourceTriggerSpec{Schedule, Manual}` |
| `ReplicationSourceConfig.Mover` = `*SourceResticConfig` | `rs.Spec.Restic = &volsyncv1alpha1.ReplicationSourceResticSpec{...}` (the type `SourceResticConfig` was defined over) |
| `ReplicationSourceConfig.Mover` = `*SourceRsyncConfig` | `rs.Spec.Rsync = &volsyncv1alpha1.ReplicationSourceRsyncSpec{...}` |
| `ReplicationSourceConfig.Mover` = `*SourceRsyncTLSConfig` | `rs.Spec.RsyncTLS = &volsyncv1alpha1.ReplicationSourceRsyncTLSSpec{...}` |
| `ReplicationSourceConfig.Mover` = `*SourceRcloneConfig` | `rs.Spec.Rclone = &volsyncv1alpha1.ReplicationSourceRcloneSpec{...}` |
| `ReplicationSourceConfig.Mover` = `*SourceSyncthingConfig` | `rs.Spec.Syncthing = &volsyncv1alpha1.ReplicationSourceSyncthingSpec{...}` |
| `ReplicationSourceConfig.Mover` = `*ExternalConfig` | `rs.Spec.External = &volsyncv1alpha1.ReplicationSourceExternalSpec{Provider, Parameters}` |
| `ReplicationDestinationConfig.Name`, `.Namespace` | `CreateReplicationDestination(name, namespace)` |
| `ReplicationDestinationConfig.Paused` | `rd.Spec.Paused` |
| `ReplicationDestinationConfig.Trigger *TriggerConfig` | `rd.Spec.Trigger = &volsyncv1alpha1.ReplicationDestinationTriggerSpec{Schedule, Manual}` |
| `ReplicationDestinationConfig.Mover` = `*DestinationResticConfig` | `rd.Spec.Restic = &volsyncv1alpha1.ReplicationDestinationResticSpec{...}` |
| `ReplicationDestinationConfig.Mover` = `*DestinationRsyncConfig` | `rd.Spec.Rsync = &volsyncv1alpha1.ReplicationDestinationRsyncSpec{...}` |
| `ReplicationDestinationConfig.Mover` = `*DestinationRsyncTLSConfig` | `rd.Spec.RsyncTLS = &volsyncv1alpha1.ReplicationDestinationRsyncTLSSpec{...}` |
| `ReplicationDestinationConfig.Mover` = `*DestinationRcloneConfig` | `rd.Spec.Rclone = &volsyncv1alpha1.ReplicationDestinationRcloneSpec{...}` |
| `ReplicationDestinationConfig.Mover` = `*ExternalConfig` | `rd.Spec.External = &volsyncv1alpha1.ReplicationDestinationExternalSpec{Provider, Parameters}` |
| `TriggerConfig.Schedule *string`, `.Manual string` | the same-named fields of the upstream trigger spec |
| `ExternalConfig.Provider`, `.Parameters` | the same-named fields of the upstream external spec |

Each `Source<Mover>Config` / `Destination<Mover>Config` had exactly the fields of
the upstream spec it was defined over, so a literal keeps its body and changes its
type name. The release 1 ledger's volsync section named `ReplicationSourceConfig.*`
as the replacement for the setters it removed; those replacements are now the
`rs.Spec.*` assignments listed above. Golden deltas: none. Every fixture in
`pkg/kubernetes/volsync/testdata` is reproduced byte for byte.
