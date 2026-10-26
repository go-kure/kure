# Generation-Result Ownership and Layout Augmentation — Design Notes

*Date: 2026-10-25 | Type: DESIGN | Scope: kure public API (`pkg/stack`, `pkg/stack/layout`)*

> **Note**: This document is not part of the kure documentation website. It is an internal
> development note about how go-kure/kure#1067 ("define generation-result ownership and layout
> augmentation contracts") is decided. The normative text is the godoc of
> `stack.ApplicationConfig`, `layout.LayoutAugmenter` and `layout.LayoutIntentAugmenter`, with
> `pkg/stack/README.md` § Generation contract; the executable form is
> `pkg/stack/layout/generation_contract_test.go` (walks) and
> `pkg/stack/generation_contract_test.go` (workflow entry points).

**Status: proposed.** The maintainer decides on the pull request that adds this note.

---

## Decision

A contract on the interfaces that exist, with executable tests, and no new API:

- **The result is the caller's.** From the moment `Generate` returns, the slice and every object
  it points to belong to the caller, with every map, slice and pointer those objects hold. kure
  writes into them in two places: `Bundle.Generate` adds the bundle's labels and annotations that an
  object lacks, and the Flux integrator adds an application's delivery annotations (undone if the
  integration is refused). A caller holding a layout may edit its objects too. An implementation
  therefore never returns an object, or anything inside one, that it keeps or has returned before.
- **Every call is independent, failed calls included.** A walk (`layout.WalkCluster`,
  `layout.WalkClusterByPackage`) calls `Generate` once for each application it renders, and every
  workflow entry point that renders applications walks once. kure memoizes nothing across calls:
  a caller that validates with one entry point and lays out with another generates twice. A
  walk in which one `Generate` fails returns no layout; a later walk uses nothing from it.
- **No concurrency.** kure never calls `Generate` concurrently and promises callers nothing about
  concurrent use. Memoization and thread safety are separate properties: a memoizing config is
  not thereby safe for concurrent use, and a safe one need not memoize.
- **Augmentation follows the call it augments.** In a walk, `AugmentLayout` runs once, right after
  the `Generate` call it belongs to, on the application's own layout. That layout's `Resources`
  are the very objects the call returned, not copies, and the augmenter reads them there, never
  from state `Generate` saved. It runs only where the walk gives the application a layout of its
  own. `WantsOwnLayout() == false` withholds that layout under flat application grouping only.
  `Application.Generate` and `Bundle.Generate` never augment. A new walk augments a new layout
  again: for an unchanged config the augmenter adds the same things.
- **Optional interfaces are found on the config value itself.** `Validator`
  (`application.go:98`), `LayoutAugmenter` (`walker.go:718`, `:759`) and `LayoutIntentAugmenter`
  (`walker.go:747`) are type assertions on `app.Config` (`walker.go` lines here are at
  `e932b15d`). A wrapper that embeds the
  `ApplicationConfig` interface has `Generate` alone in its method set and hides all three.
  The loss is silent: no validation, no augmentation and, because kure no longer sees a
  `LayoutAugmenter`, no layout of its own under flat application grouping: its resources merge
  into the bundle's directory (`walker.go:670-673`), even where the wrapped config wants its
  own. A wrapper embeds the concrete type or forwards each interface it wants kure to see.
  Forwarding `AugmentLayout` without `WantsOwnLayout` is presence-only placement: the wrapper
  gets a layout of its own under flat grouping whatever the wrapped config answers.
- **Reusing a validated result means memoizing with a copy.** A config may validate and render
  once and return a deep copy of the cached result on every call. That conforms, and the walk
  then costs a copy instead of a render. kure does not copy on receipt.

## The problem

The audit the issue asked for, at `289ba622`:

| Fact | Where |
|---|---|
| `ApplicationConfig.Generate` has no godoc on ownership, repeats or errors | `pkg/stack/application.go:38-41` |
| `Application.Generate` runs `Validate` before every `Generate` call | `application.go:65-81` |
| The walk generates once per application, skips nil entries, stops at the first error | `layout/walker.go:650-693` |
| The augmenter is handed the layout whose `Resources` are that call's objects | `walker.go:675-679`, `:714-724` |
| Flat grouping reads `WantsOwnLayout` once per application, short-circuited | `walker.go:668`, `:744-759` |
| Each entry point walks once: Flux `CreateLayoutWithResources`, `GenerateFromCluster`; Argo CD `GenerateFromCluster`, `CreateLayoutWithResources` | `fluxcd/layout_integrator.go:250`, `fluxcd/resource_generator.go:165`, `argocd/argo.go:59`, `:324` |
| No goroutines in `pkg/stack`, and nothing memoizes a generation result (its one cache holds the parsed Flux Operator install manifest) | `rg 'go func\|errgroup'` and `rg -i cache` over `pkg/stack`, non-test files; `fluxcd/flux_operator_install.go:47` |
| `Bundle.Generate` writes bundle labels and annotations into the map the object already holds | `bundle.go:487-516` |
| The Flux integrator sets delivery annotations on the application's objects, in place, with undo | `fluxcd/delivery.go:152-164`; derived Source `fluxcd/layout_integrator.go:1425-1430` |
| The only ownership text so far | `fluxcd/README.md:720-723` (delivery annotations persist on reused pointers) |
| The getting-started example aliases a config field into its output and shares one labels map three ways | `examples/getting-started/main.go:222`; labels at `:159-176` and `:209-225` |

Some implementations already build fresh objects or deep-copy a cached render. The issue cites
a Helm render that does the latter. So repeated generation is not broken everywhere. But
nothing says whether a cached pointer may be returned twice, and nothing says what an augmenter may rely on.

The audit found three ownership defects. Each has its own follow-up below:

- **kure's own output shares the caller's maps.** The Flux generator's Kustomization takes
  `Labels: b.Labels, Annotations: b.Annotations` (`fluxcd/resource_generator.go:754-755`).
  Editing a returned Kustomization's labels in place edits `Bundle.Labels`, and through it every
  later generation. A probe confirmed this for `GenerateFromCluster` (labels and annotations)
  and `CreateLayoutWithResources` (labels). The bundle builder's copy (`stack/builders.go:162-163`)
  assigns the same maps. The Argo CD Application does not: it is unstructured, and its
  `SetLabels` (`argocd/argo.go:187`) copies the map into the object, so no edit to it reaches
  `Bundle.Labels`.
- **`Bundle.Generate` writes into a shared map.** When an object's labels map is also its
  selector's, as in the getting-started example, the selector gains the bundle's labels.
  `Bundle.Generate` also dereferences each entry without the nil check the walker has
  (`bundle.go:489` against `walker.go:662-664`), so a nil entry panics there once the bundle sets
  labels or annotations.
- **The example teaches aliasing** (`examples/getting-started/main.go:159-176`, `:209-225`, `:222`).

## Options considered

### 1. kure deep-copies every result on receipt

The walker would copy what `Generate` returns. Rejected:

- Every config pays, though building fresh objects is the norm and needs no copy.
- It breaks identity that callers rely on. The per-application record holds the very object
  the layout writes (`layout/application_objects_test.go:119-127`).
- It does nothing for state an augmenter keeps.

### 2. A new entry point for pre-generated results

A walk variant that takes already-generated objects, or a kure-provided config that wraps them.
Rejected for now:

- Memoizing with a copy (option 3) gives the same result with no new API.
- A wrapper type cannot forward optional interfaces generically: Go method sets are fixed.
- `stack` cannot name `*ManifestLayout`, which is why `LayoutAugmenter` lives in `layout`
  (`layout/augmenter.go:31-34`).

Reopen it if a consumer shows that hand-written copying is error-prone in practice. The
conformance helper below is meant to show that.

### 3. A contract on the existing interfaces, with tests — chosen

Godoc and README state the rules above. The two `generation_contract_test.go` files pin kure's
half:
- one `Generate`, `Validate` and `AugmentLayout` per application per walk;
- one of each per call to every workflow entry point (Flux and Argo CD `GenerateFromCluster`,
  `CreateLayoutWithResources`);
- the augmenter sees that call's objects;
- where augmentation is skipped;
- a failed walk leaves no result;
- a caller's edit stays in its walk for fresh and copying configs, and leaks for a shared cache;
- what each wrapper shape keeps, under both application groupings: validation, augmentation and
  a layout of its own.

Six one-line mutations of `walker.go` and `application.go` were each caught, as were a second
walk in any of the four entry points and a forwarding wrapper that drops `WantsOwnLayout`. The
"no concurrency" rule is a statement about kure's code, not something a test can show.

## Copy cost

- A deep copy is linear in the object's size.
- A rendered object (template, Helm chart, decoded YAML) costs at least that much to produce again.
- A hand-built object costs about as much to build fresh as to copy.

So copying is the right price only for a config that would otherwise re-render, and that is the
config that chooses to memoize. Mandating it for every config, or doing it inside kure
(option 1), charges configs that gain nothing from it.

## Migration guidance

- **Implementations that build fresh objects**: nothing to change. Do not share a map or slice
  between objects, or between fields of one object (labels and selector). kure writes into labels and
  annotations.
- **Memoizing implementations**: return `DeepCopy()` of each cached object, never the cached
  pointer. Key the cache on everything `Generate` reads, `*Application` included, and drop it
  when the config changes.
- **Wrappers**: embed the concrete type, or forward `Validate`, `AugmentLayout` and
  `WantsOwnLayout` explicitly. A test that walks the wrapped config catches a lost interface
  only where that interface changes what it counts: `Validate` calls show a lost `Validator`
  under any grouping, and augmenter calls a lost `LayoutAugmenter` wherever the wrapped config
  would be augmented (`GroupByName`, or flat grouping where it wants its own layout). A lost
  `WantsOwnLayout` shows only under flat application grouping with a wrapped config whose
  `WantsOwnLayout()` is false, where the wrapper is then augmented once instead of never.
- **Augmenters**: read the application's objects from `ml.Resources`. Add the same things for
  the same config every time. Keep no state between calls that the next call depends on.
- **Consumers**: a layout you hold owns its objects. To integrate again after a change, walk
  again rather than reuse a layout. To validate once and lay out without re-rendering, give the
  application a memoizing config.

## Follow-ups

Registered before go-kure/kure#1067 closes, each with its completion criteria in the issue:

1. **Generated objects stop sharing the caller's maps** (go-kure/kure#1082). The Flux
   Kustomization and the bundle builder clone `Labels` and `Annotations`; the Argo CD
   Application already copies the labels and carries no annotations. Done when a test edits every generated object's maps in
   place and the input `Bundle` is unchanged.
2. **`Bundle.Generate` writes into copies and skips nil entries** (go-kure/kure#1083), as the
   walker does. Done when a test with a labels map shared with a selector, and one with a nil
   entry, both pass.
3. **The getting-started example builds every map and pointer fresh** (go-kure/kure#1084).
   Done when the example output is unchanged and `examples/getting-started/main_test.go` passes.
4. **A conformance helper** (go-kure/kure#1085). An exported test-support function that calls
   `Generate` twice and reports any object, map, slice or pointer the two results share. It
   also reports whether the config's optional interfaces survive a given wrapper. Done when it
   is documented, kure's own example configs pass it, and a sharing config fails it.
5. **Release and adoption** (go-kure/kure#1086). Done when a tagged kure release ships 1-4 with
   changelog entries, and one external consumer runs the helper in its CI, linked from the
   issue.

## What this does not solve

- **Concurrency.** Nothing here makes generation safe for concurrent use. A caller that generates
  in parallel owns that.
- **Objects the caller shares across clusters.** One `*Application` in two clusters is two
  generations under this contract, and only a fresh or copying config keeps them apart.
- **A pre-generated entry point.** Deferred (option 2), with its reopen trigger.
