+++
title = "Patching Resources"
weight = 40
+++

kure does not patch resources. The patch library, `pkg/patch`, lives in
[go-kure/launcher](https://github.com/go-kure/launcher), which owns its formats, its API and its
documentation. Read it there:

- [`pkg/patch` README](https://github.com/go-kure/launcher/tree/main/pkg/patch) - what the library
  does, the patch file formats and how to call it
- [Go package reference](https://pkg.go.dev/github.com/go-kure/launcher/pkg/patch) - the exported API

## What this is not

`kurel build` does not run `pkg/patch`, and there is no patch subcommand. The library is for callers
that modify Kubernetes resources themselves, outside the build path.

It is also not any of these, which are separate mechanisms with their own documentation:

- the `patches` field of a Flux `Kustomization`, which the Flux kustomize controller applies in the
  cluster
- Helm post-rendering

## Choosing between patching and generating

- **Set it at generation time** when you control the builder: set the replica count or the image on
  the object kure returns.
- **Patch afterwards** only when the resources come from somewhere you do not control. kure does not
  help with that step.

## Sample files

The [patch examples](/examples/patches/) in this repository are sample input files for launcher's
library. kure does not read them, and launcher's documentation is the reference for their format.
