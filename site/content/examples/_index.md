+++
title = "Examples"
weight = 40
+++

# Examples

Practical examples of using Kure to generate Kubernetes configurations.

## Mounted Examples

These examples include full documentation:

- [Getting Started](/examples/getting-started/) - The complete cluster-to-disk pipeline in Go: build a cluster, generate the layout, write it to disk
- [Patches](/examples/patches/) - Sample patch files for the patch library in go-kure/launcher

## Sample Data on GitHub

The [examples/ directory](https://github.com/go-kure/kure/tree/main/examples) also holds sample YAML under `examples/demo/`: app workloads, bootstrap, clusters and multi-OCI definitions. Kure has no command that reads these files; use them for the shape of an input, not as a tested interface.
