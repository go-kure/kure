package kio

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/go-kure/kure/pkg/k8s"
)

// ParseErrors aggregates multiple errors returned during YAML decoding.
// It implements the error interface and unwraps to the underlying errors.
type ParseErrors struct {
	Errors []error
}

func (pe *ParseErrors) Error() string {
	if len(pe.Errors) == 0 {
		return ""
	}
	if len(pe.Errors) == 1 {
		return pe.Errors[0].Error()
	}
	var b strings.Builder
	b.WriteString("multiple parse errors:")
	for _, err := range pe.Errors {
		b.WriteString(" ")
		b.WriteString(err.Error())
		b.WriteString(";")
	}
	return strings.TrimSuffix(b.String(), ";")
}

func (pe *ParseErrors) Unwrap() []error {
	return pe.Errors
}

func parse(yamlbytes []byte) ([]runtime.Object, error) {

	// Parsing approach adapted from
	// https://dx13.co.uk/articles/2021/01/15/kubernetes-types-using-go/

	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(yamlbytes), 4096)
	retVal := make([]runtime.Object, 0)

	if err := k8s.RegisterSchemes(); err != nil {
		return nil, fmt.Errorf("register schemes: %w", err)
	}
	decode := k8s.Codecs.UniversalDeserializer().Decode

	var errs []error

	for {
		var raw runtime.RawExtension
		if err := decoder.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			errs = append(errs, fmt.Errorf("decode document: %w", err))
			continue
		}
		if len(bytes.TrimSpace(raw.Raw)) == 0 {
			continue
		}
		obj, _, err := decode(raw.Raw, nil, nil)
		if err != nil {
			errs = append(errs, fmt.Errorf("decode object: %w", err))
			continue
		}
		if err := checkType(obj); err != nil {
			errs = append(errs, err)
			continue
		}
		retVal = append(retVal, obj)
	}

	if len(errs) > 0 {
		return retVal, &ParseErrors{Errors: errs}
	}
	return retVal, nil
}

// ParseFile reads the YAML file at path and returns the runtime objects
// defined within. Each object is decoded using the k8s scheme. An error is
// returned if the file cannot be read or if decoding any document fails.
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
	if err := k8s.RegisterSchemes(); err != nil {
		return fmt.Errorf("register schemes: %w", err)
	}
	expected, ok := k8s.Scheme.AllKnownTypes()[gvk]
	if !ok {
		return fmt.Errorf("unsupported object kind %s", gvk.String())
	}

	objType := reflect.TypeOf(obj)
	if objType != expected && objType != reflect.PointerTo(expected) {
		return fmt.Errorf("object kind %s expected type %v but got %T", gvk.Kind, expected, obj)
	}

	return nil
}
