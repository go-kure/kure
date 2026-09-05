# The Builder Contract — Design Notes

*Date: 2026-09-05 | Type: DESIGN | Scope: kure internal*

> **Note**: This document is not part of the kure documentation website. It is an internal
> development note about how the builder contract was decided. The normative contract is
> `pkg/kubernetes/README.md`, published as
> [Kubernetes Builders](https://www.gokure.dev/kure/api-reference/kubernetes-builders/); the
> caller-facing list of what changed is
> [`docs/builder-contract-release-1.md`](../builder-contract-release-1.md).

---

## Decision

ADR-038, "thin core + admissible sugar". The upstream Go struct is kure's construction API.
`pkg/kubernetes` supplies identity and a scheme, not a parallel vocabulary:

- every `Create<Kind>` is generated from the registered scheme and emits `apiVersion`, `kind`,
  `metadata.name` and — for a namespaced kind — `metadata.namespace`, and nothing else;
- a `Set*` / `Add*` helper survives only if the write it performs belongs to one of three classes —
  or is one of the four generic metadata helpers admitted by name, which write through
  `metav1.Object` and so match no write shape — and a `go/ast` test decides membership rather than
  a reviewer;
- everything else is a field assignment the caller writes.

`pkg/stack` is the layer above and keeps its opinions — but as exported identifiers a consumer
can read, compare against and override, never as literals inside a generator.

---

## The problem

kure had three overlapping ways to set the same field, and they disagreed.

**Constructors injected values nobody asked for.** Fifteen root constructors set a label and an
annotation `app: <name>`; others seeded spec values. A manifest built by kure therefore carried
kure's opinions with no way to tell them apart from the caller's, and no way to remove one
without post-processing the object.

**Bare field forwarders outnumbered everything else and could not keep up.** At the commit before
the epic (`17562dc`) `pkg/kubernetes/**` declared 797 exported `Create*`/`Set*`/`Add*` functions.
Most were one assignment behind a longer name. Each new upstream field was a new forwarder, or an
absence a caller reported as a gap — a treadmill whose finish line is the union of every field of
every kind kure registers.

**Some helpers wrote more than their name said.** A mover setter cleared its siblings; an
availability setter cleared the other half of a one-of; a pod-template helper existed once per
workload kind over the same `corev1.PodSpec`. These are the expensive ones: the caller reads the
name, gets the write plus something else, and the something else is invisible in the emitted YAML
until it matters.

The common cause is that the forwarders were an API of their own, competing with the upstream
types they wrapped, with no rule saying which of the two a caller should reach for.

---

## Options considered

### 1. Generate the forwarders

Keep the shape, remove the labour: generate a `Set<Kind><Field>` for every field of every
registered kind from the pinned upstream types.

Rejected. It scales the surface instead of the effort — several thousand functions, each a name a
consumer must learn that the upstream field already has. It also cannot express the interesting
cases: a one-of, a nil-init, an append all need a decision that a field walker does not have. And
it would still not be complete, because a generated setter for a nested field is only correct if
the intermediate is non-nil, which is exactly the case a caller hits first.

### 2. Thin core, no sugar at all

Delete every `Set*`/`Add*`. Constructors emit identity; the caller assigns every field.

Rejected, narrowly, and it remains the fallback if the admission rule ever stops holding. Three
operations are genuinely worse as plain assignments: appending to a slice that may be nil,
initialising a pointer field before writing through it, and building a nested upstream literal
whose shape the caller would otherwise have to look up. Removing those makes the common path
longer without making it clearer.

### 3. Thin core plus admissible sugar — chosen

Keep exactly the three operations above, and define them precisely enough that a test decides
membership:

| Class | Body | Why a plain assignment is worse |
|---|---|---|
| (a) appender | appends to a slice field, or inserts into a map field | the field may be nil; `append` on a nil slice through a struct field is a read-modify-write the caller has to spell out |
| (b) pointer / nil-init | assigns to a pointer-typed field | needs a named local or a helper to take the address of a value |
| (c) named composite | constructs an upstream struct literal with two or more fields, or a nested literal | the caller would have to know the nesting; every value still comes from an argument |

Purity is the other half of the rule: a helper writes exactly the values its arguments carry,
returns nothing, and never clears a field the caller did not name.

The reason this is a contract rather than a guideline is
`TestAdmission_SugarHelpersAreClassAdmissible`. It classifies every exported helper under
`pkg/kubernetes/...` with `go/ast` and type information and fails naming anything outside (a)–(c).
The one thing it admits outside (a)–(c) is a fixed set of four names — `SetLabels`, `AddLabel`,
`SetAnnotations`, `AddAnnotation` — which write through the `metav1.Object` interface rather than
into a field and therefore have no write shape to classify. They are a fourth outcome in the
classifier (`Exempt`), not a fourth class: a set that never grows, which is what makes one metadata
helper set able to serve kinds kure does not register.
`pkg/kubernetes/testdata/admission_exclusions.txt` held the helpers tolerated while the prune ran;
it is empty now and entries only ever leave it. A new helper is admitted by the test or it does
not merge — nobody has to remember the rule.

---

## Consumer impact

`pkg/kubernetes/**` went from 797 builder-shaped exported functions to 490, of which 128 are the
generated constructors — a net 435 fewer hand-written ones, with generated code standing where the
hand-written constructors were.

Every removal has a replacement expression, listed once in
[`docs/builder-contract-release-1.md`](../builder-contract-release-1.md). Almost all of them read
`obj.Spec.Field = value`. The migrations that are not mechanical are the ones worth reading twice:

- **Constructors no longer default.** A caller that relied on the injected `app` label or on a
  seeded spec value now sets it. A silent default is indistinguishable from a deliberate value in
  the emitted YAML, which is why they went rather than being made overrideable.
- **Two constructor signatures changed** — `CreateCronJob` and `CreateIngress` each dropped a
  third parameter.
- **Error-returning setters became void.** A helper that could only fail on a nil receiver was
  returning an error nobody could act on; a nil receiver now panics, like every other Go
  dereference.
- **Per-kind pod-template helpers folded onto `PodSpec`.** One family serves every workload kind,
  reached through `&obj.Spec.Template.Spec`.
- **Garbage collection is no longer implicit.** In `pkg/stack/fluxcd`, an unset `Bundle.Prune`
  emits `prune: false` where it emitted `prune: true`. This is the one change that alters what a
  cluster does rather than how a caller writes it.

`pkg/stack/fluxcd`'s seventeen injected values now collapse onto eleven exported identifiers in
`defaults.go`, so a consumer can grep for the identifier and find every site the value can reach
emitted YAML.

---

## Release plan

One epic, five work items, each landing on its own and each listing its own removals in the ledger
exactly once:

1. **Core.** The generic `kubernetes.Create[T]`, the generated per-kind wrappers, the admission
   classifier and its test.
2. **Prune.** Delete the bare forwarders and the sub-type constructors; fold the pod-template
   families onto `PodSpec`; rewrite the error-returning setters as void.
3. **Defaults.** Remove what the constructors injected; declare `pkg/stack/fluxcd`'s remaining
   opinions as exported names.
4. **Tables.** Derive the kinds, scope and field-maturity tables from the pinned upstream types
   and the shipped CRDs, and publish them, so coverage is reported rather than hand-kept.
5. **Docs.** This work item: state the identity the four before it produced, and add a CI check
   (`scripts/check-doc-api-refs.sh`) that fails when a page names a builder function `pkg/**` no
   longer exports.

`CHANGELOG.md` is regenerated wholesale from commit messages by git-cliff
(`scripts/release.sh:239`), so each work item's enumeration lives in its commit message and in the
ledger, and the changelog carries a pointer to the ledger rather than a copy of it.

---

## What this does not solve

- **Discoverability of upstream fields.** A caller now reads the upstream type's Go doc instead of
  a kure function list. That is the intended trade, but it is a trade: the kinds and field-maturity
  tables exist because "what can I set, and will the API server keep it?" no longer has a
  kure-shaped answer.
- **Feature-gated fields still fail silently.** For a built-in type the API server clears a field
  whose gate is off and admits the object, so the manifest reads as applied and is not. The
  maturity table reports it; nothing in kure can prevent it.
- **The admission test is syntactic.** It does not follow control flow, so a write guarded by an
  optional-value condition is not detected. Review and the helper's own golden test cover that.
- **`pkg/stack` may still hold opinions.** The contract binds `pkg/kubernetes`. The layer above is
  a workflow layer and defaults are part of its job — the rule there is only that every default is
  a name, not a literal.
- **The ledger's own references are checked against a written list, not against the page.** A
  migration ledger has to name what it removed, so `scripts/doc-api-refs-removed.txt` licenses
  those names and only those. The list is validated — nothing in it may be a name `pkg/` still
  exports — but it is a snapshot maintained by hand, and the guarantee it buys is one-directional:
  a removed name the ledger mentions and the list omits fails the build, while a name wrongly
  present in the list quietly exempts itself on that page. Deriving the list from the page instead
  was tried and rejected: not all of the ledger's tables are removal tables, so a positional rule
  exempted about thirty live builders the page recommends.
- **The checker resolves a bare name, not a qualified one.** `Type.Method` on a page resolves if
  anything under `pkg/` exports `Method`, because the symbol index keeps package and name and drops
  the receiver type. So `LayoutIntegrator.CreateLayoutWithResources`
  (`pkg/stack/fluxcd/README.md:55`) would stay green if that method
  (`pkg/stack/fluxcd/layout_integrator.go:81`) were deleted, since `WorkflowEngine` exports the same
  name (`pkg/stack/fluxcd/workflow_engine.go:84`). The check catches a builder that no longer exists
  anywhere; it does not catch one that moved between types.
- **An unqualified generic reference is not checked.** `Create[T]` written bare — as `README.md:34`
  and `docs/ARCHITECTURE.md:91` write it — is discarded by the extractor, which keeps a generic
  match only when it carries a package qualifier. The condition exists because a bare `Set[T]` or
  `Add[T]` in a non-Go code block is usually another language's syntax rather than a kure builder,
  and it takes `Create[T]` with it. Renaming the generic constructor would leave those two overview
  references stale with the check green.
- **Go comments are scanned under `pkg/` only.** The `examples/` enumeration takes `*.md`, so an
  instructional comment in a runnable example — `examples/getting-started/main.go:93-98` describes
  `CreateLayoutWithResources` several lines above the call that uses it — is not checked. A rename
  that keeps the example compiling can leave the comment beside it stale.

- **A suppressing marker containing a nested `<!--` is accepted.** The opener pattern ends in
  `[^>]*-->`, and `<!--` contains no `>`, so `<!-- doc-api-refs:ignore-start reason <!-- x -->`
  matches and opens a fence. Every other malformed suppression is an error — no terminator, nothing
  open, reversed, unrecognised, reason-less, nested opener, repeated marker — so this one is an
  inconsistency as much as a hole: a typo suppresses a passage instead of failing the build, and
  the rendered page shows the stray `-->` while the check says nothing.

- **The symbol index is grepped, not parsed.** Declarations are collected with
  `grep -oE '^func (\([^)]*\) )?[A-Z][A-Za-z0-9_]*'`, so a line beginning `func CreateSomething`
  inside a block comment in a public Go file is recorded as an exported symbol the compiler never
  sees. That is a false *positive* in the index, which makes it a false negative in the check: a
  page naming the real function stays green after the function is deleted, because its ghost in a
  comment still resolves. The same is true of such a line in a raw string literal.

- **A malformed marker is honoured when a valid one shares its line.** The marker branch tests the
  forms in order and takes the first that matches, so
  `` `CreateGone` <!-- doc-api-refs:ignore-strt typo --> <!-- doc-api-refs:ignore valid --> ``
  matches the valid single-line ignore, skips the line, and never reaches the unrecognised-marker
  error that the typo alone would have raised. The line is suppressed and the build stays green.
  Like the nested-`<!--` case, the defect is as much the inconsistency as the hole: the same typo is
  an error on a line of its own.

  These six are the same shape as the exclusions above: the check is a floor, not a proof. They
  are filed as go-kure/kure#770 (checker false negatives) rather than fixed here — this ticket's
  subject is the documentation, and the checker had already taken four hardening rounds by the time
  they surfaced.
