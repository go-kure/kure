package kustomizeschema

import (
	"path/filepath"
	"strings"
	"sync"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	"google.golang.org/protobuf/proto"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"sigs.k8s.io/kustomize/kyaml/openapi"
	"sigs.k8s.io/kustomize/kyaml/openapi/kubernetesapi"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// ScopeIsBuiltIn reports whether kyaml answers the scope of the kind
// (openapi.IsNamespaceScoped) as its built-in schema does. Definitions cannot
// tell: kyaml fills a third table, the scope of each resource type, from the
// paths of every schema added through openapi.AddSchema (kyaml v0.21.1
// openapi.go:749, :758-790), and neither the definitions nor the index by
// resource type hold it. resid.NewGvk reads that table (through
// openapi.IsCertainlyClusterScoped) for a kind kyaml does not know from its
// precomputed table of the built-in kinds (:403-410), and kustomize's namespace
// filter and name references then decide on the answer: a Widget added with a
// path alone is built without the namespace a kustomization sets. The table is
// unexported and cannot be listed, so a caller asks for the kinds a build made.
//
// The built-in answer for a kind is the one the paths of the built-in schema
// give, read the way kyaml reads them (findNamespaceability), once per
// process. A kind kyaml answers as not found is always the built-in answer:
// nothing removes an entry short of a reset, which restores the built-in
// state.
func ScopeIsBuiltIn(apiVersion, kind string) bool {
	tm := yaml.TypeMeta{APIVersion: apiVersion, Kind: kind}
	got, found := openapi.IsNamespaceScoped(tm)
	if !found {
		return true
	}
	want, ok := builtInScopes()[tm]
	return ok && want == got
}

// builtInScopes is the scope of each resource type the paths of kyaml's
// built-in Kubernetes schema declare, read as findNamespaceability reads them:
// a path with a namespace parameter makes the type namespaced, whatever the
// other paths say; any other path makes it cluster-scoped unless set already.
var builtInScopes = sync.OnceValue(func() map[yaml.TypeMeta]bool {
	name := filepath.Join("kubernetesapi", strings.ReplaceAll(kubernetesapi.DefaultOpenAPI, ".", "_"), "swagger.pb")
	doc := &openapi_v2.Document{}
	if err := proto.Unmarshal(kubernetesapi.OpenAPIMustAsset[kubernetesapi.DefaultOpenAPI](name), doc); err != nil {
		panic(err) // the asset is embedded: kyaml panics the same way
	}
	var swagger spec.Swagger
	if _, err := swagger.FromGnostic(doc); err != nil {
		panic(err)
	}
	scopes := map[yaml.TypeMeta]bool{}
	if swagger.Paths == nil {
		return scopes
	}
	for path, info := range swagger.Paths.Paths {
		if info.Get == nil {
			continue
		}
		tm, ok := declaredKind(info.Get.Extensions[gvkExtension])
		if !ok {
			continue
		}
		if strings.Contains(path, "namespaces/{namespace}") {
			scopes[tm] = true
		} else if _, set := scopes[tm]; !set {
			scopes[tm] = false
		}
	}
	return scopes
})

// declaredKind reads a group-version-kind extension value as kyaml's toTypeMeta
// does (openapi.go:355-366).
func declaredKind(ext any) (yaml.TypeMeta, bool) {
	m, ok := ext.(map[string]any)
	if !ok {
		return yaml.TypeMeta{}, false
	}
	version, vok := m["version"].(string)
	kind, kok := m["kind"].(string)
	if !vok || !kok {
		return yaml.TypeMeta{}, false
	}
	if group, _ := m["group"].(string); group != "" {
		version = group + "/" + version
	}
	return yaml.TypeMeta{APIVersion: version, Kind: kind}, true
}
