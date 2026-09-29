# Kubernetes Builders - The Builder Contract

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes)

The `kubernetes` package provides the shared scheme, the generic constructor and the
admissible sugar helpers for building Kubernetes objects. This page is the normative
text of the builder contract (ADR-038, "thin core + admissible sugar"): every package
under `pkg/kubernetes/...` follows it, and the tests described below enforce it.

## Import

<!-- doc-example:excerpt the import line alone, which every example on this page uses -->
```go
import "github.com/go-kure/kure/pkg/kubernetes"
```

Every other Go block on this page is the body of an `Example` function in `example_test.go`, which
`go test` runs: it imports this package, the upstream APIs under their usual aliases (`appsv1`,
`batchv1`, `corev1`, `netv1`, `metav1`), `k8s.io/apimachinery/pkg/api/resource`,
`k8s.io/apimachinery/pkg/runtime/schema`, `k8s.io/apimachinery/pkg/util/intstr`,
`k8s.io/utils/ptr`, and `fmt` for the line that prints what the example built.

## 1. Canonical path

For every registered kind the upstream Go struct is the construction API:

<!-- doc-example: pkg/kubernetes ExampleCreateDeployment -->
```go
d := kubernetes.CreateDeployment("web", "default")
d.Spec.Replicas = ptr.To[int32](3)
d.Spec.Template.Spec.ServiceAccountName = "web"
fmt.Println(d.Kind, *d.Spec.Replicas, d.Spec.Template.Spec.ServiceAccountName)
```
<!-- doc-example:end -->

kure does not provide, and its docs do not suggest, a kure function for plain field
access: a helper whose body assigns one argument to one value-typed field is that
assignment written twice, and §3's classifier rejects it.

A whole-spec setter is that same shape one level up, and it is admissible on exactly
the same condition — class (b) is decided on the assigned field, not on how deep it
sits. `Spec` is `*api.Rule` on both Cilium policy kinds, so
`SetCiliumNetworkPolicySpec` and `SetCiliumClusterwideNetworkPolicySpec` are class (b)
and ship; a whole-spec setter for a value-typed `Spec` would not be admitted, and none
exists. These two are the only whole-spec setters in the tree — a value-typed `Spec` is
the caller's own `obj.Spec = spec`.

There is no second construction path either. The `Kind(&Config)` layer — a function per
kind taking a kure-invented `Config` struct and translating it into the upstream spec —
was retired by release 2 of the contract, together with the sealed-interface sum types
behind it; the [release 2 migration notes](/concepts/builder-contract-release-2/) map
every removed field to the upstream field that replaces it. §3 states the rule that keeps
it out.

## 2. Constructors

`Create[T any, PT interface{ *T; client.Object }](name, namespace string) PT` allocates
`T`, sets `TypeMeta` from the registered scheme (the same lookup
`GetGroupVersionKind` uses), sets `metadata.name` and `metadata.namespace`, and
nothing else. The pointer type is inferred from the one type argument:

<!-- doc-example: pkg/kubernetes ExampleCreate -->
```go
d := kubernetes.Create[appsv1.Deployment]("web", "default")
ns := kubernetes.Create[corev1.Namespace]("platform", "") // cluster-scoped: pass ""
fmt.Println(d.APIVersion, d.Kind, ns.APIVersion, ns.Kind)
```
<!-- doc-example:end -->

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

- **(a)** appends one value to a slice field, or inserts one key into a map field, as
  `F = append(F, v)` or `F[k] = v`: alone in its statement, once per helper, with `F`
  spelled from a pointer parameter through selectors, indexes and dereferences, and
  that parameter never reassigned nor its address taken. Anything else that appends or
  inserts is refused: an append or insert into a local, a temporary, an alias or an
  element of a map or slice parameter; `F` spelled with `&` or a call; a nested,
  multi-value or spread append, or one extending another field; an append that is not
  the whole value of an assignment; `++`, `op=` or a range clause into a map element; a
  tuple assignment; a second append or insert; and an index key or append base the
  comparison cannot equate (a call, conversion or slice expression), even when spelled
  alike;
- **(b)** assigns to a pointer-typed field (`x.F = &v`; initialising a nil pointer
  intermediate before assigning through it is the same thing);
- **(c)** constructs an upstream struct literal setting two or more fields, or a
  nested literal. A slice or map literal is not class (c): it replaces a collection
  rather than composing a value.

Every admitted operation carries a value the caller supplied. An append of a constant,
a map insert of a constant, a struct literal built entirely from constants, or a pointer
allocated (`new(T)`) and never written through, all set a value the caller never named
and are inadmissible (§4). The zero-value init a helper guards a nil map, slice or
pointer field with (`if o.Labels == nil { o.Labels = map[string]string{} }`) is not such
a value. An empty value (an empty literal or `make`, or `new(T)`) written into any
other field, an interface, a struct or a channel, is one the caller did not supply and
is refused as a default, including when it is written through a pointer the helper
initialised. A `make` of a slice with a length other than a constant 0 may fill it with
elements the caller did not supply, so it is conservatively not that init; a constant
capacity or map size hint is (the grammar below, N1).
An increment, decrement or compound assignment of a field (`o.Spec.Count++`,
`o.Spec.Count += n`) writes a value computed from what the field held rather than the
caller's value, and is inadmissible for the same reason; so is the same write spelled
out (`o.Labels[k] = o.Labels[k] + v`), though extending a slice with
`o.Spec.Items = append(o.Spec.Items, s)` is not. The same holds for a map or
slice element reached through a local or a range variable, however it was made
(`labels := o.Labels; labels[k] += v`, `for _, items := range o.Spec.Groups`): a
helper body declares no local but the marshalled form of an argument, and has no
loop (the grammar below, S3 and S1).

A body that is a single assignment to a non-pointer field is inadmissible regardless
of path depth: writing `Spec.Template.Spec.ServiceAccountName` is still one assignment,
and two such assignments in one body are two forwarders, not a composite. A bare
assignment next to an admitted operation is inadmissible when its value is not an
argument: an append that also sets a scalar to a literal or a computed value touches a
field the caller did not name (§4). Forwarding a second argument alongside
(`SetHPAMinMaxReplicas(hpa, 2, 10)`) leaves the class alone, and so does a pointer
write beside the one append or insert: each write is checked on its own by the grammar
below, and the class is that of the append. A helper
that returns anything, an `error` included, is inadmissible whatever its body does
(§4 allows no error return: a nil receiver panics). A nil receiver guard admits
nothing on its own. A body that assigns `nil` to a field that can hold it, or as a
keyed value inside a literal it writes as a field or inserted value (the literal
`nil`, a typed conversion of it, or a local known to be nil), is inadmissible whatever
else it does, because it clears a field the caller did not name (§4). `nil` is no
argument (V1), and the name is read as spelled: a parameter named `nil` is refused by
that name. A helper that must replace one member of a one-of takes the whole one-of as
its argument instead.

A helper reaches the object it writes through one pointer parameter, which the body
never reassigns and never takes the address of, and every write in every class is
spelled from it (the grammar below, P1). A map, slice or interface parameter is not
such a root; the metadata helpers of §5 are admitted by name, not by class. A struct
taken by value is a copy, so a helper written that way changes nothing the caller can
see and is inadmissible; a read-modify-write through such a copy's map, slice or
pointer field still reaches the caller, and is refused as above.

`TestAdmission_SugarHelpersAreClassAdmissible` classifies every helper with `go/ast`
and type information (`pkg/kubernetes/internal/admission`) and fails naming any helper
outside (a)-(c). It is syntactic and deliberately conservative. An admitted write
inside an `if` (either branch), a loop, a `switch` or a `select`, or a range clause
that assigns to the field or to such an element (`for _, o.Spec.Ref = range refs`),
runs only on some paths and is inadmissible, as is a `goto`, which can jump over the write; so the
optional-value guard `if name != "" { ... }` that §4 forbids is detected. The single
exception is the nil-init guard `if P == nil { P = <zero value> }` with nothing else in
it: no init statement, no `else`, one statement zero-initialising the map, slice or
pointer path it tests. A set-if-unset (`if o.Spec.Ref == nil { o.Spec.Ref = ref }`) is
not that guard and is refused, and neither is a guard filling a nil interface field with
an empty value, nor one around a slice `make` with a non-zero length. No class admits an
alias: a conditionally created one
(`obj := &Obj{}; if ok { obj = o }; obj.Spec.Ref = ref`) is refused at its first
statement, which is not the one local the grammar below has a form for (S3). A helper
containing a function literal, or declaring type parameters, is inadmissible.
`pkg/kubernetes/testdata/admission_exclusions.txt` listed the
helpers tolerated while the prune work item of the epic ran; that file is now empty and stays
empty. Entries only ever leave, and a stale entry fails the test.

### The grammar of a helper body

A helper the classes admit must also be written in a closed grammar: every top-level
statement, every path it writes and every value it writes has one of the forms below,
and anything else refuses the helper. The grammar only refuses. Its verdict counts only
for a body the rules above admit, so the class of an admitted helper and the reason of
every refusal above are unchanged, and a refusal it makes names its rule:
`<what was found> (grammar S3, purity §4)`. The four helpers admitted by name (§5) are
not read.

A body is nil guards and marshalled locals first, then nil-init guards and field
writes. *The object* is the parameter the first write in the body is spelled from.
Parentheses change nothing, and names are resolved by what they declare: a local or a
method called `panic` or `Marshal` is not the builtin or the function. Where a rule
reads as written instead, it says so: N1 and P6 compare two paths, and V1 reads the
name `nil`.

| ID | Statements, at the top level of the body |
|---|---|
| S1 | A statement is an `if` or an assignment. A call statement, `var`, `defer`, `go`, a send, a loop, a `switch`, a `select`, a bare block, a label, `++`, `--` and an empty statement are refused. |
| S2 | Nil guards that panic, locals and the guards on their errors come before the first nil-init guard or field write. |
| S3 | A `:=` is exactly `v, err := json.Marshal(P)`: two new names, neither blank, and `P` a parameter other than the object. |
| S4 | The statement directly after a local is the guard on its error, `if err != nil { panic(M) }`, and that guard appears nowhere else. |
| S5 | A panic takes one argument: a string constant, or in the guard on an error `fmt.Sprintf(c, err)` with `c` a string constant. |
| S6 | A nil guard that panics, `if P == nil { panic(c) }`, tests the object, or a parameter the body writes as `*P`. On any other parameter it makes `nil` unexpressible, which is validation (§4). |
| S7 | Any other assignment has one target, one value and the token `=`: no tuple, no `op=`. |
| S8 | An `if` has no init statement, no `else` and a one-statement body, and is one of three guards: the nil guard that panics, the guard on an error, the nil-init guard. `X == nil` and `nil == X` are the same test. |
| S9 | The body holds no function literal, wherever it stands. No rule reads the body of a closure, so it could hide any write; a constant that holds one, `len([1]func(){func() {}})`, is refused too. |
| S10 | At most one statement appends or inserts. Class (a) is a single append or insert; a second one, into the same field or another, is refused at the statement that makes it. |
| S11 | Every call of `append` is the whole value of an assignment. One anywhere else is no class (a) statement, whatever it extends: in a `var`, as an argument, sliced, nested in another `append`, or inside a constant, `unsafe.Sizeof(append(…))`. |

| ID | Paths: the target of a field write or of a nil-init, and the slice an append extends |
|---|---|
| P1 | A path is spelled from the object through field selectors, indexes and dereferences, and the object is a pointer parameter the body never reassigns and never takes the address of. |
| P2 | A path names a field: `*o = x` is refused. |
| P3 | A key or index is a parameter other than the object, or a constant. |
| P4 | No location is written twice, and none is written after a write to a location above or below it on the same path. Every pointer followed counts as a step of the path, written or not; a promoted field counts as its full spelling; any two keys may name the same element. The one exception is a nil-init guard ahead of the write it initialises, on the same path with the same keys: the same parameter, or constants of one type and one value, however each is spelled. |
| P5 | A nil-init guard initialises a field, not an element of a map or slice. |
| P6 | Every nil-init is written through: by a later write below it, or by the append that extends it. A pointer nil-init is written through by a later write spelled `<its path>.…` or `*<its path>`, as written, and any such write counts: `(*o.Spec.Ref).Name`, a promoted field of an embedded pointer and an index of a pointer to an array are not that spelling, and neither is anything below a parenthesised nil-init, `(o.Spec.Ref) = &Ref{}`. |
| P7 | Behind a nil-init guard a write meets only what that guard assigned: one pointer, one map or one empty slice. As its first step there it may follow that pointer or index that map. Past that step it follows no pointer and indexes no map, because what lies behind holds its zero value, and it indexes no slice at all: `o.Spec.Items[i] = v` behind `o.Spec.Items = []string{}` has no element to write. An append needs no slice, and an array has its elements. The guard is the nearest one ahead of the write on the same path, and a nil-init guard is itself such a write behind the guard ahead of it. The rule reads the path, not the value, so the map behind `&map[K]V{}` is not indexed either. |
| N1 | A nil-init guard tests the path it assigns, spelled alike outer parentheses aside and with the same keys as under P4: `if (*o).Spec.Ref == nil { o.Spec.Ref = &Ref{} }` is refused. The path is a map, slice or pointer; an empty value in an interface or a channel is a default. The guard assigns `T{}`, `&T{}`, `new(T)` or a `make` with constant sizes. A slice `make` has length 0: `make([]T, 3)` allocates three elements the caller did not supply. |

| ID | Values: the element appended, the value inserted, the value of a field |
|---|---|
| V1 | A value is an argument passed whole, a struct literal, or the address of one. An argument is a parameter other than the object, written `P`, `&P` or `*P`, or the first name of a local; the error of a local is never a value. A struct literal, keyed or positional, holds arguments, constants, struct literals and their addresses. The name `nil`, as spelled, is not the value of a field that can hold nil, nor a keyed element, at any depth, of a literal written as a field or inserted value: a parameter named `nil` is refused by that name. An appended element is not read for it. |
| V2 | A struct literal is the element appended or inserted, `&T{...}` written to a pointer-typed field, or the value of the helper's only field write (nil-init guards not counted). A struct replaced beside another write loses fields the caller did not name. |
| V3 | A constant in a struct literal is a named constant of a defined type (`corev1.ProtocolTCP`). An inline literal (`"Deployment"`, `true`, `3`) or a constant of a basic type is a default. |
| V3b | Every struct literal, at every depth, carries an argument. |
| V4 | Each argument occurs once over all written values and marshalled locals: `&n` written to two fields gives them one pointer, and so does `T{A: p, B: p}`. Keys and guards do not count. |
| V5 | Every parameter has a place: it is the object, a key or index of a path, or an argument written or marshalled. A parameter the body leaves out, a blank one included, is a value the caller passed and the helper dropped. |
| V7 | No written value reads its target, the value of a nil-init included, wherever the read stands: in a constant (`unsafe.Sizeof(o.Spec.Items)`), in the size of a `make`, or in the type of a literal. A value computed from what the field held is not the caller's. The slice an append extends is not such a read. |

A call therefore appears in six positions only: `append` as the whole value of a class
(a) statement, `make` and `new` in a nil-init guard, `json.Marshal` in a local,
`fmt.Sprintf` in the message of the guard on an error, and `panic` as the body of a
guard. A constant is a constant however it is spelled (`int32(0)`, `len("ab")`), and
holds no function literal (S9), no `append` (S11) and no read of the target (V7); any
other call, the conversion of an argument included, refuses the helper.

S9, then S11, are read off the whole body first. Then the statements are read in
source order, and the first rule a statement breaks names the refusal: S1; for an
`if` S8, S2, S5, S4, and for a nil-init guard P1, P2, P3, P5, N1, V7; for a `:=` S3,
S2, S4; for any other assignment S7, P1, P2, P3, S10, V1, V3, V3b, V2, V7. S6, P4,
P6, P7, V4 and V5 are read off the whole body after its last statement, in that
order.

What the grammar does not see: an alias the caller made (a `*P` or `P` that points into
the object, or two pointer fields of the object sharing one struct); the caller's code
that `json.Marshal` and `%v` run; whether a constant key or index is the one the caller
would have chosen; and whether the helper's name matches the field it writes. Those
stay with review and with the helper's golden test.

A second rule covers what a helper takes, not what it writes: **no exported function under
`pkg/kubernetes/...` takes a kure-defined type where an upstream spec type exists.** A
kure-declared struct or interface as a parameter is a second vocabulary for an object the
upstream struct already describes — the shape of the retired `Kind(&Config)` layer — and
`TestAdmission_NoOwnParameterTypes` fails on one, with no exclusion list. It looks
through every layer a type can hide behind: pointer, slice, array, map and channel
layers, a callback's parameters and results, an alias or a named container declared
under `pkg/kubernetes` (`type Config = *struct{...}`, `type List []struct{...}`), and a
struct or method-bearing interface literal written in the signature itself, and the type
arguments of an instantiated generic or generic alias (`up.Wrapper[Config]`), and the
constraint of a type parameter, embedded types and union terms included (`[T Variant]`,
`[T interface{ Config | *Config }]`). It does not
enter an upstream named type otherwise. A named scalar with no upstream counterpart (`PSALevel`, a
string enum) and an interface with no methods (`any`) are not spec types and pass.
Like the sugar classifier, the check guards against the layer drifting back in through
ordinary authorship; a signature written to evade it is a review matter, not a gap in
the test.

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

<!-- doc-example: pkg/kubernetes ExampleAddLabel -->
```go
obj := kubernetes.CreateDeployment("web", "default")

kubernetes.SetLabels(obj, map[string]string{"app": "web"})
kubernetes.AddLabel(obj, "tier", "frontend") // initialises a nil map
kubernetes.SetAnnotations(obj, map[string]string{"owner": "platform"})
kubernetes.AddAnnotation(obj, "note", "rotated 2026-09")
fmt.Println(obj.Labels["app"], obj.Labels["tier"], obj.Annotations["note"])
```
<!-- doc-example:end -->

<!-- doc-api-refs:ignore-start the paragraph names removed helpers to say they are removed -->

These four are admitted by name; per-kind label and annotation helpers are not part
of the contract, and none remain — `AddNamespaceLabel`, `AddClusterAnnotation`,
`SetConfigMapLabels` and the twenty-nine others like them were removed, since the
four above already reach every kind through `metav1.Object`. Two helpers keep a
metadata-shaped name while writing something else: cilium's `Set*PolicyLabels`
write the policy's `spec.labels`, and prometheus's `Add*TargetLabel` appends to a
scrape spec's target-label list. Neither is ObjectMeta.

<!-- doc-api-refs:ignore-end -->

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

The prose scan reads prose only — marker lines are removed before it runs. A
marker's words describe that marker's own subject, not the field's maturity, and
the declarative-validation markers `k8s.io/api` introduced in v0.37.0 make the
difference load-bearing: they are spelled
`+k8s:alpha(since: "1.37")=+k8s:required`, which says the *required rule* is alpha
since 1.37, on fields that have been GA for years. Reading
those as prose reported 45 long-stable built-in fields as alpha or beta —
`StatefulSetSpec.selector`, `ClusterRole.rules`, `Secret.type` among them — and
contradicted a genuinely stable field in the same type. A field's maturity is claimed
in its documentation or by a `+featureGate`, and is never inferred from another
marker.

The same rule decides what counts inside the prose. A mention of "alpha" or "beta"
is read as a claim only when the sentence attributes that level to the field being
documented — "This is an alpha field", "This field is beta-level", a leading
"(Alpha)" tag, "Alpha, gated by …". The words also appear in prose that says
nothing about the documented field's maturity, and counting every occurrence
published five such fields as alpha:

- `CustomResourceDefinitionSpec.versions`, whose prose explains that a version name
  ends in "optionally the string `alpha` or `beta` and another number" and that
  names sort "by GA > beta > alpha" — version-name tokens.
- `FileKeySelector.key`, a required field with no gate marker, documenting a size
  limit that applies "During Alpha stage of the EnvFiles feature gate" — the gate's
  stage, and a size limit.
- `CommonPrometheusFields.scrapeConfigSelector` and its namespace counterpart,
  which note that the *ScrapeConfig* custom resource definition "is currently at
  Alpha level" — another resource's level.
- `GRPCRouteRule.filters`, which says the rule "can change in the future based on
  feedback during the alpha stage" — a statement about future changes.

The same shape reached the table once through `HorizontalPodAutoscalerSpec.minReplicas`,
GA as long as `autoscaling/v2` has been, whose prose says a value of 0 is allowed
"if the alpha feature gate HPAScaleToZero is enabled": the gate's level, not the
field's. A field whose own maturity is alpha says so in a sentence of its own, and
that still counts — including when the same doc comment also names an alpha gate. A
construction the list does not cover reports stable, which under-reports rather than
publishing a wrong level; the gate list is the signal to act on either way.

Status types are not entered. A kind's status is reported by the cluster and never
constructed by a caller, so a gate there says nothing about whether a manifest kure
builds applies as written — and that is where most of the markers are.

Against the current pins, the walk finds 126 maturity-carrying construction-side
fields, of which 41 require a feature gate (40 in `k8s.io/api`, one in
`k8s.io/apiextensions-apiserver`); 24 are documented alpha, 14 beta and 66
deprecated, the remaining 22 are gated without a documented stability claim, and 92
distinct status types are skipped. No CRD module kure pins uses `+featureGate` at
all. These numbers move with the pins and are not asserted by any test; the pins
themselves are not restated here — every generated row carries the module and
version it was read from, and the exact build versions live in the generated
[Compatibility Matrix](/api-reference/compatibility/). What the
tests assert is that every reported field exists in the pinned struct, that a set of
long-GA built-in fields is reported stable, and that the five fields named above —
the ones whose prose mentions the words about something else — claim nothing.

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
  Each module directory is walked once per process and its index kept: the walk
  decodes every manifest the module ships, and a module-cache directory never
  changes under a fixed version. A walk that fails is not kept.
- `internal/kinds` resolves a scope per registered kind; `internal/maturity` walks
  the type graph for the field table.

Every resolved scope records which of three sources answered — `marker`, `builtin`
or `crd`, surfaced on `KindInfo.ScopeSource` — so a wrong scope can be traced to the
thing that claimed it. Against the current pins that is 65 from the kind's own
marker, 44 from the built-in table and 19 from a shipped CRD. The source is part of
the public answer and not only a debugging aid: `builtin` is what makes a kind a
built-in, so a caller asking specifically about built-ins reads it rather than
keeping a list.

`kinds.Registered()` returns each kind with that resolution already applied, and it
is the only place a scope is stated: the 128-entry hand-seeded pair of sets this
package used to carry is gone, and so are the two hand-kept maps in `pkg/manifest`,
which now reads scope from the generated table — restricting to `ScopeSourceBuiltin`
where it asks about built-ins specifically. The cluster-scoped half of
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
| `zz_generated_tables.go` | the two Go tables, read through `Kinds()` and `FieldMaturities()` |
| `docs/api-tables.json` | the machine-readable copy, read as a diff on a bump PR |
| `docs/api-tables.md` | the site page (mounted via `site/docs-map.yaml`) |

`tables.go` holds the hand-written types and lookups over them — `KindFor`,
`KindForAnyVersion`, `KindByGroupKind`, `IsNamespaced`, `MaturityForType`,
`GatedFields`. Every value is a plain string or bool: the generator writes into this
package, so a table that referenced an upstream type could not be regenerated after
an API bump removed it. Each kind row names the module and version it was read from,
and `ScopeSource` names what stated the scope — `ScopeSourceMarker`,
`ScopeSourceShippedCRD` or `ScopeSourceBuiltin` — so any row is traceable to a pin,
and a caller can tell a built-in kind from a custom resource without keeping a list
of its own. `pkg/manifest`'s `IsNamespacedBuiltinKind` is exactly that: a scope
answer restricted to `ScopeSourceBuiltin`.

The tables themselves are unexported, and `Kinds()` and `FieldMaturities()` hand back
copies — deep ones, since a row's `Gates` slice would otherwise stay shared with the
table. The lookups read the tables on every call, `pkg/manifest`'s scope resolution
among them, so a consumer that edited a returned row in place to relabel a kind for
its own output would change the answer every later caller in the process gets, at a
distance and with nothing pointing back to the edit.

The kind lookups answer deliberately different questions, and the difference is the
version:

- `KindFor(apiVersion, kind)` matches the version as well as the group. `GoType`,
  `ImportPath` and `ModuleVersion` describe one version's Go type, so a group/kind
  registered at some other version is not an answer — it reads as unregistered,
  which is what it is.
- `KindForAnyVersion(apiVersion, kind)` matches the group only, and is the row
  behind the version-insensitive answers: scope, and what declared it. The row it
  returns may describe a different version than the one asked about, so anything
  version-specific on it is not an answer to the question that was asked.
- `IsNamespaced(apiVersion, kind)` ignores the version. Scope is a property of the
  resource and is the same across the versions of one group/kind, so a manifest
  written against a version kure does not register is still answered rather than
  returned as unknown. `pkg/manifest`'s `Scope` depends on that: an object at
  `autoscaling/v1` must not fall through to `ScopeUnknown` because the scheme
  happens to register `autoscaling/v2`.

Regenerate with `scripts/gen-builders.sh generate`; CI's `validate` job runs
`scripts/gen-builders.sh check`, and Renovate runs `generate` in its
`postUpgradeTasks` so a bump PR arrives with the tables already updated. Do not edit
the artifacts by hand.

Two consequences of that wiring:

- **A pin bump that changes a table changes a file in a doc-gated package.** When a
  row's only change is its `ModuleVersion` provenance field — pure version churn — the
  doc-gate exempts it automatically, needing neither a paired doc edit nor the
  maintainer `docs-skip` label. A change that adds, removes, or re-scopes a kind is not
  version churn and still trips the gate. See `docs/dependency-updates.md`.
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

<!-- doc-example: pkg/kubernetes ExampleGetGroupVersionKind -->
```go
myDeployment := kubernetes.CreateDeployment("web", "default")
allowedGVKs := []schema.GroupVersionKind{appsv1.SchemeGroupVersion.WithKind("Deployment")}

// Lazily registers every supported API group (core K8s, FluxCD, cert-manager, ...)
err := kubernetes.RegisterSchemes()
if err != nil {
    panic(err)
}

// Resolve the GVK of any registered runtime.Object
gvk, err := kubernetes.GetGroupVersionKind(myDeployment)
if err != nil {
    panic(err)
}

// Check if a GVK is in an allow list
ok := kubernetes.IsGVKAllowed(gvk, allowedGVKs)
fmt.Println(gvk, ok)
```
<!-- doc-example:end -->

## Examples

The helpers below are the surviving sugar for the core kinds. Anything not shown is
a field write on the upstream struct.

### Deployment

<!-- doc-example: pkg/kubernetes ExampleSetDeploymentReplicas -->
```go
dep := kubernetes.CreateDeployment("my-app", "default")
kubernetes.AddLabel(dep, "app", "my-app")
dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"app": "my-app"}}
dep.Spec.Template.Labels = map[string]string{"app": "my-app"}

podSpec := &dep.Spec.Template.Spec
kubernetes.AddPodSpecContainer(podSpec, &corev1.Container{Name: "app", Image: "nginx:1.25"})
kubernetes.AddPodSpecToleration(podSpec, &corev1.Toleration{Key: "dedicated", Value: "web"})
kubernetes.SetDeploymentReplicas(dep, 3)
fmt.Println(podSpec.Containers[0].Image, podSpec.Tolerations[0].Key, *dep.Spec.Replicas)
```
<!-- doc-example:end -->

There is no `AddDeploymentContainer`. <!-- doc-api-refs:ignore names a removed helper to say it is gone --> A workload kind's pod template is a
`corev1.PodSpec`, so the `PodSpec` helpers serve every kind — pass
`&dep.Spec.Template.Spec` (a Job uses `&job.Spec.Template.Spec`; a CronJob
nests one level deeper: `&cj.Spec.JobTemplate.Spec.Template.Spec`). `ServiceAccountName` and
`NodeSelector` are plain fields on that struct and are assigned directly.

### Job

<!-- doc-example: pkg/kubernetes ExampleCreateJob -->
```go
job := kubernetes.CreateJob("migrate", "default")
job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever

kubernetes.AddPodSpecContainer(&job.Spec.Template.Spec,
    &corev1.Container{Name: "migrate", Image: "busybox:1.36"})
fmt.Println(job.Spec.Template.Spec.RestartPolicy, job.Spec.Template.Spec.Containers[0].Name)
```
<!-- doc-example:end -->

`CreateJob` writes identity only, so `restartPolicy` is yours to set. Leave it
out and the API server defaults it to `Always`, which it then rejects for a Job
pod (`restartPolicy: Unsupported value: "Always"`); set `Never` or `OnFailure`.
The CronJob example below sets it for the same reason.

### CronJob

<!-- doc-example: pkg/kubernetes ExampleCreateCronJob -->
```go
cj := kubernetes.CreateCronJob("my-job", "default")
cj.Spec.Schedule = "*/5 * * * *"
cj.Spec.JobTemplate.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever

kubernetes.AddPodSpecContainer(&cj.Spec.JobTemplate.Spec.Template.Spec,
    &corev1.Container{Name: "worker", Image: "busybox:1.36"})
cj.Spec.ConcurrencyPolicy = batchv1.ForbidConcurrent
fmt.Println(cj.Spec.Schedule, cj.Spec.ConcurrencyPolicy)
```
<!-- doc-example:end -->

### Service

<!-- doc-example: pkg/kubernetes ExampleAddServicePort -->
```go
svc := kubernetes.CreateService("my-app", "default")
svc.Spec.Selector = map[string]string{"app": "my-app"}
kubernetes.AddServicePort(svc, corev1.ServicePort{Name: "http", Port: 80, TargetPort: intstr.FromInt32(8080)})
svc.Spec.Type = corev1.ServiceTypeLoadBalancer
kubernetes.AddAnnotation(svc, "external-dns.alpha.kubernetes.io/hostname", "app.example.com")
fmt.Println(svc.Spec.Ports[0].Port, svc.Spec.Ports[0].TargetPort.IntValue(), svc.Spec.Type)
```
<!-- doc-example:end -->

### Ingress

<!-- doc-example: pkg/kubernetes ExampleAddIngressRule -->
```go
ing := kubernetes.CreateIngress("my-app", "default")
kubernetes.SetIngressClassName(ing, "nginx")

rule := &netv1.IngressRule{Host: "app.example.com"}
pt := netv1.PathTypePrefix
path := kubernetes.CreateIngressPath("/", &pt, "my-app", "http")
kubernetes.AddIngressRulePath(rule, path)
kubernetes.AddIngressRule(ing, rule)
kubernetes.AddIngressTLS(ing, netv1.IngressTLS{Hosts: []string{"app.example.com"}, SecretName: "my-app-tls"})
fmt.Println(*ing.Spec.IngressClassName, ing.Spec.Rules[0].Host, ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name)
```
<!-- doc-example:end -->

### HPA and PDB

<!-- doc-example: pkg/kubernetes ExampleAddHPACPUMetric -->
```go
hpa := kubernetes.CreateHorizontalPodAutoscaler("my-app", "default")
kubernetes.SetHPAScaleTargetRef(hpa, "apps/v1", "Deployment", "my-app")
kubernetes.SetHPAMinMaxReplicas(hpa, 2, 10)
kubernetes.AddHPACPUMetric(hpa, 80)

pdb := kubernetes.CreatePodDisruptionBudget("my-app", "default")
kubernetes.SetPDBMinAvailable(pdb, intstr.FromInt32(2))
kubernetes.SetPDBSelector(pdb, &metav1.LabelSelector{MatchLabels: map[string]string{"app": "my-app"}})
fmt.Println(*hpa.Spec.MinReplicas, hpa.Spec.MaxReplicas, *hpa.Spec.Metrics[0].Resource.Target.AverageUtilization, pdb.Spec.MinAvailable.IntValue())
```
<!-- doc-example:end -->

`MinAvailable` and `MaxUnavailable` are mutually exclusive upstream, and each setter
writes only the field it names — a helper does not clear a field the caller did not
mention. Switching from one to the other is two statements:

<!-- doc-example: pkg/kubernetes ExampleSetPDBMaxUnavailable -->
```go
pdb := kubernetes.CreatePodDisruptionBudget("my-app", "default")
kubernetes.SetPDBMinAvailable(pdb, intstr.FromInt32(2))

pdb.Spec.MinAvailable = nil
kubernetes.SetPDBMaxUnavailable(pdb, intstr.FromString("25%"))
fmt.Println(pdb.Spec.MinAvailable == nil, pdb.Spec.MaxUnavailable.String())
```
<!-- doc-example:end -->

### Namespace and Pod Security Admission

<!-- doc-example: pkg/kubernetes ExamplePSALabels -->
```go
ns := kubernetes.CreateNamespace("my-app")
kubernetes.AddLabel(ns, "env", "prod")

// enforce, warn, audit; "" skips a mode, version "" omits the version labels
for k, v := range kubernetes.PSALabels(kubernetes.PSARestricted, kubernetes.PSARestricted, kubernetes.PSARestricted, "v1.28") {
    kubernetes.AddLabel(ns, k, v)
}
fmt.Println(len(ns.Labels), ns.Labels["pod-security.kubernetes.io/enforce"])
```
<!-- doc-example:end -->

`PSALabels` returns the label map and writes nothing. One argument expanding into six
labels is not something a `Set<Field>` helper may hide, so the expansion is a value
helper and the write stays with `AddLabel`.

### ConfigMap

<!-- doc-example: pkg/kubernetes ExampleAddConfigMapData -->
```go
certBytes := []byte("-----BEGIN CERTIFICATE-----")
defaults := map[string]string{"log-level": "info"}

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
fmt.Println(len(cm.Data), cm.Data["log-level"], *cm.Immutable)
```
<!-- doc-example:end -->

`SetConfigMapData`, `SetConfigMapBinaryData`, `AddConfigMapDataMap` and <!-- doc-api-refs:ignore names removed helpers to say they are gone -->
`AddConfigMapBinaryDataMap` are gone: <!-- doc-api-refs:ignore same sentence, second line -->
the first two were bare field assignments, and
a bulk merge is not one of the admitted sugar classes in any spelling — neither
`maps.Copy` nor an explicit loop classifies, because the class is a *single* insert
whose value comes from the caller.

### PSA security contexts

<!-- doc-example: pkg/kubernetes ExampleValidatePodSpecPSA -->
```go
container := &corev1.Container{Name: "app", Image: "nginx:1.25", SecurityContext: kubernetes.RestrictedSecurityContext()}
podSpec := &corev1.PodSpec{Containers: []corev1.Container{*container}}

psc, err := kubernetes.PodSecurityContextForLevel(kubernetes.PSARestricted)
if err != nil {
    panic(err)
}
podSpec.SecurityContext = psc

err = kubernetes.ValidateContainerPSA(container, kubernetes.PSARestricted)
fmt.Println(*container.SecurityContext.RunAsNonRoot, err)
err = kubernetes.ValidatePodSpecPSA(podSpec, kubernetes.PSARestricted)
fmt.Println(err)
```
<!-- doc-example:end -->

### ResourceRequirements

<!-- doc-example: pkg/kubernetes ExampleSetResourceRequest -->
```go
reqs := kubernetes.CreateResourceRequirements()
kubernetes.SetResourceRequest(reqs, corev1.ResourceCPU, resource.MustParse("100m"))
kubernetes.SetResourceLimit(reqs, corev1.ResourceMemory, resource.MustParse("512Mi"))
fmt.Println(reqs.Requests.Cpu(), reqs.Limits.Memory())
```
<!-- doc-example:end -->

Two helpers cover every resource name; there is no `SetResourceRequestCPU` or <!-- doc-api-refs:ignore names removed helpers to say they are gone -->
`SetResourceLimitMemory`. <!-- doc-api-refs:ignore same sentence, second line -->
Both take a parsed `resource.Quantity`, so the parse —
and any error it can raise — belongs to the caller: `resource.MustParse` for a
literal, `resource.ParseQuantity` when the text comes from configuration.

## Related Packages

- [fluxcd](/api-reference/fluxcd-builders/) - FluxCD resource constructors
- [prometheus](/api-reference/prometheus-builders/) - Prometheus Operator CRD builders
- [errors](/api-reference/errors/) - Structured error types used for nil-check sentinels
- [Builder Contract Migration](/concepts/builder-contract-release-1/) - removed constructor defaults and changed signatures
- [Builder Contract Migration (release 2)](/concepts/builder-contract-release-2/) - the retired `Kind(&Config)` layer, field by field
