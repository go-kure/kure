package fluxcd

import (
	"testing"
	"time"

	v1 "github.com/fluxcd/notification-controller/api/v1"
	notificationv1beta2 "github.com/fluxcd/notification-controller/api/v1beta2"
	"github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateProvider(t *testing.T) {
	spec := notificationv1beta2.ProviderSpec{Type: notificationv1beta2.SlackProvider}
	p := CreateProvider("p", "ns", spec)

	if p.Name != "p" || p.Namespace != "ns" {
		t.Fatalf("metadata mismatch")
	}
	if p.Kind != notificationv1beta2.ProviderKind {
		t.Errorf("unexpected kind %s", p.Kind)
	}
	if p.APIVersion != notificationv1beta2.GroupVersion.String() {
		t.Errorf("unexpected apiVersion %s", p.APIVersion)
	}
	if p.Spec.Type != notificationv1beta2.SlackProvider {
		t.Errorf("spec not set")
	}
}

func TestProviderSetters(t *testing.T) {
	p := CreateProvider("p", "ns", notificationv1beta2.ProviderSpec{})
	SetProviderType(p, notificationv1beta2.GitHubProvider)
	if p.Spec.Type != notificationv1beta2.GitHubProvider {
		t.Errorf("type not set")
	}
	dur := metav1.Duration{Duration: time.Minute}
	SetProviderInterval(p, dur)
	if p.Spec.Interval == nil || p.Spec.Interval.Duration != time.Minute {
		t.Errorf("interval not set")
	}
	SetProviderChannel(p, "ch")
	if p.Spec.Channel != "ch" {
		t.Errorf("channel not set")
	}
	SetProviderUsername(p, "user")
	if p.Spec.Username != "user" {
		t.Errorf("username not set")
	}
	SetProviderAddress(p, "addr")
	if p.Spec.Address != "addr" {
		t.Errorf("address not set")
	}
	tout := metav1.Duration{Duration: time.Second}
	SetProviderTimeout(p, tout)
	if p.Spec.Timeout == nil || p.Spec.Timeout.Duration != time.Second {
		t.Errorf("timeout not set")
	}
	SetProviderProxy(p, "http://proxy")
	if p.Spec.Proxy != "http://proxy" {
		t.Errorf("proxy not set")
	}
	ref := &meta.LocalObjectReference{Name: "sec"}
	SetProviderSecretRef(p, ref)
	if p.Spec.SecretRef != ref {
		t.Errorf("secretRef not set")
	}
	cref := &meta.LocalObjectReference{Name: "ca"}
	SetProviderCertSecretRef(p, cref)
	if p.Spec.CertSecretRef != cref {
		t.Errorf("cert secret ref not set")
	}
	SetProviderSuspend(p, true)
	if !p.Spec.Suspend {
		t.Errorf("suspend not set")
	}
}

func TestCreateAlert(t *testing.T) {
	spec := notificationv1beta2.AlertSpec{
		ProviderRef:   meta.LocalObjectReference{Name: "p"},
		EventSources:  []v1.CrossNamespaceObjectReference{{Kind: "GitRepository", Name: "repo"}},
		EventSeverity: "info",
	}
	a := CreateAlert("a", "ns", spec)
	if a.Name != "a" || a.Namespace != "ns" {
		t.Fatalf("metadata mismatch")
	}
	if a.Kind != notificationv1beta2.AlertKind {
		t.Errorf("unexpected kind %s", a.Kind)
	}
	if a.APIVersion != notificationv1beta2.GroupVersion.String() {
		t.Errorf("unexpected apiversion %s", a.APIVersion)
	}
	if len(a.Spec.EventSources) != 1 {
		t.Errorf("events not set")
	}
}

func TestAlertHelpers(t *testing.T) {
	a := CreateAlert("a", "ns", notificationv1beta2.AlertSpec{})
	SetAlertProviderRef(a, meta.LocalObjectReference{Name: "p"})
	if a.Spec.ProviderRef.Name != "p" {
		t.Errorf("provider not set")
	}
	src := v1.CrossNamespaceObjectReference{Kind: "Bucket", Name: "b"}
	AddAlertEventSource(a, src)
	if len(a.Spec.EventSources) != 1 {
		t.Errorf("event source not added")
	}
	AddAlertInclusion(a, "foo")
	if len(a.Spec.InclusionList) != 1 {
		t.Errorf("inclusion not added")
	}
	AddAlertExclusion(a, "bar")
	if len(a.Spec.ExclusionList) != 1 {
		t.Errorf("exclusion not added")
	}
	AddAlertEventMetadata(a, "k", "v")
	if a.Spec.EventMetadata["k"] != "v" {
		t.Errorf("metadata not added")
	}
	SetAlertEventSeverity(a, "error")
	if a.Spec.EventSeverity != "error" {
		t.Errorf("severity not set")
	}
	SetAlertSummary(a, "sum")
	if a.Spec.Summary != "sum" {
		t.Errorf("summary not set")
	}
	SetAlertSuspend(a, true)
	if !a.Spec.Suspend {
		t.Errorf("suspend not set")
	}
}

func TestCreateReceiver(t *testing.T) {
	spec := notificationv1beta2.ReceiverSpec{
		Type:      notificationv1beta2.GenericReceiver,
		Resources: []v1.CrossNamespaceObjectReference{{Kind: "Kustomization", Name: "ks"}},
		SecretRef: meta.LocalObjectReference{Name: "token"},
	}
	r := CreateReceiver("r", "ns", spec)
	if r.Name != "r" || r.Namespace != "ns" {
		t.Fatalf("metadata mismatch")
	}
	if r.Kind != notificationv1beta2.ReceiverKind {
		t.Errorf("unexpected kind %s", r.Kind)
	}
	if len(r.Spec.Resources) != 1 {
		t.Errorf("resources not set")
	}
}

func TestReceiverHelpers(t *testing.T) {
	r := CreateReceiver("r", "ns", notificationv1beta2.ReceiverSpec{})
	SetReceiverType(r, notificationv1beta2.GitHubReceiver)
	if r.Spec.Type != notificationv1beta2.GitHubReceiver {
		t.Errorf("type not set")
	}
	dur := metav1.Duration{Duration: time.Hour}
	SetReceiverInterval(r, dur)
	if r.Spec.Interval == nil || r.Spec.Interval.Duration != time.Hour {
		t.Errorf("interval not set")
	}
	AddReceiverEvent(r, "push")
	if len(r.Spec.Events) != 1 {
		t.Errorf("event not added")
	}
	ref := v1.CrossNamespaceObjectReference{Kind: "HelmRelease", Name: "hr"}
	AddReceiverResource(r, ref)
	if len(r.Spec.Resources) != 1 {
		t.Errorf("resource not added")
	}
	SetReceiverSecretRef(r, meta.LocalObjectReference{Name: "sec"})
	if r.Spec.SecretRef.Name != "sec" {
		t.Errorf("secret ref not set")
	}
	SetReceiverSuspend(r, true)
	if !r.Spec.Suspend {
		t.Errorf("suspend not set")
	}
}
