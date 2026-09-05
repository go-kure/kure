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
| `CreateCronJob(name, namespace, schedule)` | `CreateCronJob(name, namespace)` | `cj.Spec.Schedule = schedule` |
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
| `CreateConfigMap` | `data` and `binaryData` initialised to empty maps. They never rendered, but a fresh object accepted `cm.Data[k] = v` directly; both are now nil, so write through `AddConfigMapData`/`AddConfigMapBinaryData` (which nil-init), or assign a map literal first (`cm.Data = map[string]string{...}`) |
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

## Bare field forwarders removed by the prune PR

A helper that does nothing but assign one argument to one struct field is not
sugar under the contract — the assignment is already the shortest way to write
it. All 257 such forwarders are removed. The tables below give the replacement
expression for each; the field is exactly the one the helper wrote, so the
rewrite is mechanical and never changes behaviour.

At the start of this work item `pkg/kubernetes/testdata/admission_exclusions.txt`
held 344 tolerated helpers, classified by why the admission test rejected them:
257 bare field forwarders, 51 that return a value, 24 that write no field at all
(they delegate, or are no-ops), 10 that return early instead of writing, and 2
that clear a sibling field the caller did not name. The 257 go first; folding the
per-kind pod-template passthroughs (below) takes the remaining 87 down to 33,
and the later work items empty the file. Six of the 24 delegators
now read as bare field assignments rather than delegations, because the helper
they delegated to was one of the 257 and their bodies were inlined:
`SetDaemonSetServiceAccountName`, `SetDaemonSetNodeSelector`,
`SetJobServiceAccountName`, `SetJobNodeSelector`,
`SetStatefulSetServiceAccountName` and `SetStatefulSetNodeSelector`. They are
folded onto the PodSpec helpers below.

### Sub-type constructors (9)

A sub-type that is not a `client.Object` gets no constructor: a struct literal
is both shorter and the only form that shows every field being set.

Each row gives the removed function's exact former signature and a literal that
produces the same value. Where the constructor injected defaults, they are
listed with it — a caller that relied on one now writes it, and a caller that
did not gets a smaller object.

| Removed | Replacement |
|---|---|
| `CreateContainer(name string, image string, command []string, args []string) *corev1.Container` | see below — it injected six defaults |
| `CreatePodSpec() *corev1.PodSpec` | see below — it injected eleven |
| `CreateVolumeClaimTemplate(name string, opts VolumeClaimTemplateOptions) corev1.PersistentVolumeClaim` | see below |
| `CreateIngressRule(host string) *netv1.IngressRule` | see below |
| `CreateACMEIssuer(server, email string, key cmmeta.SecretKeySelector) *cmacme.ACMEIssuer` | see below |
| `CreateACMEHTTP01Solver(serviceType corev1.ServiceType, class string) cmacme.ACMEChallengeSolver` | see below |
| `CreateACMEDNS01SolverCloudflare(email string, token cmmeta.SecretKeySelector) cmacme.ACMEChallengeSolver` | see below |
| `CreateACMEDNS01SolverRoute53(region string, key cmmeta.SecretKeySelector) cmacme.ACMEChallengeSolver` | see below |
| `CreateACMEDNS01SolverGoogle(project string, sa *cmmeta.SecretKeySelector) cmacme.ACMEChallengeSolver` | see below |

`CreateContainer` set a memory limit, CPU and memory requests, an image pull
policy, and five empty collections. **A caller that took those silently now gets
a container with no resource reservation at all.** The behaviour-preserving
replacement:

```go
&corev1.Container{
    Name:    name,
    Image:   image,
    Command: command,
    Args:    args,
    Resources: corev1.ResourceRequirements{
        Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("256Mi")},
        Requests: corev1.ResourceList{
            corev1.ResourceCPU:    resource.MustParse("100m"),
            corev1.ResourceMemory: resource.MustParse("256Mi"),
        },
    },
    ImagePullPolicy: corev1.PullIfNotPresent,
}
```

The five empty collections it also set (`Ports`, `Env`, `EnvFrom`,
`VolumeMounts`, `VolumeDevices`) never rendered and need no replacement:
`append` works on a nil slice. Drop the `Resources` and `ImagePullPolicy` keys
if the defaults were not wanted — that is the point of removing them.

`CreatePodSpec` set `RestartPolicy: Always`, a zero
`TerminationGracePeriodSeconds` pointer, an empty `SecurityContext`, an empty
`Affinity`, an empty `NodeSelector` map, and five empty slices. `Always` is the
API server's own default for a Pod. Four of the five slices (`InitContainers`,
`Volumes`, `ImagePullSecrets`, `Tolerations`) carry `omitempty` upstream and so
never rendered, but `Containers` does not: `PodSpec.Containers` is required with
no `omitempty` (`k8s.io/api/core/v1/types.go:4393`), so an empty slice
serialises as `containers: []` where nil serialises as `containers: null`. To
keep the old value exactly:

```go
&corev1.PodSpec{
    Containers:                    []corev1.Container{},
    TerminationGracePeriodSeconds: new(int64),
    SecurityContext:               &corev1.PodSecurityContext{},
    Affinity:                      &corev1.Affinity{},
    NodeSelector:                  map[string]string{},
}
```

A caller that wants none of them writes `&corev1.PodSpec{}`. The `Containers`
difference disappears as soon as one container is appended, so it matters only
for a spec serialised while still empty.

`CreateVolumeClaimTemplate` took an options struct, also removed
(`VolumeClaimTemplateOptions`), and returned a value rather than a pointer. It
left `StorageClassName` nil when the option was empty, which is how the API
selects the cluster default — the literal must keep that condition:

```go
pvc := corev1.PersistentVolumeClaim{
    ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
    Spec: corev1.PersistentVolumeClaimSpec{
        AccessModes: accessModes,
        Resources: corev1.VolumeResourceRequirements{
            Requests: corev1.ResourceList{corev1.ResourceStorage: storageRequest},
        },
    },
}
if storageClass != "" {
    pvc.Spec.StorageClassName = &storageClass
}
```

`CreateIngressRule` also built the nested `HTTP` value, with `Paths` initialised
to an empty slice, so a bare `&netv1.IngressRule{Host: host}` differs in three
ways: it serialises without the `http: {}` key; a caller that writes
`rule.IngressRuleValue.HTTP.Paths` directly dereferences a nil pointer; and
`HTTPIngressRuleValue.Paths` is required with no `omitempty`
(`k8s.io/api/networking/v1/types.go:452`), so the old value serialises
`paths: []` where a nil slice serialises `paths: null`. Going through
`AddIngressRulePath` is safe either way — it nil-initialises `HTTP` before
appending — so most callers need no change. To keep the old value exactly:

```go
&netv1.IngressRule{
    Host: host,
    IngressRuleValue: netv1.IngressRuleValue{
        HTTP: &netv1.HTTPIngressRuleValue{
            Paths: []netv1.HTTPIngressPath{},
        },
    },
}
```

As with `PodSpec.Containers`, the `paths` difference disappears once a path is
appended.

The cert-manager five:

```go
// CreateACMEIssuer(server, email, key)
&cmacme.ACMEIssuer{Server: server, Email: email, PrivateKey: key}

// CreateACMEHTTP01Solver(serviceType, class) -- class == "" left the pointer nil
solver := cmacme.ACMEChallengeSolver{
    HTTP01: &cmacme.ACMEChallengeSolverHTTP01{
        Ingress: &cmacme.ACMEChallengeSolverHTTP01Ingress{ServiceType: serviceType},
    },
}
if class != "" {
    solver.HTTP01.Ingress.IngressClassName = &class
}

// CreateACMEDNS01SolverCloudflare(email, token)
cmacme.ACMEChallengeSolver{DNS01: &cmacme.ACMEChallengeSolverDNS01{
    Cloudflare: &cmacme.ACMEIssuerDNS01ProviderCloudflare{Email: email, APIToken: &token},
}}

// CreateACMEDNS01SolverRoute53(region, key)
cmacme.ACMEChallengeSolver{DNS01: &cmacme.ACMEChallengeSolverDNS01{
    Route53: &cmacme.ACMEIssuerDNS01ProviderRoute53{Region: region, SecretAccessKey: key},
}}

// CreateACMEDNS01SolverGoogle(project, sa)
cmacme.ACMEChallengeSolver{DNS01: &cmacme.ACMEChallengeSolverDNS01{
    CloudDNS: &cmacme.ACMEIssuerDNS01ProviderCloudDNS{Project: project, ServiceAccount: sa},
}}
```

`CreateACMEIssuer` also set `Solvers` to an empty slice and both boolean fields
to `false`, none of which changes what is serialised or what `append` does.

The four cert-manager solver constructors and `CreateACMEIssuer` were only ever
called from `certmanager.Issuer`/`ClusterIssuer`, which now build the literals
inline.

`VolumeClaimTemplateOptions` goes with `CreateVolumeClaimTemplate`, its only
consumer. Its four fields — `StorageClassName`, `AccessModes`, `StorageRequest`
and `Labels` — appear in the literal above, where the API names them.

### `pkg/kubernetes` (76)

| Removed | Replacement |
|---|---|
| `SetClusterRoleBindingRoleRef(crb, roleRef)` | `crb.RoleRef = roleRef` |
| `SetContainerArgs(container, args)` | `container.Args = args` |
| `SetContainerCommand(container, command)` | `container.Command = command` |
| `SetContainerImage(container, image)` | `container.Image = image` |
| `SetContainerImagePullPolicy(container, imagePullPolicy)` | `container.ImagePullPolicy = imagePullPolicy` |
| `SetContainerResources(container, resources)` | `container.Resources = resources` |
| `SetContainerStdin(container, stdin)` | `container.Stdin = stdin` |
| `SetContainerStdinOnce(container, once)` | `container.StdinOnce = once` |
| `SetContainerTTY(container, tty)` | `container.TTY = tty` |
| `SetContainerTerminationMessagePath(container, path)` | `container.TerminationMessagePath = path` |
| `SetContainerTerminationMessagePolicy(container, policy)` | `container.TerminationMessagePolicy = policy` |
| `SetContainerWorkingDir(container, dir)` | `container.WorkingDir = dir` |
| `SetCronJobConcurrencyPolicy(cron, policy)` | `cron.Spec.ConcurrencyPolicy = policy` |
| `SetCronJobNodeSelector(cron, nodeSelector)` | `cron.Spec.JobTemplate.Spec.Template.Spec.NodeSelector = nodeSelector` |
| `SetCronJobSchedule(cron, schedule)` | `cron.Spec.Schedule = schedule` |
| `SetCronJobServiceAccountName(cron, serviceAccountName)` | `cron.Spec.JobTemplate.Spec.Template.Spec.ServiceAccountName = serviceAccountName` |
| `SetDaemonSetUpdateStrategy(ds, strategy)` | `ds.Spec.UpdateStrategy = strategy` |
| `SetDeploymentMinReadySeconds(deployment, secs)` | `deployment.Spec.MinReadySeconds = secs` |
| `SetDeploymentNodeSelector(deployment, nodeSelector)` | `deployment.Spec.Template.Spec.NodeSelector = nodeSelector` |
| `SetDeploymentServiceAccountName(deployment, serviceAccountName)` | `deployment.Spec.Template.Spec.ServiceAccountName = serviceAccountName` |
| `SetDeploymentStrategy(deployment, strategy)` | `deployment.Spec.Strategy = strategy` |
| `SetHPAAnnotations(hpa, annotations)` | `hpa.Annotations = annotations` |
| `SetHPALabels(hpa, labels)` | `hpa.Labels = labels` |
| `SetHTTPRouteHostnames(route, hostnames)` | `route.Spec.Hostnames = hostnames` |
| `SetHTTPRouteParentRefs(route, refs)` | `route.Spec.ParentRefs = refs` |
| `SetHTTPRouteRuleBackendRefs(rule, refs)` | `rule.BackendRefs = refs` |
| `SetHTTPRouteRuleFilters(rule, filters)` | `rule.Filters = filters` |
| `SetHTTPRouteRuleMatches(rule, matches)` | `rule.Matches = matches` |
| `SetHTTPRouteRules(route, rules)` | `route.Spec.Rules = rules` |
| `SetNamespaceAnnotations(ns, annotations)` | `ns.Annotations = annotations` |
| `SetNamespaceFinalizers(ns, finalizers)` | `ns.Spec.Finalizers = finalizers` |
| `SetNamespaceLabels(ns, labels)` | `ns.Labels = labels` |
| `SetNetworkPolicyEgressPeers(rule, peers)` | `rule.To = peers` |
| `SetNetworkPolicyEgressPorts(rule, ports)` | `rule.Ports = ports` |
| `SetNetworkPolicyEgressRules(np, rules)` | `np.Spec.Egress = rules` |
| `SetNetworkPolicyIngressPeers(rule, peers)` | `rule.From = peers` |
| `SetNetworkPolicyIngressPorts(rule, ports)` | `rule.Ports = ports` |
| `SetNetworkPolicyIngressRules(np, rules)` | `np.Spec.Ingress = rules` |
| `SetNetworkPolicyPodSelector(np, selector)` | `np.Spec.PodSelector = selector` |
| `SetNetworkPolicyPolicyTypes(np, types)` | `np.Spec.PolicyTypes = types` |
| `SetPDBAnnotations(pdb, annotations)` | `pdb.Annotations = annotations` |
| `SetPDBLabels(pdb, labels)` | `pdb.Labels = labels` |
| `SetPVCResources(pvc, resources)` | `pvc.Spec.Resources = resources` |
| `SetPVCVolumeName(pvc, volumeName)` | `pvc.Spec.VolumeName = volumeName` |
| `SetPodSpecDNSPolicy(spec, policy)` | `spec.DNSPolicy = policy` |
| `SetPodSpecHostIPC(spec, hostIPC)` | `spec.HostIPC = hostIPC` |
| `SetPodSpecHostNetwork(spec, hostNetwork)` | `spec.HostNetwork = hostNetwork` |
| `SetPodSpecHostPID(spec, hostPID)` | `spec.HostPID = hostPID` |
| `SetPodSpecHostname(spec, hostname)` | `spec.Hostname = hostname` |
| `SetPodSpecNodeSelector(spec, selector)` | `spec.NodeSelector = selector` |
| `SetPodSpecPriorityClassName(spec, class)` | `spec.PriorityClassName = class` |
| `SetPodSpecRestartPolicy(spec, policy)` | `spec.RestartPolicy = policy` |
| `SetPodSpecSchedulerName(spec, scheduler)` | `spec.SchedulerName = scheduler` |
| `SetPodSpecServiceAccountName(spec, name)` | `spec.ServiceAccountName = name` |
| `SetPodSpecSubdomain(spec, subdomain)` | `spec.Subdomain = subdomain` |
| `SetRoleBindingRoleRef(rb, roleRef)` | `rb.RoleRef = roleRef` |
| `SetServiceAccountAnnotations(sa, annotations)` | `sa.Annotations = annotations` |
| `SetServiceAccountImagePullSecrets(sa, secrets)` | `sa.ImagePullSecrets = secrets` |
| `SetServiceAccountLabels(sa, labels)` | `sa.Labels = labels` |
| `SetServiceAccountSecrets(sa, secrets)` | `sa.Secrets = secrets` |
| `SetServiceAnnotations(svc, anns)` | `svc.Annotations = anns` |
| `SetServiceClusterIP(service, ip)` | `service.Spec.ClusterIP = ip` |
| `SetServiceExternalName(svc, name)` | `svc.Spec.ExternalName = name` |
| `SetServiceExternalTrafficPolicy(service, trafficPolicy)` | `service.Spec.ExternalTrafficPolicy = trafficPolicy` |
| `SetServiceHealthCheckNodePort(svc, port)` | `svc.Spec.HealthCheckNodePort = port` |
| `SetServiceIPFamilies(svc, fams)` | `svc.Spec.IPFamilies = fams` |
| `SetServiceLabels(svc, labels)` | `svc.Labels = labels` |
| `SetServiceLoadBalancerSourceRanges(svc, ranges)` | `svc.Spec.LoadBalancerSourceRanges = ranges` |
| `SetServicePublishNotReadyAddresses(svc, publish)` | `svc.Spec.PublishNotReadyAddresses = publish` |
| `SetServiceSelector(service, selector)` | `service.Spec.Selector = selector` |
| `SetServiceSessionAffinity(service, affinity)` | `service.Spec.SessionAffinity = affinity` |
| `SetServiceType(service, type_)` | `service.Spec.Type = type_` |
| `SetStatefulSetMinReadySeconds(sts, secs)` | `sts.Spec.MinReadySeconds = secs` |
| `SetStatefulSetPodManagementPolicy(sts, policy)` | `sts.Spec.PodManagementPolicy = policy` |
| `SetStatefulSetServiceName(sts, svc)` | `sts.Spec.ServiceName = svc` |
| `SetStatefulSetUpdateStrategy(sts, strategy)` | `sts.Spec.UpdateStrategy = strategy` |

### `pkg/kubernetes/certmanager` (4)

| Removed | Replacement |
|---|---|
| `SetCertificateIssuerRef(obj, ref)` | `obj.Spec.IssuerRef = ref` |
| `SetCertificateSpec(obj, spec)` | `obj.Spec = spec` |
| `SetClusterIssuerSpec(obj, spec)` | `obj.Spec = spec` |
| `SetIssuerSpec(obj, spec)` | `obj.Spec = spec` |

### `pkg/kubernetes/cilium` (19)

| Removed | Replacement |
|---|---|
| `SetCiliumBGPAdvertisementSpec(obj, spec)` | `obj.Spec = spec` |
| `SetCiliumBGPClusterConfigSpec(obj, spec)` | `obj.Spec = spec` |
| `SetCiliumBGPNodeConfigOverrideSpec(obj, spec)` | `obj.Spec = spec` |
| `SetCiliumBGPNodeConfigSpec(obj, spec)` | `obj.Spec = spec` |
| `SetCiliumBGPPeerConfigSpec(obj, spec)` | `obj.Spec = spec` |
| `SetCiliumCIDRGroupCIDRs(obj, cidrs)` | `obj.Spec.ExternalCIDRs = cidrs` |
| `SetCiliumClusterwideEnvoyConfigSpec(obj, spec)` | `obj.Spec = spec` |
| `SetCiliumClusterwideNetworkPolicySpecs(obj, specs)` | `obj.Specs = specs` |
| `SetCiliumEgressGatewayPolicySpec(obj, spec)` | `obj.Spec = spec` |
| `SetCiliumEnvoyConfigSpec(obj, spec)` | `obj.Spec = spec` |
| `SetCiliumLoadBalancerIPPoolAllowFirstLastIPs(obj, allow)` | `obj.Spec.AllowFirstLastIPs = allow` |
| `SetCiliumLoadBalancerIPPoolDisabled(obj, disabled)` | `obj.Spec.Disabled = disabled` |
| `SetCiliumLoadBalancerIPPoolSpec(obj, spec)` | `obj.Spec = spec` |
| `SetCiliumLocalRedirectPolicyBackend(obj, backend)` | `obj.Spec.RedirectBackend = backend` |
| `SetCiliumLocalRedirectPolicyDescription(obj, desc)` | `obj.Spec.Description = desc` |
| `SetCiliumLocalRedirectPolicyFrontend(obj, frontend)` | `obj.Spec.RedirectFrontend = frontend` |
| `SetCiliumLocalRedirectPolicySkipRedirectFromBackend(obj, skip)` | `obj.Spec.SkipRedirectFromBackend = skip` |
| `SetCiliumLocalRedirectPolicySpec(obj, spec)` | `obj.Spec = spec` |
| `SetCiliumNetworkPolicySpecs(obj, specs)` | `obj.Specs = specs` |

### `pkg/kubernetes/cnpg` (9)

| Removed | Replacement |
|---|---|
| `SetDatabaseClusterRef(obj, clusterName)` | `obj.Spec.ClusterRef = corev1.LocalObjectReference{Name: clusterName}` |
| `SetDatabaseEnsure(obj, ensure)` | `obj.Spec.Ensure = ensure` |
| `SetDatabaseOwner(obj, owner)` | `obj.Spec.Owner = owner` |
| `SetDatabaseReclaimPolicy(obj, policy)` | `obj.Spec.ReclaimPolicy = policy` |
| `SetObjectStoreDestinationPath(obj, path)` | `obj.Spec.Configuration.DestinationPath = path` |
| `SetObjectStoreEndpointURL(obj, url)` | `obj.Spec.Configuration.EndpointURL = url` |
| `SetObjectStoreRetentionPolicy(obj, policy)` | `obj.Spec.RetentionPolicy = policy` |
| `SetScheduledBackupBackupOwnerReference(obj, ref)` | `obj.Spec.BackupOwnerReference = ref` |
| `SetScheduledBackupMethod(obj, method)` | `obj.Spec.Method = method` |

### `pkg/kubernetes/externalsecrets` (7)

| Removed | Replacement |
|---|---|
| `SetClusterSecretStoreController(obj, controller)` | `obj.Spec.Controller = controller` |
| `SetClusterSecretStoreSpec(obj, spec)` | `obj.Spec = spec` |
| `SetExternalSecretSecretStoreRef(obj, ref)` | `obj.Spec.SecretStoreRef = ref` |
| `SetExternalSecretSpec(obj, spec)` | `obj.Spec = spec` |
| `SetSecretStoreController(obj, controller)` | `obj.Spec.Controller = controller` |
| `SetSecretStoreSpec(obj, spec)` | `obj.Spec = spec` |
| `SetTarget(obj, target)` | `obj.Spec.Target = target` |

### `pkg/kubernetes/fluxcd` (118)

| Removed | Replacement |
|---|---|
| `SetAlertEventSeverity(alert, sev)` | `alert.Spec.EventSeverity = sev` |
| `SetAlertProviderRef(alert, ref)` | `alert.Spec.ProviderRef = ref` |
| `SetAlertSpec(obj, spec)` | `obj.Spec = spec` |
| `SetAlertSummary(alert, summary)` | `alert.Spec.Summary = summary` |
| `SetAlertSuspend(alert, suspend)` | `alert.Spec.Suspend = suspend` |
| `SetArtifactGeneratorSpec(obj, spec)` | `obj.Spec = spec` |
| `SetBucketEndpoint(b, endpoint)` | `b.Spec.Endpoint = endpoint` |
| `SetBucketInsecure(b, insecure)` | `b.Spec.Insecure = insecure` |
| `SetBucketInterval(b, interval)` | `b.Spec.Interval = interval` |
| `SetBucketName(b, name)` | `b.Spec.BucketName = name` |
| `SetBucketPrefix(b, prefix)` | `b.Spec.Prefix = prefix` |
| `SetBucketProvider(b, provider)` | `b.Spec.Provider = provider` |
| `SetBucketRegion(b, region)` | `b.Spec.Region = region` |
| `SetBucketSpec(obj, spec)` | `obj.Spec = spec` |
| `SetBucketSuspend(b, suspend)` | `b.Spec.Suspend = suspend` |
| `SetCommitAuthor(spec, author)` | `spec.Author = author` |
| `SetCommitMessageTemplate(spec, tpl)` | `spec.MessageTemplate = tpl` |
| `SetCommitMessageTemplateValues(spec, values)` | `spec.MessageTemplateValues = values` |
| `SetCopyOperationStrategy(op, strategy)` | `op.Strategy = strategy` |
| `SetCustomHealthCheckFailed(chk, expr)` | `chk.HealthCheckExpressions.Failed = expr` |
| `SetCustomHealthCheckInProgress(chk, expr)` | `chk.HealthCheckExpressions.InProgress = expr` |
| `SetExternalArtifactSpec(obj, spec)` | `obj.Spec = spec` |
| `SetFluxInstanceDistribution(obj, dist)` | `obj.Spec.Distribution = dist` |
| `SetFluxInstanceDistributionVariant(obj, variant)` | `obj.Spec.Distribution.Variant = variant` |
| `SetFluxInstanceSpec(obj, spec)` | `obj.Spec = spec` |
| `SetFluxReportDistribution(fr, dist)` | `fr.Spec.Distribution = dist` |
| `SetFluxReportSpec(obj, spec)` | `obj.Spec = spec` |
| `SetGitCheckoutReference(spec, ref)` | `spec.Reference = ref` |
| `SetGitRepositoryInterval(gr, interval)` | `gr.Spec.Interval = interval` |
| `SetGitRepositoryProvider(gr, provider)` | `gr.Spec.Provider = provider` |
| `SetGitRepositoryRecurseSubmodules(gr, recurse)` | `gr.Spec.RecurseSubmodules = recurse` |
| `SetGitRepositoryServiceAccountName(gr, name)` | `gr.Spec.ServiceAccountName = name` |
| `SetGitRepositorySparseCheckout(gr, paths)` | `gr.Spec.SparseCheckout = paths` |
| `SetGitRepositorySpec(obj, spec)` | `obj.Spec = spec` |
| `SetGitRepositorySuspend(gr, suspend)` | `gr.Spec.Suspend = suspend` |
| `SetGitRepositoryURL(gr, url)` | `gr.Spec.URL = url` |
| `SetGitSpecCommit(spec, commit)` | `spec.Commit = commit` |
| `SetHelmChartChart(hc, chart)` | `hc.Spec.Chart = chart` |
| `SetHelmChartIgnoreMissingValuesFiles(hc, ignore)` | `hc.Spec.IgnoreMissingValuesFiles = ignore` |
| `SetHelmChartInterval(hc, interval)` | `hc.Spec.Interval = interval` |
| `SetHelmChartReconcileStrategy(hc, strategy)` | `hc.Spec.ReconcileStrategy = strategy` |
| `SetHelmChartSourceRef(hc, ref)` | `hc.Spec.SourceRef = ref` |
| `SetHelmChartSpec(obj, spec)` | `obj.Spec = spec` |
| `SetHelmChartSuspend(hc, suspend)` | `hc.Spec.Suspend = suspend` |
| `SetHelmChartValuesFiles(hc, files)` | `hc.Spec.ValuesFiles = files` |
| `SetHelmChartVersion(hc, version)` | `hc.Spec.Version = version` |
| `SetHelmReleaseInterval(obj, interval)` | `obj.Spec.Interval = interval` |
| `SetHelmReleaseReleaseName(obj, name)` | `obj.Spec.ReleaseName = name` |
| `SetHelmReleaseServiceAccountName(obj, name)` | `obj.Spec.ServiceAccountName = name` |
| `SetHelmReleaseSpec(obj, spec)` | `obj.Spec = spec` |
| `SetHelmReleaseStorageNamespace(obj, ns)` | `obj.Spec.StorageNamespace = ns` |
| `SetHelmReleaseSuspend(obj, suspend)` | `obj.Spec.Suspend = suspend` |
| `SetHelmReleaseTargetNamespace(obj, ns)` | `obj.Spec.TargetNamespace = ns` |
| `SetHelmRepositoryInsecure(hr, insecure)` | `hr.Spec.Insecure = insecure` |
| `SetHelmRepositoryInterval(hr, interval)` | `hr.Spec.Interval = interval` |
| `SetHelmRepositoryPassCredentials(hr, v)` | `hr.Spec.PassCredentials = v` |
| `SetHelmRepositoryProvider(hr, provider)` | `hr.Spec.Provider = provider` |
| `SetHelmRepositorySpec(obj, spec)` | `obj.Spec = spec` |
| `SetHelmRepositorySuspend(hr, suspend)` | `hr.Spec.Suspend = suspend` |
| `SetHelmRepositoryType(hr, typ)` | `hr.Spec.Type = typ` |
| `SetHelmRepositoryURL(hr, url)` | `hr.Spec.URL = url` |
| `SetImageRefDigest(ref, digest)` | `ref.Digest = digest` |
| `SetImageRefName(ref, name)` | `ref.Name = name` |
| `SetImageRefTag(ref, tag)` | `ref.Tag = tag` |
| `SetImageUpdateAutomationInterval(auto, interval)` | `auto.Spec.Interval = interval` |
| `SetImageUpdateAutomationSourceRef(auto, ref)` | `auto.Spec.SourceRef = ref` |
| `SetImageUpdateAutomationSpec(obj, spec)` | `obj.Spec = spec` |
| `SetImageUpdateAutomationSuspend(auto, suspend)` | `auto.Spec.Suspend = suspend` |
| `SetKustomizationDeletionPolicy(k, policy)` | `k.Spec.DeletionPolicy = policy` |
| `SetKustomizationForce(k, force)` | `k.Spec.Force = force` |
| `SetKustomizationIgnoreMissingComponents(k, ignore)` | `k.Spec.IgnoreMissingComponents = ignore` |
| `SetKustomizationInterval(k, interval)` | `k.Spec.Interval = interval` |
| `SetKustomizationNamePrefix(k, prefix)` | `k.Spec.NamePrefix = prefix` |
| `SetKustomizationNameSuffix(k, suffix)` | `k.Spec.NameSuffix = suffix` |
| `SetKustomizationPath(k, path)` | `k.Spec.Path = path` |
| `SetKustomizationPrune(k, prune)` | `k.Spec.Prune = prune` |
| `SetKustomizationServiceAccountName(k, name)` | `k.Spec.ServiceAccountName = name` |
| `SetKustomizationSourceRef(k, ref)` | `k.Spec.SourceRef = ref` |
| `SetKustomizationSpec(obj, spec)` | `obj.Spec = spec` |
| `SetKustomizationSuspend(k, suspend)` | `k.Spec.Suspend = suspend` |
| `SetKustomizationTargetNamespace(k, namespace)` | `k.Spec.TargetNamespace = namespace` |
| `SetKustomizationWait(k, wait)` | `k.Spec.Wait = wait` |
| `SetOCIRepositoryInsecure(or, insecure)` | `or.Spec.Insecure = insecure` |
| `SetOCIRepositoryInterval(or, interval)` | `or.Spec.Interval = interval` |
| `SetOCIRepositoryProvider(or, provider)` | `or.Spec.Provider = provider` |
| `SetOCIRepositoryServiceAccountName(or, name)` | `or.Spec.ServiceAccountName = name` |
| `SetOCIRepositorySpec(obj, spec)` | `obj.Spec = spec` |
| `SetOCIRepositorySuspend(or, suspend)` | `or.Spec.Suspend = suspend` |
| `SetOCIRepositoryURL(or, url)` | `or.Spec.URL = url` |
| `SetObservedPolicies(auto, policies)` | `auto.Status.ObservedPolicies = policies` |
| `SetOutputArtifactOriginRevision(out, originRevision)` | `out.OriginRevision = originRevision` |
| `SetOutputArtifactRevision(out, revision)` | `out.Revision = revision` |
| `SetProviderAddress(provider, address)` | `provider.Spec.Address = address` |
| `SetProviderChannel(provider, channel)` | `provider.Spec.Channel = channel` |
| `SetProviderProxy(provider, proxy)` | `provider.Spec.Proxy = proxy` |
| `SetProviderSpec(obj, spec)` | `obj.Spec = spec` |
| `SetProviderSuspend(provider, suspend)` | `provider.Spec.Suspend = suspend` |
| `SetProviderType(provider, t)` | `provider.Spec.Type = t` |
| `SetProviderUsername(provider, username)` | `provider.Spec.Username = username` |
| `SetPushBranch(spec, branch)` | `spec.Branch = branch` |
| `SetPushOptions(spec, opts)` | `spec.Options = opts` |
| `SetPushRefspec(spec, refspec)` | `spec.Refspec = refspec` |
| `SetReceiverSpec(obj, spec)` | `obj.Spec = spec` |
| `SetReceiverSuspend(receiver, suspend)` | `receiver.Spec.Suspend = suspend` |
| `SetReceiverType(receiver, t)` | `receiver.Spec.Type = t` |
| `SetResourceSetInputProviderServiceAccountName(obj, name)` | `obj.Spec.ServiceAccountName = name` |
| `SetResourceSetInputProviderSpec(obj, spec)` | `obj.Spec = spec` |
| `SetResourceSetInputProviderType(obj, typ)` | `obj.Spec.Type = typ` |
| `SetResourceSetInputProviderURL(obj, url)` | `obj.Spec.URL = url` |
| `SetResourceSetResourcesTemplate(rs, tpl)` | `rs.Spec.ResourcesTemplate = tpl` |
| `SetResourceSetServiceAccountName(rs, name)` | `rs.Spec.ServiceAccountName = name` |
| `SetResourceSetSpec(obj, spec)` | `obj.Spec = spec` |
| `SetResourceSetWait(rs, wait)` | `rs.Spec.Wait = wait` |
| `SetScheduleTimeZone(s, tz)` | `s.TimeZone = tz` |
| `SetScheduleWindow(s, d)` | `s.Window = d` |
| `SetSourceReferenceNamespace(ref, namespace)` | `ref.Namespace = namespace` |
| `SetUpdateStrategyName(spec, name)` | `spec.Strategy = name` |
| `SetUpdateStrategyPath(spec, path)` | `spec.Path = path` |

### `pkg/kubernetes/metallb` (15)

| Removed | Replacement |
|---|---|
| `SetBFDProfileSpec(obj, spec)` | `obj.Spec = spec` |
| `SetBGPAdvertisementLocalPref(obj, pref)` | `obj.Spec.LocalPref = pref` |
| `SetBGPAdvertisementSpec(obj, spec)` | `obj.Spec = spec` |
| `SetBGPPeerBFDProfile(obj, profile)` | `obj.Spec.BFDProfile = profile` |
| `SetBGPPeerEBGPMultiHop(obj, multi)` | `obj.Spec.EBGPMultiHop = multi` |
| `SetBGPPeerHoldTime(obj, d)` | `obj.Spec.HoldTime = d` |
| `SetBGPPeerKeepaliveTime(obj, d)` | `obj.Spec.KeepaliveTime = d` |
| `SetBGPPeerPassword(obj, pw)` | `obj.Spec.Password = pw` |
| `SetBGPPeerPort(obj, port)` | `obj.Spec.Port = port` |
| `SetBGPPeerRouterID(obj, id)` | `obj.Spec.RouterID = id` |
| `SetBGPPeerSpec(obj, spec)` | `obj.Spec = spec` |
| `SetBGPPeerSrcAddress(obj, addr)` | `obj.Spec.SrcAddress = addr` |
| `SetIPAddressPoolAvoidBuggyIPs(obj, avoid)` | `obj.Spec.AvoidBuggyIPs = avoid` |
| `SetIPAddressPoolSpec(obj, spec)` | `obj.Spec = spec` |
| `SetL2AdvertisementSpec(obj, spec)` | `obj.Spec = spec` |

### `pkg/kubernetes/prometheus` (9)

| Removed | Replacement |
|---|---|
| `SetPodMonitorJobLabel(obj, label)` | `obj.Spec.JobLabel = label` |
| `SetPodMonitorNamespaceSelector(obj, ns)` | `obj.Spec.NamespaceSelector = ns` |
| `SetPodMonitorSelector(obj, selector)` | `obj.Spec.Selector = selector` |
| `SetPodMonitorSpec(obj, spec)` | `obj.Spec = spec` |
| `SetPrometheusRuleSpec(obj, spec)` | `obj.Spec = spec` |
| `SetServiceMonitorJobLabel(obj, label)` | `obj.Spec.JobLabel = label` |
| `SetServiceMonitorNamespaceSelector(obj, ns)` | `obj.Spec.NamespaceSelector = ns` |
| `SetServiceMonitorSelector(obj, selector)` | `obj.Spec.Selector = selector` |
| `SetServiceMonitorSpec(obj, spec)` | `obj.Spec = spec` |

## Per-kind pod-template passthroughs folded onto `PodSpec`

A workload kind's pod template is a `corev1.PodSpec`. One helper per
pod-template field, per workload kind, is five copies of the same line — so the
`PodSpec`-level helper is the only one kept and the caller names the template it
means. The 51 per-kind passthroughs below are removed.

The `PodSpec` appenders lose their `error` return in the same change: they are
void and panic on a nil `*PodSpec` or a nil element, matching the `PodSpec`
setters, which already did. `AddPodSpecTopologySpreadConstraints` no longer
silently ignores a nil constraint.

The receiver expression is the same for every row of a kind:

| Kind | Pod template |
|---|---|
| `Deployment` | `&dep.Spec.Template.Spec` |
| `StatefulSet` | `&sts.Spec.Template.Spec` |
| `DaemonSet` | `&ds.Spec.Template.Spec` |
| `Job` | `&job.Spec.Template.Spec` |
| `CronJob` | `&cj.Spec.JobTemplate.Spec.Template.Spec` |

Below, `spec` stands for that expression.

### `Deployment` (9)

| Removed | Replacement |
|---|---|
| `SetDeploymentPodSpec(dep, s)` | `dep.Spec.Template.Spec = *s` |
| `AddDeploymentContainer(dep, c)` | `AddPodSpecContainer(spec, c)` |
| `AddDeploymentInitContainer(dep, c)` | `AddPodSpecInitContainer(spec, c)` |
| `AddDeploymentVolume(dep, v)` | `AddPodSpecVolume(spec, v)` |
| `AddDeploymentImagePullSecret(dep, s)` | `AddPodSpecImagePullSecret(spec, s)` |
| `AddDeploymentToleration(dep, t)` | `AddPodSpecToleration(spec, t)` |
| `AddDeploymentTopologySpreadConstraints(dep, c)` | `AddPodSpecTopologySpreadConstraints(spec, c)` |
| `SetDeploymentSecurityContext(dep, sc)` | `SetPodSpecSecurityContext(spec, sc)` |
| `SetDeploymentAffinity(dep, aff)` | `SetPodSpecAffinity(spec, aff)` |

### `StatefulSet` (11)

| Removed | Replacement |
|---|---|
| `SetStatefulSetPodSpec(sts, s)` | `sts.Spec.Template.Spec = *s` |
| `AddStatefulSetContainer(sts, c)` | `AddPodSpecContainer(spec, c)` |
| `AddStatefulSetInitContainer(sts, c)` | `AddPodSpecInitContainer(spec, c)` |
| `AddStatefulSetVolume(sts, v)` | `AddPodSpecVolume(spec, v)` |
| `AddStatefulSetImagePullSecret(sts, s)` | `AddPodSpecImagePullSecret(spec, s)` |
| `AddStatefulSetToleration(sts, t)` | `AddPodSpecToleration(spec, t)` |
| `AddStatefulSetTopologySpreadConstraints(sts, c)` | `AddPodSpecTopologySpreadConstraints(spec, c)` |
| `SetStatefulSetSecurityContext(sts, sc)` | `SetPodSpecSecurityContext(spec, sc)` |
| `SetStatefulSetAffinity(sts, aff)` | `SetPodSpecAffinity(spec, aff)` |
| `SetStatefulSetServiceAccountName(sts, n)` | `spec.ServiceAccountName = n` |
| `SetStatefulSetNodeSelector(sts, ns)` | `spec.NodeSelector = ns` |

### `DaemonSet` (11)

| Removed | Replacement |
|---|---|
| `SetDaemonSetPodSpec(ds, s)` | `ds.Spec.Template.Spec = *s` |
| `AddDaemonSetContainer(ds, c)` | `AddPodSpecContainer(spec, c)` |
| `AddDaemonSetInitContainer(ds, c)` | `AddPodSpecInitContainer(spec, c)` |
| `AddDaemonSetVolume(ds, v)` | `AddPodSpecVolume(spec, v)` |
| `AddDaemonSetImagePullSecret(ds, s)` | `AddPodSpecImagePullSecret(spec, s)` |
| `AddDaemonSetToleration(ds, t)` | `AddPodSpecToleration(spec, t)` |
| `AddDaemonSetTopologySpreadConstraints(ds, c)` | `AddPodSpecTopologySpreadConstraints(spec, c)` |
| `SetDaemonSetSecurityContext(ds, sc)` | `SetPodSpecSecurityContext(spec, sc)` |
| `SetDaemonSetAffinity(ds, aff)` | `SetPodSpecAffinity(spec, aff)` |
| `SetDaemonSetServiceAccountName(ds, n)` | `spec.ServiceAccountName = n` |
| `SetDaemonSetNodeSelector(ds, ns)` | `spec.NodeSelector = ns` |

### `Job` (11)

| Removed | Replacement |
|---|---|
| `SetJobPodSpec(job, s)` | `job.Spec.Template.Spec = *s` |
| `AddJobContainer(job, c)` | `AddPodSpecContainer(spec, c)` |
| `AddJobInitContainer(job, c)` | `AddPodSpecInitContainer(spec, c)` |
| `AddJobVolume(job, v)` | `AddPodSpecVolume(spec, v)` |
| `AddJobImagePullSecret(job, s)` | `AddPodSpecImagePullSecret(spec, s)` |
| `AddJobToleration(job, t)` | `AddPodSpecToleration(spec, t)` |
| `AddJobTopologySpreadConstraint(job, c)` | `AddPodSpecTopologySpreadConstraints(spec, c)` |
| `SetJobSecurityContext(job, sc)` | `SetPodSpecSecurityContext(spec, sc)` |
| `SetJobAffinity(job, aff)` | `SetPodSpecAffinity(spec, aff)` |
| `SetJobServiceAccountName(job, n)` | `spec.ServiceAccountName = n` |
| `SetJobNodeSelector(job, ns)` | `spec.NodeSelector = ns` |

### `CronJob` (9)

| Removed | Replacement |
|---|---|
| `SetCronJobPodSpec(cj, s)` | `cj.Spec.JobTemplate.Spec.Template.Spec = *s` |
| `AddCronJobContainer(cj, c)` | `AddPodSpecContainer(spec, c)` |
| `AddCronJobInitContainer(cj, c)` | `AddPodSpecInitContainer(spec, c)` |
| `AddCronJobVolume(cj, v)` | `AddPodSpecVolume(spec, v)` |
| `AddCronJobImagePullSecret(cj, s)` | `AddPodSpecImagePullSecret(spec, s)` |
| `AddCronJobToleration(cj, t)` | `AddPodSpecToleration(spec, t)` |
| `AddCronJobTopologySpreadConstraint(cj, c)` | `AddPodSpecTopologySpreadConstraints(spec, c)` |
| `SetCronJobSecurityContext(cj, sc)` | `SetPodSpecSecurityContext(spec, sc)` |
| `SetCronJobAffinity(cj, aff)` | `SetPodSpecAffinity(spec, aff)` |

### Effect on the exclusion list

Of the 51, 47 were on the exclusion list; `SetDeploymentSecurityContext`,
`SetDeploymentAffinity`, `SetCronJobSecurityContext` and `SetCronJobAffinity`
already passed as class (b) pointer assignments and are removed as duplicates,
not as contract violations. Dropping those 47 plus the seven `PodSpec` appenders
that no longer return an error takes the file from 87 tolerated helpers to 33.

### The seven `PodSpec` appender signatures

These are the survivors every table above forwards to, and their own signatures
changed: the `error` return is gone, and a nil argument panics instead of
returning a sentinel. Code that checked the error no longer compiles, so each is
listed by name with its before and after.

| Before | After |
|---|---|
| `AddPodSpecContainer(spec, c) error` | `AddPodSpecContainer(spec, c)` |
| `AddPodSpecInitContainer(spec, c) error` | `AddPodSpecInitContainer(spec, c)` |
| `AddPodSpecEphemeralContainer(spec, c) error` | `AddPodSpecEphemeralContainer(spec, c)` |
| `AddPodSpecVolume(spec, v) error` | `AddPodSpecVolume(spec, v)` |
| `AddPodSpecImagePullSecret(spec, s) error` | `AddPodSpecImagePullSecret(spec, s)` |
| `AddPodSpecToleration(spec, t) error` | `AddPodSpecToleration(spec, t)` |
| `AddPodSpecTopologySpreadConstraints(spec, c) error` | `AddPodSpecTopologySpreadConstraints(spec, c)` |

Drop the error check; there is nothing left to check.

```go
// before
if err := AddPodSpecEphemeralContainer(spec, ec); err != nil {
    return err
}

// after
AddPodSpecEphemeralContainer(spec, ec)
```

A nil `spec` or nil element panics, which is what the contract requires of every
sugar helper. `AddPodSpecTopologySpreadConstraints` is the one whose nil-element
behaviour also changed — it used to return nil and silently drop the constraint;
see its own entry below.

The `pkg/errors` sentinels those helpers returned (`ErrNilDeployment`,
`ErrNilDaemonSet`, `ErrNilStatefulSet`, `ErrNilJob`, `ErrNilCronJob`,
`ErrNilVolume`, `ErrNilToleration`, `ErrNilImagePullSecret`,
`ErrNilInitContainer`, `ErrNilEphemeralContainer`, `ErrNilSpec`) are now unused
inside the module. They stay exported for this release; retiring them is a
separate `pkg/errors` change, not part of the builder surface, tracked in
issue go-kure/kure#758. `ErrNilContainer` and `ErrNilPodSpec` keep real users in
`pkg/kubernetes/psa.go`, whose validators return errors by design.

## Error-returning helpers rewritten as void

The purity rule (§4) allows no error return: a nil receiver panics rather than
being reported, so a result slot has nothing to carry, and validation inside a
helper is defaulting by another name.

### `ResourceRequirements` quantities

The eight quantity setters took a `string` and parsed it, so each could fail on
the parse and each carried an `error`. Parsing belongs to the caller, and once
the value arrives as a `resource.Quantity` the body is a map insert with a
nil-map guard — class (a), void. The six typed convenience wrappers are the same
insert with a constant key and are removed with the rest of the per-field
duplication.

| Removed | Replacement |
|---|---|
| `SetResourceRequestCPU(rr, "100m")` | `SetResourceRequest(rr, corev1.ResourceCPU, resource.MustParse("100m"))` |
| `SetResourceRequestMemory(rr, "256Mi")` | `SetResourceRequest(rr, corev1.ResourceMemory, resource.MustParse("256Mi"))` |
| `SetResourceRequestEphemeralStorage(rr, "10Gi")` | `SetResourceRequest(rr, corev1.ResourceEphemeralStorage, resource.MustParse("10Gi"))` |
| `SetResourceLimitCPU(rr, "500m")` | `SetResourceLimit(rr, corev1.ResourceCPU, resource.MustParse("500m"))` |
| `SetResourceLimitMemory(rr, "1Gi")` | `SetResourceLimit(rr, corev1.ResourceMemory, resource.MustParse("1Gi"))` |
| `SetResourceLimitEphemeralStorage(rr, "20Gi")` | `SetResourceLimit(rr, corev1.ResourceEphemeralStorage, resource.MustParse("20Gi"))` |

| Signature changed | From | To |
|---|---|---|
| `SetResourceRequest` | `(rr, name, value string) error` | `(rr, name, qty resource.Quantity)` |
| `SetResourceLimit` | `(rr, name, value string) error` | `(rr, name, qty resource.Quantity)` |

`resource.MustParse` for a literal, `resource.ParseQuantity` when the text comes
from configuration and the error has somewhere to go.

### `SetHelmReleaseValuesFromMap`

| Signature changed | From | To |
|---|---|---|
| `fluxcd.SetHelmReleaseValuesFromMap` | `(obj, values map[string]any) error` | `(obj, values map[string]any)` |

Its only error came from `json.Marshal`, which fails on a map holding a channel,
a function or a NaN — a programming error, so it panics. The body stays class
(b): a one-field literal, built from the caller's map, assigned to the pointer
field `Spec.Values`.

A sugar helper cannot hand back a marshalling failure, so values that could
hold something `encoding/json` refuses must be marshalled by the caller and
passed to `SetHelmReleaseValues`, which takes already-encoded JSON. Anything
decoded from YAML or JSON into `map[string]any` always marshals; the panic is
reachable only from a channel, a function, a NaN or `+Inf` float, or a cyclic
structure.

These take the exclusion list from 33 tolerated helpers to 24.

## Helpers that wrote more than the field they named

Three survivors of the passes above kept their names but broke §4's rule that a
helper touches no field the caller did not name. Each is resolved by narrowing
the write, not by widening the contract.

### `PodDisruptionBudget` availability

`Spec.MinAvailable` and `Spec.MaxUnavailable` are mutually exclusive upstream,
and each setter used to nil the other. That made the pair order-dependent and
hid a write the caller never asked for; it also classified as a `nilClear`,
which no admission class covers. Both now assign only their own field.

| Behaviour change | Before | After |
|---|---|---|
| `SetPDBMinAvailable(pdb, v)` | sets `MinAvailable`, clears `MaxUnavailable` | sets `MinAvailable` only |
| `SetPDBMaxUnavailable(pdb, v)` | sets `MaxUnavailable`, clears `MinAvailable` | sets `MaxUnavailable` only |

Setting both now produces a `PodDisruptionBudget` the API server rejects.
Enforcing the exclusion is upstream's job; switching from one to the other is
an explicit two statements at the call site.

### Pod Security Admission labels

`SetNamespacePSALabels` failed §4 twice: it delegated every write to
`AddNamespaceLabel` so it contained no field write of its own, and it was built
out of `if level != "" { set(level) }` — the conditional-write shape the
contract names outright. Its replacement writes nothing at all.

| Removed | Replacement |
|---|---|
| `SetNamespacePSALabels(ns, enforce, warn, audit, version)` | `for k, v := range PSALabels(enforce, warn, audit, version) { AddLabel(ns, k, v) }` |

`PSALabels(enforce, warn, audit PSALevel, version string) map[string]string`
returns up to six labels and touches no object. One argument expanding into a
family of labels is not something a `Set<Field>` helper may hide, so the
expansion became a value helper and the write stayed with the generic
`AddLabel`. Skipping an empty mode is honest in a function whose result *is* the
label set, where the same skip inside a setter was a silent partial write.

No merge helper was added for this. The generic metadata set is fixed at four by
name (§5) and the exempt list in the admission test does not grow, so a
map-merging variant would have been a contract change to save one loop.

### volsync trigger, spec and mover setters

`pkg/kubernetes/volsync` carried ten `Set*` helpers alongside a config-struct
constructor that already covers the same fields. Nine are removed.

| Removed | Replacement |
|---|---|
| `SetReplicationSourceSourcePVC(rs, pvc)` | `ReplicationSourceConfig.SourcePVC`, or `rs.Spec.SourcePVC = pvc` |
| `SetReplicationSourcePaused(rs, b)` | `ReplicationSourceConfig.Paused`, or `rs.Spec.Paused = b` |
| `SetReplicationDestinationPaused(rd, b)` | `ReplicationDestinationConfig.Paused`, or `rd.Spec.Paused = b` |
| `SetReplicationSourceSchedule(rs, s)` | `ReplicationSourceConfig.Trigger` |
| `SetReplicationSourceManualTrigger(rs, s)` | `ReplicationSourceConfig.Trigger` |
| `SetReplicationDestinationSchedule(rd, s)` | `ReplicationDestinationConfig.Trigger` |
| `SetReplicationDestinationManualTrigger(rd, s)` | `ReplicationDestinationConfig.Trigger` |
| `SetReplicationSourceMover(rs, m)` | `ReplicationSourceConfig.Mover` |
| `SetReplicationDestinationMover(rd, m)` | `ReplicationDestinationConfig.Mover` |

The three spec setters were bare field assignments. The four trigger setters
nil-initialised `Spec.Trigger` and then wrote one of its fields, leaving the
sibling trigger field standing — their doc comments claimed to replace the whole
trigger and did not.

The two mover setters were the substantive removal. Each cleared all six mover
arms and then set one, a verbatim duplicate of the type switch in
`ReplicationSource` / `ReplicationDestination`. A sealed-union discriminator is
a multi-field write by nature, so it belongs where the invariant is already
owned — the constructor — not behind a `Set<Field>` name. Every arm of that
switch, typed-nil cases included, is covered by the constructor's own tests.

`AddSyncthingPeer` stays: appending to `cfg.Peers` is class (a). Its nil
handling is a behaviour break in its own right:

| Behaviour change | Before | After |
|---|---|---|
| `AddSyncthingPeer(nil, addr, id, introducer)` | `if cfg == nil { return }` — the peer is silently discarded | panics with `AddSyncthingPeer: cfg must not be nil` |

A nil receiver is a programming error, and swallowing the caller's write is the
one outcome that cannot be diagnosed from the output. Every other appender in
the package already panicked; this one no longer differs.

### `AddPodSpecTopologySpreadConstraints`

Named here because it is a deliberate behaviour change rather than a signature
one. It previously returned early on a nil constraint, discarding the caller's
write without a word. It now panics, like the six other appenders in
`podspec.go` — a nil element is a programming error, and one appender silently
dropping input while the rest reject it was the worse inconsistency.

These take the exclusion list from 24 tolerated helpers to 11, all of them
`ConfigMap` helpers.

## `ConfigMap` helpers un-delegated

All eleven `ConfigMap` helpers in `pkg/kubernetes` were one-line forwarders into
`internal/kubernetes`. A helper that only calls another function contains no
field write, so every one of them classified as inadmissible however
class-shaped the code at the far end was. The bodies now live in
`pkg/kubernetes/configmap.go`, with the nil-receiver panic the rest of the
package uses.

Four are removed rather than inlined:

| Removed | Replacement |
|---|---|
| `SetConfigMapData(cm, data)` | `cm.Data = data` |
| `SetConfigMapBinaryData(cm, data)` | `cm.BinaryData = data` |
| `AddConfigMapDataMap(cm, data)` | `for k, v := range data { AddConfigMapData(cm, k, v) }` |
| `AddConfigMapBinaryDataMap(cm, data)` | `for k, v := range data { AddConfigMapBinaryData(cm, k, v) }` |

The first two are bare field assignments. The two merges are not admissible in
any spelling: class (a) is a *single* insert whose value comes from the caller,
and neither `maps.Copy(cm.Data, data)` nor an explicit `for k, v := range data`
loop satisfies it — both were tried against the classifier and both stayed
inadmissible. Rather than widen a class for one shape that appears twice in the
whole tree, merging is a loop at the call site, the same answer PSA labels got.

`AddConfigMapData`, `AddConfigMapBinaryData` and `SetConfigMapImmutable` survive
inlined: the first two are class (a) map inserts with a nil-map guard, the third
a class (b) pointer assignment. `AddConfigMapLabel`, `AddConfigMapAnnotation`,
`SetConfigMapLabels` and `SetConfigMapAnnotations` are handled with the rest of
the per-kind metadata helpers below.

`pkg/kubernetes/configmap.go` was the only importer of
`internal/kubernetes`, so that package now has no callers inside the module.
Removing it is a separate change, tracked in issue go-kure/kure#756.

That leaves 2 tolerated helpers, both `ConfigMap` metadata.

## Per-kind metadata helpers removed

The contract admits exactly four generic metadata helpers by name — `AddLabel`,
`SetLabels`, `AddAnnotation`, `SetAnnotations` — and that set never grows. They
take a `metav1.Object`, so they already reach every kind in the module,
including kinds kure has never heard of. The thirty-two per-kind helpers below
reached one kind each and are removed.

Every one of them has the same replacement shape, so the tables below give the
kinds rather than repeating one row per function:

```go
// before
cnpg.AddClusterLabel(cluster, "env", "prod")
certmanager.AddIssuerAnnotation(issuer, "note", "production")

// after
kubernetes.AddLabel(cluster, "env", "prod")
kubernetes.AddAnnotation(issuer, "note", "production")
```

Every removed symbol, so that searching this file for an old name finds it:

| Package | Removed | Replacement |
|---|---|---|
| `pkg/kubernetes` | `AddConfigMapLabel`, `AddNamespaceLabel`, `AddServiceLabel`, `AddServiceAccountLabel` | `kubernetes.AddLabel(obj, key, value)` |
| `pkg/kubernetes` | `AddConfigMapAnnotation`, `AddNamespaceAnnotation`, `AddServiceAnnotation`, `AddServiceAccountAnnotation` | `kubernetes.AddAnnotation(obj, key, value)` |
| `pkg/kubernetes` | `SetConfigMapLabels(cm, labels)` | `cm.Labels = labels` |
| `pkg/kubernetes` | `SetConfigMapAnnotations(cm, anns)` | `cm.Annotations = anns` |
| `pkg/kubernetes/certmanager` | `AddCertificateLabel`, `AddIssuerLabel`, `AddClusterIssuerLabel` | `kubernetes.AddLabel(obj, key, value)` |
| `pkg/kubernetes/certmanager` | `AddCertificateAnnotation`, `AddIssuerAnnotation`, `AddClusterIssuerAnnotation` | `kubernetes.AddAnnotation(obj, key, value)` |
| `pkg/kubernetes/cnpg` | `AddClusterLabel`, `AddDatabaseLabel`, `AddObjectStoreLabel`, `AddScheduledBackupLabel` | `kubernetes.AddLabel(obj, key, value)` |
| `pkg/kubernetes/cnpg` | `AddClusterAnnotation`, `AddDatabaseAnnotation`, `AddObjectStoreAnnotation`, `AddScheduledBackupAnnotation` | `kubernetes.AddAnnotation(obj, key, value)` |
| `pkg/kubernetes/externalsecrets` | `AddExternalSecretLabel`, `AddSecretStoreLabel`, `AddClusterSecretStoreLabel` | `kubernetes.AddLabel(obj, key, value)` |
| `pkg/kubernetes/externalsecrets` | `AddExternalSecretAnnotation`, `AddSecretStoreAnnotation`, `AddClusterSecretStoreAnnotation` | `kubernetes.AddAnnotation(obj, key, value)` |
| `pkg/kubernetes/fluxcd` | `AddHelmReleaseLabel` | `kubernetes.AddLabel(obj, key, value)` |
| `pkg/kubernetes/fluxcd` | `AddHelmReleaseAnnotation` | `kubernetes.AddAnnotation(obj, key, value)` |

Thirty of the thirty-two take the generic helper unchanged — same arguments,
same nil-map initialisation, same result. `SetConfigMapLabels` and
`SetConfigMapAnnotations` are whole-map replacements: a generic pair taking a
whole map would do the same thing, but a bare
field assignment is what the contract prefers for a bare write.

Three groups of metadata-shaped helpers stay, because none of them writes
`ObjectMeta`:

| Kept | Field it writes |
|---|---|
| `cilium.SetCiliumNetworkPolicyLabels`, `cilium.SetCiliumClusterwideNetworkPolicyLabels` | `spec.labels` (`labels.LabelArray`) |
| `prometheus.AddServiceMonitorTargetLabel`, `prometheus.AddPodMonitorPodTargetLabel` | `spec.targetLabels` / `spec.podTargetLabels` (`[]string`) |
| `fluxcd.AddCommonMetadataLabel`, `fluxcd.AddCommonMetadataAnnotation` | `spec.commonMetadata` on a `Kustomization` — a spec sub-type, not a `metav1.Object`, so the generic helpers cannot reach it |

With these gone the exclusion list is empty: every exported `Set*`/`Add*` under
`pkg/kubernetes` now satisfies the contract, and
`testdata/admission_exclusions.txt` records no tolerated helper at all.

## Flux workflow defaults, declared by the defaults PR

`pkg/kubernetes` injects nothing at all once the constructors are identity-only
and the sub-type constructors are gone — the two sections above list all of it.
`pkg/stack/fluxcd` is a different case. It is a workflow layer above the
builders and may legitimately hold opinions, so most of its **twenty-six**
injected values are not deleted:

| Disposition | Sites |
|---|---|
| declared inputs with exported named defaults | 17 |
| pass-through tri-state inputs | 4 |
| deferred to the package that owns the value | 3 |
| never defaults at all; removed outright | 2 |

The seventeen exported-default sites collapse onto **eleven** identifiers.
Interval, namespace, source name, bootstrap name and the Flux directory name
were each written as a separate literal in two places, and the source kind in
two more with opposite polarities — six identifiers covering twelve sites, plus
five that appear once each. Those copies could have been changed apart. They now
cannot.

`defaults.go` declares a twelfth name, `ModeGotk`, which is not a default —
nothing falls back to it. It exists so the bootstrap mode set has one authority:
`DefaultFluxMode` is both the fallback for an empty `FluxMode` and the value
`GenerateBootstrap` dispatches on, so the two cannot be changed apart into a
state where an empty `FluxMode` resolves to a mode the switch rejects.

The ticket's own figure was twenty, and that number was too low. It counted
`resource_generator.go` and `bootstrap_generator.go` only: `layout_integrator.go`
holds five further sites and was never in the inventory, and the bootstrap
Kustomization's `spec.path` root was missed inside a file that was. The count
above is recomputed from the package as it stands, and the two remaining
categories below account for every site that is not simply renamed.

### Exported so a consumer can see and override them

Every fallback either generator applies is now one of these identifiers in
`pkg/stack/fluxcd/defaults.go`. Grep for the identifier to find every place its
value can reach emitted YAML. How each one is overridden differs, and the
paragraph after the table gives the split — four are copied into an exported
generator field by the constructor, four yield to a named input, and three are
fixed path segments with no override.

| Identifier | Value | Replaces the literal formerly at |
|---|---|---|
| `DefaultInterval` | `60 * time.Minute` | `NewResourceGenerator`, `NewBootstrapGenerator` — and retained when a non-empty `Bundle.Interval` fails to parse |
| `DefaultNamespace` | `"flux-system"` | both constructors |
| `DefaultMode` | `layout.KustomizationExplicit` | `NewResourceGenerator` |
| `DefaultSourceName` | `"flux-system"` | `generateGitSource`, `generateOCISource` |
| `DefaultBootstrapName` | `"flux-system"` | the bootstrap `Kustomization`'s name, the `FluxInstance`'s name |
| `DefaultFluxMode` | `"flux-operator"` | `GenerateBootstrap`'s empty-mode fallback, its dispatch switch, and `SupportedBootstrapModes` |
| `DefaultSourceKind` | `"OCIRepository"` | `generateFluxSystemKustomization`, `generateFluxInstance` |
| `DefaultBootstrapPathRoot` | `"manifests"` | `generateFluxSystemKustomization` |
| `DefaultFluxDirName` | `"flux-system"` | `addSeparateFluxToLayout`, twice |
| `DefaultSourceRef` | `"latest"` | `generateOCISource` |
| `DefaultSyncPath` | `"./"` | `generateFluxInstance` |

`DefaultBootstrapName` also gains `BootstrapGenerator.BootstrapName`. It was the
one default with no override path at all — `BootstrapConfig` carries no name
input and the constant was applied directly at both emission sites — while the
package documented every default as overrideable. Four identifiers are now
copied into exported generator fields (`DefaultInterval`, `DefaultNamespace`,
`DefaultMode`, `DefaultBootstrapName`); the rest are overridden by naming the
corresponding input, and the three with no override (`DefaultBootstrapPathRoot`,
`DefaultFluxDirName`, `DefaultSyncPath`) now say so instead of being described
as though they had one.

Adding that field made a nameless object reachable for the first time, so an
empty `BootstrapName` resolves back to `DefaultBootstrapName` at emission. Both
names were previously a literal and could not be absent; a consumer that builds
`BootstrapGenerator` as a struct literal rather than through
`NewBootstrapGenerator` leaves the new field zero, and a `Kustomization` or
`FluxInstance` with an empty `metadata.name` is rejected by the API server. The
field is an override, not a way to remove the name. `DefaultNamespace` and
`DefaultInterval` have the same hazard on a struct literal, but they had it
before this change as well, and resolving them at emission would alter existing
behaviour rather than preserve it — so they are left as they were.

`DefaultFluxDirName` shares `DefaultNamespace`'s value and is a separate
identifier on purpose: one is a path segment, the other a Kubernetes namespace,
and renaming the namespace must not silently rename the directory.

Except for `DefaultSourceKind`, none of these changes emitted YAML — they change
where the value comes from. `DefaultSourceKind` does change it, and has its own
entry below.

### Deferred, not renamed: three values this package does not own

Two sites in `addSeparateFluxToLayout` and `normalizeRulesPlacement` fell back to
`layout.FilePerResource` and `layout.FluxSeparate`. Both are already declared by
`layout.DefaultLayoutRules()` (`pkg/stack/layout/types.go:154-163`), which is
also where the layout walker resolves unset options from
(`pkg/stack/layout/walker.go:42-43`). They now read from that function rather
than getting an identifier here, because a constant in `pkg/stack/fluxcd` would
be a second copy of a value that can change independently — the failure mode the
rest of this work exists to remove.

The third is the separate Flux layout's own `Mode`, which was
`layout.KustomizationExplicit` inline and is now left unset. It is deferred
rather than named because it cannot take effect at that site at all: the layout
writer resolves an unset mode to `KustomizationExplicit`
(`pkg/stack/layout/manifest.go:324-326`), and the separate Flux layout never
gains children, so the branch a mode selects lists the resource files either way
(`manifest.go:352`). Seeding it from `DefaultMode` would have made that
identifier look overrideable through `ResourceGenerator.Mode` at a site that
never reads the field — the same false-override claim this work removes
elsewhere.

### Changed: one resolved source kind, and the dangling reference it hid

The kind of source object bootstrap emits was decided in three places, and they
did not agree. `generateSource` emitted a `GitRepository` only when
`SourceKind == "GitRepository"` and an `OCIRepository` for everything else,
including the empty string. `generateFluxInstance`'s sync block used the same
test. But `generateFluxSystemKustomization` used the opposite one — it wrote
`sourceRef.Kind: OCIRepository` only when `SourceKind == "OCIRepository"`, and
`GitRepository` otherwise.

The three agreed only when `SourceKind` named a kind exactly:

| `SourceKind` | Emitted source | `sourceRef.Kind` before | after |
|---|---|---|---|
| `"GitRepository"` | GitRepository | `GitRepository` | `GitRepository` |
| `"OCIRepository"` | OCIRepository | `OCIRepository` | `OCIRepository` |
| `""` | OCIRepository | `GitRepository` | `OCIRepository` |
| anything else | OCIRepository | `GitRepository` | `OCIRepository` |

The last two rows are the same defect as the `sourceRef.name` one below, in the
other field of the same reference: a Kustomization pointing at an object that was
never created. An unnamed `SourceKind` is the default case, not an exotic one.

`resolvedSourceKind` is now the only place the decision is made, and all three
sites call it. The exported default is `DefaultSourceKind = "OCIRepository"`,
which is what an unnamed kind actually yields. The previous
`DefaultBootstrapSourceKind = "GitRepository"` named the losing side of the
disagreement rather than the package's behaviour, so it is gone rather than
renamed.

### Removed: the placeholder registry URL

`generateOCISource` substituted `oci://registry.example.com/flux-system` when
`BootstrapConfig.SourceURL` was empty, so a missing input produced a manifest
pointing at a registry that does not exist. An absent input is an error, not a
placeholder: `generateSource`, `generateGitSource` and `generateOCISource` now
return `(client.Object, error)` and reject an empty `SourceURL` naming the field.

The reachable behaviour is unchanged for every caller that supplied a URL, and
`GenerateBootstrap` still emits no source at all when `SourceURL` is empty —
that means "the caller supplies the source", and it is not an error.

### Removed: a dangling `sourceRef`

The bootstrap `Kustomization` hardcoded `spec.sourceRef.name: flux-system` while
the source object it points at is named after the root node. With a named root
node the Kustomization referenced a source that was never emitted. Both names
now come from one derivation, so they cannot drift apart.

| Root node | `sourceRef.name` before | after | Emitted source's name |
|---|---|---|---|
| named `prod` | `flux-system` | `prod` | `prod` |
| unnamed | `flux-system` | `flux-system` | `flux-system` |

### Fixed: a nil root node panicked

Consolidating the name derivation exposed an adjacent defect. Every site that
read the root node's name tolerated a nil node except the bootstrap
Kustomization's `spec.path`, which dereferenced `rootNode.Name` directly — one
line above the nil-guarded call for `sourceRef.name`, in the same struct
literal. `GenerateBootstrap(config, nil)` in gotk mode therefore panicked, while
the same call in flux-operator mode returned a valid `FluxInstance`.

`rootName` is now the single place the name is read, and returns `""` for a nil
or unnamed node. A nil root node yields `spec.path: manifests` and
`sourceRef.name: flux-system`, which is what the unnamed-node case already
produced.

### Changed: `prune` and `wait` are pass-through inputs

`Bundle.Prune` and `Bundle.Wait` were already `*bool`. The generator collapsed
them. They are now passed through untouched, and `BootstrapConfig` gains a
`Prune *bool` so the bootstrap `Kustomization` has the same input.
`ResourceGenerator` gains a `Prune *bool` for Kustomizations generated from a
`layout.ManifestLayout`, which carries no prune setting of its own.

The two resolve differently, because their upstream tags differ. Both tags are in
`kustomize-controller`'s `api/v1/kustomization_types.go`:

| Field | Upstream tag | Declared at | Unset input emits |
|---|---|---|---|
| `KustomizationSpec.Prune` | `+required`, no `omitempty` | `kustomization_types.go:99-101` | `prune: false` — the key cannot be left out |
| `KustomizationSpec.Wait` | `+optional`, `omitempty` | `kustomization_types.go:174-178` | nothing; Flux's own default, `false`, applies |

**This is the behaviour change with teeth.** An unset `Prune` used to emit
`prune: true`, enabling destructive garbage collection for a caller who never
asked for it. It now emits `prune: false`. A caller that wants garbage
collection sets `Prune` explicitly:

```go
prune := true
bundle.Prune = &prune
```

### Changed: the umbrella no longer forces `wait: true`

An umbrella bundle (`Bundle.Children` non-empty) had `spec.wait` forced to
`true` alongside the auto-generated `spec.healthChecks` entry per child. That
was self-defeating: upstream's own doc comment on the field says that when it is
enabled "the HealthChecks are ignored"
(`kustomize-controller api/v1/kustomization_types.go:175-177`), so the entries
the generator had just built were inert. Removing the force is what makes them
take effect, and the umbrella still becomes `Ready` only when every child is.

A caller that prefers upstream's whole-of-resources health assessment sets
`Wait` explicitly; that gates on the child Kustomizations too, and then the
`healthChecks` entries are the ones upstream ignores.

`stack.Bundle.Validate` still rejects an explicit `Wait=false` on a bundle with
`Children` (`pkg/stack/bundle.go:196-199`). With the force gone that rejects a
state whose emitted YAML is identical to the accepted unset one, since `Wait`
carries `omitempty`. Which way to resolve the asymmetry is a semantic call about
what an umbrella promises, not a mechanical fix, so it is tracked separately in
issue go-kure/kure#760 rather than settled here.
