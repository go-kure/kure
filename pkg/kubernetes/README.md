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
constructor. A struct literal is the idiom, and the hand-written constructors that used
to wrap one are gone: build the value directly, as
`&corev1.Container{Name: "app", Image: "nginx"}`.

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
caller sees, so it admits nothing. `pkg/kubernetes/testdata/admission_exclusions.txt` lists the helpers
tolerated until the prune work item of the epic deletes them, or rewrites the
class-shaped ones that only fail §4 (an `error` return) as void helpers; entries only ever leave,
and a stale entry fails the test.

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
of the contract.

## 6. Names

No rename wave. A surviving function keeps its name unless the name is wrong.

## 7. Consumers are never blocked

If a caller needs a kure change to reach a field, the contract is broken. Sugar is
added on demand, by the caller's PR, with its test and golden file. There is no
completeness claim and no coverage oracle.

## 8. Feature-gated and deprecated fields

Ordinary fields. kure cannot know a target cluster's gates, so withholding a field
would be a policy judgement inside a pure library. Maturity is a label, never
enforced; it arrives with the generated kinds/scope/maturity tables of the later
work item in the builder-contract epic (the current kind registry records scope
only).

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

err := kubernetes.AddDeploymentContainer(dep, &corev1.Container{Name: "app", Image: "nginx:1.25"})
kubernetes.SetDeploymentReplicas(dep, 3)
kubernetes.AddDeploymentToleration(dep, &corev1.Toleration{Key: "dedicated", Value: "web"})
```

### CronJob

```go
cj := kubernetes.CreateCronJob("my-job", "default")
cj.Spec.Schedule = "*/5 * * * *"
cj.Spec.JobTemplate.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever

err := kubernetes.AddCronJobContainer(cj, &corev1.Container{Name: "worker", Image: "busybox:1.36"})
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

### Namespace and Pod Security Admission

```go
ns := kubernetes.CreateNamespace("my-app")
kubernetes.AddLabel(ns, "env", "prod")

// enforce, warn, audit; "" skips a mode, version "" omits the version labels
kubernetes.SetNamespacePSALabels(ns, kubernetes.PSARestricted, kubernetes.PSARestricted, kubernetes.PSARestricted, "v1.28")
```

### ConfigMap

```go
cm := kubernetes.CreateConfigMap("my-config", "default")
kubernetes.AddConfigMapData(cm, "key", "value")
kubernetes.AddConfigMapBinaryData(cm, "cert", certBytes)
kubernetes.SetConfigMapImmutable(cm, true)
```

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
kubernetes.SetResourceRequestCPU(reqs, "100m")
kubernetes.SetResourceLimitMemory(reqs, "512Mi")
```

## Related Packages

- [fluxcd](/api-reference/fluxcd-builders/) - FluxCD resource constructors
- [prometheus](/api-reference/prometheus-builders/) - Prometheus Operator CRD builders
- [errors](/api-reference/errors/) - Structured error types used for nil-check sentinels
- [Builder Contract Migration](/concepts/builder-contract-release-1/) - removed constructor defaults and changed signatures
