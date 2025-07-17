package fluxcd

import (
	v1 "github.com/fluxcd/notification-controller/api/v1"
	notificationv1beta2 "github.com/fluxcd/notification-controller/api/v1beta2"
	"github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

func SetProviderType(provider *notificationv1beta2.Provider, t string) {
	provider.Spec.Type = t
}

func SetProviderInterval(provider *notificationv1beta2.Provider, d metav1.Duration) {
	provider.Spec.Interval = &d
}

func SetProviderChannel(provider *notificationv1beta2.Provider, channel string) {
	provider.Spec.Channel = channel
}

func SetProviderUsername(provider *notificationv1beta2.Provider, username string) {
	provider.Spec.Username = username
}

func SetProviderAddress(provider *notificationv1beta2.Provider, address string) {
	provider.Spec.Address = address
}

func SetProviderTimeout(provider *notificationv1beta2.Provider, d metav1.Duration) {
	provider.Spec.Timeout = &d
}

func SetProviderProxy(provider *notificationv1beta2.Provider, proxy string) {
	provider.Spec.Proxy = proxy
}

func SetProviderSecretRef(provider *notificationv1beta2.Provider, ref *meta.LocalObjectReference) {
	provider.Spec.SecretRef = ref
}

func SetProviderCertSecretRef(provider *notificationv1beta2.Provider, ref *meta.LocalObjectReference) {
	provider.Spec.CertSecretRef = ref
}

func SetProviderSuspend(provider *notificationv1beta2.Provider, suspend bool) {
	provider.Spec.Suspend = suspend
}

// Alert helpers
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

func SetAlertProviderRef(alert *notificationv1beta2.Alert, ref meta.LocalObjectReference) {
	alert.Spec.ProviderRef = ref
}

func AddAlertEventSource(alert *notificationv1beta2.Alert, ref v1.CrossNamespaceObjectReference) {
	alert.Spec.EventSources = append(alert.Spec.EventSources, ref)
}

func AddAlertInclusion(alert *notificationv1beta2.Alert, regex string) {
	alert.Spec.InclusionList = append(alert.Spec.InclusionList, regex)
}

func AddAlertExclusion(alert *notificationv1beta2.Alert, regex string) {
	alert.Spec.ExclusionList = append(alert.Spec.ExclusionList, regex)
}

func AddAlertEventMetadata(alert *notificationv1beta2.Alert, key, value string) {
	if alert.Spec.EventMetadata == nil {
		alert.Spec.EventMetadata = make(map[string]string)
	}
	alert.Spec.EventMetadata[key] = value
}

func SetAlertEventSeverity(alert *notificationv1beta2.Alert, sev string) {
	alert.Spec.EventSeverity = sev
}

func SetAlertSummary(alert *notificationv1beta2.Alert, summary string) {
	alert.Spec.Summary = summary
}

func SetAlertSuspend(alert *notificationv1beta2.Alert, suspend bool) {
	alert.Spec.Suspend = suspend
}

// Receiver helpers
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

func SetReceiverType(receiver *notificationv1beta2.Receiver, t string) {
	receiver.Spec.Type = t
}

func SetReceiverInterval(receiver *notificationv1beta2.Receiver, d metav1.Duration) {
	receiver.Spec.Interval = &d
}

func AddReceiverEvent(receiver *notificationv1beta2.Receiver, event string) {
	receiver.Spec.Events = append(receiver.Spec.Events, event)
}

func AddReceiverResource(receiver *notificationv1beta2.Receiver, ref v1.CrossNamespaceObjectReference) {
	receiver.Spec.Resources = append(receiver.Spec.Resources, ref)
}

func SetReceiverSecretRef(receiver *notificationv1beta2.Receiver, ref meta.LocalObjectReference) {
	receiver.Spec.SecretRef = ref
}

func SetReceiverSuspend(receiver *notificationv1beta2.Receiver, suspend bool) {
	receiver.Spec.Suspend = suspend
}
