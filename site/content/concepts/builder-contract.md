+++
title = "Builder Contract"
weight = 25
+++

# Builder Contract

kure's Kubernetes foundation is deliberately thin: it hands you the upstream Go struct with its
identity filled in, and stops. This page is the consumer's view of what that means in practice —
how to build an object, when a kure helper exists and when there is none, how to read the generated
maturity table, and why the layers above the foundation state their opinions as names you can grep
for.

The normative text is the [Kubernetes Builders](/api-reference/kubernetes-builders/) page, which is
`pkg/kubernetes/README.md` mounted here. Where this page and that one differ, that one is right.

## Building an object

Every registered kind has a constructor. It returns the upstream type, carrying `apiVersion`,
`kind`, `metadata.name` and — for a namespaced kind — `metadata.namespace`. Nothing else.

```go
d := kubernetes.CreateDeployment("web", "default")
d.Spec.Replicas = ptr.To[int32](3)
d.Spec.Template.Spec.ServiceAccountName = "web"
```

Cluster-scoped kinds take one argument, and the signature is how you tell:

```go
ns := kubernetes.CreateNamespace("platform")
```

There is also a generic form, for a registered type that has no wrapper yet:

```go
d := kubernetes.Create[appsv1.Deployment]("web", "default")
```

The wrappers are generated from the registered scheme, so the set of constructors and the set of
kinds kure knows about cannot drift apart. An unregistered type panics — that is a programming
error, like a nil receiver, not a runtime condition to handle.

### What a constructor will not do

It will not set a label, a selector, a replica count, a schedule, an access mode or an empty slice.
Earlier releases did some of this: `CreateDeployment` wrote `app: <name>` into both the labels and
the selector, `CreatePersistentVolumeClaim` requested 1Gi, `CreateServiceAccount` wrote
`automountServiceAccountToken: false`. All of it is gone, and the
[release-1 migration notes](/concepts/builder-contract-release-1/) list every value with the line
that restores it.

The reason is that a default you did not ask for is indistinguishable, in the YAML, from one you
did. A 1Gi PVC that you never sized reads exactly like a 1Gi PVC you sized deliberately, and the
difference only becomes visible when the volume fills.

## When sugar exists, and when it does not

A `Set*` or `Add*` helper exists only where a plain assignment cannot do the job. There are three
such cases, and `pkg/kubernetes/admission_test.go` parses every exported helper in the tree and
fails the build on one that fits none of them.

| Class | Why an assignment is not enough | Example |
|---|---|---|
| Appender | `append` is not an assignment, and doing it by hand is easy to get subtly wrong | `AddPodSpecContainer`, `AddRoleRule` |
| Pointer / nil-init | The field is a pointer, or a map that has to be created before the first write | `SetDeploymentReplicas`, `AddConfigMapData` |
| Composite | Several fields belong together, and the helper's name states the opinion | `SetHPAMinMaxReplicas`, `AddHPACPUMetric` |

Everything else you write yourself:

<!-- doc-api-refs:ignore-start names a removed helper to say it is removed -->

```go
// There is no SetDeploymentStrategy. This is what it would have done.
d.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}
```

<!-- doc-api-refs:ignore-end -->

That is the contract working as intended, not a gap. A helper whose whole body is
`obj.Field = value` is the assignment with a longer name and one more thing to keep in sync with
upstream — and there were 257 of them before release 1.

Two consequences are worth planning around:

- **Helpers take the pod template's `PodSpec`, not the workload kind.** There is one
  `AddPodSpecContainer`, not one per workload:
  `AddPodSpecContainer(&d.Spec.Template.Spec, c)`, and for a CronJob
  `AddPodSpecContainer(&cj.Spec.JobTemplate.Spec.Template.Spec, c)`.
- **Metadata is one set of helpers over every kind.** `SetLabels`, `AddLabel`, `SetAnnotations` and
  `AddAnnotation` take a `metav1.Object`, so they work on kinds kure has never heard of. Per-kind
  label helpers do not exist.

Sugar returns nothing and panics on a nil object. If you need a field a helper does not reach,
assign it; if you need a helper that does not exist, the contract says to send the PR — there is no
completeness claim to wait on.

## Reading the maturity table

[Supported kinds and field maturity](/api-reference/api-tables/) is generated from the upstream
modules this build pins, and every row names the module and version it was read from. It has two
tables.

**Kinds** answers "does kure know this kind, and is it namespaced?" The `Scope from` column says
what settled the scope — the kind's own `+kubebuilder:resource` marker, the
`CustomResourceDefinition` its module ships, or the built-in table for kinds whose scope the
Kubernetes API defines. A kind no source can answer for is a generation-time error, never a
namespaced default.

**Field maturity** lists construction-side fields carrying a stability signal: a `+featureGate`
marker, or a doc comment that claims alpha, beta, or Go's `Deprecated:` prefix for the field being
documented. Read it as a report, not a warning:

- **kure never filters or refuses on this.** It cannot know your cluster's gates, so withholding a
  field would be a policy judgement inside a library.
- **The `Feature gates` column is the one to act on.** For a built-in type, the API server does not
  reject a field whose gate is off — it silently clears the field and admits the object. The
  manifest reads as applied and is not, which is the failure this column exists to make visible.
- **A blank stability with a gate listed** means upstream gates the field but does not document a
  level. That is still a gate, and still silent.
- **Status fields are not in the table at all.** A status is reported by the cluster, never built by
  a caller, and that is where most of the markers live.

The numbers move with the pins, by design: bump a module and the tables are regenerated, so "what
does kure support" is answered from the pin rather than from prose someone forgot to update.

## Opinions as nouns

The foundation holds no opinions. The workflow layer above it — `pkg/stack/fluxcd` — legitimately
does: something has to decide a reconcile interval, a namespace, a source kind. The rule there is
not "have no defaults", it is **every default is a name you can see**.

```go
// pkg/stack/fluxcd/defaults.go
fluxcd.DefaultInterval    // 60 * time.Minute
fluxcd.DefaultNamespace   // "flux-system"
fluxcd.DefaultSourceKind  // "OCIRepository"
```

Eleven exported identifiers replaced seventeen anonymous literals, several of which were the same
value written in two places and could therefore have drifted apart. For a consumer this buys three
things:

1. **Discoverability** — the package's opinions are a file you can read, not a value you find by
   diffing generated YAML against what you expected.
2. **Grep-ability** — one identifier tells you every place a value can reach emitted YAML.
3. **Override, explicitly** — four are copied into exported generator fields, four yield to a named
   input, and three are fixed path segments with no override. The three that cannot be overridden
   now say so, instead of being documented as though they could.

The same principle runs through the foundation in negative: an opinion that has no name and no
owner does not get to exist. That is why the constructor defaults were removed rather than
documented.

## Where to go next

- [Kubernetes Builders](/api-reference/kubernetes-builders/) — the normative contract, with the
  admission classes and the derivation in full
- [Release-1 migration notes](/concepts/builder-contract-release-1/) — every removed helper and the
  expression that replaces it
- [Supported kinds and field maturity](/api-reference/api-tables/) — the generated tables
- [Using Kure as a Library](/guides/library-usage/) — import paths and end-to-end examples
