+++
title = "Getting Started"
weight = 10
+++

# Getting Started with Kure

Kure is a Go library for programmatically building Kubernetes resources.

## Using as a Library

Add Kure to your Go project:

```bash
go get github.com/go-kure/kure
```

Then import the packages you need:

```go
import (
    "github.com/go-kure/kure/pkg/stack"
    "github.com/go-kure/kure/pkg/kubernetes/fluxcd"
    "github.com/go-kure/kure/pkg/io"
)
```

## Next Steps

- Follow the [Quickstart](/getting-started/quickstart/) guide
- Read about the [Domain Model](/concepts/domain-model) and [Design Philosophy](/concepts/design-philosophy)
- Explore the [Guides](/guides) for common workflows
- Try the [Examples](/examples)
