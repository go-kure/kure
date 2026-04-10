package fluxcd

import (
	_ "embed"
	"fmt"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/client"

	kio "github.com/go-kure/kure/pkg/io"
)

// FluxOperatorVersion is the upstream flux-operator release whose install
// manifest is vendored as fluxOperatorInstallYAML. It is pinned to match
// the github.com/controlplaneio-fluxcd/flux-operator Go module version in
// kure's go.mod so that the generated FluxInstance type and the install
// bundle stay in lockstep.
//
// To refresh this vendoring:
//  1. Bump github.com/controlplaneio-fluxcd/flux-operator in go.mod.
//  2. Download the matching install.yaml from the flux-operator GitHub
//     release page:
//     https://github.com/controlplaneio-fluxcd/flux-operator/releases/download/{version}/install.yaml
//  3. Replace pkg/stack/fluxcd/flux_operator_install.yaml with it.
//  4. Update this constant.
//  5. Run the tests in this package and confirm the resource inventory in
//     TestFluxOperatorInstallObjects still matches.
const FluxOperatorVersion = "v0.40.0"

//go:embed flux_operator_install.yaml
var fluxOperatorInstallYAML []byte

var (
	fluxOperatorInstallOnce    sync.Once
	fluxOperatorInstallObjects []client.Object
	fluxOperatorInstallErr     error
)

// FluxOperatorInstallObjects returns the parsed Flux Operator install
// manifest: Namespace, CRDs, RBAC, ServiceAccount, Service, and
// controller Deployment. The bytes are embedded at build time from
// flux_operator_install.yaml (version FluxOperatorVersion).
//
// The returned slice is cached on first parse; callers must treat it as
// read-only. To mutate any object, deep-copy first.
//
// The order of objects matches the order in the upstream install.yaml
// (Namespace → CRDs → RBAC → ServiceAccount → Deployment → Service),
// which is also a valid apply order.
func FluxOperatorInstallObjects() ([]client.Object, error) {
	fluxOperatorInstallOnce.Do(func() {
		objs, err := kio.ParseYAML(fluxOperatorInstallYAML)
		if err != nil {
			fluxOperatorInstallErr = fmt.Errorf("parse vendored flux-operator install.yaml (%s): %w", FluxOperatorVersion, err)
			return
		}
		fluxOperatorInstallObjects = objs
	})
	return fluxOperatorInstallObjects, fluxOperatorInstallErr
}
