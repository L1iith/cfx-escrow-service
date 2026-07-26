package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/L1iith/cfx-escrow-service/internal/config"
)

func TestResourcePathValidation(t *testing.T) {
	current := New(config.Config{ResourceRoot: "server-files/resources"})
	valid, err := current.resourcePath("server-files/resources/[qbx]/qbx_houserobbery")
	if err != nil {
		t.Fatal(err)
	}
	if valid != filepath.Clean(filepath.FromSlash("server-files/resources/[qbx]/qbx_houserobbery")) {
		t.Fatal("unexpected normalized path")
	}
	if _, err := current.resourcePath("../secret"); err == nil {
		t.Fatal("expected traversal to fail")
	}
	if _, err := current.resourcePath("other/example"); err == nil {
		t.Fatal("expected out-of-root path to fail")
	}
}

func TestReadAssetID(t *testing.T) {
	directory := t.TempDir()
	numeric := filepath.Join(directory, "numeric")
	if err := os.WriteFile(numeric, []byte("1057477\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	id, err := readAssetID(numeric)
	if err != nil || id != 1057477 {
		t.Fatalf("unexpected numeric marker: %d %v", id, err)
	}
	jsonMarker := filepath.Join(directory, "json")
	if err := os.WriteFile(jsonMarker, []byte(`{"id":1057478}`), 0o600); err != nil {
		t.Fatal(err)
	}
	id, err = readAssetID(jsonMarker)
	if err != nil || id != 1057478 {
		t.Fatalf("unexpected JSON marker: %d %v", id, err)
	}
}
