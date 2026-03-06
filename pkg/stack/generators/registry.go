package generators

import (
	"github.com/go-kure/kure/pkg/gvk"
)

// Re-export GVK type for backward compatibility
type GVK = gvk.GVK

// Re-export common functions for backward compatibility
var (
	ParseAPIVersion = gvk.ParseAPIVersion
)
