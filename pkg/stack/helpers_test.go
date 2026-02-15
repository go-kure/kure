package stack

import "sigs.k8s.io/controller-runtime/pkg/client"

// fakeConfig implements the ApplicationConfig interface for testing.
type fakeConfig struct {
	objs []*client.Object
	err  error
}

func (f *fakeConfig) Generate(_ *Application) ([]*client.Object, error) {
	return f.objs, f.err
}

// validatingConfig implements both ApplicationConfig and Validator for testing.
type validatingConfig struct {
	objs           []*client.Object
	err            error
	validateErr    error
	generateCalled bool
}

func (v *validatingConfig) Validate() error {
	return v.validateErr
}

func (v *validatingConfig) Generate(_ *Application) ([]*client.Object, error) {
	v.generateCalled = true
	return v.objs, v.err
}
