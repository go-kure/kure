package errors

import (
	"errors"
	"fmt"
	"strings"
)

// Wrap wraps an error with a message using Go's standard error wrapping.
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// Wrapf wraps an error with a formatted message using Go's standard error wrapping.
func Wrapf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf(format+": %w", append(args, err)...)
}

// Standard error variables using the standard library
var (
	ErrGVKNotFound   = errors.New("could not determine GroupVersionKind")
	ErrGVKNotAllowed = errors.New("GroupVersionKind is not allowed")
	ErrNilObject     = errors.New("provided object is nil")
)

// Common Kubernetes resource validation errors
var (
	// Nil resource errors
	ErrNilDeployment        = ResourceValidationError("Deployment", "", "deployment", "deployment cannot be nil", nil)
	ErrNilPod               = ResourceValidationError("Pod", "", "pod", "pod cannot be nil", nil)
	ErrNilPodSpec           = ResourceValidationError("PodSpec", "", "spec", "pod spec cannot be nil", nil)
	ErrNilContainer         = ResourceValidationError("Container", "", "container", "container cannot be nil", nil)
	ErrNilStatefulSet       = ResourceValidationError("StatefulSet", "", "statefulset", "statefulset cannot be nil", nil)
	ErrNilDaemonSet         = ResourceValidationError("DaemonSet", "", "daemonset", "daemonset cannot be nil", nil)
	ErrNilJob               = ResourceValidationError("Job", "", "job", "job cannot be nil", nil)
	ErrNilCronJob           = ResourceValidationError("CronJob", "", "cronjob", "cronjob cannot be nil", nil)
	ErrNilService           = ResourceValidationError("Service", "", "service", "service cannot be nil", nil)
	ErrNilSecret            = ResourceValidationError("Secret", "", "secret", "secret cannot be nil", nil)
	ErrNilConfigMap         = ResourceValidationError("ConfigMap", "", "configmap", "configmap cannot be nil", nil)
	ErrNilServiceAccount    = ResourceValidationError("ServiceAccount", "", "serviceaccount", "serviceaccount cannot be nil", nil)
	ErrNilIngress           = ResourceValidationError("Ingress", "", "ingress", "ingress cannot be nil", nil)
	ErrNilBundle            = ResourceValidationError("Bundle", "", "bundle", "bundle cannot be nil", nil)

	// Common field validation errors
	ErrNilSpec              = ResourceValidationError("Resource", "", "spec", "spec cannot be nil", nil)
	ErrNilInitContainer     = ResourceValidationError("PodSpec", "", "container", "init container cannot be nil", nil)
	ErrNilEphemeralContainer = ResourceValidationError("PodSpec", "", "container", "ephemeral container cannot be nil", nil)
	ErrNilVolume            = ResourceValidationError("PodSpec", "", "volume", "volume cannot be nil", nil)
	ErrNilImagePullSecret   = ResourceValidationError("PodSpec", "", "secret", "image pull secret cannot be nil", nil)
	ErrNilToleration        = ResourceValidationError("PodSpec", "", "toleration", "toleration cannot be nil", nil)
	
	// Additional resource errors
	ErrNilNamespace            = ResourceValidationError("Namespace", "", "namespace", "namespace cannot be nil", nil)
	ErrNilRole                 = ResourceValidationError("Role", "", "role", "role cannot be nil", nil)
	ErrNilClusterRole          = ResourceValidationError("ClusterRole", "", "clusterrole", "cluster role cannot be nil", nil)
	ErrNilRoleBinding          = ResourceValidationError("RoleBinding", "", "rolebinding", "role binding cannot be nil", nil)
	ErrNilClusterRoleBinding   = ResourceValidationError("ClusterRoleBinding", "", "clusterrolebinding", "cluster role binding cannot be nil", nil)
	ErrNilServicePort          = ResourceValidationError("Service", "", "port", "service port cannot be nil", nil)
	ErrNilPodDisruptionBudget  = ResourceValidationError("PodDisruptionBudget", "", "pdb", "pod disruption budget cannot be nil", nil)
	ErrNilKustomization        = ResourceValidationError("Kustomization", "", "kustomization", "kustomization cannot be nil", nil)
	
	// Flux resources
	ErrNilFluxInstance         = ResourceValidationError("FluxInstance", "", "fluxinstance", "flux instance cannot be nil", nil)
	
	// MetalLB resources
	ErrNilIPAddressPool        = ResourceValidationError("IPAddressPool", "", "ipaddresspool", "ip address pool cannot be nil", nil)
	ErrNilBGPPeer              = ResourceValidationError("BGPPeer", "", "bgppeer", "bgp peer cannot be nil", nil)
	ErrNilBGPAdvertisement     = ResourceValidationError("BGPAdvertisement", "", "bgpadvertisement", "bgp advertisement cannot be nil", nil)
	ErrNilL2Advertisement      = ResourceValidationError("L2Advertisement", "", "l2advertisement", "l2 advertisement cannot be nil", nil)
	ErrNilBFDProfile           = ResourceValidationError("BFDProfile", "", "bfdprofile", "bfd profile cannot be nil", nil)
)

// Common file operation errors
var (
	ErrFileNotFound        = errors.New("file not found")
	ErrDirectoryNotFound   = errors.New("directory not found")
	ErrInvalidPath         = errors.New("invalid file path")
)

// Common parse/processing errors  
var (
	ErrNilRuntimeObject    = errors.New("nil runtime object provided")
	ErrSchemeRegistration  = errors.New("failed to register schemes")
	ErrUnsupportedKind     = errors.New("unsupported object kind")
	ErrInteractiveMode     = errors.New("interactive mode not yet implemented")
)

// Common configuration errors
var (
	ErrInvalidOutputFormat = errors.New("invalid output format")
	ErrInvalidGrouping     = errors.New("invalid grouping option")
	ErrInvalidPlacement    = errors.New("invalid placement option")
)

// ErrorType represents the category of error
type ErrorType string

const (
	ErrorTypeValidation    ErrorType = "validation"
	ErrorTypeResource      ErrorType = "resource"
	ErrorTypePatch         ErrorType = "patch"
	ErrorTypeParse         ErrorType = "parse"
	ErrorTypeFile          ErrorType = "file"
	ErrorTypeConfiguration ErrorType = "configuration"
	ErrorTypeInternal      ErrorType = "internal"
)

// KureError is the base interface for all Kure-specific errors
type KureError interface {
	error
	Type() ErrorType
	Suggestion() string
	Context() map[string]interface{}
}

// BaseError provides common functionality for all Kure errors
type BaseError struct {
	ErrType    ErrorType              `json:"type"`
	Message    string                 `json:"message"`
	Cause      error                  `json:"cause,omitempty"`
	ErrContext map[string]interface{} `json:"context,omitempty"`
	Help       string                 `json:"suggestion,omitempty"`
}

func (e *BaseError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *BaseError) Type() ErrorType {
	return e.ErrType
}

func (e *BaseError) Suggestion() string {
	return e.Help
}

func (e *BaseError) Context() map[string]interface{} {
	return e.ErrContext
}

func (e *BaseError) Unwrap() error {
	return e.Cause
}

// ValidationError represents validation failures with suggestions
type ValidationError struct {
	*BaseError
	Field       string   `json:"field"`
	Value       string   `json:"value"`
	ValidValues []string `json:"validValues,omitempty"`
	Component   string   `json:"component"`
}

func NewValidationError(field, value, component string, validValues []string) *ValidationError {
	help := ""
	if len(validValues) > 0 {
		help = fmt.Sprintf("Valid values are: %s", strings.Join(validValues, ", "))
	}

	return &ValidationError{
		BaseError: &BaseError{
			ErrType: ErrorTypeValidation,
			Message: fmt.Sprintf("invalid %s for %s: %s", field, component, value),
			Help:    help,
			ErrContext: map[string]interface{}{
				"field":     field,
				"value":     value,
				"component": component,
			},
		},
		Field:       field,
		Value:       value,
		ValidValues: validValues,
		Component:   component,
	}
}

// ResourceError represents resource-related errors
type ResourceError struct {
	*BaseError
	ResourceType string   `json:"resourceType"`
	Name         string   `json:"name"`
	Namespace    string   `json:"namespace,omitempty"`
	Available    []string `json:"available,omitempty"`
}

func ResourceNotFoundError(resourceType, name, namespace string, available []string) *ResourceError {
	help := "Check that the resource exists and is spelled correctly"
	if len(available) > 0 {
		help = fmt.Sprintf("Available resources: %s", strings.Join(available, ", "))
	}

	message := fmt.Sprintf("%s '%s' not found", resourceType, name)
	if namespace != "" {
		message += fmt.Sprintf(" in namespace '%s'", namespace)
	}

	return &ResourceError{
		BaseError: &BaseError{
			ErrType: ErrorTypeResource,
			Message: message,
			Help:    help,
			ErrContext: map[string]interface{}{
				"resourceType": resourceType,
				"name":         name,
				"namespace":    namespace,
			},
		},
		ResourceType: resourceType,
		Name:         name,
		Namespace:    namespace,
		Available:    available,
	}
}

func ResourceValidationError(resourceType, name, field, reason string, cause error) *ResourceError {
	message := fmt.Sprintf("validation failed for %s '%s'", resourceType, name)
	if field != "" {
		message += fmt.Sprintf(" field '%s'", field)
	}
	if reason != "" {
		message += fmt.Sprintf(": %s", reason)
	}

	return &ResourceError{
		BaseError: &BaseError{
			ErrType:    ErrorTypeResource,
			Message:    message,
			Cause:      cause,
			Help:       "Check the resource specification and ensure all required fields are present",
			ErrContext: map[string]interface{}{
				"resourceType": resourceType,
				"name":         name,
				"field":        field,
				"reason":       reason,
			},
		},
		ResourceType: resourceType,
		Name:         name,
	}
}

// PatchError represents patch-specific errors
type PatchError struct {
	*BaseError
	Operation    string `json:"operation"`
	Path         string `json:"path"`
	ResourceName string `json:"resourceName"`
}

func NewPatchError(operation, path, resourceName, reason string, cause error) *PatchError {
	message := fmt.Sprintf("patch operation '%s' failed", operation)
	if resourceName != "" {
		message += fmt.Sprintf(" on resource '%s'", resourceName)
	}
	if path != "" {
		message += fmt.Sprintf(" at path '%s'", path)
	}
	if reason != "" {
		message += fmt.Sprintf(": %s", reason)
	}

	help := "Check the patch syntax and ensure the target path exists"
	if strings.Contains(reason, "not found") || strings.Contains(reason, "missing") {
		help = "Verify the resource name and path are correct, or use graceful mode to skip missing targets"
	}

	return &PatchError{
		BaseError: &BaseError{
			ErrType:    ErrorTypePatch,
			Message:    message,
			Cause:      cause,
			Help:       help,
			ErrContext: map[string]interface{}{
				"operation":    operation,
				"path":         path,
				"resourceName": resourceName,
				"reason":       reason,
			},
		},
		Operation:    operation,
		Path:         path,
		ResourceName: resourceName,
	}
}

// ParseError represents parsing errors with location information
type ParseError struct {
	*BaseError
	Source string `json:"source"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
}

func NewParseError(source, reason string, line, column int, cause error) *ParseError {
	message := fmt.Sprintf("parse error in %s", source)
	if line > 0 {
		message += fmt.Sprintf(" at line %d", line)
		if column > 0 {
			message += fmt.Sprintf(", column %d", column)
		}
	}
	if reason != "" {
		message += fmt.Sprintf(": %s", reason)
	}

	help := "Check the file syntax and format"
	if strings.Contains(reason, "YAML") || strings.Contains(reason, "yaml") {
		help = "Verify YAML syntax, check indentation and special characters"
	} else if strings.Contains(reason, "JSON") || strings.Contains(reason, "json") {
		help = "Verify JSON syntax, check brackets and commas"
	}

	return &ParseError{
		BaseError: &BaseError{
			ErrType:    ErrorTypeParse,
			Message:    message,
			Cause:      cause,
			Help:       help,
			ErrContext: map[string]interface{}{
				"source": source,
				"line":   line,
				"column": column,
				"reason": reason,
			},
		},
		Source: source,
		Line:   line,
		Column: column,
	}
}

// FileError represents file operation errors
type FileError struct {
	*BaseError
	Operation string `json:"operation"`
	Path      string `json:"path"`
}

func NewFileError(operation, path, reason string, cause error) *FileError {
	message := fmt.Sprintf("file %s failed for '%s'", operation, path)
	if reason != "" {
		message += fmt.Sprintf(": %s", reason)
	}

	help := getFileErrorSuggestion(operation, cause)

	return &FileError{
		BaseError: &BaseError{
			ErrType:    ErrorTypeFile,
			Message:    message,
			Cause:      cause,
			Help:       help,
			ErrContext: map[string]interface{}{
				"operation": operation,
				"path":      path,
				"reason":    reason,
			},
		},
		Operation: operation,
		Path:      path,
	}
}

func getFileErrorSuggestion(operation string, cause error) string {
	if cause == nil {
		return "Check file path and permissions"
	}

	errMsg := cause.Error()
	switch {
	case strings.Contains(errMsg, "permission denied"):
		return "Check file permissions and ensure you have appropriate access"
	case strings.Contains(errMsg, "no such file"):
		return "Verify the file path exists and is spelled correctly"
	case strings.Contains(errMsg, "is a directory"):
		return "The path points to a directory, specify a file instead"
	case strings.Contains(errMsg, "disk") || strings.Contains(errMsg, "space"):
		return "Check available disk space"
	default:
		return "Check file path and permissions"
	}
}

// ConfigError represents configuration errors
type ConfigError struct {
	*BaseError
	Source      string   `json:"source"`
	Field       string   `json:"field"`
	ValidValues []string `json:"validValues,omitempty"`
}

func NewConfigError(source, field, value, reason string, validValues []string) *ConfigError {
	message := fmt.Sprintf("configuration error in %s", source)
	if field != "" {
		message += fmt.Sprintf(" for field '%s'", field)
	}
	if value != "" {
		message += fmt.Sprintf(" with value '%s'", value)
	}
	if reason != "" {
		message += fmt.Sprintf(": %s", reason)
	}

	help := "Check the configuration syntax and values"
	if len(validValues) > 0 {
		help = fmt.Sprintf("Valid values for %s: %s", field, strings.Join(validValues, ", "))
	}

	return &ConfigError{
		BaseError: &BaseError{
			ErrType:    ErrorTypeConfiguration,
			Message:    message,
			Help:       help,
			ErrContext: map[string]interface{}{
				"source": source,
				"field":  field,
				"value":  value,
				"reason": reason,
			},
		},
		Source:      source,
		Field:       field,
		ValidValues: validValues,
	}
}

// IsKureError checks if an error is a Kure-specific error
func IsKureError(err error) bool {
	_, ok := err.(KureError)
	return ok
}

// GetKureError extracts a KureError from an error chain
func GetKureError(err error) KureError {
	var kureErr KureError
	if errors.As(err, &kureErr) {
		return kureErr
	}
	return nil
}

// IsType checks if an error is of a specific Kure error type
func IsType(err error, errType ErrorType) bool {
	kureErr := GetKureError(err)
	if kureErr == nil {
		return false
	}
	return kureErr.Type() == errType
}
