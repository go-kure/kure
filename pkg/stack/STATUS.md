# Stack Module Status

> This document reflects current implementation state. For roadmap, see open issues.

## Implemented

- Hierarchical domain model: Cluster → Node → Bundle → Application
- FluxCD workflow engine: full resource generation, layout integration, bootstrap (gotk and flux-operator modes)
- ArgoCD workflow engine: Application generation and layout integration; **bootstrap not implemented**
- GVK registry and ApplicationConfig system
- Generators: AppWorkload, FluxHelm, KurelPackage — all implemented; system deprecated (see kure#539)
- Layout generation with WriteToDisk, WriteToTar, WriteManifest
- Patch system, umbrella bundles, flatten-single-tier

## Known Limitations

- ArgoCD bootstrap (`GenerateBootstrap`) is not implemented and returns an error
- Generator system (`pkg/stack/generators`) is deprecated (kure#539)
