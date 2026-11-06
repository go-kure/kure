package fluxcd_test

import (
	"fmt"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for go-kure/kure#1247: under UmbrellasDoNotWait a Kustomization the
// tree does not hold must not stand in for the copy a build makes of a member
// when that copy has lost the wait the rewrite owed it.

// labelDependsOnZero copies the name of the first dependsOn entry into the
// label d of the object it patches, and replaces its spec.path with path. The
// label reads the umbrella before the rewrite and its first terminal leaf after.
func labelDependsOnZero(path string) string {
	return "- op: add\n  path: /metadata/labels\n  value: {}\n" +
		"- op: copy\n  from: /spec/dependsOn/0/name\n  path: /metadata/labels/d\n" +
		"- op: replace\n  path: /spec/path\n  value: " + path + "\n"
}

// A hand-written Kustomization that builds the directory holding s with the name
// prefix x- and two patches. The first labels the copy with its first dependsOn
// entry and points its spec.path at a directory no writer builds, which holds
// the Kustomization z, so the copy is one that writes a Kustomization. The
// second selects the label d=y-infra and removes spec.wait: before the rewrite
// d is the umbrella and nothing is removed; after it d is the first terminal
// leaf and the copy x-s loses its wait. The copy is not a copy of an umbrella,
// so the loss is refused. Without the replaced spec.path, and without the
// second patch, the tree is as it was before.
func TestStandIn_NonMemberLosesWaitThroughALabelSelectedPatch(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		replacePath, dropWait bool
		refused               bool
	}{
		"path replaced, wait removed":    {true, true, true},
		"path kept, wait removed (ctl)":  {false, true, true},
		"path replaced, wait kept (ctl)": {true, false, false},
		"path kept, wait kept (ctl)":     {false, false, false},
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				rules := withPlacement(nodeOnly, placement)
				build := func(host, stray string) *stack.Cluster {
					c := builtClassCluster([]string{"v"})
					if host == "" {
						return c
					}
					k := handWritten("hand-written")
					k.Spec.Path, k.Spec.NamePrefix = host, "x-"
					first := labelDependsOnZero(stray)
					if !tc.replacePath {
						first = "- op: add\n  path: /metadata/labels\n  value: {}\n" +
							"- op: copy\n  from: /spec/dependsOn/0/name\n  path: /metadata/labels/d\n"
					}
					k.Spec.Patches = []kustomize.Patch{{Patch: first, Target: &kustomize.Selector{Kind: "Kustomization", Name: "s"}}}
					if tc.dropWait {
						k.Spec.Patches = append(k.Spec.Patches, kustomize.Patch{
							Patch:  "- op: remove\n  path: /spec/wait\n",
							Target: &kustomize.Selector{Kind: "Kustomization", LabelSelector: "d=y-infra"},
						})
					}
					c.Node.Children = append(c.Node.Children, &stack.Node{Name: "patcher", Bundle: srBundle("patcher", objApp("patcher-app", k))})
					return c
				}
				host := hostPathOf(t, func() *stack.Cluster { return build("", "") }, rules, "s")
				// The stray layout's path depends on the root layout alone.
				stray := strayLayout(mustWalk(t, build("", ""), rules), handWritten("z")).FullRepoPath()
				c := build(host, stray)
				ml := mustWalk(t, c, rules)
				strayLayout(ml, handWritten("z"))
				err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules)
				if tc.refused {
					wantRefusedWith(t, err, `Flux Kustomization "flux-system/x-s" has a different spec.wait after the rewrite than before it`)
					return
				}
				if err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
			})
		}
	}
}

// A hand-written Kustomization builds the directory holding s with the name
// prefix x-, and the first patch labels the copy x-s with its first dependsOn
// entry and gives it the spec.path of the generated umbrella platform, so x-s
// applies Flux Kustomizations. The second patch selects the label d=y-infra,
// which only the rewritten dependsOn gives, and removes spec.healthChecks or
// spec.wait: x-s loses what it held. A copy of platform that a build makes has
// the whole spec of platform before the rewrite, and only it loses them without a
// refusal. x-s is a leaf of its own, with a spec of its own, whatever else it
// shares with platform: with a health check added, with the dependsOn of s (it
// keeps platform's path and nothing else), or with the dependsOn of platform and
// another interval, it is refused. With the dependsOn of platform removed from
// it as well, nothing of its spec differs from platform's: it is a duplicate of
// the umbrella and loses what the umbrella loses. The specs are compared as the
// typed Kustomization encodes them.
func TestStandIn_LeafWithItsOwnReadinessIsNotTakenForACopyOfAnUmbrella(t *testing.T) {
	t.Parallel()
	const check = "- op: add\n  path: /spec/healthChecks\n" +
		"  value: [{apiVersion: apps/v1, kind: Deployment, name: manual-check, namespace: default}]\n"
	// platform depends on nothing before the rewrite, so removing s's own
	// dependsOn gives x-s the dependsOn of platform.
	const noDeps = "- op: remove\n  path: /spec/dependsOn\n"
	const interval = "- op: add\n  path: /spec/interval\n  value: 17m0s\n"
	for name, tc := range map[string]struct {
		addCheck, noDeps, interval bool
		drop                       string
		refused                    string
	}{
		"check added, check removed":                                {true, false, false, "healthChecks", "spec.healthChecks"},
		"check added, wait removed":                                 {true, false, false, "wait", "spec.wait"},
		"check added, kept (ctl)":                                   {true, false, false, "", ""},
		"no check, wait removed":                                    {false, false, false, "wait", "spec.wait"},
		"no check, nothing removed (ctl)":                           {false, false, false, "", ""},
		"same dependsOn, interval changed, wait removed":            {false, true, true, "wait", "spec.wait"},
		"same dependsOn, interval changed, kept (ctl)":              {false, true, true, "", ""},
		"same dependsOn and interval, wait removed (duplicate ctl)": {false, true, false, "wait", ""},
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				rules := withPlacement(nodeOnly, placement)
				build := func(host, umbrellaPath string) *stack.Cluster {
					c := builtClassCluster([]string{"v"})
					if host == "" {
						return c
					}
					k := handWritten("hand-written")
					k.Spec.Path, k.Spec.NamePrefix = host, "x-"
					first := labelDependsOnZero(umbrellaPath)
					if tc.addCheck {
						first += check
					}
					if tc.noDeps {
						first += noDeps
					}
					if tc.interval {
						first += interval
					}
					k.Spec.Patches = []kustomize.Patch{{Patch: first, Target: &kustomize.Selector{Kind: "Kustomization", Name: "s"}}}
					if tc.drop != "" {
						k.Spec.Patches = append(k.Spec.Patches, kustomize.Patch{
							Patch:  "- op: remove\n  path: /spec/" + tc.drop + "\n",
							Target: &kustomize.Selector{Kind: "Kustomization", LabelSelector: "d=y-infra"},
						})
					}
					c.Node.Children = append(c.Node.Children, &stack.Node{Name: "patcher", Bundle: srBundle("patcher", objApp("patcher-app", k))})
					return c
				}
				probed, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(build("", ""), rules)
				if err != nil {
					t.Fatalf("CreateLayoutWithResources: %v", err)
				}
				platform := findKustomization(probed, "platform")
				if platform == nil {
					t.Fatal("no generated Kustomization platform")
				}
				host := hostPathOf(t, func() *stack.Cluster { return build("", "") }, rules, "s")
				c := build(host, platform.Spec.Path)
				_, err = fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(c, rules)
				if tc.refused != "" {
					wantRefusedWith(t, err, `Flux Kustomization "flux-system/x-s" has a different `+tc.refused+` after the rewrite than before it`)
					return
				}
				if err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
			})
		}
	}
}

// The build before the rewrite fails (remove /spec/dependsOn/2 is out of range
// for [platform v] and valid for [y-infra y-services v]), so the pass judges
// the written entries from the tree alone. The copy x-s loses its wait on v. The
// tree also holds an x-s in a directory no writer builds, a clone of the
// generated s whose dependsOn is [y-infra y-services], what the copy has after
// the patch: it must not stand for the copy. With the dependsOn the copy does
// not have, [y-infra y-services v], it never did. With no clone held at all the
// copy is not held either, and is refused as before.
//
// The clone is generated under the generator the tree is integrated with
// (noWaitGenerator): the default one leaves an unset wait false, this one
// resolves it to true (waitFor, resource_generator.go), and graphFields
// includes wait, so a clone from the default generator would differ from the
// copy on wait as well and be refused for that reason alone. With one generator
// on both sides, dependsOn is the only graph field on which the two rows differ.
func TestStandIn_HeldNonMemberDoesNotHideALostWaitOnTheFallback(t *testing.T) {
	for name, held := range map[string][]string{
		"held depends on the copy's list after the patch": {"y-infra", "y-services"},
		"held depends on the whole list (ctl)":            {"y-infra", "y-services", "v"},
		"nothing held (ctl)":                              nil,
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				rules := withPlacement(nodeOnly, placement)
				patch := "- op: remove\n  path: /spec/dependsOn/2\n"
				build := func(path string) *stack.Cluster {
					c := builtClassCluster([]string{"v"})
					if path != "" {
						k := handWritten("hand-written")
						k.Spec.Path, k.Spec.NamePrefix = path, "x-"
						k.Spec.Patches = []kustomize.Patch{{Patch: patch, Target: &kustomize.Selector{Kind: "Kustomization", Name: "s"}}}
						c.Node.Children = append(c.Node.Children, &stack.Node{Name: "patcher", Bundle: srBundle("patcher", objApp("patcher-app", k))})
					}
					return c
				}
				probe := build("")
				probed, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(probe, rules)
				if err != nil {
					t.Fatalf("CreateLayoutWithResources: %v", err)
				}
				generated := findKustomization(probed, "s")
				if generated == nil {
					t.Fatal("no generated Kustomization s")
				}
				clone := generated.DeepCopy()
				clone.Name = "x-s"
				clone.Spec.DependsOn = handWritten("held", held...).Spec.DependsOn
				var asObject client.Object = clone

				c := build(hostPathOf(t, func() *stack.Cluster { return build("") }, rules, "s"))
				ml := mustWalk(t, c, rules)
				if held != nil {
					strayLayout(ml, asObject)
				}
				err = fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules)
				wantRefusedWith(t, err, `the build of the tree makes Flux Kustomization "flux-system/x-s", which the tree does not hold`)
			})
		}
	}
}

// A hand-written Kustomization builds the directory that holds the generated
// umbrella platform, with the name prefix x-: it makes x-platform, a renamed
// copy of the umbrella that writes the same Kustomizations. Nothing depends on
// the copy and no patch touches it. Before the rewrite the copy waits as the
// umbrella did; after it the umbrella, and the copy of it, wait for nothing. The
// copy loses its wait and the tree is admitted: that loss is the one the copied
// exemption exists for, and the stand-in rule must leave it alone.
func TestStandIn_RenamedUmbrellaCopyLosingItsWaitIsAdmitted(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			rules := withPlacement(nodeOnly, placement)
			build := func(path string) *stack.Cluster {
				c := builtClassCluster([]string{"v"})
				if path != "" {
					k := handWritten("hand-written")
					k.Spec.Path, k.Spec.NamePrefix = path, "x-"
					c.Node.Children = append(c.Node.Children, &stack.Node{Name: "patcher", Bundle: srBundle("patcher", objApp("patcher-app", k))})
				}
				return c
			}
			host := hostPathOf(t, func() *stack.Cluster { return build("") }, rules, "platform")
			_, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(build(host), rules)
			if err != nil {
				t.Fatalf("got %v, want the tree admitted", err)
			}
		})
	}
}

// The tree holds an application's own Kustomization l, a leaf that applies the
// Kustomization z and waits. No build changes it: its fields after the rewrite
// are the ones it had, and the tree is admitted. It copies no umbrella (its
// spec.path is its own), so no rule about copies applies to it.
func TestStandIn_HandWrittenLeafKeepsItsFields(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			rules := withPlacement(nodeOnly, placement)
			build := func(path string) *stack.Cluster {
				c := builtClassCluster([]string{"v"})
				if path != "" {
					l := handWritten("l")
					l.Spec.Path, l.Spec.Wait = path, true
					c.Node.Children = append(c.Node.Children, &stack.Node{Name: "owner", Bundle: srBundle("owner", objApp("owner-app", l))})
				}
				return c
			}
			// The stray layout's path depends on the root layout alone.
			stray := strayLayout(mustWalk(t, build(""), rules), handWritten("z")).FullRepoPath()
			c := build(stray)
			ml := mustWalk(t, c, rules)
			strayLayout(ml, handWritten("z"))
			if err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules); err != nil {
				t.Fatalf("got %v, want the tree admitted", err)
			}
		})
	}
}

// heldCloneOnTheFallback integrates the tree of the fallback tests above: a
// hand-written Kustomization builds the directory holding s with the name prefix
// x- and the patch on s, so the build before the rewrite fails (index 2 is out of
// range for [platform v]) and the copy x-s is made after it. The tree also holds
// a clone of the generated s named x-s in a directory no writer builds, whose
// dependsOn is held and which edit may change. It returns the integration's error.
func heldCloneOnTheFallback(t *testing.T, placement layout.FluxPlacement, patch string, held []string, edit func(*kustv1.Kustomization)) error {
	t.Helper()
	rules := withPlacement(nodeOnly, placement)
	build := func(path string) *stack.Cluster {
		c := builtClassCluster([]string{"v"})
		if path != "" {
			k := handWritten("hand-written")
			k.Spec.Path, k.Spec.NamePrefix = path, "x-"
			k.Spec.Patches = []kustomize.Patch{{Patch: patch, Target: &kustomize.Selector{Kind: "Kustomization", Name: "s"}}}
			c.Node.Children = append(c.Node.Children, &stack.Node{Name: "patcher", Bundle: srBundle("patcher", objApp("patcher-app", k))})
		}
		return c
	}
	probed, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(build(""), rules)
	if err != nil {
		t.Fatalf("CreateLayoutWithResources: %v", err)
	}
	generated := findKustomization(probed, "s")
	if generated == nil {
		t.Fatal("no generated Kustomization s")
	}
	clone := generated.DeepCopy()
	clone.Name = "x-s"
	clone.Spec.DependsOn = handWritten("held", held...).Spec.DependsOn
	if edit != nil {
		edit(clone)
	}
	var asObject client.Object = clone

	c := build(hostPathOf(t, func() *stack.Cluster { return build("") }, rules, "s"))
	ml := mustWalk(t, c, rules)
	strayLayout(ml, asObject)
	return fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules)
}

// The same tree as the fallback test above, with a held clone whose dependsOn is
// the copy's list after the patch, [y-infra y-services], so the copy loses its
// wait on v. The patch also sets a graph field on the copy that no member of the
// tree has, and the clone carries the same value: the clone has the copy's graph
// fields besides dependsOn, and the copy matches no member on them. A held
// object that matches the copy on every graph field is no proof that it was not
// made by a build of a member, so it does not stand for the copy: the loss is
// refused. Without the field on the clone it differs from the copy and is
// refused as before (ctl).
func TestStandIn_HeldCloneWithAPatchedGraphFieldDoesNotHideALostWaitOnTheFallback(t *testing.T) {
	const drop = "- op: remove\n  path: /spec/dependsOn/2\n"
	for name, tc := range map[string]struct {
		patch string
		edit  func(*kustv1.Kustomization)
	}{
		"ignoreMissingComponents": {
			drop + "- op: add\n  path: /spec/ignoreMissingComponents\n  value: true\n",
			func(k *kustv1.Kustomization) { k.Spec.IgnoreMissingComponents = true },
		},
		"targetNamespace": {
			drop + "- op: add\n  path: /spec/targetNamespace\n  value: ns-x\n",
			func(k *kustv1.Kustomization) { k.Spec.TargetNamespace = "ns-x" },
		},
		"ignoreMissingComponents, clone without it (ctl)": {
			drop + "- op: add\n  path: /spec/ignoreMissingComponents\n  value: true\n",
			nil,
		},
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				err := heldCloneOnTheFallback(t, placement, tc.patch, []string{"y-infra", "y-services"}, tc.edit)
				wantRefusedWith(t, err, `the build of the tree makes Flux Kustomization "flux-system/x-s", which the tree does not hold`)
			})
		}
	}
}

// The patch only tests index 2 (it fails on [platform v] and holds on
// [y-infra y-services v]) and changes nothing, so the copy x-s loses no wait,
// and the tree holds a clone of exactly the copy. The copy is a Kustomization a
// build of a hand-written Kustomization with a name prefix made, so it is not
// the held object, however equal they are: the tree is refused. A deliberate
// over-refusal, the price of not guessing which of the two the controller
// applies, stated in the README.
func TestStandIn_HeldCloneEqualToTheCopyOfATransformingWriterIsRefusedOnTheFallback(t *testing.T) {
	const keep = "- op: test\n  path: /spec/dependsOn/2/name\n  value: v\n"
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			err := heldCloneOnTheFallback(t, placement, keep, []string{"y-infra", "y-services", "v"}, nil)
			wantRefusedWith(t, err, `the build of the tree makes Flux Kustomization "flux-system/x-s", which the tree does not hold`)
		})
	}
}

// An application's own Kustomization l, a leaf that applies the Kustomization z
// and waits, is written by the plain Kustomization of its bundle: no patch, no
// name prefix, no target namespace, no postBuild. The tree cannot be built
// before the rewrite (web patches s with a test of index 2, which fails on
// [platform v] and holds on [y-infra y-services v], and changes nothing), so the
// fallback judges what the build makes. l and z are the objects the tree holds,
// written by Kustomizations that transform nothing: the tree is admitted.
func TestStandIn_HeldLeafOfAPlainWriterStandsOnTheFallback(t *testing.T) {
	const keep = "- op: test\n  path: /spec/dependsOn/2/name\n  value: v\n"
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			rules := withPlacement(nodeOnly, placement)
			build := func(path string) *stack.Cluster {
				c := builtClassCluster([]string{"v"}, patchOnS(keep))
				if path != "" {
					l := handWritten("l")
					l.Spec.Path, l.Spec.Wait = path, true
					c.Node.Children = append(c.Node.Children, &stack.Node{Name: "owner", Bundle: srBundle("owner", objApp("owner-app", l))})
				}
				return c
			}
			// The stray layout's path depends on the root layout alone.
			stray := strayLayout(mustWalk(t, build(""), rules), handWritten("z")).FullRepoPath()
			c := build(stray)
			ml := mustWalk(t, c, rules)
			strayLayout(ml, handWritten("z"))
			if err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules); err != nil {
				t.Fatalf("got %v, want the tree admitted", err)
			}
		})
	}
}

// The tree of the plain-writer test above, in which the Kustomization l applies
// the held Kustomization z, with one difference: l substitutes variables
// (spec.postBuild) and transforms nothing else, so it has no patch, component,
// name prefix, name suffix or target namespace. The substitution could rewrite
// what the directory of z holds, so a held z does not prove it is the object the
// controller applies: the postBuild clause of the fallback refuses it, on its
// own. Without the postBuild (ctl) l is the plain writer of the test above and
// the tree is admitted; that pair pins the clause, so that removing it cannot
// silently widen the exemption.
func TestStandIn_HeldLeafOfAPostBuildOnlyWriterIsRefusedOnTheFallback(t *testing.T) {
	const keep = "- op: test\n  path: /spec/dependsOn/2/name\n  value: v\n"
	for name, tc := range map[string]struct {
		edit    func(*kustv1.Kustomization)
		refused bool
	}{
		"postBuild only": {
			func(l *kustv1.Kustomization) {
				l.Spec.PostBuild = &kustv1.PostBuild{Substitute: map[string]string{"OTHER": "red"}}
			},
			true,
		},
		"no postBuild (ctl)": {nil, false},
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				rules := withPlacement(nodeOnly, placement)
				build := func(path string) *stack.Cluster {
					c := builtClassCluster([]string{"v"}, patchOnS(keep))
					if path != "" {
						l := handWritten("l")
						l.Spec.Path, l.Spec.Wait = path, true
						if tc.edit != nil {
							tc.edit(l)
						}
						c.Node.Children = append(c.Node.Children, &stack.Node{Name: "owner", Bundle: srBundle("owner", objApp("owner-app", l))})
					}
					return c
				}
				// The stray layout's path depends on the root layout alone.
				stray := strayLayout(mustWalk(t, build(""), rules), handWritten("z")).FullRepoPath()
				c := build(stray)
				ml := mustWalk(t, c, rules)
				strayLayout(ml, handWritten("z"))
				err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules)
				if tc.refused {
					wantRefusedWith(t, err, `the build of the tree makes Flux Kustomization "flux-system/z", which the tree does not hold`)
					return
				}
				if err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
			})
		}
	}
}

// The umbrella platform builds the umbrella mid (interval 1m), which builds the
// leaf. A hand-written Kustomization builds the directory that holds mid, with
// the name prefix x-: it makes x-mid, a renamed copy of the umbrella. A patch of
// platform sets the interval of mid to 17m. That patch is platform's own spec:
// it applies to the mid that platform builds, not to the copy, which is made
// from the directory as written and keeps 1m. So before the rewrite the copy has
// the spec the umbrella was held with, not the one it is applied with, and it
// loses the wait the rewrite clears on mid, which is the loss the exemption for a
// copy exists for. The tree is admitted (go-kure/kure#1272): a copy is judged
// against the umbrella as the tree holds it as well as against the umbrella as
// applied. A patch of the copy that gives it an interval of its own (33m, which
// equals neither the held 1m nor the applied 17m) makes it a leaf with a spec of
// its own, and the loss is refused: that row pins the refusal the README
// documents. With one that gives it the applied 17m it is a duplicate of the
// umbrella as applied, and is admitted.
func TestStandIn_UntouchedCopyOfAPatchedUmbrellaIsAdmitted(t *testing.T) {
	t.Parallel()
	const interval = "- op: replace\n  path: /spec/interval\n  value: %s\n"
	for name, tc := range map[string]struct {
		platformPatch string
		copyPatch     string
		refused       bool
	}{
		"umbrella patched, copy untouched":         {"17m0s", "", false},
		"umbrella patched, copy patched (ctl)":     {"17m0s", "33m0s", true},
		"umbrella patched, copy equal (ctl)":       {"17m0s", "17m0s", false},
		"umbrella untouched, copy untouched (ctl)": {"", "", false},
		"umbrella untouched, copy patched (ctl)":   {"", "33m0s", true},
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				rules := withPlacement(nodeOnly, placement)
				build := func(host string) *stack.Cluster {
					mid := srBundle("mid")
					mid.Interval = "1m"
					mid.Children = []*stack.Bundle{srBundle("leaf", cmApp("leaf-app"))}
					platform := srBundle("platform")
					platform.Children = []*stack.Bundle{mid}
					if tc.platformPatch != "" {
						platform.Patches = []stack.Patch{{
							Patch:  fmt.Sprintf(interval, tc.platformPatch),
							Target: &stack.PatchSelector{Kind: "Kustomization", Name: "mid"},
						}}
					}
					n := &stack.Node{Name: "platform", Bundle: platform}
					root := &stack.Node{Name: "demo", Children: []*stack.Node{n}}
					n.SetParent(root)
					c := &stack.Cluster{Name: "demo", Node: root}
					if host != "" {
						k := handWritten("hand-written")
						k.Spec.Path, k.Spec.NamePrefix = host, "x-"
						if tc.copyPatch != "" {
							k.Spec.Patches = []kustomize.Patch{{
								Patch:  fmt.Sprintf(interval, tc.copyPatch),
								Target: &kustomize.Selector{Kind: "Kustomization", Name: "mid"},
							}}
						}
						patcher := &stack.Node{Name: "patcher", Bundle: srBundle("patcher", objApp("patcher-app", k))}
						patcher.SetParent(root)
						root.Children = append(root.Children, patcher)
					}
					return c
				}
				host := hostPathOf(t, func() *stack.Cluster { return build("") }, rules, "mid")
				_, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(build(host), rules)
				if tc.refused {
					wantRefusedWith(t, err, `Flux Kustomization "flux-system/x-mid" has a different spec.wait after the rewrite than before it`)
					return
				}
				if err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
			})
		}
	}
}
