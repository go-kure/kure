# Builder contract: release 1 migration notes

This page tracks what the builder contract (ADR-038, the "thin core + admissible
sugar" decision) changes for callers across its first release. The normative
contract text is the [Kubernetes Builders](/api-reference/kubernetes-builders/)
page; this page is the ledger of removed behaviour, kept so each work item of the
epic lists its own removals exactly once.

## Constructors are identity-only

Every `Create<Kind>(name, namespace)` (or `Create<Kind>(name)` for cluster-scoped
kinds) now returns an object carrying only `apiVersion`, `kind`, `metadata.name`
and, for namespaced kinds, `metadata.namespace`. The wrappers are generated from
the registered scheme; the generic `kubernetes.Create[T]` behind them is the
single implementation.

Two signatures changed:

| Before | After | Set the removed value with |
|---|---|---|
| `CreateCronJob(name, namespace, schedule)` | `CreateCronJob(name, namespace)` | `SetCronJobSchedule(cj, schedule)` or `cj.Spec.Schedule = schedule` |
| `CreateIngress(name, namespace, classname)` | `CreateIngress(name, namespace)` | `SetIngressClassName(ing, classname)` |

## Constructor defaults removed by the contract PR

The hand-written constructors injected the values below. They are gone; a caller
that relied on one sets it explicitly. The defaults work item of the epic owns
every other default (composite helpers, the Flux workflow packages) and does not
list these again.

### Metadata injected by every root constructor

`CreateConfigMap`, `CreateCronJob`, `CreateDaemonSet`, `CreateDeployment`,
`CreateHorizontalPodAutoscaler`, `CreateHTTPRoute`, `CreateIngress`, `CreateJob`,
`CreateNamespace`, `CreateNetworkPolicy`, `CreatePersistentVolumeClaim`,
`CreatePodDisruptionBudget`, `CreateService`, `CreateServiceAccount` and
`CreateStatefulSet` all set:

- label `app: <name>`
- annotation `app: <name>`

Restore with `kubernetes.AddLabel(obj, "app", name)` and
`kubernetes.AddAnnotation(obj, "app", name)`.

### Spec values injected per kind

| Constructor | Removed default |
|---|---|
| `CreateCronJob` | `spec.schedule` from the third argument; `spec.jobTemplate.spec.template.metadata.labels.app: <name>`; `spec.jobTemplate.spec.template.spec.restartPolicy: Never` |
| `CreateDaemonSet` | `spec.selector.matchLabels.app: <name>`; `spec.template.metadata.labels.app: <name>` |
| `CreateDeployment` | `spec.selector.matchLabels.app: <name>`; `spec.template.metadata.labels.app: <name>` |
| `CreateHTTPRoute` | empty `spec.hostnames` and `spec.rules` slices |
| `CreateIngress` | `spec.ingressClassName` from the third argument; empty `spec.rules` and `spec.tls` slices |
| `CreateJob` | `spec.template.metadata.labels.app: <name>` |
| `CreateNamespace` | empty `spec.finalizers` slice |
| `CreateNetworkPolicy` | `spec.podSelector.matchLabels.app: <name>`; empty `spec.policyTypes`, `spec.ingress` and `spec.egress` slices |
| `CreatePersistentVolumeClaim` | `spec.resources.requests.storage: 1Gi`; `spec.volumeMode: Filesystem`; empty `spec.accessModes` slice |
| `CreateService` | empty `spec.selector` map and `spec.ports` slice |
| `CreateServiceAccount` | `automountServiceAccountToken: false` (a pointer to `false`, serialised); empty `secrets` and `imagePullSecrets` slices |
| `CreateStatefulSet` | `spec.replicas: 0` (a pointer to zero, serialised); `spec.selector.matchLabels.app: <name>`; `spec.template.metadata.labels.app: <name>`; `spec.podManagementPolicy: OrderedReady`; empty `spec.volumeClaimTemplates` slice |
| `prometheus.CreateServiceMonitor` | empty `spec.endpoints` slice |
| `prometheus.CreatePodMonitor` | empty `spec.podMetricsEndpoints` slice |
| `prometheus.CreatePrometheusRule` | empty `spec.groups` slice |
| `cilium.CreateCiliumCIDRGroup` | empty `spec.externalCIDRs` slice (also observable through `cilium.CiliumCIDRGroup(cfg)` with no CIDRs, which now leaves the field nil) |

Empty slices and maps serialise as `[]` / `{}` where the upstream struct has no
`omitempty`; nil serialises as `null` or is omitted. A golden file that captured
the old output shows exactly that difference and nothing else.

The other hand-written constructors (cert-manager, CloudNativePG, External Secrets,
Flux, MetalLB, VolSync, the remaining Cilium kinds, RBAC) were already
identity-only; their removal changes no output.

## Scheme additions

`autoscaling/v2`, `policy/v1` and the Barman Cloud plugin API
(`barmancloud.cnpg.io/v1`) are now registered in `kubernetes.Scheme`. kure built
these kinds before but could not resolve their GVK; `GetGroupVersionKind` now
succeeds for them.
