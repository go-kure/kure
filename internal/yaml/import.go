package yaml

import (
	"fmt"
	"log"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
)

// ParseErrors aggregates errors encountered while parsing a YAML document.
type ParseErrors struct {
	Errs []error
}

func (e *ParseErrors) Error() string {
	if len(e.Errs) == 0 {
		return ""
	}
	if len(e.Errs) == 1 {
		return e.Errs[0].Error()
	}
	sb := strings.Builder{}
	sb.WriteString("multiple parse errors:")
	for _, err := range e.Errs {
		sb.WriteString("\n - ")
		sb.WriteString(err.Error())
	}
	return sb.String()
}

func (e *ParseErrors) Unwrap() []error { return e.Errs }

func parse(yamlbytes []byte) ([]runtime.Object, error) {

	/*
	   https://dx13.co.uk/articles/2021/01/15/kubernetes-types-using-go/
	*/

	fileAsString := string(yamlbytes[:])
	sepYamlfiles := strings.Split(fileAsString, "---")
	retVal := make([]runtime.Object, 0, len(sepYamlfiles))

	if err := kustv1.AddToScheme(scheme.Scheme); err != nil {
		log.Printf("failed to register kustomize scheme: %v", err)
	}
	decode := scheme.Codecs.UniversalDeserializer().Decode
	errs := make([]error, 0)

	for _, f := range sepYamlfiles {
		// skip empty documents, `Decode` will fail on them
		if len(strings.TrimSpace(f)) == 0 {
			continue
		}
		obj, _, err := decode([]byte(f), nil, nil)

		if err != nil {
			errs = append(errs, fmt.Errorf("decode error: %w", err))
			continue
		}
		if err := checkType(obj); err != nil {
			errs = append(errs, err)
			continue
		}
		retVal = append(retVal, obj)
	}

	if len(errs) > 0 {
		return retVal, &ParseErrors{Errs: errs}
	}
	return retVal, nil
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
