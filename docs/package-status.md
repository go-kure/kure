# Package status

What each part of kure does today, what it leaves to you, and what is not built. The rows describe
the code at the revision the page was built from.

## Words used on this site

| Word | Meaning |
|---|---|
| **Development** | Built from the latest commit on `main`. The notice at the top of every page of the development site names the source commit. |
| **Published prerelease** | A tagged release candidate or beta. There is no stable release yet, so every API can still change. |
| **Current supported behavior** | What the code at the built revision does. A row below says *Supported* only for that. |
| **Proposed** | A design that was written down and not built. Its own file says "proposed, not shipped", and its Go names do not resolve against `pkg/`. |

A status in the table is one of four:

- **Supported**: implemented, documented and tested.
- **Limited**: implemented with a gap the row names.
- **Consumer-provided**: kure defines the interface, and you write the implementation.
- **Not implemented**: nothing in the code does it.

## Packages

| Area | Package | Status | What it means |
|---|---|---|---|
| Domain model | `pkg/stack` | Supported | Cluster, Node, Bundle, Application. `ApplicationConfig` is consumer-provided: you implement it, and `pkg/stack/stacktest` checks the generation contract. |
| Flux workflow | `pkg/stack/fluxcd` | Supported | Generates the Flux objects and the bootstrap manifests, in gotk and flux-operator modes. |
| Layout | `pkg/stack/layout` | Supported | Writes the generated tree with `WriteToDisk`, `WriteToTar` and `WriteManifest`. |
| Argo CD workflow | `pkg/stack/argocd` | Limited | Generates one Application for each layout directory that renders bundles. Bootstrap is not implemented: `GenerateBootstrap` returns an error when bootstrap is enabled. The repository URL is a placeholder until you call `SetRepoURL`. |
| Helm rendering | `pkg/stack/helm` | Supported | A library: `RenderChart` renders a chart without a cluster, and `SplitByHookWeight` groups the result by hook phase. No workflow calls them; turning a hook group into a Flux Kustomization is your code. |
| Kubernetes builders | `pkg/kubernetes` and its operator subpackages | Supported | Constructors, per-kind wrappers and sugar helpers. The [kinds table](/api-reference/api-tables/) lists every registered kind. |
| Serialization | `pkg/io`, `pkg/manifest` | Supported | YAML and JSON output, resource printing, CRD and scope classification. |
| Errors, logging, versions | `pkg/errors`, `pkg/logger`, `pkg/versions` | Supported | Structured errors, a logger, and the supported-version metadata. |
| Group, version, kind registry | `pkg/gvk` | Supported | A standalone registry and YAML type detection. No other package in this repository imports it. |
| Command line | none | Not implemented | kure ships no CLI. `kurel` belongs to go-kure/launcher. |
| Plugin architecture, end-user experience design | `docs/plugin-architecture-design.md`, `docs/ux-design.md` | Proposed | Designs that were never built. Do not read them as a description of the library. |

Argo CD, Helm and `pkg/gvk` have a README in the repository that this site does not publish.
