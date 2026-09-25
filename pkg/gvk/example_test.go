package gvk_test

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/go-kure/kure/pkg/gvk"
)

// The README's Go blocks are generated from these functions
// (scripts/gen-doc-examples.sh); each prints what it built so go test checks
// the example as well as compiling it.

// MyConfigType is the spec type the examples register.
type MyConfigType struct {
	Replicas int `yaml:"replicas"`
}

// OldType and NewType are two versions of one spec, for the conversion example:
// the newer version renamed the field.
type OldType struct{ Name string }

type NewType struct{ DisplayName string }

func ExampleRegistry_Create() {
	myGVK := gvk.GVK{
		Group:   "generators.gokure.dev",
		Version: "v1alpha1",
		Kind:    "MyGenerator",
	}

	registry := gvk.NewRegistry[MyConfigType]()
	registry.Register(myGVK, func() MyConfigType {
		return MyConfigType{Replicas: 1}
	})

	instance, err := registry.Create(myGVK)
	if err != nil {
		panic(err)
	}
	fmt.Println(myGVK.APIVersion(), instance.Replicas)
	// Output: generators.gokure.dev/v1alpha1 1
}

func ExampleNewTypedWrapper() {
	registry := gvk.NewRegistry[MyConfigType]()
	registry.Register(gvk.GVK{Group: "generators.gokure.dev", Version: "v1alpha1", Kind: "MyGenerator"},
		func() MyConfigType { return MyConfigType{} })

	data := []byte(`
apiVersion: generators.gokure.dev/v1alpha1
kind: MyGenerator
metadata:
  name: web
spec:
  replicas: 3
`)

	wrapper := gvk.NewTypedWrapper(registry)
	if err := yaml.Unmarshal(data, wrapper); err != nil {
		panic(err)
	}
	// wrapper.Spec contains the correctly typed instance
	fmt.Println(wrapper.GetName(), wrapper.Spec.Replicas)
	// Output: web 3
}

func ExampleConversionRegistry_RegisterFunc() {
	oldGVK := gvk.GVK{Group: "generators.gokure.dev", Version: "v1alpha1", Kind: "MyGenerator"}
	newGVK := gvk.GVK{Group: "generators.gokure.dev", Version: "v1beta1", Kind: "MyGenerator"}

	conversions := gvk.NewConversionRegistry()
	conversions.RegisterFunc(oldGVK, newGVK, func(from any) (any, error) {
		old := from.(OldType)
		return NewType{DisplayName: old.Name}, nil
	})

	converted, err := conversions.Convert(oldGVK, newGVK, OldType{Name: "web"})
	if err != nil {
		panic(err)
	}
	fmt.Printf("%+v\n", converted)
	// Output: {DisplayName:web}
}
