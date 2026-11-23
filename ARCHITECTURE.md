# Kure architecture

A map of the code as it is on `main`. Read this first, then follow the links. It describes what exists, not
what is planned. The long reference, with the extension recipes, the security model and the testing notes, is
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md). A change that touches the Map, the flows, the rules or the
boundaries below updates this file in the same pull request.

## What this is

Kure is a Go library that describes a GitOps repository as a tree of clusters, nodes, bundles and applications,
and writes that tree as plain YAML on disk. It has a domain model, a layout engine, and a thin layer for building
Kubernetes objects. It has no command line of its own.

## Map

| Path | What it owns | Must not | More |
|---|---|---|---|
| `pkg/stack` | The model: `Cluster`, `Node`, `Bundle`, `Application`, the `ApplicationConfig` and `Workflow` interfaces, and the fluent `ClusterBuilder`. | Import `pkg/stack/layout` (the layout imports the model). | [README](pkg/stack/README.md) |
| `pkg/stack/layout` | Turns a cluster into a tree of directories and files, and writes it to disk or to a tar stream. | | [README](pkg/stack/layout/README.md) |
| `pkg/stack/fluxcd` | The Flux workflow: Kustomizations and their sources, and the Flux bootstrap manifests. Registers itself on import. | | [README](pkg/stack/fluxcd/README.md) |
| `pkg/stack/argocd` | The Argo CD workflow: one Application per directory that renders bundles. Registers itself on import. Bootstrap is not implemented. | | [README](pkg/stack/argocd/README.md) |
| `pkg/stack/helm` | Renders a Helm chart to YAML without a cluster, for callers that build their own GitOps objects. No kure package imports it today. | | [README](pkg/stack/helm/README.md) |
| `pkg/stack/stacktest` | Checks from a config author's own tests that `Generate` returns objects that nothing else shares. | | [README](pkg/stack/stacktest/README.md) |
| `pkg/kubernetes` | The scheme, the generic `Create[T]`, the per-kind constructors (generated), and the admitted `Set*`/`Add*` helpers. | Default, validate or hide a field: a constructor sets identity only. | [README](pkg/kubernetes/README.md) |
| `pkg/kubernetes/{certmanager,cilium,cnpg,externalsecrets,fluxcd,metallb,prometheus,volsync}` | The same for the eight CRD families kure builds. | The same. | [docs-map](site/docs-map.yaml) |
| `pkg/io` | Reads and writes YAML for Kubernetes objects. | | [README](pkg/io/README.md) |
| `pkg/manifest` | Recognises CRDs and tells whether an object is namespaced. | | [README](pkg/manifest/README.md) |
| `pkg/gvk` | A registry of group, version and kind, with typed wrappers. No other kure package imports it. | | [README](pkg/gvk/README.md) |
| `pkg/errors` | Structured error types and wrapping helpers. | | [README](pkg/errors/README.md) |
| `pkg/logger` | A structured logger. No library package calls it; only `examples/getting-started` does. | | [README](pkg/logger/README.md) |
| `pkg/versions` | The supported-version data as a Go API, generated from `versions.yaml`. | | [README](pkg/versions/README.md) |
| `pkg/kubernetes/internal/*` | The generator and what it reads: `gen`, `kinds`, `markers`, `upstream`, `maturity`, and the `admission` classifier. Build time and tests only. | Be imported by a consumer. | doc comments |
| `internal/{built,nilobj,kustomizeschema,gotk}` | Code the library itself imports: how kustomize reads a written resource, the nil-object test, kustomize's schema setup, the embedded Flux toolkit manifests. | Be imported by a consumer. | doc comments |
| `internal/{crds,crdvalidate,kuretest,releaseconfig}` | Test support that validates output against the pinned CRDs, and a check of the release configuration. `crds` is also read by the generator, to find the scope of a kind. | Be imported by a consumer. | doc comments |

Outside the code: `docs/` (reference, reviews, history), `examples/` (runnable programs), `site/` (the
documentation site and `docs-map.yaml`), `scripts/` (generators and checks), `versions.yaml` and `VERSION` (the
version data).

## How data moves

**1. From a model to a Flux tree on disk.** `examples/getting-started/main.go` runs this end to end.

1. `stack.NewClusterBuilder` builds a `*stack.Cluster`. Each `Application` holds an `ApplicationConfig`.
2. Import `pkg/stack/fluxcd` for its effect, then call `stack.NewWorkflow("flux")` (or
   `fluxcd.NewWorkflowEngine()`).
3. `layout.DefaultLayoutRules()` gives the rules that decide which levels get a directory.
4. `CreateLayoutWithResources(cluster, rules)` first calls `layout.WalkCluster`, which validates the cluster, asks
   each application's config for its objects (`ApplicationConfig.Generate`) and builds the directory tree. Then
   the Flux integrator (`IntegrateWithLayout` in `pkg/stack/fluxcd`) adds the Flux Kustomizations to it.
5. `WriteToDisk(basePath)` first refuses a tree in which two layouts land in one directory, then writes each
   directory. Each gets a `kustomization.yaml`, except a recursive one, where Flux generates it.
6. Resource files are written with `io.EncodeObjectsToYAML`. The layout writer writes each `kustomization.yaml`
   as text, and writes the extra files an augmenter supplies byte for byte. `WriteToTar` writes the tree into a
   stream. `GenerateBootstrap` produces the manifests that install Flux itself.

**2. Building one Kubernetes object.**

1. `kubernetes.RegisterSchemes` fills the shared scheme the first time any constructor needs it.
2. `kubernetes.Create[T](name, namespace)` allocates the type with its type metadata, name and namespace, and
   nothing else. The per-kind wrappers (`CreateDeployment`, ...) are one-line calls to it.
3. The caller sets fields on the upstream struct, with the admitted helpers where one fits.
4. The object comes back from an `ApplicationConfig.Generate`.

**3. Build-time generation.**

1. `pkg/kubernetes/internal/gen` reads the scheme, `internal/kinds` and the pinned API module sources.
2. It writes the `zz_generated_*` files and `docs/api-tables.{json,md}` (`./scripts/gen-builders.sh generate`).
3. `./scripts/sync-versions.sh generate` writes the version documents and the `pkg/versions` Go API from
   `versions.yaml` and `go.mod`.
4. CI runs both scripts with `check` and fails when a generated file is stale.

## Rules the code keeps

| Rule | Where it is enforced |
|---|---|
| Only helpers of an admitted class are exported as `Set*`/`Add*`. | `TestAdmission_SugarHelpersAreClassAdmissible` in `pkg/kubernetes/admission_test.go`. It skips under `go test -short`. |
| A constructor sets identity and nothing else; every registered kind has a wrapper, except three deprecated kinds that a newer version of the same kind replaces (MetalLB `BGPPeer` v1beta1, Flux `Alert` and `Provider` v1beta3, listed in `pkg/kubernetes/internal/kinds/skips.go`). | `TestIdentity_ConstructorsEmitIdentityOnly` and `TestIdentity_EveryRegisteredKindHasAWrapper` in `pkg/kubernetes/identity_test.go`. |
| Generated files match their generator. | CI job `validate`: `gen-builders.sh check` and `sync-versions.sh check`. |
| `pkg/stack` never imports `pkg/stack/layout`. | The Go compiler (an import cycle). |
| The layout refuses two layouts in one directory before it writes anything. | `checkLayoutTree` in `pkg/stack/layout/treecheck.go`, called from `WriteToDisk` in `manifest.go`. |
| Emitted objects are valid against the CRDs of the pinned API modules. This holds for the Flux, Cilium, cert-manager, CloudNativePG, MetalLB and VolSync families. The built-in Kubernetes types, External Secrets and Prometheus Operator have no definitions to read: only their metadata is checked. | Tests that use `internal/kuretest`. |
| An `ApplicationConfig` returns objects nothing else shares. | `stacktest.CheckGeneration`, but only in the tests of whoever calls it. |
| Every public package has an entry in `site/docs-map.yaml`. | CI job `docs-build` (`check-doc-sync`). |
| A non-test source change to a mapped package touches that package's README or a mapped guide. | CI job `doc-gate`. Test files and a generated change that only moves a module version are exempt, the maintainer `docs-skip` label bypasses it, and the `internal/` packages at the module root are not mapped. The wider rule, code and documentation in one pull request, is review policy. |
| Coverage is at least 90 percent in total and for each package outside `examples/`. | CI job `coverage-check`. |
| No downstream project is named. | CI job `forbidden-terms`. |
| Kure does not import launcher. | Review only: `go.mod` and the sources hold no import, and no check exists. |
| Errors come from `pkg/errors`, not `fmt.Errorf`. | Review only. `pkg/stack/builders.go` still has 13 `fmt.Errorf` sites. |
| The library does not log. | Review only. |

## Boundaries

- **Import paths.** The packages in the Map without `internal` are the public surface. Each has an entry in
  `site/docs-map.yaml`.
- **Files written.** The layout tree: one directory per level the rules choose, a `kustomization.yaml` in each
  except a recursive layout, where Flux generates it (`pkg/stack/layout/manifest.go`), the resource files, any
  extra files an augmenter supplies, and optionally one tar stream (`pkg/stack/layout/tar.go`).
- **Files and data read.** YAML through `pkg/io`; the Flux toolkit manifests embedded from
  `internal/gotk/manifests.tar.gz`; the Flux Operator install file embedded in `pkg/stack/fluxcd`; Helm charts
  from an OCI registry or an HTTP repository, in `pkg/stack/helm`.
- **Build-time data.** `versions.yaml`, `VERSION`, and the pinned API modules in `go.mod`.
- **No schema of its own.** Kure writes the upstream API types. It defines no custom resource.
- **Consumers.** [go-kure/launcher](https://github.com/go-kure/launcher) imports `pkg/stack`, `pkg/kubernetes`
  and the CRD families, `pkg/io`, `pkg/manifest`, `pkg/stack/layout` and `pkg/stack/helm`.

## Where to start reading

1. [`README.md`](README.md) and [`pkg/stack/doc.go`](pkg/stack/doc.go): the model in one page.
2. [`pkg/stack/workflow.go`](pkg/stack/workflow.go): the `Workflow` interface that Flux and Argo CD implement.
3. [`pkg/stack/layout/walker.go`](pkg/stack/layout/walker.go) (`WalkCluster`), then
   [`manifest.go`](pkg/stack/layout/manifest.go) (`WriteToDisk`): how a tree becomes files.
4. [`pkg/kubernetes/doc.go`](pkg/kubernetes/doc.go) and [`create.go`](pkg/kubernetes/create.go): how an object is built.
5. [`site/docs-map.yaml`](site/docs-map.yaml): which page documents which package.
