package io

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/kubernetes"
)

func parse(yamlbytes []byte) ([]client.Object, error) {

	// Parsing approach adapted from
	// https://dx13.co.uk/articles/2021/01/15/kubernetes-types-using-go/

	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(yamlbytes), 4096)
	retVal := make([]runtime.Object, 0)

	if err := kubernetes.RegisterSchemes(); err != nil {
		return nil, errors.Wrapf(err, "register schemes")
	}
	decode := kubernetes.Codecs.UniversalDeserializer().Decode

	var errs []error

	for {
		var raw runtime.RawExtension
		if err := decoder.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			errs = append(errs, errors.NewParseError("YAML document", "failed to decode document", 0, 0, err))
			continue
		}
		if len(bytes.TrimSpace(raw.Raw)) == 0 {
			continue
		}
		obj, _, err := decode(raw.Raw, nil, nil)
		if err != nil {
			errs = append(errs, errors.NewParseError("Kubernetes object", "failed to decode object", 0, 0, err))
			continue
		}
		if err := checkType(obj); err != nil {
			errs = append(errs, err)
			continue
		}
		retVal = append(retVal, obj)
	}

	retValCO := make([]client.Object, len(retVal))
	for i, obj := range retVal {
		retValCO[i] = obj.(client.Object)
	}
	if len(errs) > 0 {
		return retValCO, &errors.ParseErrors{Errors: errs}
	}
	return retValCO, nil
}

// ParseFile reads the YAML file at path and returns the runtime objects
// defined within. Each object is decoded using the k8s scheme. An error is
// returned if the file cannot be read or if decoding any document fails.
func ParseFile(path string) ([]client.Object, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parse(data)
}

// ParseYAML parses YAML bytes and returns the runtime objects
// defined within. Each object is decoded using the k8s scheme. An error is
// returned if decoding any document fails.
func ParseYAML(data []byte) ([]client.Object, error) {
	return parse(data)
}

func checkType(obj runtime.Object) error {
	if obj == nil {
		return errors.ErrNilRuntimeObject
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	if err := kubernetes.RegisterSchemes(); err != nil {
		return errors.Wrapf(err, "register schemes")
	}
	expected, ok := kubernetes.Scheme.AllKnownTypes()[gvk]
	if !ok {
		return errors.Wrapf(errors.ErrUnsupportedKind, "kind %s", gvk.String())
	}

	objType := reflect.TypeOf(obj)
	if objType != expected && objType != reflect.PointerTo(expected) {
		return errors.NewParseError("Kubernetes object", fmt.Sprintf("kind %s expected type %v but got %T", gvk.Kind, expected, obj), 0, 0, nil)
	}

	return nil
}
