package volsync

import "testing"

func TestCreateReplicationSource(t *testing.T) {
	rs := CreateReplicationSource("backup", "data")
	if rs == nil {
		t.Fatal("expected non-nil ReplicationSource")
	}
	if rs.Kind != "ReplicationSource" {
		t.Errorf("Kind = %q, want ReplicationSource", rs.Kind)
	}
	if rs.APIVersion != "volsync.backube/v1alpha1" {
		t.Errorf("APIVersion = %q, want volsync.backube/v1alpha1", rs.APIVersion)
	}
	if rs.Name != "backup" {
		t.Errorf("Name = %q, want backup", rs.Name)
	}
	if rs.Namespace != "data" {
		t.Errorf("Namespace = %q, want data", rs.Namespace)
	}
}

func TestCreateReplicationDestination(t *testing.T) {
	rd := CreateReplicationDestination("restore", "dr")
	if rd == nil {
		t.Fatal("expected non-nil ReplicationDestination")
	}
	if rd.Kind != "ReplicationDestination" {
		t.Errorf("Kind = %q, want ReplicationDestination", rd.Kind)
	}
	if rd.APIVersion != "volsync.backube/v1alpha1" {
		t.Errorf("APIVersion = %q, want volsync.backube/v1alpha1", rd.APIVersion)
	}
	if rd.Name != "restore" {
		t.Errorf("Name = %q, want restore", rd.Name)
	}
	if rd.Namespace != "dr" {
		t.Errorf("Namespace = %q, want dr", rd.Namespace)
	}
}
