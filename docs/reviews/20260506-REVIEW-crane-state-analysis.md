# Crane Codebase State Analysis

**Date**: 2026-05-06  
**Scope**: Full codebase review — 9 domains × 8 dimensions  
**Codebase**: `gitlab.com/autops/wharf/crane` at `/home/serge/src/autops/wharf/crane`

---

## Executive Summary

- **Silent data loss in 4 of 7 component handlers**: `valueFrom` environment variables (secret/configmap references) are silently discarded in the main containers of `webservice`, `worker`, `daemonset`, and `cronjob`. The `statefulset` handler uses the correct shared helper; the others don't.
- **Security scanner bypassed for all `stack.compile` requests**: `crane.stack.compile` never calls `ScanResources()` or `CheckPSARestricted()`. Privileged containers, `hostNetwork`, `hostPath`, etc. pass through unchecked regardless of `SecurityPolicy`.
- **Bootstrap NATS handler is broken for the registry-fallback case**: `nats.Config.DefaultRegistry` is never populated from config or env — there is no `CRANE_DEFAULT_REGISTRY` mapping — so any `crane.bootstrap.render` request relying on the server-side registry default will fail with `ErrMissingRegistry` in production.
- **Pervasive kure boundary violations**: `helmrelease.go` (raw unstructured), `configmap.go` trait (raw unstructured), `externalsecret.go` (direct spec mutation), `render_flux.go` OCIRepository/Kustomization struct literals, `namespace.go` Namespace struct literal, all of `infracomponents/` — most require kure-side additions; `render_flux.go` can be fixed in crane immediately (kure already has public builders for both types).
- **AGENTS.md is stale in at least four places**: wrong handler file names, no table of CRANE_* env vars, an incorrect security integration point reference, and an error handling example that bypasses the type system.

---

## Severity Legend

- **CRITICAL**: Correctness bug with direct user impact (data loss, security bypass, broken functionality)
- **HIGH**: Violates AGENTS.md rules, known incorrect behavior, or significant design non-compliance
- **MEDIUM**: Maintainability issues, incomplete coverage, API quality problems
- **LOW**: Style, naming, documentation, minor inconsistencies

---

## Domain 1: OAM Layer

**Files**: `internal/oam/types.go`, `parser.go`, `validate.go`, `parser_test.go`, `parser_bench_test.go`

### Findings

#### Technical Debt

**HIGH — Missing validation: `metadata.namespace` format not validated (`validate.go`)**

`Metadata.Namespace` is parsed but never validated against DNS-1123 format. A namespace like `INVALID_NS` or `invalid_namespace` passes OAM validation silently. `metadata.name` is correctly validated via `validation.IsDNS1123Subdomain`; the same must be applied to namespace.

**HIGH — Missing cross-field validation: policy properties reference component names that are never verified**

`dependency` and `placement` policy types accept `properties.components`/`properties.component` as free-form strings. The validator does not check whether referenced component names exist in `spec.components`. A placement policy referencing a non-existent component passes validation and produces a silent no-op or a confusing transform error downstream.

**HIGH — Strict mode: `BenchmarkParseAndValidate` calls `Validate` twice (`parser_bench_test.go:48–54`)**

`Parse()` calls `Validate()` internally. The benchmark then calls `Validate(app)` again explicitly. The measurement is wrong — it times parse + validate + validate. This benchmark is misleading about relative performance.

#### Design Compliance

**HIGH — `oamValidationError()` produces structurally incomplete errors (`validate.go:178–187`)**

The helper bypasses `NewValidationError`, leaving `ErrContext` nil and `Help` empty on all OAM-level validation errors. It also hardcodes `Component: "application"` for all errors including trait-level ones. Any code calling `err.Context()` or `err.Suggestion()` on these errors gets nothing, and the `Component` field is unreliable.

**MEDIUM — `oamValidationError` is not using `pkg/api/errors.go` error codes directly**

The OAM package uses `kureerrors` types; translation to `api.ErrOAMParseError`/`api.ErrOAMValidationError` happens in `internal/nats/errors.go`. This works for the NATS path but the CLI path (`cmd/crane/validate.go:41`) prints errors with no `ErrorCode`.

#### Dead Code

**MEDIUM — `IsValidComponentType` and `IsValidTraitType` are exported but never called (`validate.go:190,195`)**

Both are at 0% coverage. They are exported from an `internal/` package, so they cannot be called from outside the module. Dead code.

#### Code Smells

**MEDIUM — Missing validation: duplicate traits on a single component not detected**

Two `ingress` traits on the same component pass validation. Duplicate traits produce undefined behavior (second handler typically overwrites the first). Duplicate component and policy names are caught; duplicate trait types are not.

**MEDIUM — Magic string `"supported types: webservice, worker"` hardcoded in trait restriction error (`validate.go:144`)**

Should be derived from `traitComponentRestrictions[t.Type]`. Will go stale silently when new types are added.

**MEDIUM — `SupportedComponentTypes()`, `SupportedTraitTypes()`, `SupportedPolicyTypes()` return non-sorted slices**

Map iteration order is random per Go spec. User-facing "Valid values are: ..." suggestion strings are non-deterministic across runs.

#### Function APIs

**MEDIUM — Parse errors discard YAML line/column information (`parser.go:22,46`)**

`gopkg.in/yaml.v3` returns `*yaml.TypeError` with per-error line info. Both `Parse()` and `ParseMulti()` pass `line=0, column=0` to `NewParseError`, losing all location hints.

**LOW — `MustParse` exported but only suitable for tests** — should be unexported or moved to test helpers.

#### External Code Usage

**MEDIUM — `"no documents found"` classified as ParseError not ValidationError (`parser.go:55`)**

An empty YAML stream is semantic, not syntactic. Wrapping it in `ParseError` mis-classifies it as `api.ErrOAMParseError` instead of `api.ErrOAMValidationError`. Callers cannot distinguish "malformed YAML" from "valid YAML with wrong content."

### Summary

The OAM layer is generally correct with 91.7% test coverage. The most critical gaps are three missing validation rules (namespace format, policy-to-component cross-references, duplicate traits) that allow structurally invalid inputs to pass the OAM gate and surface as confusing errors downstream. The `oamValidationError` helper produces structurally incomplete error structs with no context or help text. `IsValidComponentType` and `IsValidTraitType` are dead exported functions.

---

## Domain 2: Transform Core

**Files**: `internal/transform/transformer.go`, `classify.go`, `serialize.go`, `policy.go`, `mutators.go`, `namespace.go`, `healthchecks.go`, `defaults/`

### Findings

#### Design Compliance

**HIGH — `wharf.zone/managed-by` label value is `"wharf"` but design doc specifies `"crane"` (`transformer.go:759`)**

`namespace-model.md §4.7` shows `wharf.zone/managed-by: crane`. Production code sets `"wharf"`. The test at `transformer_test.go:299` asserts `"wharf"` (namespace resource test), confirming this is the production value. The test at line 389 that asserts `"crane"` never executes because `cluster.Node.Children` is empty for umbrella clusters after the multi-tier refactor — the loop body is dead. This bug is live and undetected.

#### Kure Boundary Verification

**MEDIUM — `namespace.go:20-33` constructs `corev1.Namespace` via struct literal**

Violates AGENTS.md Rule 7. A `CreateNamespace` builder exists in `kure/internal/kubernetes/namespace.go:8` but is not exposed in `pkg/kubernetes/`. The fix requires exposing it in kure's public API first:

```go
ns := &corev1.Namespace{
    TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
    ObjectMeta: metav1.ObjectMeta{Name: app.Namespace, Labels: ...},
}
```

#### Function APIs

**MEDIUM — `Transform` and `TransformWithPolicy` have no `context.Context` parameter**

No cancellation, deadline propagation, or tracing. Adding it later requires a breaking API change across all callers.

#### Dead Code

**LOW — `TestTransform_Labels` assertion body (`transformer_test.go:385-397`) never executes**

The loop `for _, child := range cluster.Node.Children` iterates zero times for umbrella clusters. All assertions inside the loop are dead. This is how the `"wharf"` vs `"crane"` label inconsistency goes undetected.

#### Technical Debt

**HIGH — `policy_reconciliation.go` and `policy_healthchecks.go` have no dedicated unit tests**

`applyReconciliationSettings`, `determineBundleTier`, and `parseHealthCheckEntries` are only tested indirectly.

**MEDIUM — Namespace resolution logic copy-pasted in 4 locations (`transformer.go:123-129`, `265-272`, `323-330`, `447-454`)**

Identical 5-line pattern should be extracted to a single private `resolveNamespace(app, ctx)` helper.

#### Code Smells

**MEDIUM — `buildDependencyAwareCluster` (`transformer.go:446-557`, ~110 lines) handles 5 concerns**

Creates namespace bundle, per-component bundles with traits, explicit dependency edges, auto cross-tier edges, cycle detection. A prime candidate for extraction.

**LOW — `tierIntervalDefaults` in `policy_reconciliation.go:24-28` maps all three tiers to `"60m"` — the differentiation is functionally dead**

**LOW — Magic string `"_namespace"` at `transformer.go:474`** — should be a named constant.

### Summary

The transform core is architecturally sound: stateless, delegate-based, well-tested via golden file and scenario tests. The most critical issue is the `wharf.zone/managed-by` label value discrepancy (`"wharf"` in production vs `"crane"` in the design document), compounded by a vacuously-passing test that never executes its assertion body. The `corev1.Namespace` struct literal in `namespace.go` violates the kure boundary rule. No `context.Context` propagation exists, requiring a breaking API change if cancellation or tracing is ever needed.

---

## Domain 3: Component Handlers

**Files**: `internal/transform/components/` — all 7 handlers plus `common.go`

### Findings

#### External Code Usage / Code Smells

**CRITICAL — `valueFrom` env vars silently dropped in 4 of 7 handlers (`webservice.go:223`, `worker.go:205`, `daemonset.go:177`, `cronjob.go:185`)**

All four use:
```go
for _, env := range c.Env {
    _ = kubernetes.AddContainerEnv(container, corev1.EnvVar{Name: env.Name, Value: env.Value})
}
```

The `ValueFrom` field (secret/configmap references) is silently discarded. `statefulset.go:244` correctly uses `buildEnvVars(c.Env)` which handles `ValueFrom`. Any workload using `env[].valueFrom` (secret injection) via webservice, worker, daemonset, or cronjob will receive an empty string env var in the generated manifest. No golden test covers this case because all golden files use `valueFrom` only on init containers (which go through `buildInitContainer` in `common.go` correctly). The fix is a one-line change: replace the inline loop with `buildEnvVars(c.Env)` in each of the four files.

#### Kure Boundary Verification

**HIGH — `helmrelease.go:417-586` bypasses kure's FluxCD builders, uses raw `*unstructured.Unstructured`**

Kure's `HelmReleaseConfig` is severely under-specified (name, namespace, chart, version, sourceRef, interval, releaseName only). Crane's HelmRelease needs values, valuesFrom, driftDetection, chartRef mode, OCI source type, remediation, rollback, postRenderers. The builders are too thin to use. Fix belongs in kure. Hard-coded apiVersion strings: `"helm.toolkit.fluxcd.io/v2"`, `"source.toolkit.fluxcd.io/v1"`.

**HIGH — `createPooler` constructs CNPG Pooler as raw `*unstructured.Unstructured` (`postgresql.go:757-792`)**

Documented at line 755 with reference to kure#187. The architectural violation is active.

**MEDIUM — `validAccessModes` re-enumerates K8s access mode constants (`common.go:981-986`)**

Technically uses `corev1.*` constants (not hardcoded strings) but AGENTS.md Rule 8 considers any re-enumeration of K8s well-known values a violation.

#### Design Compliance

**MEDIUM — Errors from component handlers do not use `pkg/api` error codes**

All `ToApplicationConfig` implementations return plain `fmt.Errorf(...)`. None use `api.ErrTransformError` or any code from `pkg/api/errors.go`. AGENTS.md requires error codes.

**LOW — `postgresql.go` does not validate `imageName` with `ValidateImageRef` (`postgresql.go:277-279`)**

All other image-accepting handlers validate images. PostgreSQL silently accepts `:latest` or untagged images.

#### Technical Debt

**MEDIUM — `cronjob` handler lacks volumes, sidecars, and affinity** (present in webservice, worker, statefulset)

**MEDIUM — `daemonset` handler lacks sidecars and affinity** (present in webservice, worker, statefulset)

**LOW — `statefulset` handler lacks topology spread constraints** (present in webservice, worker)

**MEDIUM — PostgreSQL affinity parsed inline instead of reusing `parseAffinity` from `common.go`**

5 flat fields (`AffinityEnabled`, `AffinityEnablePodAntiAffinity`, etc.) diverge from the shared `AffinityConfig` used by all other handlers.

#### Coding Standard Inconsistencies

**MEDIUM — `doc.go` is stale**: lists only `webservice, worker, postgresql, cronjob`; misses `daemonset`, `statefulset`, `helmrelease`.

### Summary

The component handler layer has two active bugs and two tracked kure boundary violations. The critical bug — silent `valueFrom` data loss in 4 of 7 handlers — is a one-line fix per handler but represents a serious runtime failure for any workload using secret-injected environment variables. The boundary violations (`helmrelease.go`, `createPooler`) require kure-side work first. The postgresql handler has multiple consistency gaps vs other handlers.

---

## Domain 4: Trait Handlers

**Files**: `internal/transform/traits/` — all 16 trait handlers

### Findings

#### Kure Boundary Verification

**CRITICAL — `configmap.go:91` constructs ConfigMap via `unstructured.Unstructured` struct literal**

A `CreateConfigMap` builder exists in `kure/internal/kubernetes/configmap.go:11` but is not exposed in `pkg/kubernetes/`. The fix requires exposing it in kure's public API first:
```go
cm := &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap", ...}}
```

**HIGH — `externalsecret.go:287-353` directly mutates ExternalSecret spec fields lacking kure setters**

After calling the kure builder `externalsecrets.ExternalSecret()`, crane directly sets:
- `es.Spec.RefreshInterval` (line 293) — no granular kure setter
- `es.Spec.Target` (lines 295-308) — no granular kure setter  
- `es.Spec.DataFrom` (lines 327-349) — no granular kure setter
- `es.Spec.Data` appended via raw `esv1` struct literals, bypassing `AddExternalSecretData`

A bulk setter `SetExternalSecretSpec(obj, spec)` exists at `kure/pkg/kubernetes/externalsecrets/update.go:10`, but it replaces the entire spec rather than setting individual fields — not suitable for the current incremental-build pattern. Full resolution requires adding granular setters to kure.

**MEDIUM — `httproute.go:987-1038` constructs `gatewayv1.HTTPRouteRule`, `HTTPRouteMatch`, `HTTPBackendRef`, and filter structs directly**

Kure provides `AddHTTPRouteRule` etc. but not builders for the rule's interior objects. This is a kure API gap requiring expansion.

**MEDIUM — `cilium_networkpolicy.go:90-104` uses `unstructured.Unstructured` for CiliumNetworkPolicy**

Intentional and documented, but AGENTS.md Rule 7 says to add the builder to kure first.

#### Design Compliance

**LOW — Zero error codes from `pkg/api` used across all 16 trait handlers**

Every trait returns plain `fmt.Errorf(...)`. None use `api.NewError(api.ErrTransformError, ...)`. AGENTS.md requires error codes. This is pervasive — hundreds of error return sites.

**MEDIUM — `certificate.go` hardcodes `"ClusterIssuer"` as default issuer kind (line 68)**

Undocumented opinionated default. Trait emits a Certificate without a real issuer if OAM specifies `issuerRef` directly.

#### Technical Debt

**MEDIUM — `VolsyncConfig.Generate()` does not label the ReplicationSource**

All other traits set at minimum `"app": c.ComponentName`. VolSync sets no labels. The `LabelMutator` adds `wharf.zone/*` labels, but `"app"` will be absent. Compare with `backup.go:117` which explicitly sets the `"app"` label.

**MEDIUM — No cron schedule validation in `backup.go` or `volsync.go`**

Invalid cron expressions pass crane and fail only when the operator rejects the CR.

**LOW — `volsync_test.go` auto-writes golden files when missing** — silent bad fixture creation on first run. Should fail and ask for `UPDATE_GOLDEN=1`.

**LOW — `httproute.go` is 1313 lines** — split into `httproute_filters.go` + `httproute_config.go`.

#### Code Smells

**MEDIUM — `IngressConfig.Generate()` ignores errors from kure builder calls at 3 sites (lines 231, 236, 240)**

`_ = kubernetes.AddIngressRule(ingress, ...)` — errors silently discarded. Pattern pervasive across multiple trait handlers.

**MEDIUM — `scaler.go` hardcodes `"50%"` minAvailable in PDB (`buildPDB`, line 162)**

Not configurable from OAM properties. Undocumented. Inappropriate for all workload sizes.

**LOW — `coerceInt32` in `httproute.go` and `proputil.ToInt32` are the same utility defined twice**

#### Coding Standard Inconsistencies

**LOW — Triplicate test helpers**: `resourcesToYAML`/`normalizeYAML` defined independently in `ingress_test.go`, `networkpolicy_test.go`, `httproute_test.go`.

**LOW — `doc.go` is stale**: lists only 5 MVP traits, now 16 implemented.

### Summary

The trait handler layer is consistent and well-tested (all 16 traits have unit tests; most have golden tests, but `fluxcd-patches` and `fluxcd-postbuild` lack golden fixture files). The most critical issue is the ExternalSecret handler directly mutating spec fields for which kure provides no granular setters — a genuine boundary violation requiring kure additions. The ConfigMap handler uses unstructured because kure's `CreateConfigMap` builder is not yet exposed in `pkg/kubernetes/`. Zero error codes are used across all 16 handlers. The VolSync handler doesn't set the `"app"` label, diverging from all other traits.

---

## Domain 5: Stack & Infrastructure Compilation

**Files**: `internal/transform/stackcompile/`, `infracompile/`, `infracomponents/`, `proputil/`

### Findings

#### Dead Code

**HIGH — `BuildSplit` is exported dead code with inconsistent KS naming convention (`builder.go:813-990`)**

The NATS handler uses only `BuildMultiSplit` for split cases. `BuildSplit` is never called from production code — only from one test. It also uses bare `app.Name` as Kustomization CR names while `Build` and `BuildMultiSplit` use `<group>-<appname>`, making it a correctness trap. Previous analysis (`docs/archive/development/20260505-ANALYSIS-crane-state.md:152`) also flagged this. Should be deleted.

#### Design Compliance

**HIGH — `ErrStackUnknownGroupApp` has no `api.ErrorCode` and is absent from `classifyStackBuildError`**

Returned from `Build` (line 266) and `BuildMultiSplit` (line 1076) when `ApplicationGroups` references an unknown app. `pkg/api/errors.go` has no corresponding constant. `classifyStackBuildError` has no `errors.Is(err, stackcompile.ErrStackUnknownGroupApp)` case. The error falls through to `api.ErrInternalError`, misclassifying a user input error as an internal server error.

#### External Code Usage / Kure Boundary

**HIGH — `infracomponents` constructs Kubernetes struct literals directly — boundary violation**

All three translators construct resources as raw struct literals:
- `helm.go`: `corev1.Namespace{}`, `sourcev1.HelmRepository{}`, `helmv2.HelmRelease{}`
- `oci.go`: `corev1.Namespace{}`, `sourcev1.OCIRepository{}`, `kustv1.Kustomization{}`
- `manifests.go`: `corev1.Namespace{}`, `unstructured.Unstructured{}`

Similarly `stackcompile/builder.go` constructs `sourcev1.OCIRepository{}` (line 577), `kustv1.Kustomization{}` (lines 602, 659, 748), and `infracompile/layer3.go` constructs `kustv1.Kustomization{}` (line 201) directly. 12+ direct struct constructions violating AGENTS.md Rule 7.

#### Technical Debt

**MEDIUM — `TranslatedComponent.Namespace` contract not fulfilled by its caller**

`translator.go:39-42` doc comment: "Caller should set this on the per-component Kustomization's `spec.targetNamespace`." `buildLayer3Kustomization` never reads `translated.Namespace`. For `manifests` and `oci` source types, the Layer 3 Kustomization has an empty `targetNamespace`, applying objects into whatever namespace each manifest specifies rather than the component's derived namespace.

**MEDIUM — `spec.namespace` override only works for `helm` type components**

`helm.go:45-48` checks `def.Spec.Namespace`; `manifests.go` and `oci.go` always call `deriveNamespace()` and ignore the override. Inconsistent behavior for the same typed field.

**MEDIUM — Stale `doc.go` comment claims app-to-app dependency support is deferred (crane#144)**

Fully implemented and tested. Misleading.

#### Coding Standard Inconsistencies

**MEDIUM — `cloneLabels`, `mustParseInterval`, `fluxSystemNamespace`, `defaultInterval` duplicated verbatim between `stackcompile` and `infracompile`**

builder.go even has a comment acknowledging the duplication but referencing a non-existent `infracompile.buildClusterLabels`.

#### Code Smells

**MEDIUM — `BuildMultiSplit` is 430 lines (`builder.go:1015-1450`) handling 10+ concerns**

Parse validation, InfraDependsOn, app-dep validation, group assignment, Layer 3 build, OCI set computation, per-app Layer 2, catch-all partition, bootstrap root, per-split-group OCI assembly.

### Summary

The stack and infrastructure compilation layer is architecturally sound — no shared mutable state, deterministic YAML output. The most critical issues are: `BuildSplit` is dead code with a dangerous inconsistent naming convention; `ErrStackUnknownGroupApp` is user input but surfaces as `ErrInternalError`; `infracomponents` constructs K8s struct literals directly in violation of the boundary rule; the `TranslatedComponent.Namespace` contract is documented but not implemented.

---

## Domain 6: NATS & CloudEvents Layer

**Files**: `internal/nats/`, `pkg/api/`

### Findings

#### Design Compliance

**MEDIUM — `publishCompiledEvent` sets `ClusterID = event.TenantID` (`events.go:132`)**

```go
ClusterID: event.TenantID, // TODO: Get from request target
```

Any downstream consumer filtering `app.compiled` JetStream events by `cluster_id` receives the tenant ID instead. The cluster ID is available at the call site (`target.Spec.Cluster` in `handleCompile`) but is not threaded through.

**LOW — Placement policy enforcement missing from `handleStackCompile`**

`handleCompile` explicitly calls `policy.EnforcePlacement()` at lines 118-127. `handleStackCompile` has no equivalent check.

#### External Code Usage

**MEDIUM — CloudEvent IDs derived deterministically from request ID — collision risk**

`events.go:56`: `ID: reqEvent.ID + "-event"`; `server.go:395`: `ID: req.ID + "-response"`. CloudEvents spec requires IDs unique per source. On retry with the same request ID, response and event IDs collide.

**LOW — No server-side request timeout**

If a transform hangs, the caller receives a NATS timeout but crane continues consuming resources indefinitely. No deadline is derived from message receipt time.

**LOW — `conn.Subscribe` used instead of `conn.QueueSubscribe`**

In a horizontally-scaled deployment, every crane instance processes every message.

#### Dead Code

**LOW — `kureVersion` variable resolved at init time but never read by any production code**

`kure_version.go:9` resolves the kure module version. No handler, response, log line, or health check uses it.

#### Technical Debt

**LOW — `classifyStackBuildError` final fallthrough returns `ErrInvalidBundleDefinition` for all unknown errors**

Should return `ErrInternalError` for unrecognized errors, consistent with `classifySecurityError` and `classifySchemaError`.

#### Code Smells

**LOW — `generateFromBundle` uses duck-typing on `any` instead of the known concrete type `*stack.Bundle`**

The function is also unused in the stack compile path.

**LOW — `IsRetriable()` always returns `false` — stub with zero callers**

### Summary

The NATS layer is well-structured: consistent guard sequences across all three handlers, structured error responses on all paths, JetStream events for every terminal outcome, clean `pkg/api` wire boundary with no kure/k8s type leakage, correct server lifecycle with graceful drain. The most impactful bug is `ClusterID = TenantID` in `publishCompiledEvent`. The absence of server-side timeouts means stuck transforms leak goroutine resources. `kureVersion` is fully computed but never exposed.

---

## Domain 7: Bootstrap Chain

**Files**: `internal/bootstrap/render.go`, `layer0.go`, `render_cilium.go`, `render_flux.go`

### Findings

#### Design Compliance

**HIGH — `bootstrap.ErrMissingRegistry` not mapped to `api.ErrMissingRegistry` in the NATS handler**

`render_flux.go:71` defines and documents `ErrMissingRegistry` as "Surfaced by the NATS handler as `api.ErrMissingRegistry`." But `handler_bootstrap_render.go:117-126` only checks one sentinel:

```go
code := api.ErrInternalError
if errors.Is(err, bootstrap.ErrUnsupportedImplementation) {
    code = api.ErrUnsupportedComponent
}
```

`ErrMissingRegistry` is not checked. A missing registry URL returns `api.ErrInternalError` instead of `api.ErrMissingRegistry`, hiding a non-retriable configuration error behind a generic internal error.

**MEDIUM — Bootstrap phase ordering (bare → agent) not enforced programmatically**

`api.BootstrapRenderRequest.Validate()` only checks `phase != ""`. An unknown phase value (e.g., `"foo"`) passes validation, matches no catalog components, returns an empty success response, and Beacon applies it as a silent no-op. Caller bugs and misconfigured catalogs are invisible.

#### Kure Boundary Verification

**HIGH — `buildRootOCIRepository` (`render_flux.go:197-213`) and `buildRootKustomization` (`render_flux.go:220-241`) construct FluxCD CRs via struct literals**

Kure provides `kurekfluxcd.OCIRepository(cfg)` (create.go:75) and `kurekfluxcd.Kustomization(cfg)` (create.go:88). Both should be used. `layer0.go` correctly uses kure builders (`kurekfluxcd.HelmRepository`, `kurekfluxcd.HelmRelease`).

#### Technical Debt

**HIGH — Cilium rendering has zero functional coverage in CI**

`render_cilium_test.go:77-104` wraps the only real functional test in `t.Skip("CILIUM_CHART_URL not set...")`. Not tested: values passthrough from `DefaultValues`, `Ref` fallback when `Version` is empty, `ResourceCount` correctness, checksum shape. `mise run test` never exercises the Cilium path.

**MEDIUM — `fluxOperatorHelmVersion = ">=0.0.0-0"` is a floating constraint** (`layer0.go:25`)

`FluxSystemHelmObjects` (used by stackcompile in the infrastructure OCI artifact) will install whatever the latest chart version is at reconciliation time. Not reproducible.

**MEDIUM — `defaultSourceRef = "latest"` hardcoded in root OCIRepository** (`render_flux.go:65`)

No API surface for callers to override this tag except a full `ociURLOverride`. Cannot pin to an immutable tag via the NATS API.

#### Function APIs

**MEDIUM — `RenderFluxBootstrap` and `FluxSystemHelmObjects` take 5-7 positional string parameters**

Silent transposition bugs for `fluxRegistry`/`fluxVersion`. A `FluxRenderConfig` struct would be self-documenting.

**MEDIUM — `FluxSystemHelmObjects` is exported from `internal/bootstrap` but only consumed by `stackcompile`**

Architecturally confusing: the function belongs to the steady-state OCI artifact path, not the day-0 bootstrap handler path.

#### Code Smells

**LOW — Stale package doc comment says Cilium is deferred but Cilium is wired** (`render_flux.go:1-25`)

### Summary

The bootstrap chain is architecturally isolated and well-tested for the FluxCD path. The critical issue is `ErrMissingRegistry` not being mapped by the NATS handler, causing configuration errors to surface as generic internal errors. Two FluxCD CRs are constructed as raw struct literals despite kure having public builders for both. The Cilium render path has zero CI coverage. The `latest` tag on the root OCIRepository cannot be overridden via the NATS API.

---

## Domain 8: Configuration, Policy & Security

**Files**: `internal/config/`, `internal/policy/`, `internal/security/`

### Findings

#### Design Compliance

**CRITICAL — `crane.stack.compile` path bypasses the security scanner entirely**

`handler_stack_compile.go` delegates to `stackcompile.Build()`/`BuildMultiSplit()`, neither of which calls `security.ScanResources()`. The `stackcompile` package has no import of `internal/security`. Privileged containers, `hostNetwork`, `hostPath`, etc. pass unchecked for all stack-compile requests regardless of `SecurityPolicy`. No tracking issue, no TODO comment.

**CRITICAL — `CheckPSARestricted()` has no production call site**

`security/scanner.go:133-162` defines `CheckPSARestricted()`. Only called from tests. Neither `handler_compile.go` nor any stack-compile path calls it. The PSA Phase 1 compliance described in `psa-compliance.md` is not enforced at runtime.

**MEDIUM — `policy.ViolationError` not mapped to `api.ErrPolicyViolation` in stack-compile path**

`classifyTransformError()` (nats/errors.go:25-33) correctly maps `*policy.ViolationError` → `api.ErrPolicyViolation` for app-compile. `classifyStackBuildError` does not check for `*policy.ViolationError`. A policy violation from stack-compile is reported as `INVALID_BUNDLE_DEFINITION` instead of `POLICY_VIOLATION`.

**MEDIUM — No per-tenant privilege ceiling — `SecurityPolicy` trusted verbatim from caller**

The `SecurityPolicy` embedded in the request is applied as-is. There is no server-side ceiling preventing a caller from granting themselves `AllowPrivileged: true`. `psa-compliance.md §Layer 5` proposes a `MaxPSAProfile` ceiling field; it is not implemented in `SecurityPolicy` or enforced anywhere.

#### Dead Code

**MEDIUM — `CheckPSARestricted()` is tested but never called from production code** (also a design compliance issue)

#### Technical Debt

**LOW — `CRANE_*` environment variables not documented in AGENTS.md**

27 environment variables implemented in `internal/config/config.go`. AGENTS.md contains no listing. The full set is only discoverable by reading the config code or `docs/guides/local-development.md`.

**LOW — AGENTS.md references `handler_generate.go` as the security scan integration point** — file does not exist; it's `handler_compile.go`.

**LOW — Ephemeral containers not scanned by `checkPrivileged()` (`scanner.go:94-99`)**

`checkPrivileged` iterates `spec.Containers` and `spec.InitContainers` but not `spec.EphemeralContainers`. Lower risk for static manifest generation, but the gap is undocumented.

#### Code Smells

**LOW — `envKeyReplacer` double-strips `CRANE_` prefix** (`config.go:235`)

koanf strips the prefix before calling the replacer. The replacer also strips it — redundant but not a bug. The function's doc comment examples show the full `CRANE_*` prefix, suggesting the author expected to receive it. Tests call the function directly with full `CRANE_*` keys, further obscuring the discrepancy.

**LOW — `SigningMode` type defined twice** (`internal/config/config.go:117-125` and `pkg/api/types.go:542-549`)

### Summary

The config and policy layers are well-structured with good test coverage. The most critical issue is that `crane.stack.compile` bypasses the security scanner entirely — a silent, untested gap that allows privileged patterns through regardless of policy. `CheckPSARestricted()` has no production call site, meaning PSA Phase 1 compliance is never enforced. The missing `MaxPSAProfile` ceiling means the security model depends entirely on callers sending correct policies.

---

## Domain 9: CLI & Supporting Services

**Files**: `cmd/crane/`, `internal/health/`, `internal/catalog/`, `internal/clusterprofile/`, `internal/workload/`

### Findings

#### External Code Usage

**CRITICAL — `nats.Config.DefaultRegistry` is never populated from config (`serve.go:44-68`)**

`internal/nats.Config.DefaultRegistry` (server.go:63) is used in `handler_bootstrap_render.go:115` to build the OCI URL when a `crane.bootstrap.render` request omits `spec.oci_url_override`. The field comment: "Empty → handler_bootstrap_render returns ErrMissingRegistry." `config.Config` has no corresponding field, no `CRANE_DEFAULT_REGISTRY` env var exists, and `serve.go` never sets this field. Every `crane.bootstrap.render` NATS request that relies on the server-side registry fallback silently fails with `ErrMissingRegistry` in production.

#### Function APIs

**HIGH — Health server background error channel discarded (`serve.go:80`)**

```go
if _, err := hs.ListenAndServe(ctx); err != nil {
    return fmt.Errorf("health server: %w", err)
}
```

The `(<-chan error, error)` first return value is discarded. If the health server crashes after starting, the NATS server continues with no health endpoint and no log output. Probes against `/health` and `/ready` will time out silently.

#### Technical Debt

**HIGH — `internal/catalog` and `internal/clusterprofile` packages are orphaned — zero production callers**

Both are complete NATS KV client implementations with tests. Neither is imported by any production code. The design shifted to inline catalog/profile data (ADR-023: "crane performs no NATS KV lookups; the caller carries all needed data"). These packages are pre-ADR-023 artifacts. Should be deleted or transferred to the calling service.

**MEDIUM — `serve.go` manually maps 25 fields from `config.Config` to `nats.Config` with no compile-time safety**

This is exactly how `DefaultRegistry` ended up missing. A `nats.ConfigFromAppConfig(cfg)` constructor would prevent silent omissions.

**MEDIUM — `collectBundles` in `transform.go:229-241` only walks `node.Children`, not `bundle.Children`**

`command_helpers.go:158-170` (the NATS handler path) walks the full bundle subtree via `collectBundleSubtree`. The CLI flat-output path silently drops manifests from child bundles in umbrella+tier layouts — a silent data loss bug in `crane transform --output`.

**LOW — `kureVersion` variable computed at init, never read in production — dead overhead**

**LOW — `ErrorCode.IsRetriable()` always returns `false`, zero callers — remove or implement**

**LOW — Flux workflow flags on `transform` silently ignored in flat output mode**

`--source-ref-kind`, `--flux-placement`, `--bundle-grouping`, etc. have no effect when `--output-dir` is absent. No warning emitted to the user.

#### Code Smells

**MEDIUM — `serve.go:44-68` field-by-field struct mapping with no abstraction**

25 lines of repetitive mapping where new fields are silently missed (see `DefaultRegistry`).

#### Coding Standard Inconsistencies

**LOW — AGENTS.md lists `handler_generate.go` and `handler_validate.go` which do not exist** (same as Domain 6, 8 findings)

**LOW — CLI test flag state not fully reset between tests**

`transformNamespace`, `transformCluster`, `fluxPlacement`, `bundleGrouping` etc. persist across test runs. Tests that set `--namespace production` leave state for subsequent tests.

### Summary

The CLI is well-structured: signal handling is correct, NATS drains gracefully, `transform` and `validate` are properly stateless. The most critical operational risk is `DefaultRegistry` never populated, silently breaking all NATS bootstrap requests that rely on the server-side registry default. The health server's error channel being discarded means a crashed health server is invisible. `internal/catalog` and `internal/clusterprofile` are orphaned by ADR-023 and should be deleted. The CLI flat-output path silently drops manifests from multi-tier bundle trees.

---

## Cross-Domain Issues

### AGENTS.md is stale in at least 4 places

| Location | Issue |
|----------|-------|
| `AGENTS.md:41-42` | Lists `handler_generate.go`, `handler_validate.go` — files don't exist |
| `AGENTS.md:454` | References `handler_generate.go` as security scan integration point — it's `handler_compile.go` |
| `AGENTS.md:289-291` | Error handling example uses `fmt.Errorf("OAM_VALIDATION_ERROR: ...")` bypassing the type system — wrong |
| `AGENTS.md` | No table of CRANE_* environment variables |
| `AGENTS.md:203` | MVP component types table is outdated (lists 4, codebase has 7) |

### No error codes from `pkg/api` used in domain layers

AGENTS.md requires all errors to include an error code. Zero component handlers and zero trait handlers use `api.NewError()` or any code from `pkg/api/errors.go`. The wrapping to structured codes happens only at the NATS handler layer, meaning the domain-layer error origin is lost.

### No `context.Context` in the transformation pipeline

`Transform`/`TransformWithPolicy`, `ComponentHandler.ToApplicationConfig`, `stack.ApplicationConfig.Generate` — none accept a `context.Context`. Adding it later is a breaking change across all callers and all implementors. Cancellation and tracing are impossible until this is addressed.

### Kure boundary violations — 6 active areas

Most require kure-side additions before crane can be fixed; exceptions noted:

| Location | What crane does | Fix required |
|----------|----------------|--------------|
| `namespace.go:20-33` | `corev1.Namespace{}` struct literal | Expose existing `kure/internal/kubernetes/namespace.go:CreateNamespace` in `pkg/kubernetes/` |
| `helmrelease.go:417-586` | Raw `*unstructured.Unstructured` | Expand `HelmReleaseConfig` + `HelmRepositoryConfig` in kure |
| `configmap.go:91` | Raw `*unstructured.Unstructured` | Expose existing `kure/internal/kubernetes/configmap.go:CreateConfigMap` in `pkg/kubernetes/` |
| `externalsecret.go:293-349` | Direct `es.Spec` mutation | Add granular setters for RefreshInterval, Target, DataFrom in kure (`SetExternalSecretSpec` bulk setter exists but is insufficient) |
| `render_flux.go:197-241` | `sourcev1.OCIRepository{}`, `kustv1.Kustomization{}` struct literals | **Crane-only fix**: kure already has public builders `kurekfluxcd.OCIRepository()` and `kurekfluxcd.Kustomization()` at `pkg/kubernetes/fluxcd/create.go:75,88` — just use them |
| `infracomponents/` (all 3) | 12+ K8s struct literals | Expose namespace builder, expand Flux builders in kure |

### `classifyStackBuildError` fallthrough returns wrong code

Multiple domains identify this: the catch-all in `handler_stack_compile.go` returns `ErrInvalidBundleDefinition` for all unknown errors, misclassifying internal errors as client errors. Should be `ErrInternalError` as its final fallthrough.

### Security scanning asymmetry between `app.compile` and `stack.compile`

Domain 8 identifies that `stack.compile` never calls `ScanResources()`. Domain 6 confirms the NATS handler level is where app-compile scanning happens. Domain 5 confirms `stackcompile.Build()` has no import of `internal/security`. The security gap is consistent across all three analysis perspectives.

---

## Priority Fix List

Ordered by urgency and impact:

1. **[CRITICAL, Domain 3]** Fix `valueFrom` env var data loss in `webservice.go`, `worker.go`, `daemonset.go`, `cronjob.go` — replace inline `corev1.EnvVar{Name: env.Name, Value: env.Value}` loop with `buildEnvVars(c.Env)`. One-line fix per file, massive correctness impact.

2. **[CRITICAL, Domain 8]** Wire `security.ScanResources()` into `handleStackCompile` and `stackcompile.Build()`/`BuildMultiSplit()` — all `crane.stack.compile` requests currently bypass security scanning entirely.

3. **[CRITICAL, Domain 9]** Add `DefaultRegistry` field to `config.Config`, add `CRANE_DEFAULT_REGISTRY` env var mapping, and populate `nats.Config.DefaultRegistry` in `serve.go`. Without this, all NATS bootstrap requests using the server-side registry fallback fail in production.

4. **[CRITICAL, Domain 8]** Wire `CheckPSARestricted()` into the production pipeline — either inside `handler_compile.go` after `ScanResources()` or inside the scanner itself. PSA Phase 1 compliance is not enforced at runtime.

5. **[HIGH, Domain 2]** Fix `wharf.zone/managed-by` label value: change `"wharf"` → `"crane"` in `transformer.go:759`. Fix the dead assertion loop in `transformer_test.go:385-397` so it actually executes.

6. **[HIGH, Domain 7]** Add `errors.Is(err, bootstrap.ErrMissingRegistry)` check to `handler_bootstrap_render.go:117-126` to map it to `api.ErrMissingRegistry`.

7. **[HIGH, Domain 6]** Fix `publishCompiledEvent` (`events.go:132`): thread cluster ID from `handleCompile` call site (`target.Spec.Cluster`) through to the event instead of using `event.TenantID`.

8. **[HIGH, Domain 9]** Fix discarded health server error channel in `serve.go:80` — select on the error channel and log/terminate when the health server crashes.

9. **[HIGH, Domain 5]** Delete `BuildSplit` — it's exported dead code with inconsistent KS naming that is a correctness trap. Fold its one test into `BuildMultiSplit` coverage.

10. **[HIGH, Domain 5]** Add `ErrStackUnknownGroupApp` to `pkg/api/errors.go` and add its `errors.Is` case to `classifyStackBuildError` so it maps to the correct user-facing error code instead of `ErrInternalError`.

11. **[HIGH, Domain 9]** Delete `internal/catalog` and `internal/clusterprofile` packages — orphaned by ADR-023, zero production callers, actively misleading to contributors who might try to use them.

12. **[HIGH, Domain 4]** Add kure setters for `ExternalSecret.Spec.RefreshInterval`, `Target`, `DataFrom` (kure side) then remove direct spec mutation from `externalsecret.go`.

13. **[HIGH, Domain 7]** Add CI coverage for Cilium rendering that does not require a live OCI registry — mock the Helm renderer or use the existing chart URL fixtures.

14. **[HIGH, Domain 3]** Expand kure's `HelmReleaseConfig` to cover values, valuesFrom, driftDetection, chartRef mode, OCI source type, remediation — then migrate `helmrelease.go` away from raw unstructured.

15. **[MEDIUM, All]** Add `context.Context` as first parameter to `Transform`/`TransformWithPolicy`, `ComponentHandler.ToApplicationConfig`, and `stack.ApplicationConfig.Generate` interfaces before the caller surface grows further.

---

## Issue Tracking

Issues filed as a result of this review and the preceding [2026-05-05 analysis](https://gitlab.com/autops/wharf/crane/-/blob/main/docs/archive/development/20260505-ANALYSIS-crane-state.md).

### crane GitLab issues (pre-existing, filed 2026-05-05)

| Issue | Title | Coverage |
|-------|-------|----------|
| [#190](https://gitlab.com/autops/wharf/crane/-/work_items/190) | `valueFrom` env var data loss (4 handlers) | D3 webservice/worker/daemonset/cronjob |
| [#191](https://gitlab.com/autops/wharf/crane/-/work_items/191) | Runtime panic, security scanner bypass, DefaultRegistry missing, health server errCh | D7, D8, D9 |
| [#192](https://gitlab.com/autops/wharf/crane/-/work_items/192) | `managed-by` label wrong value, `version` label never set, dead test loop | D2 |
| [#193](https://gitlab.com/autops/wharf/crane/-/work_items/193) | `ErrStackUnknownGroupApp` ErrorCode missing, `ManifestSummary.APIVersion` json tag | D5, D6 |
| [#194](https://gitlab.com/autops/wharf/crane/-/work_items/194) | Delete dead `internal/catalog` and `internal/clusterprofile` (ADR-023 orphans) | D9 |
| [#195](https://gitlab.com/autops/wharf/crane/-/work_items/195) | Dead `kureVersion` variable, dead `OCIConfig` block | D6, D9 |
| [#196](https://gitlab.com/autops/wharf/crane/-/work_items/196) | Use existing kure builders in `httproute.go`, `networkpolicy.go`, `render_flux.go` | D4, D7 |
| [#197](https://gitlab.com/autops/wharf/crane/-/work_items/197) | AGENTS.md stale (handler names, security point, docs), README, DEVELOPMENT.md | D9 |
| [#198](https://gitlab.com/autops/wharf/crane/-/work_items/198) | `oamValidationError` incomplete, dead OAM exports, `ClusterID=TenantID`, buildPDB 50% | D1, D4, D6 |

### crane GitLab issues (new, filed 2026-05-06)

| Issue | Title | Coverage |
|-------|-------|----------|
| [#199](https://gitlab.com/autops/wharf/crane/-/work_items/199) | OAM validation gaps: namespace format, duplicate traits, policy cross-references | D1 |
| [#200](https://gitlab.com/autops/wharf/crane/-/work_items/200) | `TranslatedComponent.Namespace` contract not fulfilled (wrong namespace for manifests/oci) | D5 |
| [#201](https://gitlab.com/autops/wharf/crane/-/work_items/201) | Bootstrap: Cilium zero CI coverage; unknown phase values accepted silently | D7 |
| [#202](https://gitlab.com/autops/wharf/crane/-/work_items/202) | `collectBundles` flat-output silently drops child-bundle manifests | D9 |
| [#203](https://gitlab.com/autops/wharf/crane/-/work_items/203) | `CheckPSARestricted` not wired; placement policy not enforced in stack.compile | D8 |
| [#204](https://gitlab.com/autops/wharf/crane/-/work_items/204) | Missing per-tenant privilege ceiling; CloudEvent ID collision on retry | D6, D8 |

### kure GitHub issues (pre-existing, filed 2026-05-05)

| Issue | Title | Coverage |
|-------|-------|----------|
| [#493](https://github.com/go-kure/kure/issues/493) | `HelmRepositoryConfig`: add `Type` field for OCI registries | D3 helmrelease.go |
| [#495](https://github.com/go-kure/kure/issues/495) | ConfigMap builder: promote internal builder to `pkg/kubernetes` | D4 configmap.go |
| [#496](https://github.com/go-kure/kure/issues/496) | HelmRelease builder (scope expanded 2026-05-06 — see comments) | D3 helmrelease.go |
| [#497](https://github.com/go-kure/kure/issues/497) | VolumeClaimTemplate-embedded PVC builder | D3 statefulset.go |
| [#498](https://github.com/go-kure/kure/issues/498) | Namespace builder with PSA labels | D2 namespace.go, D5 infracomponents |
| [#499](https://github.com/go-kure/kure/issues/499) | Infrastructure component builders (scope expanded 2026-05-06 — see comments) | D5 stackcompile/builder.go, infracompile/layer3.go |

### kure GitHub issues (new, filed 2026-05-06)

| Issue | Title | Coverage |
|-------|-------|----------|
| [#500](https://github.com/go-kure/kure/issues/500) | `pkg/kubernetes/externalsecrets`: add granular setters for RefreshInterval, Target, DataFrom | D4 externalsecret.go |
| [#501](https://github.com/go-kure/kure/issues/501) | `pkg/kubernetes/cilium`: add CiliumNetworkPolicy builder | D4 cilium_networkpolicy.go |
| [#502](https://github.com/go-kure/kure/issues/502) | `pkg/kubernetes/cnpg`: add Pooler builder | D3 postgresql.go |
