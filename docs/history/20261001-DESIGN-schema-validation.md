# Schema Validation of Generated Output — Design Notes

*Date: 2026-10-01 | Type: DESIGN | Scope: kure internal*

> **Note**: This document is not part of the kure documentation website. It is an internal
> development note about how go-kure/kure#462 ("validate generated manifests against
> flux-schema in CI") was decided. What the tests hold kure's output to is stated in
> `pkg/kubernetes/README.md` § Schema validation test; the helper is `internal/kuretest`.

---

## Decision

Kure's own test suite validates what kure emits the way a cluster would, in process and offline:

- **Tier 1, implemented.** Every object is validated as the YAML a user would apply, against the
  `CustomResourceDefinition` of the exact module version the kinds table pins, with the API
  server's own create-time routines (`k8s.io/apiextensions-apiserver` v0.37.1: decode-time
  coercion with strict field validation, defaulting, status dropped behind a status subresource,
  schema, ObjectMeta, scale paths, embedded ObjectMeta, list set and map invariants, and the
  `x-kubernetes-validations` CEL rules unless a blocking schema error stands). A kind whose
  module ships no definition gets the server's ObjectMeta validation and nothing more. A coverage
  test names every registered kind as schema-backed or uncovered, with the reason, and goes red
  when a table entry is stale, a definition's scope disagrees with the kinds table, or an
  uncovered module starts shipping definitions.
- **No CLI, no network.** The validator is a Go package (`internal/crdvalidate`) that depends on
  apimachinery and apiextensions-apiserver only. If a validation command is ever wanted it belongs
  in the consumer that wraps kure's capabilities, as an implementation of this one, and the
  package is shaped so it can be promoted to a public API then.
- **Strict, no allowlist.** Every fixture the validator rejected was fixed or completed; none was
  exempted.
- **Cross-object and layout consistency** (a Kustomization whose `spec.path` no directory
  renders, a `sourceRef` no emitted source matches) is a separate issue, prioritised next.
- **envtest is deferred.** The per-kind Go validation a real apiserver applies to built-in kinds
  is not reproduced. Reopen trigger: a built-in-kind defect in kure output reaches a consumer or a
  user.

Maintainer decisions of 2026-10-01: in-process over a CLI; tier 1 as above with the coverage test
and the envtest reopen trigger; complete the partial fixtures rather than allowlist them.

---

## The problem

go-kure/kure#462 proposed validating generated manifests against the schema catalog of
fluxcd/flux-schema, a Flux CLI plugin (v0.0.2 at the time, CLI-first, no Go library API, a catalog
of the stable Kubernetes APIs and the Flux ecosystem's definitions only), and parked the idea until
that tool had a usable Go API. The 2026-10-01 re-investigation of the 56 golden fixtures against
the exact pinned definitions found:

- **Two real defects.** The cnpg `ObjectStore` fixture carried `spec.configuration.serverName`,
  which the plugin-barman-cloud definition forbids with a CEL rule (`!has(self.serverName)`,
  message "use the 'serverName' plugin parameter in the Cluster resource"); the layer that wrote
  the fixture had set it anyway. The cilium `CiliumLocalRedirectPolicy` fixture and the published
  example used `protocol: ANY`, outside the definition's `TCP | UDP` enum.
- **Five partial fixtures.** The cilium `CiliumNetworkPolicy` and `CiliumClusterwideNetworkPolicy`
  `specs` entries lacked the selector and rule the schema's `oneOf`/`anyOf` require;
  `CiliumEgressGatewayPolicy` lacked `selectors` and `egressGateway.nodeSelector`;
  `CiliumEnvoyConfig` and `CiliumClusterwideEnvoyConfig` lacked `resources`. Each would have been
  rejected on apply.
- **Nothing in CI could see any of it.** The golden tests compared bytes with bytes; the identity
  and admission tests cover what a constructor and a helper write, not whether the result is
  valid.

Kure already held the exact definitions offline: the pinned modules ship their CRD manifests in the
module cache (cert-manager, cilium, cloudnative-pg, plugin-barman-cloud, flux-operator,
gateway-api, metallb, volsync), and the vendored flux2 install bundle carries the Flux toolkit's.

---

## Options considered

### 1. An external schema validator in CI (flux-schema, kubeconform)

Run a JSON-schema tool over rendered YAML in a CI job. Rejected: a second toolchain and a second
schema source to pin, drifting from the module versions kure actually compiles against, and a
catalog that covers Kubernetes and Flux but none of the other eight CRD modules kure pins; JSON
Schema cannot evaluate the `x-kubernetes-validations` CEL rules (the cnpg defect is one), the
list-type invariants or the pruning the apiserver applies; and it would validate rendered files,
not the objects the tests build, so a golden fixture could still be written invalid.

### 2. envtest

Start a real kube-apiserver with the pinned definitions and create every object. Catches
everything the server catches, built-in kinds included. Rejected for now: binaries to download per
Kubernetes version, tens of seconds of start-up per test binary against a 15-minute race-detector
budget, and a network dependency in the ordinary test job — for a gain that, on the evidence, is
the built-in kinds' per-kind Go rules (name formats stricter than a DNS subdomain for `Service` and
`Namespace`, for example), where no kure defect has been observed. Deferred with a reopen trigger.

### 3. The apiserver's validation routines in process — chosen

`k8s.io/apiextensions-apiserver` exposes the routines the server runs for a custom resource
create, and they take a definition and an unstructured object and nothing else: no etcd, no
network. `internal/crdvalidate` runs them in the server's order, compiling each served version on
first use as the server does when it starts serving a definition. The definitions come from the
module cache through `go list`, so the schema is always the one the pinned version ships; the
Flux toolkit's come from the install bundle already vendored for gotk-mode bootstrap, moved to the
module root (`internal/gotk`) so the bootstrap generator and the test helper can share it without
an import cycle.

One place it is deliberately stricter than the server: a `metadata.namespace` on a cluster-scoped
object is reported, where the server would drop it silently. Kure's identity test already asserts
no constructor emits one, and a caller who sets one has a bug the server would hide.

The dependency cost is one direct require, `k8s.io/apiserver` (for the CEL cost budgets), already
in every consumer's graph through apiextensions-apiserver, and one test-only direct require,
`github.com/envoyproxy/go-control-plane/envoy`, to build the real Envoy listener the completed
cilium fixtures carry.

---

## What this does not solve

- Built-in kinds (`k8s.io/api`, 43 kinds) and the `CustomResourceDefinition` kind: ObjectMeta
  validation only. Their per-kind rules live in the apiserver's Go code, not in a schema.
- external-secrets and prometheus-operator kinds (11): their API modules ship no definitions; the
  coverage test turns red the day they do.
- Admission webhooks, which several of the pinned operators run.
- Versions other than the pins, and a cluster that serves a different version of a definition.
- Gateway API experimental-channel fields: validation is against the standard channel.
- Cross-object and layout consistency, tracked separately.
- `kustomization.yaml`: kustomize configuration, skipped by group, not validated.
