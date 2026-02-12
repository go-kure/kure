+++
title = "Getting Started"
weight = 10
+++

# Getting Started with Kure

Kure is primarily a Go library. You can also install its CLI tools for package management and code generation.

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

## Installing CLI Tools

```bash
# kure — main CLI for resource generation
go install github.com/go-kure/kure/cmd/kure@latest

# kurel — package system for reusable application bundles
go install github.com/go-kure/kure/cmd/kurel@latest
```

Verify the installation:

```bash
kure version
kurel version
```

## Next Steps

- Follow the [Quickstart](quickstart) guide
- Read about the [Domain Model](/concepts/domain-model) and [Design Philosophy](/concepts/design-philosophy)
- Explore the [Guides](/guides) for common workflows
- Try the [Examples](/examples)
