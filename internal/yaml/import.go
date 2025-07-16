package yaml

import (
	"bytes"
	"io"
	"log"
	"os"
	"regexp"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
)

// Parse converts a YAML document containing one or more Kubernetes resources
// into a slice of runtime.Objects. Each document must be separated by `---`.
func Parse(data []byte) ([]runtime.Object, error) {
	if err := kustv1.AddToScheme(scheme.Scheme); err != nil {
		return nil, err
	}

	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 1024)
	decode := scheme.Codecs.UniversalDeserializer().Decode

	objs := []runtime.Object{}
	for {
		var raw runtime.RawExtension
		if err := decoder.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if len(raw.Raw) == 0 {
			continue
		}

		obj, _, err := decode(raw.Raw, nil, nil)
		if err != nil {
			return nil, err
		}
		objs = append(objs, obj)
	}
	return objs, nil
}

// ParseFile reads the YAML file at the given path and returns the Kubernetes
// objects it defines.
func ParseFile(path string) ([]runtime.Object, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

func checkType(obj runtime.Object) {
	acceptedK8sTypes := regexp.MustCompile(`(Role|ClusterRole|RoleBinding|ClusterRoleBinding|ServiceAccount)`)
	groupVersionKind := obj.GetObjectKind().GroupVersionKind()
	if !acceptedK8sTypes.MatchString(groupVersionKind.Kind) {
		log.Printf("The custom-roles configMap contained K8s object types which are not supported! Skipping object with type: %s", groupVersionKind.Kind)
	} else {
		switch obj.(type) {
		case nil:
		case *corev1.Pod:
		case *rbacv1.Role:
		case *rbacv1.RoleBinding:
		case *rbacv1.ClusterRole:
		case *rbacv1.ClusterRoleBinding:
		case *corev1.ServiceAccount:
		default:
		}
	}
}
