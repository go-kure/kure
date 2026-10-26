# Stacktest - Generation Contract Checks

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/stack/stacktest.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/stack/stacktest)

Checks an `ApplicationConfig` against the generation contract from the config author's own tests.

The contract (`stack.ApplicationConfig`; the [Stack README](../README.md) § Generation contract)
says that what `Generate` returns belongs to the caller: kure writes to the returned objects, and
so may the caller. A config that hands out the same object, map, slice or pointer twice therefore
lets one result's edits reach another, and nothing fails at the point of the mistake. Neither does
a wrapper that hides the config's optional interfaces. This package finds both.

## Checking a config in a test

<!-- doc-example:excerpt the test-function form of AssertGeneration; every kure example config runs it, e.g. pkg/stack/example_test.go TestMyConfig_Conformance -->
```go
func TestMyConfig_Conformance(t *testing.T) {
    stacktest.AssertGeneration(t, stack.NewApplication("web", "default", &MyConfig{}))
}
```

`AssertGeneration` calls `Application.Generate` twice, so `Validate` runs before each call as in a
walk, and fails the test for an error or for every value it reports, naming both paths.
`CheckGeneration` returns the same findings as a `Report` instead:

<!-- doc-example: pkg/stack/stacktest ExampleCheckGeneration -->
```go
app := stack.NewApplication("web", "default", &cachingConfig{})
report, err := stacktest.CheckGeneration(app)
if err != nil {
    fmt.Println(err)
    return
}
for _, s := range report.Shared {
    fmt.Printf("%s shared: %s\n", s.Kind, s.First)
}
```
<!-- doc-example:end -->

`cachingConfig` keeps the labels map it built on the first call and puts it on every ConfigMap,
so this prints `map shared: [0].(*v1.ConfigMap).ObjectMeta.Labels`.

## What is reported

| Field | Holds |
|---|---|
| `Report.Shared` | values the two results share |
| `Report.Internal` | values one result reaches at two paths, such as one map used as both labels and selector |
| `Report.Interfaces` | the optional interfaces the config implements |

Each `Sharing` names the `Kind` of the value (`object`, `map`, `slice` for a slice's backing
array, `pointer` for any other pointer) and its two paths. A path starts at the result slice:
`[1].(*v1.Deployment).Spec.Selector.MatchLabels` is the selector map of the Deployment the
second entry holds, and the empty path is the result slice itself.

Storage counts as shared when it overlaps, not only when two pointers are equal: a re-sliced
cached slice, a pointer into another result's slice and a pointer into a struct another result
holds are all reported. A value inside one already reported at both of its paths is not listed
again, so a shared `*Deployment` is one entry, not one per map inside it.

## Wrappers and optional interfaces

kure finds `stack.Validator`, `layout.LayoutAugmenter` and `layout.LayoutIntentAugmenter` by type
assertion on the config itself. A wrapper that embeds the `ApplicationConfig` interface hides
them; compare it with the config it wraps:

<!-- doc-example: pkg/stack/stacktest ExampleInterfacesOf -->
```go
inner := &validatedConfig{}
missing := stacktest.InterfacesOf(wrapped{inner}).Missing(stacktest.InterfacesOf(inner))
fmt.Println(missing)
```
<!-- doc-example:end -->

`wrapped` embeds `stack.ApplicationConfig` and `validatedConfig` implements `Validate`, so this
prints `[stack.Validator]`.

## Limits

- Two calls are compared, not every call: a config that shares only from its third call on
  passes.
- A `time.Time` (so a `metav1.Time`) is not walked: `time.Now` gives every value the same
  package-level location, and a deep copy shares it too. Strings are immutable and are not
  checked.
- A value that contains itself is walked once and is not reported.
- A func, channel or unsafe pointer in a result is returned as an error, and so is a map with two
  keys that print alike (two NaN keys, or `1` and `"1"` in a `map[any]any`), whose entries would
  share a path. Kubernetes objects hold none of these.
- Concurrency safety and what an augmenter adds to the layout are not checked.
