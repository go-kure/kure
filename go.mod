module github.com/go-kure/kure

go 1.26.1

// Replace directives: pin all k8s.io packages to the same patch release.
//
// Why: Kubernetes client libraries (api, apimachinery, client-go, etc.) must
// be used at the same version to avoid type incompatibilities and runtime
// panics. Kure's transitive dependencies (cert-manager, cloudnative-pg,
// flux-operator, metallb, prometheus-operator, etc.) each require different
// k8s.io minor versions (v0.30–v0.35). Without explicit pins, `go mod tidy`
// could pull in mismatched versions during dependency updates.
//
// Current pin: v0.35.1 (Kubernetes 1.35)
//
// Removal condition: these directives can be removed when ALL direct and
// transitive dependencies converge on the same k8s.io minor version, making
// Go's MVS sufficient to maintain lockstep. Check with:
//   go mod graph | grep 'k8s.io/' | awk '{print $2}' | sort -u
replace (
	k8s.io/api => k8s.io/api v0.35.1
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.35.1
	k8s.io/apimachinery => k8s.io/apimachinery v0.35.1
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.35.1
	k8s.io/client-go => k8s.io/client-go v0.35.1
)

require (
	github.com/blang/semver/v4 v4.0.0
	github.com/cert-manager/cert-manager v1.20.0
	github.com/cloudnative-pg/barman-cloud v0.5.0
	github.com/cloudnative-pg/cloudnative-pg v1.28.1
	github.com/cloudnative-pg/machinery v0.3.3
	github.com/cloudnative-pg/plugin-barman-cloud v0.11.0
	github.com/controlplaneio-fluxcd/flux-operator v0.40.0
	github.com/evanphx/json-patch/v5 v5.9.11
	github.com/external-secrets/external-secrets/apis v0.0.0-20260213133823-31b0c7c37342
	github.com/fluxcd/flux2/v2 v2.8.2
	github.com/fluxcd/helm-controller/api v1.5.2
	github.com/fluxcd/image-automation-controller/api v1.1.1
	github.com/fluxcd/kustomize-controller/api v1.8.2
	github.com/fluxcd/notification-controller/api v1.8.2
	github.com/fluxcd/pkg/apis/acl v0.9.0
	github.com/fluxcd/pkg/apis/kustomize v1.15.1
	github.com/fluxcd/pkg/apis/meta v1.25.1
	github.com/fluxcd/source-controller/api v1.8.1
	github.com/google/cel-go v0.27.0
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2
	github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring v0.89.0
	github.com/spf13/cobra v1.10.2
	github.com/spf13/pflag v1.0.10
	github.com/spf13/viper v1.21.0
	github.com/stretchr/testify v1.11.1
	go.universe.tf/metallb v0.15.3
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/api v0.35.2
	k8s.io/apiextensions-apiserver v0.35.2
	k8s.io/apimachinery v0.35.2
	k8s.io/cli-runtime v0.35.2
	sigs.k8s.io/controller-runtime v0.23.3
	sigs.k8s.io/gateway-api v1.5.1
	sigs.k8s.io/kustomize/api v0.21.1
	sigs.k8s.io/yaml v1.6.0
)

require (
	cel.dev/expr v0.25.1 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20250102033503-faa5f7b0171c // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudnative-pg/cnpg-i v0.3.1 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/cyphar/filepath-securejoin v0.6.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.13.0 // indirect
	github.com/fluxcd/pkg/kustomize v1.27.1 // indirect
	github.com/fluxcd/pkg/tar v0.17.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/go-errors/errors v1.5.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/zapr v1.3.0 // indirect
	github.com/go-openapi/jsonpointer v0.22.4 // indirect
	github.com/go-openapi/jsonreference v0.21.4 // indirect
	github.com/go-openapi/swag v0.25.4 // indirect
	github.com/go-openapi/swag/cmdutils v0.25.4 // indirect
	github.com/go-openapi/swag/conv v0.25.4 // indirect
	github.com/go-openapi/swag/fileutils v0.25.4 // indirect
	github.com/go-openapi/swag/jsonname v0.25.4 // indirect
	github.com/go-openapi/swag/jsonutils v0.25.4 // indirect
	github.com/go-openapi/swag/loading v0.25.4 // indirect
	github.com/go-openapi/swag/mangling v0.25.4 // indirect
	github.com/go-openapi/swag/netutils v0.25.4 // indirect
	github.com/go-openapi/swag/stringutils v0.25.4 // indirect
	github.com/go-openapi/swag/typeutils v0.25.4 // indirect
	github.com/go-openapi/swag/yamlutils v0.25.4 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/gnostic-models v0.7.1 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kubernetes-csi/external-snapshotter/client/v8 v8.4.0 // indirect
	github.com/lib/pq v1.11.1 // indirect
	github.com/liggitt/tabwriter v0.0.0-20181228230101-89fcab3d43de // indirect
	github.com/moby/spdystream v0.5.0 // indirect
	github.com/moby/term v0.5.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.67.5 // indirect
	github.com/prometheus/procfs v0.19.2 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/sagikazarmark/locafero v0.11.0 // indirect
	github.com/sourcegraph/conc v0.3.1-0.20240121214520-5f936abd7ae8 // indirect
	github.com/spf13/afero v1.15.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xlab/treeprint v1.2.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.1 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/exp v0.0.0-20251017212417-90e834f514db // indirect
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/oauth2 v0.35.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/term v0.40.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.5.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260128011058-8636f8732409 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260217215200-42d3e9bedb6d // indirect
	google.golang.org/grpc v1.79.3 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.13.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/client-go v0.35.2 // indirect
	k8s.io/klog/v2 v2.140.0 // indirect
	k8s.io/kube-openapi v0.0.0-20260127142750-a19766b6e2d4 // indirect
	k8s.io/utils v0.0.0-20260210185600-b8788abfbbc2 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/kustomize/kyaml v0.21.1 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.2 // indirect
)
