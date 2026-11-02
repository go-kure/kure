package fluxcd

import (
	"fmt"
	"runtime/debug"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
)

// kustomizeAPIModule is the module whose version kustomizeAPIAudited names.
const kustomizeAPIModule = "sigs.k8s.io/kustomize/api"

// readBuildInfo is debug.ReadBuildInfo; a test replaces it.
var readBuildInfo = debug.ReadBuildInfo

// decrypts reports whether kure treats Flux as decrypting the files and the
// objects of k's build: spec.decryption is set, with any provider. No
// provider and no data path is told apart, so none can be missed. Both
// mergeUnread and buildGuard key on it (go-kure/kure#1178).
func decrypts(k *kustv1.Kustomization) bool {
	return k.Spec.Decryption != nil
}

// mergeUnread says why a build that merges into a Secret is not read, or ""
// when it is: kustomize hashes a merged Secret's name over the merged content
// inside the build, so the name kure builds is the one Flux computes, but for
// two inputs it cannot build the way Flux does.
//
// Decryption. kustomize-controller decrypts sources before the build only with
// spec.decryption set, and resources after it (internal/decryptor and
// internal/controller/kustomization_controller.go:846-874, v1.10.1). Any
// spec.decryption is therefore enough to refuse: no provider and no data path
// is tracked, so none can be missed (go-kure/kure#1178).
//
// Version. A merge into a Secret whose base holds stringData hashes
// differently in kustomize api v0.21.2 than in v0.21.1, the version
// kustomize-controller v1.10.1 builds with. A module that imports kure
// resolves v0.21.2 unless it repeats kure's replace directives.
func mergeUnread(k *kustv1.Kustomization, info func() (*debug.BuildInfo, bool)) string {
	if decrypts(k) {
		return "the Kustomization sets spec.decryption, and decrypted data may change the hash a merged Secret's name carries"
	}
	return unauditedKustomize(info)
}

// unauditedKustomize says why the kustomize api this binary links is not the
// audited one, or "" when it is. The module that is linked is the replacement
// when the build replaces it: Go keeps the original version in Dep.Version and
// records the replacement apart, so kure itself (it requires v0.21.2 and
// replaces it with v0.21.1) is the audited build only by its replacement.
// Path and version of that module must both match; a replacement with no
// version (a local directory) never does.
func unauditedKustomize(info func() (*debug.BuildInfo, bool)) string {
	bi, ok := info()
	if !ok {
		return "this binary carries no build information, so the kustomize it links cannot be told from the one kustomize-controller builds with"
	}
	for _, dep := range bi.Deps {
		if dep.Path != kustomizeAPIModule {
			continue
		}
		linked := dep
		if dep.Replace != nil {
			linked = dep.Replace
		}
		if linked.Path == kustomizeAPIModule && linked.Version == kustomizeAPIAudited {
			return ""
		}
		return fmt.Sprintf("it is built with %s %s, not the %s %s that kustomize-controller builds with", linked.Path, linked.Version, kustomizeAPIModule, kustomizeAPIAudited)
	}
	return "this binary does not list " + kustomizeAPIModule + " among its modules, so the kustomize it links cannot be told from the one kustomize-controller builds with"
}
