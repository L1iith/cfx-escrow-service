package store

import (
	"testing"
	"time"

	"github.com/L1iith/cfx-escrow-service/internal/model"
)

func TestStorePersistsAndDeduplicatesJobs(t *testing.T) {
	directory := t.TempDir()
	current, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	job := &model.Job{
		ID:             "job-1",
		IdempotencyKey: "key-1",
		Status:         model.StatusQueued,
		CreatedAt:      time.Now().UTC(),
	}
	created, fresh, err := current.Create(job)
	if err != nil {
		t.Fatal(err)
	}
	if !fresh || created.ID != job.ID {
		t.Fatal("expected a new job")
	}
	duplicate := *job
	duplicate.ID = "job-2"
	existing, fresh, err := current.Create(&duplicate)
	if err != nil {
		t.Fatal(err)
	}
	if fresh || existing.ID != job.ID {
		t.Fatal("expected the existing idempotent job")
	}
	reopened, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := reopened.Get(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.IdempotencyKey != job.IdempotencyKey {
		t.Fatal("persisted job did not match")
	}
}

func TestStoreCapsLogs(t *testing.T) {
	current, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	logs := make([]string, 1000)
	for index := range logs {
		logs[index] = "line"
	}
	job := &model.Job{ID: "job-1", Status: model.StatusQueued, CreatedAt: time.Now().UTC(), Logs: logs}
	if _, _, err := current.Create(job); err != nil {
		t.Fatal(err)
	}
	if err := current.AppendLog(job.ID, "last"); err != nil {
		t.Fatal(err)
	}
	loaded, err := current.Get(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Logs) != 1000 {
		t.Fatalf("expected 1000 logs, got %d", len(loaded.Logs))
	}
	if loaded.Logs[999] != "last" {
		t.Fatal("expected newest log to be retained")
	}
}
