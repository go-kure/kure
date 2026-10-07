package fluxcd

import (
	"fmt"
	"slices"
	"strings"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/provider"
	"sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/kustomize/kyaml/resid"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// patchBuild is one Kustomization a bundle's patches can be written on: the
// bundle's own, or that of a layout of one of its applications.
type patchBuild struct {
	// layout is the directory the Kustomization builds, its spec.path.
	layout *layout.ManifestLayout
	// name is the Kustomization's name.
	name string
	// cr is the Kustomization when this pass created it. It is nil for one an
	// earlier integration placed and this pass kept: that one is left as it
	// is, and still counts as a build that can hold a patch's object.
	cr *kustv1.Kustomization
}

// bundlePatchBuilds are the Kustomizations of one bundle under
// FluxIntegratedPerLayout, as place met them.
type bundlePatchBuilds struct {
	// own is the Kustomization of the directory that renders the bundle.
	own patchBuild
	// merged says that directory renders other bundles as well, so that own
	// holds their patches too. generateForUnit refuses an untargeted patch on
	// such a bundle, and its targeted ones stay on own as they are.
	merged bool
	// layouts are the per-layout Kustomizations the bundle is the holding
	// bundle of (holdingBundle), in the order place met them.
	layouts []patchBuild
}

// recordUnitBuild records the Kustomization place generated for l, the
// directory that renders bundles, as the own build of each of them. cr is the
// generated object; it is kept as the bundle's only when host holds it, which
// it does not when an earlier integration's was kept instead (add).
func (p *integratedPlacement) recordUnitBuild(host, l *layout.ManifestLayout, bundles []*stack.Bundle, cr *kustv1.Kustomization) {
	build := patchBuild{layout: l, name: cr.Name}
	if holds(host, cr) {
		build.cr = cr
	}
	for _, b := range bundles {
		p.patchBuildsOf(b).own = build
		p.patchBuildsOf(b).merged = len(bundles) > 1
	}
}

// recordLayoutBuild records the per-layout Kustomization named name, which
// builds child, with the bundle that holds child's application. A layout
// without a holding bundle inherits no patch and is not recorded. cr is the
// Kustomization this pass created, or nil for one it kept; as in
// recordUnitBuild, a created one counts only when host holds it.
func (p *integratedPlacement) recordLayoutBuild(host, child *layout.ManifestLayout, name string, cr *kustv1.Kustomization) {
	holder := p.holdingBundle(child)
	if holder == nil {
		return
	}
	build := patchBuild{layout: child, name: name}
	if cr != nil && holds(host, cr) {
		build.cr = cr
	}
	builds := p.patchBuildsOf(holder)
	builds.layouts = append(builds.layouts, build)
}

// patchBuildsOf returns the record of b's Kustomizations, starting it on first
// use.
func (p *integratedPlacement) patchBuildsOf(b *stack.Bundle) *bundlePatchBuilds {
	if builds, ok := p.patchBuilds[b]; ok {
		return builds
	}
	builds := &bundlePatchBuilds{}
	p.patchBuilds[b] = builds
	p.patchOrder = append(p.patchOrder, b)
	return builds
}

// holds reports whether cr is one of host's resources.
func holds(host *layout.ManifestLayout, cr *kustv1.Kustomization) bool {
	for _, r := range host.Resources {
		if k, ok := r.(*kustv1.Kustomization); ok && k == cr {
			return true
		}
	}
	return false
}

// placeBundlePatches writes the patches of each bundle on the Kustomizations
// of the layouts of its applications, and takes from the bundle's own
// Kustomization the untargeted ones whose object it does not build
// (go-kure/kure#1021). It runs under FluxIntegratedPerLayout once every
// Kustomization and Source is placed, since what a build holds includes the
// Kustomizations hosted in it.
//
// A bundle none of whose applications got a per-layout Kustomization is left
// alone: its own Kustomization keeps every patch, and nothing is refused.
// So is, on any bundle, a Kustomization an earlier integration placed and
// this pass kept.
//
// A patch with a target is written on every per-layout Kustomization of the
// bundle, after the bundle's own: kustomize applies it to what the target
// selects in that build and to nothing where it selects nothing.
//
// A patch without a target that kustomize can build is a strategic-merge
// patch, and kustomize fails the build of a Kustomization that does not hold
// the object each of its documents names. It is therefore written on the
// Kustomizations, the bundle's own among them, whose build holds every object
// it names, and on no other. An object is compared as kustomize compares it (group, version, kind,
// name and effective namespace); a build holds the objects of the directories
// buildScope lists, the Kustomizations and Sources hosted there included, and
// the ConfigMap each configMapGenerator entry of those directories generates.
// Two cases are refused, naming the bundle, the patch by its index and the
// object:
//
//   - no Kustomization of the bundle builds an object the patch names. Before
//     the per-layout Kustomizations took patches this failed on the cluster,
//     in the build of the bundle's own Kustomization; a tree that built there
//     is not refused, since that build held every such object;
//   - every object is built, but no one Kustomization builds them all: the
//     documents of one patch cannot be split, so the caller writes one patch
//     per object.
//
// The objects a build holds are compared as generated, and that is what a
// patch names only while no earlier entry of the list has changed an
// identity. Placement by object therefore holds for a patch only while every
// entry of the list before it, and the patch itself, is a plain
// strategic-merge patch (plainStrategicMerge): one that parses as resources,
// of which no document carries an annotation of kustomize's own build state
// and none can remove its object from the build ($patch: delete, or the
// local-config annotation). A plain patch keeps the
// identity of what it is merged into, by
// kustomize api v0.21.2:
//
//   - without a target it is merged into the object of its own apiVersion,
//     kind, name and namespace (PatchTransformer.transformStrategicMerge,
//     internal/builtins/PatchTransformer.go:117-125, finds it by that
//     identity, resid.ResId.Equals), and Resource.ApplySmPatch
//     (resource/resource.go:495-516) then restores the kind, name and
//     namespace the object had;
//   - with a target it is given the apiVersion of each object it selects
//     (resWrangler.ApplySmPatch, resmap/reswrangler.go:742-750) and merged
//     by the same Resource.ApplySmPatch. A Flux patch has no options that
//     allow a change of name or kind; a build pins this in
//     TestKustomize_APlainStrategicMergePatchKeepsTheIdentity.
//
// Any other entry is not known to keep it. A JSON6902 patch with a target may
// replace metadata.name, metadata.namespace, kind or apiVersion, and
// kustomize then matches a later strategic-merge patch against the new
// identity as well as the one before. A build-state annotation in the patch
// text is read as kustomize reads its own: one allows the patch to change the
// name or the kind of the object it is merged into, with a target or without,
// and others give a document the identity it names its object by, which
// kustomize panics on when they do not agree (Resource.OrgId). A document
// with $patch: delete, or one that gives its object kustomize's local-config
// annotation at any value but "false", can remove the object from the build
// altogether, a per-layout Kustomization in its parent's build among them.
//
// An entry that is not plain is therefore written where it was before
// per-layout Kustomizations took patches, is never refused and is not read:
// with a target on the bundle's own Kustomization and on every per-layout
// one, without one on the bundle's own only (a JSON6902 patch without a
// target, which kustomize refuses wherever it is, among them). Every
// untargeted plain patch that comes after it is not placed by object and
// never refused either: it stays on the bundle's own Kustomization, where
// every patch was before, and is written besides on each per-layout
// Kustomization whose build holds every object it names as generated. What
// the entry does is not read, so one that changes no identity counts as
// well. What that leaves as it was, the build of the bundle's own
// Kustomization failing on the cluster as it did:
//
//   - such a later patch for an object only a layout builds: it is on that
//     layout's Kustomization as well, which builds and patches the object,
//     and on the bundle's own, which does not build it;
//   - such a later patch for the identity an earlier entry gives an object in
//     a layout's build: it is on no Kustomization that builds the object;
//   - an untargeted patch with a build-state annotation, or one that removes
//     its object, for an object only a layout builds: it is on the bundle's
//     own Kustomization alone.
func (p *integratedPlacement) placeBundlePatches() error {
	if !p.perLayout {
		return nil
	}
	rf := provider.NewDefaultDepProvider().GetResourceFactory()
	built := map[*layout.ManifestLayout][]resid.ResId{}
	objectsOf := func(l *layout.ManifestLayout) ([]resid.ResId, error) {
		if ids, ok := built[l]; ok {
			return ids, nil
		}
		ids, err := buildIDs(l)
		if err != nil {
			return nil, err
		}
		built[l] = ids
		return ids, nil
	}

	for _, b := range p.patchOrder {
		builds := p.patchBuilds[b]
		if len(builds.layouts) == 0 || len(b.Patches) == 0 {
			continue
		}
		// The builds an untargeted patch can land on, the bundle's own first.
		candidates := make([]patchBuild, 0, len(builds.layouts)+1)
		if !builds.merged && builds.own.layout != nil {
			candidates = append(candidates, builds.own)
		}
		candidates = append(candidates, builds.layouts...)

		// Where each patch of the bundle is written: on the bundle's own
		// Kustomization (own), on that of every layout (everyLayout), or on
		// the candidates whose build holds its objects (held).
		type placement struct {
			own         bool
			everyLayout bool
			held        []patchBuild
		}
		placements := make([]placement, len(b.Patches))
		// Set once an entry that is not a plain strategic-merge patch has
		// passed: from there the identities as generated no longer say what
		// an untargeted patch names.
		identityChanged := false
		for i, patch := range b.Patches {
			docs, plain := plainStrategicMerge(rf, patch.Patch)
			if !plain {
				// Written where it was before, and its documents, if it has
				// any, are not read.
				placements[i] = placement{own: true, everyLayout: patch.Target != nil}
				identityChanged = true
				continue
			}
			if patch.Target != nil {
				placements[i] = placement{own: true, everyLayout: true}
				continue
			}
			on := candidates
			var split []string
			for _, doc := range docs {
				id := doc.OrgId()
				var holders []patchBuild
				for _, c := range candidates {
					ids, err := objectsOf(c.layout)
					if err != nil {
						return err
					}
					if slices.ContainsFunc(ids, id.Equals) {
						holders = append(holders, c)
					}
				}
				if len(holders) == 0 && !identityChanged {
					return errors.ResourceValidationError("Bundle", b.GetPath(), "patches",
						fmt.Sprintf("patch %d has no target and names %s, which no Kustomization of the bundle builds (%s): kustomize fails the build of a Kustomization that holds a strategic-merge patch for an object it does not build; give the patch a target (for a patch of several objects: one patch per object first, each with its target), or place the object in a build of the bundle",
							i, describeID(id), describeBuilds(candidates)), nil)
				}
				split = append(split, fmt.Sprintf("%s is built by %s", describeID(id), describeBuilds(holders)))
				on = slices.DeleteFunc(slices.Clone(on), func(c patchBuild) bool {
					return !slices.ContainsFunc(holders, func(h patchBuild) bool { return h.layout == c.layout })
				})
			}
			if identityChanged {
				placements[i] = placement{own: true, held: on}
				continue
			}
			if len(on) == 0 {
				return errors.ResourceValidationError("Bundle", b.GetPath(), "patches",
					fmt.Sprintf("patch %d has no target and names objects that no one Kustomization of the bundle builds together (%s): the documents of one patch are applied in one build; write one patch per object",
						i, strings.Join(split, "; ")), nil)
			}
			placements[i] = placement{held: on}
		}

		heldBy := func(i int, c patchBuild) bool {
			return slices.ContainsFunc(placements[i].held, func(h patchBuild) bool { return h.layout == c.layout })
		}
		if own := builds.own.cr; own != nil && !builds.merged {
			var kept []kustomize.Patch
			for i, patch := range b.Patches {
				if placements[i].own || heldBy(i, builds.own) {
					kept = append(kept, fluxPatch(patch))
				}
			}
			own.Spec.Patches = kept
		}
		for _, lay := range builds.layouts {
			if lay.cr == nil {
				continue
			}
			for i, patch := range b.Patches {
				if placements[i].everyLayout || heldBy(i, lay) {
					lay.cr.Spec.Patches = append(lay.cr.Spec.Patches, fluxPatch(patch))
				}
			}
		}
	}
	return nil
}

// plainStrategicMerge reports whether text, the text of a patch, is a plain
// strategic-merge patch, and returns its documents when it is. It is one when
// it parses as resources, which is how kustomize takes an entry for a
// strategic-merge patch and not for a JSON6902 one, no document of it carries
// an annotation of kustomize's own build state (carriesBuildAnnotation) and
// none can remove the object it is merged into from the build (removesObject).
// placeBundlePatches reads the documents of a plain patch and of no other.
func plainStrategicMerge(rf *resource.Factory, text string) ([]*resource.Resource, bool) {
	docs, err := rf.SliceFromBytes([]byte(text))
	if err != nil || len(docs) == 0 ||
		slices.ContainsFunc(docs, carriesBuildAnnotation) || slices.ContainsFunc(docs, removesObject) {
		return nil, false
	}
	return docs, true
}

// removesObject reports whether doc, a document of a patch, can remove the
// object it is merged into from the build, by kustomize api and kyaml v0.21.2:
//
//   - its top-level strategic-merge directive is $patch: delete, which
//     kustomize reads as the deletion of the whole object
//     (merge2.Merger.VisitMap, kyaml yaml/merge2/merge2.go:53-91);
//   - it carries the annotation config.kubernetes.io/local-config
//     (konfig.IgnoredByKustomizeAnnotation) with any value but "false": where
//     the merge gives the object that annotation, kustomize drops the object
//     once the build has run its patches (KustTarget.IgnoreLocal,
//     internal/target/kusttarget.go:162; Factory.DropLocalNodes,
//     resource/factory.go:149-151).
//
// A document counts by what it carries, not by what the merge leaves, which
// is not read: one that sets the annotation to null, which removes it
// (kyaml yaml/merge2/merge2.go:102-103, yaml/fns.go:733-734), counts, and
// so does one whose annotations also carry $patch: delete, which deletes
// them and leaves the object (yaml/merge2/merge2.go:60-86).
//
// When the object is removed it is in no build, so what an untargeted patch
// after it names, as generated, no longer says where it is built: one for an
// object inside a removed Kustomization's build would land on that
// Kustomization alone, which nothing applies any more.
func removesObject(doc *resource.Resource) bool {
	if directive := doc.Field("$patch"); directive != nil && directive.Value.YNode().Value == "delete" {
		return true
	}
	local, ok := doc.GetAnnotations()[konfig.IgnoredByKustomizeAnnotation]
	return ok && local != "false"
}

// carriesBuildAnnotation reports whether doc, a document of a patch, has an
// annotation in the domain kustomize keeps its own build state in
// (konfig.ConfigAnnoDomain). Among them are the two that allow a patch to
// change a name or a kind and the three that hold an object's previous
// identities. The whole domain counts, not those five: none of it is a
// caller's to write, and what kustomize reads from it may grow.
func carriesBuildAnnotation(doc *resource.Resource) bool {
	for key := range doc.GetAnnotations() {
		if strings.HasPrefix(key, konfig.ConfigAnnoDomain+"/") {
			return true
		}
	}
	return false
}

// buildIDs returns the identity of every object a kustomize build of l holds:
// the resources of the layouts buildScope lists, as kustomize builds them
// (resourceItems), and the ConfigMap each of their configMapGenerator entries
// generates, which a patch names by the entry's name, the name it has before
// kustomize appends the content hash.
//
// A generated ConfigMap is v1 ConfigMap, the entry's name and no namespace,
// which is how kustomize holds it: a ConfigMapGeneratorSpec has no namespace
// field, and the kustomization.yaml the writers write sets no namespace for
// the directory either. kustomize reads no namespace as "default", so a patch
// document names such a ConfigMap without a namespace or in "default", and
// one that names it in another namespace names an object the build does not
// hold.
func buildIDs(l *layout.ManifestLayout) ([]resid.ResId, error) {
	var ids []resid.ResId
	for _, dir := range buildScope(l) {
		objs, err := resourceItems(dir)
		if err != nil {
			return nil, err
		}
		for _, obj := range objs {
			ids = append(ids, resourceID(obj))
		}
		for _, gen := range dir.ConfigMapGenerators {
			ids = append(ids, resid.NewResIdWithNamespace(resid.NewGvk("", "v1", "ConfigMap"), gen.Name, ""))
		}
	}
	return ids, nil
}

// describeID words the object a patch document names, for a refusal: its
// kind and name, its apiVersion, and its namespace when the document sets
// one.
func describeID(id resid.ResId) string {
	apiVersion := id.Version
	if id.Group != "" {
		apiVersion = id.Group + "/" + id.Version
	}
	out := fmt.Sprintf("%s %q (%s", id.Kind, id.Name, apiVersion)
	if id.Namespace != "" {
		out += fmt.Sprintf(", namespace %q", id.Namespace)
	}
	return out + ")"
}

// describeBuilds lists Kustomizations by name and spec.path, for a refusal.
func describeBuilds(builds []patchBuild) string {
	parts := make([]string, 0, len(builds))
	for _, b := range builds {
		parts = append(parts, fmt.Sprintf("%q, spec.path %q", b.name, b.layout.FullRepoPath()))
	}
	return strings.Join(parts, "; ")
}
