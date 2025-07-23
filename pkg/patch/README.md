# patch package

The `patch` package provides helpers for modifying Kubernetes objects using a very
small YAML based syntax. It is used by the `kure` CLI but can also be consumed
directly from Go code.

Patches operate on [`*unstructured.Unstructured`](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1/unstructured)
objects and support replacing values, deleting fields, and manipulating list
entries. Each patch is applied to an existing base resource; no templating or
overlay system is required.

## Loading resources and patches

Resources can be loaded from one or more YAML documents using
`LoadResourcesFromMultiYAML`. Patch instructions are loaded with
`LoadPatchFile`. Both helpers accept an `io.Reader`, so inputs may come from
files or memory.

```go
resources, _ := patch.LoadResourcesFromMultiYAML(r)
ops, _ := patch.LoadPatchFile(p)
set, _ := patch.NewPatchableAppSet(resources, ops)
```

`LoadPatchableAppSet` is a convenience that performs all of the above steps in
one call.

## Patch format

Patches are written as a simple mapping or as a list of targeted patches. The
following YAML updates two objects:

```yaml
- target: demo-config
  patch:
    data.foo: qux
    metadata.labels.env: prod
- target: demo-deploy
  patch:
    spec.replicas: 3
    spec.template.spec.containers[0].image: myapp:v2
```

Paths may include list selectors. Supported operations are:

- `replace` (default)
- `delete` or `delete=selector`
- `insertBefore` / `insertAfter`
- `append` via the `[-]` suffix

Examples:

```yaml
spec.template.spec.containers[+=name=main]: { name: sidecar, image: sidecar:v1 }
metadata.labels[delete=app]: ""
```

Targets are inferred from the first path segment (e.g. `deployment.app`), but a
`target:` field can be provided when multiple resources match.

## Applying patches

Call `Resolve` on a `PatchableAppSet` to group patches with their target
resources, then `Apply` to mutate the objects:

```go
resolved, _ := set.Resolve()
for _, r := range resolved {
    _ = r.Apply()
}
```

Set `KURE_DEBUG=1` to log patch resolution details.

Further examples are available in [`examples/patch`](../../examples/patch).
