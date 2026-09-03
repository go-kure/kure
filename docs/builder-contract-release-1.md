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
| `CreateConfigMap` | `data` and `binaryData` initialised to empty maps. They never rendered, but a fresh object accepted `cm.Data[k] = v` directly; both are now nil, so write through `AddConfigMapData`/`AddConfigMapBinaryData` (which nil-init) or `SetConfigMapData`/`SetConfigMapBinaryData`, or assign a map literal first |
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

| Removed | Replacement |
|---|---|
| `CreateContainer(name, image, command, args)` | `&corev1.Container{Name: name, Image: image, Command: command, Args: args}` |
| `CreatePodSpec()` | `&corev1.PodSpec{}` |
| `CreateVolumeClaimTemplate(name, storageClass, size, accessModes)` | a `corev1.PersistentVolumeClaim` literal |
| `CreateIngressRule(host)` | `&netv1.IngressRule{Host: host}` |
| `CreateACMEIssuer(cfg)` | a `cmacme.ACMEIssuer` literal |
| `CreateACMEHTTP01Solver(cfg)` | a `cmacme.ACMEChallengeSolver` literal |
| `CreateACMEDNS01SolverCloudflare(cfg)` | a `cmacme.ACMEChallengeSolver` literal |
| `CreateACMEDNS01SolverRoute53(cfg)` | a `cmacme.ACMEChallengeSolver` literal |
| `CreateACMEDNS01SolverGoogle(cfg)` | a `cmacme.ACMEChallengeSolver` literal |

The four cert-manager solver constructors and `CreateACMEIssuer` were only ever
called from `certmanager.Issuer`/`ClusterIssuer`, which now build the literals
inline.

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
void and panic on a nil `*PodSpec` or a nil element, matching the `SetPodSpec*`
setters that already did. `AddPodSpecTopologySpreadConstraints` no longer
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
not as contract violations. Dropping those 47 plus the 7 `AddPodSpec*` helpers
that no longer return an error takes the file from 87 tolerated helpers to 33.

The `pkg/errors` sentinels those helpers returned (`ErrNilDeployment`,
`ErrNilDaemonSet`, `ErrNilStatefulSet`, `ErrNilJob`, `ErrNilCronJob`,
`ErrNilVolume`, `ErrNilToleration`, `ErrNilImagePullSecret`,
`ErrNilInitContainer`, `ErrNilEphemeralContainer`, `ErrNilSpec`) are now unused
inside the module. They stay exported for this release; retiring them is a
separate `pkg/errors` change, not part of the builder surface.

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

These take the exclusion list from 33 tolerated helpers to 24.
