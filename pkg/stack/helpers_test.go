package stack

import "sigs.k8s.io/controller-runtime/pkg/client"

// fakeConfig implements the Config interface for testing.
type fakeConfig struct {
	objs []*client.Object
	err  error
}

func (f *fakeConfig) Generate(_ *Application) ([]*client.Object, error) {
	return f.objs, f.err
}
