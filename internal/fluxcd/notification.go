package fluxcd

import (
	v1 "github.com/fluxcd/notification-controller/api/v1"
	notificationv1beta2 "github.com/fluxcd/notification-controller/api/v1beta2"
	"github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateProvider returns a new Provider object with the given name, namespace
// and specification.
func CreateProvider(name, namespace string, spec notificationv1beta2.ProviderSpec) *notificationv1beta2.Provider {
	obj := &notificationv1beta2.Provider{
		TypeMeta: metav1.TypeMeta{
			Kind:       notificationv1beta2.ProviderKind,
			APIVersion: notificationv1beta2.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	return obj
}

// SetProviderType sets the notification provider type.
func SetProviderType(provider *notificationv1beta2.Provider, t string) {
	provider.Spec.Type = t
}

// SetProviderInterval configures the interval at which events are sent.
func SetProviderInterval(provider *notificationv1beta2.Provider, d metav1.Duration) {
	provider.Spec.Interval = &d
}

// SetProviderChannel specifies the target channel for notifications.
func SetProviderChannel(provider *notificationv1beta2.Provider, channel string) {
	provider.Spec.Channel = channel
}

// SetProviderUsername configures the username on the provider spec.
func SetProviderUsername(provider *notificationv1beta2.Provider, username string) {
	provider.Spec.Username = username
}

// SetProviderAddress sets the provider address.
func SetProviderAddress(provider *notificationv1beta2.Provider, address string) {
	provider.Spec.Address = address
}

// SetProviderTimeout sets the timeout for sending notifications.
func SetProviderTimeout(provider *notificationv1beta2.Provider, d metav1.Duration) {
	provider.Spec.Timeout = &d
}

// SetProviderProxy sets the HTTP proxy used when sending events.
func SetProviderProxy(provider *notificationv1beta2.Provider, proxy string) {
	provider.Spec.Proxy = proxy
}

// SetProviderSecretRef attaches a Secret reference to the provider.
func SetProviderSecretRef(provider *notificationv1beta2.Provider, ref *meta.LocalObjectReference) {
	provider.Spec.SecretRef = ref
}

// SetProviderCertSecretRef attaches a certificate Secret reference to the provider.
func SetProviderCertSecretRef(provider *notificationv1beta2.Provider, ref *meta.LocalObjectReference) {
	provider.Spec.CertSecretRef = ref
}

// SetProviderSuspend sets the suspend flag on the provider.
func SetProviderSuspend(provider *notificationv1beta2.Provider, suspend bool) {
	provider.Spec.Suspend = suspend
}

// Alert helpers

// CreateAlert returns a new Alert object configured with the given name,
// namespace and specification.
func CreateAlert(name, namespace string, spec notificationv1beta2.AlertSpec) *notificationv1beta2.Alert {
	obj := &notificationv1beta2.Alert{
		TypeMeta: metav1.TypeMeta{
			Kind:       notificationv1beta2.AlertKind,
			APIVersion: notificationv1beta2.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	return obj
}

// SetAlertProviderRef sets the provider reference for an alert.
func SetAlertProviderRef(alert *notificationv1beta2.Alert, ref meta.LocalObjectReference) {
	alert.Spec.ProviderRef = ref
}

// AddAlertEventSource appends an event source to the alert specification.
func AddAlertEventSource(alert *notificationv1beta2.Alert, ref v1.CrossNamespaceObjectReference) {
	alert.Spec.EventSources = append(alert.Spec.EventSources, ref)
}

// AddAlertInclusion adds a regex pattern to the inclusion list.
func AddAlertInclusion(alert *notificationv1beta2.Alert, regex string) {
	alert.Spec.InclusionList = append(alert.Spec.InclusionList, regex)
}

// AddAlertExclusion adds a regex pattern to the exclusion list.
func AddAlertExclusion(alert *notificationv1beta2.Alert, regex string) {
	alert.Spec.ExclusionList = append(alert.Spec.ExclusionList, regex)
}

// AddAlertEventMetadata sets a metadata key/value on the alert.
func AddAlertEventMetadata(alert *notificationv1beta2.Alert, key, value string) {
	if alert.Spec.EventMetadata == nil {
		alert.Spec.EventMetadata = make(map[string]string)
	}
	alert.Spec.EventMetadata[key] = value
}

// SetAlertEventSeverity sets the severity level for events.
func SetAlertEventSeverity(alert *notificationv1beta2.Alert, sev string) {
	alert.Spec.EventSeverity = sev
}

// SetAlertSummary sets the alert summary message.
func SetAlertSummary(alert *notificationv1beta2.Alert, summary string) {
	alert.Spec.Summary = summary
}

// SetAlertSuspend toggles the suspend flag for the alert.
func SetAlertSuspend(alert *notificationv1beta2.Alert, suspend bool) {
	alert.Spec.Suspend = suspend
}

// Receiver helpers

// CreateReceiver returns a new Receiver object configured with the given name,
// namespace and specification.
func CreateReceiver(name, namespace string, spec notificationv1beta2.ReceiverSpec) *notificationv1beta2.Receiver {
	obj := &notificationv1beta2.Receiver{
		TypeMeta: metav1.TypeMeta{
			Kind:       notificationv1beta2.ReceiverKind,
			APIVersion: notificationv1beta2.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	return obj
}

// SetReceiverType sets the receiver type.
func SetReceiverType(receiver *notificationv1beta2.Receiver, t string) {
	receiver.Spec.Type = t
}

// SetReceiverInterval configures how often resources are scanned.
func SetReceiverInterval(receiver *notificationv1beta2.Receiver, d metav1.Duration) {
	receiver.Spec.Interval = &d
}

// AddReceiverEvent appends an event to the receiver specification.
func AddReceiverEvent(receiver *notificationv1beta2.Receiver, event string) {
	receiver.Spec.Events = append(receiver.Spec.Events, event)
}

// AddReceiverResource registers a resource reference on the receiver.
func AddReceiverResource(receiver *notificationv1beta2.Receiver, ref v1.CrossNamespaceObjectReference) {
	receiver.Spec.Resources = append(receiver.Spec.Resources, ref)
}

// SetReceiverSecretRef adds a Secret reference to the receiver.
func SetReceiverSecretRef(receiver *notificationv1beta2.Receiver, ref meta.LocalObjectReference) {
	receiver.Spec.SecretRef = ref
}

// SetReceiverSuspend toggles the suspend flag for the receiver.
func SetReceiverSuspend(receiver *notificationv1beta2.Receiver, suspend bool) {
	receiver.Spec.Suspend = suspend
}
