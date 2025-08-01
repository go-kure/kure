package stack_test

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
)

// fakeConfig implements the ApplicationConfig interface for testing.
type fakeConfig struct {
	objs []*client.Object
	err  error
}

func (f *fakeConfig) Generate(_ *stack.Application) ([]*client.Object, error) {
	return f.objs, f.err
}
