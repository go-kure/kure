module github.com/go-kure/kure

go 1.24.1

replace (
	k8s.io/api => k8s.io/api v0.31.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.31.6
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.31.6
	k8s.io/client-go => k8s.io/client-go v0.31.6
)

require (
	github.com/fluxcd/helm-controller/api v1.2.0
	github.com/fluxcd/kustomize-controller/api v1.5.1
	github.com/fluxcd/notification-controller/api v1.6.0
	github.com/fluxcd/pkg/apis/kustomize v1.9.0
	github.com/fluxcd/pkg/apis/meta v1.12.0
	github.com/fluxcd/source-controller/api v1.5.0
	k8s.io/api v0.33.0
	k8s.io/apimachinery v0.33.0
	k8s.io/cli-runtime v0.31.6
	k8s.io/client-go v0.33.0
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/fluxcd/pkg/apis/acl v0.6.0 // indirect
	github.com/fxamacker/cbor/v2 v2.8.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/liggitt/tabwriter v0.0.0-20181228230101-89fcab3d43de // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	golang.org/x/net v0.40.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.25.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/apiextensions-apiserver v0.33.0 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/utils v0.0.0-20250321185631-1f6e0b77f77e // indirect
	sigs.k8s.io/controller-runtime v0.21.0 // indirect
	sigs.k8s.io/json v0.0.0-20241014173422-cfa47c3a1cc8 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.7.0 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)
