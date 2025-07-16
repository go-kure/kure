package yaml

import (
	"fmt"
	"log"
	"reflect"
	"strings"

	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	imagev1 "github.com/fluxcd/image-automation-controller/api/v1beta2"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	notificationv1beta2 "github.com/fluxcd/notification-controller/api/v1beta2"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

func init() {
	if err := kustv1.AddToScheme(scheme.Scheme); err != nil {
		log.Printf("failed to register kustomize scheme: %v", err)
	}
	if err := helmv2.AddToScheme(scheme.Scheme); err != nil {
		log.Printf("failed to register helm scheme: %v", err)
	}
	if err := imagev1.AddToScheme(scheme.Scheme); err != nil {
		log.Printf("failed to register image automation scheme: %v", err)
	}
	if err := notificationv1beta2.AddToScheme(scheme.Scheme); err != nil {
		log.Printf("failed to register notification scheme: %v", err)
	}
	if err := sourcev1.AddToScheme(scheme.Scheme); err != nil {
		log.Printf("failed to register source scheme: %v", err)
	}
	if err := certv1.AddToScheme(scheme.Scheme); err != nil {
		log.Printf("failed to register cert-manager scheme: %v", err)
	}
	if err := metallbv1beta1.AddToScheme(scheme.Scheme); err != nil {
		log.Printf("failed to register metallb scheme: %v", err)
	}
}

func parse(yamlbytes []byte) []runtime.Object {

	/*
	   https://dx13.co.uk/articles/2021/01/15/kubernetes-types-using-go/
	*/

	fileAsString := string(yamlbytes[:])
	sepYamlfiles := strings.Split(fileAsString, "---")
	retVal := make([]runtime.Object, 0, len(sepYamlfiles))

	decode := scheme.Codecs.UniversalDeserializer().Decode

	for _, f := range sepYamlfiles {
		// skip empty documents, `Decode` will fail on them
		if len(strings.TrimSpace(f)) == 0 {
			continue
		}
		obj, _, err := decode([]byte(f), nil, nil)

		if err != nil {
			log.Fatal(fmt.Sprintf("Error while decoding YAML object. Err was: %s", err))
			continue
		}
		if err := checkType(obj); err != nil {
			log.Printf("skipping unsupported object: %v", err)
			continue
		}
		retVal = append(retVal, obj)
	}

	return retVal
}

func checkType(obj runtime.Object) error {
	if obj == nil {
		return fmt.Errorf("nil runtime object provided")
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	expected, ok := scheme.Scheme.AllKnownTypes()[gvk]
	if !ok {
		return fmt.Errorf("unsupported object kind %s", gvk.String())
	}

	objType := reflect.TypeOf(obj)
	if objType != expected && objType != reflect.PointerTo(expected) {
		return fmt.Errorf("object kind %s expected type %v but got %T", gvk.Kind, expected, obj)
	}

	return nil
}
