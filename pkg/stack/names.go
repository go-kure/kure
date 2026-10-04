package stack

import (
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/go-kure/kure/pkg/errors"
)

// KustomizationNameMaxLength is the longest name ValidateKustomizationName
// accepts. A Kustomization is a DNS-1123 subdomain, which Kubernetes allows
// up to 253 characters, but kustomize-controller writes the Kustomization's
// name into the label kustomize.toolkit.fluxcd.io/name on every object it
// applies, and a label value is at most 63 characters: a longer name is
// admitted by the API server and then fails every apply.
//
// Read in kustomize-controller v1.9.5: the reconciler hands the
// Kustomization's name to the apply manager as the owner of the objects
// (internal/controller/kustomization_controller.go:462), and the manager
// writes it as a label value (github.com/fluxcd/pkg/ssa v0.76.2,
// manager.go:66-78).
const KustomizationNameMaxLength = 63

// ValidateKustomizationName reports whether name can be the metadata.name of
// a Flux Kustomization that kustomize-controller can reconcile: a DNS-1123
// subdomain of at most KustomizationNameMaxLength characters. kure does not
// shorten or rewrite a name; the caller chooses a valid one.
func ValidateKustomizationName(name string) error {
	if len(name) > KustomizationNameMaxLength {
		return errors.Errorf("%q is %d characters long: a Flux Kustomization name is at most %d characters, because Flux writes it into a label value", name, len(name), KustomizationNameMaxLength)
	}
	if problems := validation.IsDNS1123Subdomain(name); len(problems) > 0 {
		return errors.Errorf("%q is not a valid Flux Kustomization name: %s", name, strings.Join(problems, "; "))
	}
	return nil
}

// ValidateDirectoryName reports whether name can be the name of one directory
// in a rendered tree: a single path segment. It refuses an empty name, "."
// and "..", and a name containing a path separator. Both "/" and "\" count on
// every platform: the tree is joined with path/filepath, which reads "\" as a
// separator on Windows, so a name such as `..\outside` would otherwise leave
// the directory it is joined under.
func ValidateDirectoryName(name string) error {
	switch {
	case name == "":
		return errors.New("a directory name must not be empty")
	case name == "." || name == "..":
		return errors.Errorf("%q is not a directory name of its own", name)
	case strings.ContainsAny(name, `/\`):
		return errors.Errorf("%q contains a path separator: a directory name is one path segment", name)
	}
	return nil
}
