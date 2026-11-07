# Stack Module Status

> This document reflects current implementation state. For roadmap, see open issues.

## Implemented

- Hierarchical domain model: Cluster → Node → Bundle → Application
- FluxCD workflow engine: full resource generation, layout integration, bootstrap (gotk and flux-operator modes)
- ArgoCD workflow engine: Application generation and layout integration; **bootstrap not implemented**
- ApplicationConfig system (callers supply the implementations)
- Layout generation with WriteToDisk, WriteToTar, WriteManifest
- Umbrella bundles, flatten-single-tier

## Known Limitations

- ArgoCD bootstrap (`GenerateBootstrap`) is not implemented and returns an error
