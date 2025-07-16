package yaml

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/runtime"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"

	certv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	imagev1 "github.com/fluxcd/image-automation-controller/api/v1beta2"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	notifv1beta2 "github.com/fluxcd/notification-controller/api/v1beta2"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
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

var (
	schemeOnce sync.Once
	schemeErr  error
)

func registerSchemes() error {
	schemeOnce.Do(func() {
		var errs []error
		if err := kustv1.AddToScheme(scheme.Scheme); err != nil {
			errs = append(errs, fmt.Errorf("kustomize: %w", err))
		}
		if err := helmv2.AddToScheme(scheme.Scheme); err != nil {
			errs = append(errs, fmt.Errorf("helm: %w", err))
		}
		if err := imagev1.AddToScheme(scheme.Scheme); err != nil {
			errs = append(errs, fmt.Errorf("image: %w", err))
		}
		if err := notifv1beta2.AddToScheme(scheme.Scheme); err != nil {
			errs = append(errs, fmt.Errorf("notification: %w", err))
		}
		if err := sourcev1.AddToScheme(scheme.Scheme); err != nil {
			errs = append(errs, fmt.Errorf("source: %w", err))
		}
		if err := certv1.AddToScheme(scheme.Scheme); err != nil {
			errs = append(errs, fmt.Errorf("cert-manager: %w", err))
		}
		if err := metallbv1beta1.AddToScheme(scheme.Scheme); err != nil {
			errs = append(errs, fmt.Errorf("metallb: %w", err))
		}
		if len(errs) > 0 {
			schemeErr = &ParseErrors{Errors: errs}
		}
	})
	return schemeErr
}

func parse(yamlbytes []byte) ([]runtime.Object, error) {

	/*
	   https://dx13.co.uk/articles/2021/01/15/kubernetes-types-using-go/
	*/

	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(yamlbytes), 4096)
	retVal := make([]runtime.Object, 0)

	if err := registerSchemes(); err != nil {
		return nil, fmt.Errorf("register schemes: %w", err)
	}
	decode := scheme.Codecs.UniversalDeserializer().Decode

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
