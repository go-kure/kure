+++
title = "Concepts"
weight = 20
+++

Understand the ideas behind Kure's design.

- [Architecture](/concepts/architecture/) - System architecture and component overview
- [Domain Model](/concepts/domain-model/) - The Cluster, Node, Bundle, Application hierarchy
- [Builder Contract](/concepts/builder-contract/) - Building an object, when sugar exists, reading the maturity table
- [Design Philosophy](/concepts/design-philosophy/) - The upstream type over templating, GitOps-native output
- [Package Status](/concepts/package-status/) - What is supported, limited, consumer-provided, not implemented or only proposed
- [Layout and Flux Naming](/concepts/layout-and-flux-naming/) - How directories, files and Flux objects are named
- [OCI Artifact Layout](/concepts/oci-layout/) - How manifests are packaged as OCI artifacts

## How the kure and launcher models relate

kure and [go-kure/launcher](https://github.com/go-kure/launcher) are two projects with two models, and
both use the word *Application* for something different.

| | kure | launcher |
|---|---|---|
| What it is | A Go library | An OAM-native package manager; its command line is `kurel` |
| Its model | Cluster, Node, Bundle, Application: Go types in `pkg/stack` | Application, ClusterProfile and Package documents, written in YAML |
| What an Application is | A Go struct with a name, a namespace and an `ApplicationConfig` that you implement to generate Kubernetes objects | A YAML document of kind `Application` in the API group `launcher.gokure.dev/v1alpha1` |
| When it runs | In your build, which writes plain YAML | At compile time, in your build or pipeline, when a package config and a site config are resolved into static manifests. No launcher component runs in the cluster |

The `launcher.gokure.dev/v1alpha1` API is launcher's own. OAM inspired it, and it is not
`core.oam.dev/v1beta1`: launcher does not claim compatibility with that API. launcher uses kure to
generate Kubernetes resources. The launcher documents describe launcher's model; start with its
[design document](https://github.com/go-kure/launcher/blob/main/docs/design.md).
