package yaml

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
)

func parse(yamlbytes []byte) ([]runtime.Object, error) {

	/*
	   https://dx13.co.uk/articles/2021/01/15/kubernetes-types-using-go/
	*/

	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(yamlbytes), 4096)
	retVal := make([]runtime.Object, 0)

	if err := kustv1.AddToScheme(scheme.Scheme); err != nil {
		log.Printf("failed to register kustomize scheme: %v", err)
	}
	decode := scheme.Codecs.UniversalDeserializer().Decode

	for {
		var raw runtime.RawExtension
		if err := decoder.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal(fmt.Sprintf("Error while decoding YAML object. Err was: %s", err))
			continue
		}
		if len(bytes.TrimSpace(raw.Raw)) == 0 {
			continue
		}
		obj, _, err := decode(raw.Raw, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("decode YAML object: %w", err)
		}
		if err := checkType(obj); err != nil {
			log.Printf("skipping unsupported object: %v", err)
			continue
		}
		retVal = append(retVal, obj)
	}

	return retVal, nil
}

// ParseFile reads the YAML file at path and returns the runtime objects
// defined within. Each object is decoded using the client-go scheme. An error
// is returned if the file cannot be read or if decoding any document fails.
func ParseFile(path string) ([]runtime.Object, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parse(data)
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
