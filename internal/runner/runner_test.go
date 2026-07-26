package runner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/L1iith/cfx-escrow-service/internal/config"
	"github.com/L1iith/cfx-escrow-service/internal/model"
)

func TestResourcePathValidation(t *testing.T) {
	current := New(config.Config{})
	valid, err := current.resourcePath("server-files/resources", "server-files/resources/[qbx]/qbx_houserobbery")
	if err != nil {
		t.Fatal(err)
	}
	if valid != filepath.Clean(filepath.FromSlash("server-files/resources/[qbx]/qbx_houserobbery")) {
		t.Fatal("unexpected normalized path")
	}
	if _, err := current.resourcePath("server-files/resources", "../secret"); err == nil {
		t.Fatal("expected traversal to fail")
	}
	if _, err := current.resourcePath("server-files/resources", "other/example"); err == nil {
		t.Fatal("expected out-of-root path to fail")
	}
	if _, err := current.resourcePath(".", "[Maps]/example"); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryValidation(t *testing.T) {
	current := New(config.Config{
		Repositories: map[string]config.Repository{
			"owner/assets": {
				URL:          "https://github.com/owner/assets.git",
				Branch:       "main",
				ResourceRoot: ".",
			},
		},
	})
	repository, exists := current.cfg.RepositoryConfig("owner/assets")
	if !exists {
		t.Fatal("expected repository")
	}
	if err := current.validate(model.JobRequest{
		Repository: "owner/assets",
		Branch:     "main",
		Operation:  model.OperationUpload,
		Resources:  []string{"[Maps]/example"},
	}, repository); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryCacheName(t *testing.T) {
	current := repositoryCacheName("owner/assets")
	if current != repositoryCacheName("owner/assets") {
		t.Fatal("expected stable cache name")
	}
	if filepath.Base(current) != current || filepath.Ext(current) != ".git" {
		t.Fatal("unexpected cache name")
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
