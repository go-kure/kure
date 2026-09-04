# Kubernetes Builders - The Builder Contract

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes)

The `kubernetes` package provides the shared scheme, the generic constructor and the
admissible sugar helpers for building Kubernetes objects. This page is the normative
text of the builder contract (ADR-038, "thin core + admissible sugar"): every package
under `pkg/kubernetes/...` follows it, and the tests described below enforce it.

## Import

```go
import "github.com/go-kure/kure/pkg/kubernetes"
```

## 1. Canonical path

For every registered kind the upstream Go struct is the construction API:

```go
d := kubernetes.CreateDeployment("web", "default")
d.Spec.Replicas = ptr.To[int32](3)
d.Spec.Template.Spec.ServiceAccountName = "web"
```

kure does not provide, and its docs do not suggest, a kure function for plain field
access. A whole-spec setter is a bare assignment and is not part of the contract.

## 2. Constructors

`Create[T any, PT interface{ *T; client.Object }](name, namespace string) PT` allocates
`T`, sets `TypeMeta` from the registered scheme (the same lookup
`GetGroupVersionKind` uses), sets `metadata.name` and `metadata.namespace`, and
nothing else. The pointer type is inferred from the one type argument:

```go
d  := kubernetes.Create[appsv1.Deployment]("web", "default")
ns := kubernetes.Create[corev1.Namespace]("platform", "")   // cluster-scoped: pass ""
```

An unregistered type panics. That is a programming error, the same rule as a nil
receiver, not a runtime condition to handle.

Per-kind wrappers keep call sites readable and carry the scope in their signature:
`CreateDeployment(name, namespace)` for namespaced kinds, `CreateNamespace(name)` for
cluster-scoped ones. They live in `zz_generated_create.go` in each package, are
generated from the scheme and the scope table in `pkg/kubernetes/internal/kinds`, and
are never hand-written. A kind registered in the scheme without a wrapper fails the
identity test; a wrapper that sets anything beyond identity fails it too.

Sub-types that are not `client.Object` (`Container`, `PodSpec`,
`ResourceRequirements`, an `IngressRule`, a PVC used as a template) get no generated
constructor. A struct literal is the idiom: build the value directly, as
`&corev1.Container{Name: "app", Image: "nginx"}`.

In this package two hand-written sub-type constructors survive, both because
they do more than wrap a literal and both under review for the next work item:
`CreateResourceRequirements()` returns empty `Requests` and `Limits` maps, and
`CreateIngressPath(path, pathType, service, port)` assembles a nested
`HTTPIngressPath`. Everything else in that shape has been removed from this
package.

The kind sub-packages are a different matter and are not in scope here.
`pkg/kubernetes/fluxcd` still exports twenty-four hand-written sub-type
constructors (`CreateSourceReference`, `CreatePostBuild`, `CreateDecryption`,
`CreateCommonMetadata`, `CreateDriftDetection`, `CreatePostRendererKustomize`,
`CreateGitSpec` and the rest), and `pkg/kubernetes/prometheus` exports
`CreateRuleGroup`. They remain available and unchanged; whether they belong under
the contract is the sealing work item's question, not this one's.

### Regenerating the wrappers

```bash
make gen-builders      # or: mise run builders:generate
make check-builders    # or: mise run builders:check -- exit 1 when stale
```

`check-builders` runs in the CI `validate` job, so a dependency bump that adds or drops
a kind fails until the wrappers are regenerated and committed. Renovate runs
`scripts/gen-builders.sh generate` itself after any Go module bump.

## 3. Sugar admission

An exported `Set*` / `Add*` function under `pkg/kubernetes/...` is admissible when its
body does one of:

- **(a)** appends to a slice field, or inserts into a map field. Going through a local
  counts only when that local came from the field it is written back to: a collection
  the helper builds itself and then assigns replaces the field's contents, which is
  not adding to it;
- **(b)** assigns to a pointer-typed field (`x.F = &v`; initialising a nil pointer
  intermediate before assigning through it is the same thing);
- **(c)** constructs an upstream struct literal setting two or more fields, or a
  nested literal. A slice or map literal is not class (c): it replaces a collection
  rather than composing a value.

Every admitted operation carries a value the caller supplied. An append of a constant,
a map insert of a constant, a struct literal built entirely from constants, or a pointer
allocated (`new(T)`) and never written through, all set a value the caller never named
and are inadmissible (§4). The zero-value init a helper guards a nil field with
(`if o.Labels == nil { o.Labels = map[string]string{} }`) is not such a value.

A body that is a single assignment to a non-pointer field is inadmissible regardless
of path depth: writing `Spec.Template.Spec.ServiceAccountName` is still one assignment,
and two such assignments in one body are two forwarders, not a composite. A bare
assignment next to an admitted operation is inadmissible when its value is not an
argument: an append that also sets a scalar to a literal or a computed value touches a
field the caller did not name (§4). Forwarding a second argument alongside
(`SetHPAMinMaxReplicas(hpa, 2, 10)`) leaves the class alone. A helper
that returns anything, an `error` included, is inadmissible whatever its body does
(§4 allows no error return: a nil receiver panics). A nil receiver guard admits
nothing on its own. A body that assigns `nil` to any field,
directly or as a keyed value inside a literal (the literal `nil`, a typed conversion
of it, or a local known to be nil), is inadmissible whatever else it does, because it clears a field the caller did not name (§4); a
helper that must replace one member of a one-of takes the whole one-of as its
argument instead.

A helper reaches the object it writes through a parameter that can carry the write back
to the caller: a pointer, map, slice or interface. A struct taken by value is a copy, so
a helper written that way changes nothing the caller can see and is inadmissible.

`TestAdmission_SugarHelpersAreClassAdmissible` classifies every helper with `go/ast`
and type information (`pkg/kubernetes/internal/admission`) and fails naming any helper
outside (a)-(c). It is syntactic and deliberately conservative: it does not follow
control flow, so a write guarded by an optional-value condition
(`if name != "" { ... }`, forbidden by §4) is not detected by the test. That idiom is
caught by review and by the helper's own golden test. A function literal's body is not
the helper's own body: an append inside a closure the helper never calls is a no-op no
caller sees, so it admits nothing. `pkg/kubernetes/testdata/admission_exclusions.txt` listed the
helpers tolerated while the prune work item of the epic ran; that file is now empty and stays
empty. Entries only ever leave, and a stale entry fails the test.

## 4. Purity

- Sugar takes exactly the value it writes. Value arguments are fine
  (`SetDeploymentReplicas(d, 3)`) because `nil` stays expressible on the canonical path.
- No defaulting. No validation: `Validate*` helpers stay explicit, opt-in calls. No
  touching a field the caller did not name. No error return, because nothing in an
  assignment can fail; a nil receiver panics.
- The `if x != "" { set(x) }` idiom is forbidden in sugar. A composite that treats an
  argument as optional documents that per argument in its doc comment.
- Opinions are nouns. kure may hold knowledge a caller names
  (`RestrictedSecurityContext()`, `AddHPACPUMetric(hpa, 80)`) and never applies it to
  something the caller did not ask about. Every composite carries a golden test of its
  complete output so an injected value is visible in the diff that adds it.

## 5. Metadata

One helper set over `metav1.Object` covers every kind, including kinds kure never
names:

```go
kubernetes.SetLabels(obj, map[string]string{"app": "web"})
kubernetes.AddLabel(obj, "tier", "frontend")           // initialises a nil map
kubernetes.SetAnnotations(obj, map[string]string{"owner": "platform"})
kubernetes.AddAnnotation(obj, "note", "rotated 2026-09")
```

These four are admitted by name; per-kind label and annotation helpers are not part
of the contract, and none remain — `AddNamespaceLabel`, `AddClusterAnnotation`,
`SetConfigMapLabels` and the twenty-nine others like them were removed, since the
four above already reach every kind through `metav1.Object`. Two helpers keep a
metadata-shaped name while writing something else: cilium's `Set*PolicyLabels`
write the policy's `spec.labels`, and prometheus's `Add*TargetLabel` appends to a
scrape spec's target-label list. Neither is ObjectMeta.

## 6. Names

No rename wave. A surviving function keeps its name unless the name is wrong.

## 7. Consumers are never blocked

If a caller needs a kure change to reach a field, the contract is broken. Sugar is
added on demand, by the caller's PR, with its test and golden file. There is no
completeness claim and no coverage oracle.

## 8. Feature-gated and deprecated fields

Ordinary fields. kure cannot know a target cluster's gates, so withholding a field
would be a policy judgement inside a pure library. Maturity is a label, never
enforced: kure reports what the pinned API sources say and a consumer with cluster
knowledge decides.

The label is worth carrying because the failure it describes is silent. For built-in
types the API server does not reject a field whose feature gate is disabled — it
clears the field and admits the object, so the manifest reads as applied and is not.

`pkg/kubernetes/internal/maturity` walks every type reachable from a registered
kind's own struct and records the fields carrying a signal: a `+featureGate` marker,
or a doc comment declaring alpha, beta or Go's conventional `Deprecated:` prefix. The
marker is the precise signal; the prose scan is the best-effort complement for fields
upstream documents without gating, and it matches whole words, so "alphabetical" is
not alpha.

Status types are not entered. A kind's status is reported by the cluster and never
constructed by a caller, so a gate there says nothing about whether a manifest kure
builds applies as written — and that is where most of the markers are.

Against the current pins, the walk finds 177 maturity-carrying construction-side
fields, of which 41 require a feature gate (40 in `k8s.io/api`, one in
`k8s.io/apiextensions-apiserver`); 48 are documented alpha, 42 beta and 66
deprecated, and 92 distinct status types are skipped. No CRD module kure pins uses
`+featureGate` at all. These numbers move with the pins and are not asserted by any
test; the tests assert that every reported field exists in the pinned struct.

## 9. Where scope and maturity come from

Both are derived from the pinned upstream module sources, not kept by hand beside
them. Five internal packages do it, and none of them is part of the public API:

- `internal/markers` parses the controller-gen markers kure reads —
  `+kubebuilder:resource` for scope, `+featureGate` for maturity. Pure text, no I/O.
- `internal/upstream` loads the pinned modules with `go/packages` and returns each
  named type with its doc comment, fields, file-scoped import aliases and the module,
  version and module directory it came from. A field tagged `json:"-"` is left out:
  it is not serialised, so it has no name a manifest could carry, and recording it
  would file it under the name `-` and make its type reachable in the maturity walk
  (cilium's `XDSResource` embeds `*anypb.Any` that way).
- `internal/crds` reads the `CustomResourceDefinition` manifests a module ships in
  that directory, which is where the scope comes from for a type carrying no marker.
- `internal/kinds` resolves a scope per registered kind; `internal/maturity` walks
  the type graph for the field table.

Every resolved scope records which of three sources answered — `marker`, `builtin`
or `crd` — so a wrong scope can be traced to the thing that claimed it. Against the
current pins that is 65 from the kind's own marker, 44 from the built-in table and
19 from a shipped CRD.

`kinds.Registered()` returns each kind with that resolution already applied, and it
is the only place a scope is stated: the 128-entry hand-seeded pair of sets this
package used to carry is gone, and so are the two hand-kept maps in `pkg/manifest`,
which now reads `IsNamespaced` from the generated table. The cluster-scoped half of
the old table survives as a frozen fixture in the `internal/kinds` tests, dated to
the pins it was taken at. That is deliberate: the derivation fails silently by
construction — an absent, unread or detached marker resolves to `Namespaced`, which
is also the right answer for 95 of the 128 kinds — so without a literal to compare
against, a regression in the comment reattachment below would turn 31 kinds
namespaced with nothing going red. A pin bump that legitimately re-scopes a kind is
an edit to that fixture, made with the upstream change named in the commit message.

Three things about that derivation are worth knowing before changing it.

**A marker the parser cannot read is a fatal error, never a default.** The one
legitimate default — an absent `+kubebuilder:resource:scope`, which upstream defines
as `Namespaced` — is indistinguishable from a marker whose spelling the parser failed
to match. Treating the second as the first emits a namespaced wrapper for a
cluster-scoped kind and puts a `metadata.namespace` on an object that must not carry
one. Two spellings are in use across the pinned modules and both are accepted: the
bare `scope=Cluster`, and `scope="Cluster"` quoted inside a comma-separated list whose
other values contain braced commas.

**controller-gen separates its marker block from the type's prose doc comment with a
blank line**, which detaches the markers from the declaration's `Doc` in `go/ast` and
files them as free-floating comments on the file. Reading `Doc` alone derives 31 of
the 33 cluster-scoped kinds as namespaced — silently, since every wrong answer is the
default. `internal/upstream` reattaches the preceding comment group, bounded by the
end of the previous declaration, and drops it for grouped `type (...)` blocks where it
cannot be attributed to one spec.

**Many types carry no `+kubebuilder:resource` marker at all**, and none of them gets
the default handed to it:

- The built-in modules (`k8s.io/api`, `k8s.io/apiextensions-apiserver`) have no
  markers because the API server, not a generator, defines their scope. Sixteen
  explicit entries in `internal/kinds` name the cluster-scoped built-ins; every other
  built-in kind is namespaced.
- A CRD module marks a type only when it needs a non-default setting, so an unmarked
  root type is ordinary rather than exceptional: 19 of the registered kinds are in
  that state, across cnpg, metallb and `plugin-barman-cloud`. Their scope is read
  from the `CustomResourceDefinition` the module itself ships — controller-gen's own
  output, generated from the same source, which states the scope explicitly whether
  or not a marker was needed to produce it. That is a second upstream source, not a
  second guess, and it keeps the answer out of a table maintained here.

  Only final manifests count. A file is read when a `kind:` key at the start of a
  line names a `CustomResourceDefinition`, and it is skipped when it contains Go
  template delimiters: a Helm template is the input to a rendering step, not a
  definition. metallb ships exactly that shape — a chart whose `crds.yaml` opens
  with real CRDs and later uses `{{ .Release.Namespace }}` as a map key — and its
  real manifests sit in `config/crd/bases` beside it. Within a file that is read,
  a document that does not decode is an error naming the file and the document's
  index, never a short read: stopping quietly there drops every definition after
  it, which loses a kind's only answer or one half of a scope conflict with
  nothing to show anything was skipped.

  Those files are read out of the module zip as unpacked in the local module cache —
  the directory `go list -m` reports for the pinned version — not fetched from the
  project's repository. So the manifests read are exactly the ones the pinned version
  publishes, and they are covered by the module checksum like any other file in it.
  The consequence to know: a module that stops shipping its CRDs in a later version
  turns the next Renovate bump into a hard generation failure on that PR, which is the
  intended behaviour — the scope becomes unanswerable and is declined rather than
  silently defaulted to `Namespaced`.
- A kind with neither a marker nor a shipped CRD is an error. The namespaced default
  and a marker that was not read give the same answer, so a kind nothing can answer
  for is a question kure declines rather than resolves. Both halves are probed by
  fixtures: a partly-marked module (one root marked, one not — also the shape a
  grouped `type (...)` block produces), a shipped CRD declaring `Cluster` for an
  unmarked kind, and a module whose CRDs do not cover the kind at all.

## 10. The generated tables

The derivation above is committed as three artifacts, all written by
`pkg/kubernetes/internal/gen` from one derivation pass so they cannot disagree:

| Artifact | For |
|---|---|
| `zz_generated_tables.go` | `Kinds` and `FieldMaturities`, the exported Go values |
| `docs/api-tables.json` | the machine-readable copy, read as a diff on a bump PR |
| `docs/api-tables.md` | the site page (mounted via `site/docs-map.yaml`) |

`tables.go` holds the hand-written types and lookups over them — `KindFor`,
`KindByGroupKind`, `IsNamespaced`, `MaturityForType`, `GatedFields`. Every value is a
plain string or bool: the generator writes into this package, so a table that
referenced an upstream type could not be regenerated after an API bump removed it.
Each kind row names the module and version it was read from, and `ScopeSource` names
what stated the scope, so any row is traceable to a pin.

Regenerate with `scripts/gen-builders.sh generate`; CI's `validate` job runs
`scripts/gen-builders.sh check`, and Renovate runs `generate` in its
`postUpgradeTasks` so a bump PR arrives with the tables already updated. Do not edit
the artifacts by hand.

Two consequences of that wiring:

- **A pin bump that changes a table changes a file in a doc-gated package.** The
  doc-gate's `// doc-gate:trivial` exemption cannot help — it only applies to lines
  containing `=` whose declaration prefix is unchanged, and a table row has none — so
  pure version churn in these artifacts needs the maintainer `docs-skip` label. See
  `docs/dependency-updates.md`.
- **`recover` does not delete `zz_generated_tables.go`.** It holds no upstream type
  names, so an API bump cannot make it uncompilable, and `tables.go` reads the values
  it declares. Its absence is a compile error on purpose: an empty table would report
  every kind as unregistered, which is a wrong answer rather than a missing one.

## Identity test

`TestIdentity_ConstructorsEmitIdentityOnly` walks every kind the scheme registers,
calls its generated wrapper and compares the result with `reflect.DeepEqual` against
a zero value carrying only GVK, name and (when namespaced) namespace. Any injected
label, selector or default turns it red. `TestIdentity_EveryRegisteredKindHasAWrapper`
fails on a registered kind with no wrapper and on a wrapper with no registered kind.

## GVK utilities and scheme

```go
// Lazily registers every supported API group (core K8s, FluxCD, cert-manager, ...)
err := kubernetes.RegisterSchemes()

// Resolve the GVK of any registered runtime.Object
gvk, err := kubernetes.GetGroupVersionKind(myDeployment)

// Check if a GVK is in an allow list
ok := kubernetes.IsGVKAllowed(gvk, allowedGVKs)
```

## Examples

The helpers below are the surviving sugar for the core kinds. Anything not shown is
a field write on the upstream struct.

### Deployment

```go
dep := kubernetes.CreateDeployment("my-app", "default")
kubernetes.AddLabel(dep, "app", "my-app")
dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"app": "my-app"}}
dep.Spec.Template.Labels = map[string]string{"app": "my-app"}

podSpec := &dep.Spec.Template.Spec
kubernetes.AddPodSpecContainer(podSpec, &corev1.Container{Name: "app", Image: "nginx:1.25"})
kubernetes.AddPodSpecToleration(podSpec, &corev1.Toleration{Key: "dedicated", Value: "web"})
kubernetes.SetDeploymentReplicas(dep, 3)
```

There is no `AddDeploymentContainer`. A workload kind's pod template is a
`corev1.PodSpec`, so the `PodSpec` helpers serve every kind — pass
`&dep.Spec.Template.Spec` (a CronJob nests one level deeper:
`&cj.Spec.JobTemplate.Spec.Template.Spec`). `ServiceAccountName` and
`NodeSelector` are plain fields on that struct and are assigned directly.

### CronJob

```go
cj := kubernetes.CreateCronJob("my-job", "default")
cj.Spec.Schedule = "*/5 * * * *"
cj.Spec.JobTemplate.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever

kubernetes.AddPodSpecContainer(&cj.Spec.JobTemplate.Spec.Template.Spec,
	&corev1.Container{Name: "worker", Image: "busybox:1.36"})
cj.Spec.ConcurrencyPolicy = batchv1.ForbidConcurrent
```

### Service

```go
svc := kubernetes.CreateService("my-app", "default")
svc.Spec.Selector = map[string]string{"app": "my-app"}
kubernetes.AddServicePort(svc, corev1.ServicePort{Name: "http", Port: 80, TargetPort: intstr.FromInt32(8080)})
svc.Spec.Type = corev1.ServiceTypeLoadBalancer
kubernetes.AddAnnotation(svc, "external-dns.alpha.kubernetes.io/hostname", "app.example.com")
```

### Ingress

```go
ing := kubernetes.CreateIngress("my-app", "default")
kubernetes.SetIngressClassName(ing, "nginx")

rule := &netv1.IngressRule{Host: "app.example.com"}
pt := netv1.PathTypePrefix
path := kubernetes.CreateIngressPath("/", &pt, "my-app", "http")
kubernetes.AddIngressRulePath(rule, path)
kubernetes.AddIngressRule(ing, rule)
kubernetes.AddIngressTLS(ing, netv1.IngressTLS{Hosts: []string{"app.example.com"}, SecretName: "my-app-tls"})
```

### HPA and PDB

```go
hpa := kubernetes.CreateHorizontalPodAutoscaler("my-app", "default")
kubernetes.SetHPAScaleTargetRef(hpa, "apps/v1", "Deployment", "my-app")
kubernetes.SetHPAMinMaxReplicas(hpa, 2, 10)
kubernetes.AddHPACPUMetric(hpa, 80)

pdb := kubernetes.CreatePodDisruptionBudget("my-app", "default")
kubernetes.SetPDBMinAvailable(pdb, intstr.FromInt32(2))
kubernetes.SetPDBSelector(pdb, &metav1.LabelSelector{MatchLabels: map[string]string{"app": "my-app"}})
```

`MinAvailable` and `MaxUnavailable` are mutually exclusive upstream, and each setter
writes only the field it names — a helper does not clear a field the caller did not
mention. Switching from one to the other is two statements:

```go
pdb.Spec.MinAvailable = nil
kubernetes.SetPDBMaxUnavailable(pdb, intstr.FromString("25%"))
```

### Namespace and Pod Security Admission

```go
ns := kubernetes.CreateNamespace("my-app")
kubernetes.AddLabel(ns, "env", "prod")

// enforce, warn, audit; "" skips a mode, version "" omits the version labels
for k, v := range kubernetes.PSALabels(kubernetes.PSARestricted, kubernetes.PSARestricted, kubernetes.PSARestricted, "v1.28") {
    kubernetes.AddLabel(ns, k, v)
}
```

`PSALabels` returns the label map and writes nothing. One argument expanding into six
labels is not something a `Set<Field>` helper may hide, so the expansion is a value
helper and the write stays with `AddLabel`.

### ConfigMap

```go
cm := kubernetes.CreateConfigMap("my-config", "default")
kubernetes.AddConfigMapData(cm, "key", "value")
kubernetes.AddConfigMapBinaryData(cm, "cert", certBytes)
kubernetes.SetConfigMapImmutable(cm, true)

// Replacing a map wholesale is an assignment, not a helper
cm.Data = map[string]string{"key": "value"}

// Merging one is a loop over the single-key helper
for k, v := range defaults {
    kubernetes.AddConfigMapData(cm, k, v)
}
```

`SetConfigMapData`, `SetConfigMapBinaryData`, `AddConfigMapDataMap` and
`AddConfigMapBinaryDataMap` are gone: the first two were bare field assignments, and
a bulk merge is not one of the admitted sugar classes in any spelling — neither
`maps.Copy` nor an explicit loop classifies, because the class is a *single* insert
whose value comes from the caller.

### PSA security contexts

```go
sc := kubernetes.RestrictedSecurityContext()
psc := kubernetes.PodSecurityContextForLevel(kubernetes.PSARestricted)

err := kubernetes.ValidateContainerPSA(container, kubernetes.PSARestricted)
violations := kubernetes.ValidatePodSpecPSA(podSpec, kubernetes.PSARestricted)
```

### ResourceRequirements

```go
reqs := kubernetes.CreateResourceRequirements()
kubernetes.SetResourceRequest(reqs, corev1.ResourceCPU, resource.MustParse("100m"))
kubernetes.SetResourceLimit(reqs, corev1.ResourceMemory, resource.MustParse("512Mi"))
```

Two helpers cover every resource name; there is no `SetResourceRequestCPU` or
`SetResourceLimitMemory`. Both take a parsed `resource.Quantity`, so the parse —
and any error it can raise — belongs to the caller: `resource.MustParse` for a
literal, `resource.ParseQuantity` when the text comes from configuration.

## Related Packages

- [fluxcd](/api-reference/fluxcd-builders/) - FluxCD resource constructors
- [prometheus](/api-reference/prometheus-builders/) - Prometheus Operator CRD builders
- [errors](/api-reference/errors/) - Structured error types used for nil-check sentinels
- [Builder Contract Migration](/concepts/builder-contract-release-1/) - removed constructor defaults and changed signatures
