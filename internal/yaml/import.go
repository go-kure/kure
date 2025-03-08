package yaml

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
)

func parse(yamlbytes []byte) []runtime.Object {

	/*
	   https://dx13.co.uk/articles/2021/01/15/kubernetes-types-using-go/
	*/

	fileAsString := string(yamlbytes[:])
	sepYamlfiles := strings.Split(fileAsString, "---")
	retVal := make([]runtime.Object, 0, len(sepYamlfiles))

	err := kustv1.AddToScheme(scheme.Scheme)
	if err != nil {
	}
	decode := scheme.Codecs.UniversalDeserializer().Decode

	for _, f := range sepYamlfiles {
		// skip empty documents, `Decode` will fail on them
		if len(f) == 0 {
			continue
		}
		obj, _, err := decode([]byte(f), nil, nil)

		if err != nil {
			log.Fatal(fmt.Sprintf("Error while decoding YAML object. Err was: %s", err))
			continue
		}
		retVal = append(retVal, obj)
	}

	return retVal
}

func checktype(obj runtime.Object) {
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
