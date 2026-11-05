package fluxcd_test

import (
	"testing"

	"github.com/fluxcd/pkg/apis/kustomize"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
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
