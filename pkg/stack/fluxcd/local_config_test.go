package fluxcd_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// localConfigAnnotation is kustomize's local-config annotation: a build drops
// an object that carries it at any string value but "false".
const localConfigAnnotation = "config.kubernetes.io/local-config"

// annotateLocalConfig sets the local-config annotation on obj to value, or
// leaves obj without it when value is nil.
func annotateLocalConfig(obj client.Object, value *string) client.Object {
	if value != nil {
		obj.SetAnnotations(map[string]string{localConfigAnnotation: *value})
	}
	return obj
}

// localConfigCases is the annotation table every local-config test runs: an
// object kustomize builds is still judged, one it drops is not.
var localConfigCases = []struct {
	name    string
	value   *string
	dropped bool
}{
	{name: "absent", value: nil, dropped: false},
	{name: "false", value: ptr("false"), dropped: false},
	{name: "true", value: ptr("true"), dropped: true},
	{name: "empty", value: ptr(""), dropped: true},
}

func ptr(s string) *string { return &s }

// TestLocalConfig_IssueReproducer is go-kure/kure#1059 as reported: an
// application emits a local-config Kustomization under the name the
// integration generates for its bundle. kustomize drops it, so it is not an
// existing Kustomization.
func TestLocalConfig_IssueReproducer(t *testing.T) {
	for _, placement := range []layout.FluxPlacement{layout.FluxIntegratedPerLayout, layout.FluxIntegratedPerBundle} {
		for _, tc := range localConfigCases {
			t.Run(string(placement)+"/"+tc.name, func(t *testing.T) {
				decoy := stack.NewApplication("decoy", "default", &fakeAppConfig{objs: func() []*client.Object {
					o := annotateLocalConfig(fluxKustomization("applications-storefront", "./"), tc.value)
					return []*client.Object{&o}
				}()})
				storefront := &stack.Bundle{Name: "storefront", KustomizationName: "applications-storefront", SourceRef: testSR(), Applications: []*stack.Application{decoy, cmApp("storefront-app")}}
				apps := &stack.Node{Name: "applications", Bundle: storefront}
				c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", cmApp("platform-app")), Children: []*stack.Node{apps}}}
				rules := propertyGroupings["nodeOnly"]
				rules.FluxPlacement = placement
				ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, rules)
				checkLocalConfigVerdict(t, err, tc.dropped, `"applications-storefront"`)
				if err == nil {
					checkWritable(t, ml)
				}
			})
		}
	}
}

// TestLocalConfig_KustomizationNamesake: an application elsewhere in the tree
// emits a Kustomization with a generated CR's identity. Built, it collides
// under every placement; dropped by kustomize, it does not, and the generated
// CR is in the tree.
func TestLocalConfig_KustomizationNamesake(t *testing.T) {
	for _, placement := range allPlacements {
		for _, tc := range localConfigCases {
			t.Run(string(placement)+"/"+tc.name, func(t *testing.T) {
				platformApp := stack.NewApplication("platform-ks", "default", &fakeAppConfig{objs: func() []*client.Object {
					o := annotateLocalConfig(fluxKustomization("web-bundle", "elsewhere"), tc.value)
					return []*client.Object{&o}
				}()})
				web := &stack.Node{Name: "web", Bundle: srBundle("web-bundle", cmApp("web-app"))}
				c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", platformApp), Children: []*stack.Node{web}}}
				rules := propertyGroupings["nodeOnly"]
				rules.FluxPlacement = placement
				ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, rules)
				checkLocalConfigVerdict(t, err, tc.dropped, `"web-bundle"`)
				if err != nil {
					return
				}
				// The generated CR is there beside the dropped namesake.
				var built int
				walkLayouts(ml, func(l *layout.ManifestLayout) {
					for _, r := range l.Resources {
						if isKustomization(r, "web-bundle") && !droppedByBuild(r) {
							built++
						}
					}
				})
				if built != 1 {
					t.Errorf("got %d built web-bundle Kustomizations, want the generated one", built)
				}
				checkWritable(t, ml)
			})
		}
	}
}

// TestLocalConfig_SourceNamesake: an application emits a Source with a
// generated Source's identity but another URL. Built, it is a competing
// definition under every placement; dropped by kustomize, it is not, and the
// Source the build keeps is the generated one.
func TestLocalConfig_SourceNamesake(t *testing.T) {
	webRef := &stack.SourceRef{Kind: "GitRepository", Name: "web-git", Namespace: "flux-system", URL: "https://example.com/web.git", Branch: "main"}
	for _, placement := range allPlacements {
		for _, tc := range localConfigCases {
			t.Run(string(placement)+"/"+tc.name, func(t *testing.T) {
				other := gitRepository("web-git", "https://example.com/other.git")
				annotateLocalConfig(other, tc.value)
				var o client.Object = other
				platformApp := stack.NewApplication("sources", "default", &fakeAppConfig{objs: []*client.Object{&o}})
				web := &stack.Node{Name: "web", Bundle: &stack.Bundle{Name: "web", SourceRef: webRef, Applications: []*stack.Application{cmApp("web-app")}}}
				c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", platformApp), Children: []*stack.Node{web}}}
				rules := propertyGroupings["nodeOnly"]
				rules.FluxPlacement = placement
				ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, rules)
				checkLocalConfigVerdict(t, err, tc.dropped, `"web-git"`)
				if err != nil {
					return
				}
				var urls []string
				walkLayouts(ml, func(l *layout.ManifestLayout) {
					for _, r := range l.Resources {
						if r.GetObjectKind().GroupVersionKind().Kind == "GitRepository" && r.GetName() == "web-git" {
							if droppedByBuild(r) {
								continue
							}
							u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(r)
							if err != nil {
								t.Fatal(err)
							}
							url, _, _ := unstructured.NestedString(u, "spec", "url")
							urls = append(urls, url)
						}
					}
				})
				if len(urls) != 1 || urls[0] != webRef.URL {
					t.Errorf("built web-git URLs = %v, want only the generated %q", urls, webRef.URL)
				}
				checkWritable(t, ml)
			})
		}
	}
}

// TestLocalConfig_SourceNamesakeDelivery: the application emitting the
// Source namesake asks for prune protection and forced replacement. A copy
// kustomize builds is the application's Source, and the generated one, being
// the same object, carries the application's delivery annotations; a dropped
// copy is not, so the generated Source carries none of them. The dropped copy
// is annotated all the same: kustomize patches it before it drops it.
func TestLocalConfig_SourceNamesakeDelivery(t *testing.T) {
	webRef := &stack.SourceRef{Kind: "GitRepository", Name: "web-git", Namespace: "flux-system", URL: "https://example.com/web.git", Branch: "main"}
	for _, placement := range integratedPlacements {
		for _, tc := range localConfigCases {
			if !tc.dropped {
				continue // a competing definition, refused (TestLocalConfig_SourceNamesake)
			}
			t.Run(string(placement)+"/"+tc.name, func(t *testing.T) {
				other := gitRepository("web-git", "https://example.com/other.git")
				annotateLocalConfig(other, tc.value)
				var o client.Object = other
				platformApp := stack.NewApplication("sources", "default", &fakeAppConfig{objs: []*client.Object{&o}})
				platformApp.Delivery = stack.DeliveryIntent{PruneProtection: true, ForceReplace: true}
				web := &stack.Node{Name: "web", Bundle: &stack.Bundle{Name: "web", SourceRef: webRef, Applications: []*stack.Application{cmApp("web-app")}}}
				c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", platformApp), Children: []*stack.Node{web}}}
				rules := propertyGroupings["nodeOnly"]
				rules.FluxPlacement = placement
				ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, rules)
				if err != nil {
					t.Fatalf("CreateLayoutWithResources: %v", err)
				}
				generated := 0
				walkLayouts(ml, func(l *layout.ManifestLayout) {
					for _, r := range l.Resources {
						if r.GetObjectKind().GroupVersionKind().Kind != "GitRepository" || r.GetName() != "web-git" {
							continue
						}
						a := r.GetAnnotations()
						if droppedByBuild(r) {
							if a[stack.AnnotationFluxPruneKey] != stack.AnnotationFluxPruneDisabled || a[stack.AnnotationFluxForceKey] != stack.AnnotationFluxForceEnabled {
								t.Errorf("the dropped copy's annotations = %v, want the application's delivery annotations", a)
							}
							continue
						}
						generated++
						if _, ok := a[stack.AnnotationFluxPruneKey]; ok {
							t.Errorf("the generated web-git carries %s: %v", stack.AnnotationFluxPruneKey, a)
						}
						if _, ok := a[stack.AnnotationFluxForceKey]; ok {
							t.Errorf("the generated web-git carries %s: %v", stack.AnnotationFluxForceKey, a)
						}
					}
				})
				if generated != 1 {
					t.Errorf("found %d generated web-git, want 1", generated)
				}
			})
		}
	}
}

// TestLocalConfig_DuplicateNameOneDropped: one Kustomization name present
// twice. In directories applied separately, the integration refuses it when
// kustomize builds both copies and admits it when it drops one, and the tree
// is written. In one directory the writers refuse it either way: kustomize
// registers both copies before it drops one, and refuses the second.
func TestLocalConfig_DuplicateNameOneDropped(t *testing.T) {
	for _, placement := range allPlacements {
		for _, tc := range localConfigCases {
			t.Run(string(placement)+"/separate builds/"+tc.name, func(t *testing.T) {
				extra := func(name, path string, value *string) *stack.Application {
					return stack.NewApplication(name, "default", &fakeAppConfig{objs: func() []*client.Object {
						o := annotateLocalConfig(fluxKustomization("extra", path), value)
						return []*client.Object{&o}
					}()})
				}
				web := &stack.Node{Name: "web", Bundle: srBundle("web-bundle", cmApp("web-app"), extra("web-extra", "two", tc.value))}
				c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", extra("extra", "one", nil)), Children: []*stack.Node{web}}}
				rules := propertyGroupings["nodeOnly"]
				rules.FluxPlacement = placement
				ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, rules)
				checkLocalConfigVerdict(t, err, tc.dropped, `"extra"`)
				if err == nil {
					checkWritable(t, ml)
				}
			})
			t.Run(string(placement)+"/one directory/"+tc.name, func(t *testing.T) {
				extra := stack.NewApplication("extra", "default", &fakeAppConfig{objs: func() []*client.Object {
					a := fluxKustomization("extra", "one")
					b := annotateLocalConfig(fluxKustomization("extra", "two"), tc.value)
					return []*client.Object{&a, &b}
				}()})
				web := &stack.Node{Name: "web", Bundle: srBundle("web-bundle", cmApp("web-app"))}
				c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", extra), Children: []*stack.Node{web}}}
				rules := propertyGroupings["nodeOnly"]
				rules.FluxPlacement = placement
				ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, rules)
				checkLocalConfigVerdict(t, err, tc.dropped, `"extra"`)
				if err == nil {
					checkRefusedByWriters(t, ml, "holds the same object kustomize.toolkit.fluxcd.io/Kustomization flux-system/extra twice")
				}
			})
		}
	}
}

// TestLocalConfig_PlacementChange: a tree that holds a placed Kustomization is
// not re-placed, but one kustomize drops places nothing.
func TestLocalConfig_PlacementChange(t *testing.T) {
	for _, tc := range localConfigCases {
		t.Run(tc.name, func(t *testing.T) {
			rules := propertyGroupings["nodeOnly"]
			rules.FluxPlacement = layout.FluxIntegratedPerLayout
			c := propertyShapes["different-name"]()
			ml, err := layout.WalkCluster(c, rules)
			if err != nil {
				t.Fatal(err)
			}
			ml.Resources = append(ml.Resources, annotateLocalConfig(fluxKustomization("placed", "elsewhere"), tc.value))
			ml.FluxPlacement = layout.FluxIntegratedPerBundle
			err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
			checkLocalConfigVerdict(t, err, tc.dropped, `"placed"`)
		})
	}
}

// TestLocalConfig_BuildMarksFollowGeneratedCRs: a dropped namesake of a
// generated CR does not mark the directory its spec.path names as a Flux
// build. The directory is a plain one below the root, which nothing builds
// under FluxIntegratedPerBundle (FluxIntegratedPerLayout generates a CR for
// every layout, so there every directory is a build).
func TestLocalConfig_BuildMarksFollowGeneratedCRs(t *testing.T) {
	for _, tc := range localConfigCases {
		if !tc.dropped {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			rules := propertyGroupings["nodeOnly"]
			rules.FluxPlacement = layout.FluxIntegratedPerBundle
			c := propertyShapes["different-name"]()
			ml, err := layout.WalkCluster(c, rules)
			if err != nil {
				t.Fatal(err)
			}
			plain := &layout.ManifestLayout{Name: "hooks", Namespace: ml.FullRepoPath(), Resources: []client.Object{*cmObj("hooks-cm")}}
			ml.Children = append(ml.Children, plain)
			ml.Resources = append(ml.Resources, annotateLocalConfig(fluxKustomization("web-bundle", plain.FullRepoPath()), tc.value))
			if err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules); err != nil {
				t.Fatal(err)
			}
			if plain.FluxBuild() {
				t.Errorf("layout %q is marked a Flux build by a Kustomization kustomize drops", plain.FullRepoPath())
			}
		})
	}
}

// TestLocalConfig_UmbrellaTerminalsSkipsDroppedKustomizations: a dropped
// Kustomization is neither a member nor another CR of the tree
// UmbrellaTerminals reads: it hides no generated namesake and adds no child.
func TestLocalConfig_UmbrellaTerminalsSkipsDroppedKustomizations(t *testing.T) {
	rules := propertyGroupings["nodeOnly"]
	rules.FluxPlacement = layout.FluxIntegratedPerLayout
	plain := integrated(t, propertyShapes["umbrella"](), rules)
	want, err := fluxstack.UmbrellaTerminals(plain)
	if err != nil {
		t.Fatal(err)
	}

	ml := integrated(t, propertyShapes["umbrella"](), rules)
	// A dropped namesake of every generated CR, beside it.
	walkLayouts(ml, func(l *layout.ManifestLayout) {
		var namesakes []client.Object
		for _, r := range l.Resources {
			if r.GetObjectKind().GroupVersionKind().Kind == "Kustomization" {
				namesakes = append(namesakes, annotateLocalConfig(fluxKustomization(r.GetName(), "elsewhere"), ptr("true")))
			}
		}
		l.Resources = append(l.Resources, namesakes...)
	})
	got, err := fluxstack.UmbrellaTerminals(ml)
	if err != nil {
		t.Fatalf("UmbrellaTerminals with dropped namesakes: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("UmbrellaTerminals with dropped namesakes = %v, want %v", got, want)
	}
}

func checkLocalConfigVerdict(t *testing.T, err error, dropped bool, name string) {
	t.Helper()
	switch {
	case dropped && err != nil:
		t.Errorf("an object kustomize drops was judged: %v", err)
	case !dropped && (err == nil || !strings.Contains(err.Error(), name)):
		t.Errorf("got %v, want a refusal naming %s", err, name)
	}
}

// checkWritable writes ml: a tree the integration admits must be one the
// writers write.
func checkWritable(t *testing.T, ml *layout.ManifestLayout) {
	t.Helper()
	if err := ml.WriteToDisk(t.TempDir()); err != nil {
		t.Errorf("the integrated tree is not written: %v", err)
	}
}

// checkRefusedByWriters writes ml and wants the writers to refuse it with an
// error holding want.
func checkRefusedByWriters(t *testing.T, ml *layout.ManifestLayout, want string) {
	t.Helper()
	err := ml.WriteToDisk(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("writing the integrated tree: got %v, want a refusal holding %q", err, want)
	}
}

func walkLayouts(l *layout.ManifestLayout, fn func(*layout.ManifestLayout)) {
	if l == nil {
		return
	}
	fn(l)
	for _, c := range l.Children {
		walkLayouts(c, fn)
	}
}

// droppedByBuild reports whether a kustomize build drops obj: its local-config
// annotation is set to any value but "false".
func droppedByBuild(obj client.Object) bool {
	v, ok := obj.GetAnnotations()[localConfigAnnotation]
	return ok && v != "false"
}

// gitRepository returns a typed GitRepository flux-system/name with url.
func gitRepository(name, url string) *sourcev1.GitRepository {
	g := &sourcev1.GitRepository{}
	g.SetGroupVersionKind(sourcev1.GroupVersion.WithKind("GitRepository"))
	g.Name = name
	g.Namespace = "flux-system"
	g.Spec.URL = url
	g.Spec.Interval.Duration = time.Minute
	return g
}

func isKustomization(obj client.Object, name string) bool {
	gvk := obj.GetObjectKind().GroupVersionKind()
	return gvk.Group == "kustomize.toolkit.fluxcd.io" && gvk.Kind == "Kustomization" && obj.GetName() == name
}

// TestLocalConfig_IdenticalSourceCopy: the caller placed a dropped copy of a
// generated Source in the root layout, the build that hosts the generated
// one. It does not stand in for the generated one, which stays the one built
// copy. The writers then refuse the tree: kustomize registers both copies
// before it drops one, and refuses the second.
func TestLocalConfig_IdenticalSourceCopy(t *testing.T) {
	webRef := &stack.SourceRef{Kind: "GitRepository", Name: "web-git", Namespace: "flux-system", URL: "https://example.com/web.git", Branch: "main"}
	for _, placement := range integratedPlacements {
		for _, tc := range localConfigCases {
			if !tc.dropped {
				continue
			}
			t.Run(string(placement)+"/"+tc.name, func(t *testing.T) {
				gen, err := fluxstack.NewResourceGenerator().GenerateForBundle(&stack.Bundle{Name: "web", SourceRef: webRef}, "x")
				if err != nil {
					t.Fatal(err)
				}
				web := &stack.Node{Name: "web", Bundle: &stack.Bundle{Name: "web", SourceRef: webRef, Applications: []*stack.Application{cmApp("web-app")}}}
				c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", cmApp("platform-app")), Children: []*stack.Node{web}}}
				rules := propertyGroupings["nodeOnly"]
				rules.FluxPlacement = placement
				ml, err := layout.WalkCluster(c, rules)
				if err != nil {
					t.Fatal(err)
				}
				ml.Resources = append(ml.Resources, annotateLocalConfig(gen[1], tc.value))
				if err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules); err != nil {
					t.Fatal(err)
				}
				var built int
				walkLayouts(ml, func(l *layout.ManifestLayout) {
					for _, r := range l.Resources {
						if r.GetObjectKind().GroupVersionKind().Kind == "GitRepository" && r.GetName() == "web-git" && !droppedByBuild(r) {
							built++
						}
					}
				})
				if built != 1 {
					t.Errorf("got %d built copies of GitRepository web-git, want 1", built)
				}
				checkRefusedByWriters(t, ml, "holds the same object source.toolkit.fluxcd.io/GitRepository flux-system/web-git twice")
			})
		}
	}
}

// TestLocalConfig_UmbrellasDoNotWaitSkipsDroppedChild: under
// UmbrellasDoNotWait, an application's Kustomization that kustomize builds
// makes its bundle an umbrella, and a rewrite onto it can close a cycle; one
// it drops is no child, in whatever form the application hands it over.
func TestLocalConfig_UmbrellasDoNotWaitSkipsDroppedChild(t *testing.T) {
	for _, placement := range integratedPlacements {
		for _, form := range kustomizationForms {
			for _, tc := range localConfigCases {
				t.Run(string(placement)+"/"+form+"/"+tc.name, func(t *testing.T) {
					k := handWritten("hand-written", "web")
					annotateLocalConfig(k, tc.value)
					app := formApp("platform-app", k, form)
					_, err := integrateNoWait(noWaitCluster(srBundle("platform", app), true), nodeOnly, placement)
					checkLocalConfigVerdict(t, err, tc.dropped, `hand-written`)
				})
			}
		}
	}
}

// TestLocalConfig_UmbrellasDoNotWaitMembersAreGenerated: a dropped namesake
// of a generated umbrella, with a spec.path that builds nothing of the tree,
// does not stand in for it: web's dependsOn on umbrella platform is still
// rewritten onto platform's terminal leaf.
func TestLocalConfig_UmbrellasDoNotWaitMembersAreGenerated(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			build := func(decoy bool) *layout.ManifestLayout {
				u := umbrellaOf("platform", srBundle("a", cmApp("a-app")))
				c := noWaitCluster(u, true)
				if decoy {
					web := c.Node.Children[1].Bundle
					web.Applications = append(web.Applications, formApp("decoy", annotateLocalConfig(handWritten("platform"), ptr("true")).(*kustv1.Kustomization), "v1"))
				}
				ml, err := integrateNoWait(c, nodeOnly, placement)
				if err != nil {
					t.Fatal(err)
				}
				return ml
			}
			want := dependsOnNames(mustKustomization(t, build(false), "web"))
			got := dependsOnNames(mustKustomization(t, build(true), "web"))
			if !reflect.DeepEqual(got, want) {
				t.Errorf("web dependsOn with a dropped namesake of platform = %v, want %v", got, want)
			}
		})
	}
}
