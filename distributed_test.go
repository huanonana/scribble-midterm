package scribble

import (
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestReplicationWriteAndDelete(t *testing.T) {
	secondary, err := New(filepath.Join(t.TempDir(), "secondary"), &Options{NodeID: "secondary"})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(secondary.HTTPHandler())
	defer server.Close()

	primary, err := New(filepath.Join(t.TempDir(), "primary"), &Options{
		NodeID: "primary",
		Peers:  []string{server.URL},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := primary.Write("fish", "red", Fish{Type: "red"}); err != nil {
		t.Fatal(err)
	}
	var replicated Fish
	if err := secondary.Read("fish", "red", &replicated); err != nil {
		t.Fatal(err)
	}
	if replicated.Type != "red" {
		t.Fatalf("expected red, got %q", replicated.Type)
	}

	if err := primary.Delete("fish", "red"); err != nil {
		t.Fatal(err)
	}
	if err := secondary.Read("fish", "red", &replicated); err == nil {
		t.Fatal("expected replicated delete")
	}
}

func TestSyncFromSnapshot(t *testing.T) {
	source, err := New(filepath.Join(t.TempDir(), "source"), &Options{NodeID: "source"})
	if err != nil {
		t.Fatal(err)
	}
	if err := source.Write("fish", "blue", Fish{Type: "blue"}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(source.HTTPHandler())
	defer server.Close()

	target, err := New(filepath.Join(t.TempDir(), "target"), &Options{NodeID: "target"})
	if err != nil {
		t.Fatal(err)
	}
	if err := target.SyncFrom(server.URL); err != nil {
		t.Fatal(err)
	}

	var fish Fish
	if err := target.Read("fish", "blue", &fish); err != nil {
		t.Fatal(err)
	}
	if fish.Type != "blue" {
		t.Fatalf("expected blue, got %q", fish.Type)
	}
}
