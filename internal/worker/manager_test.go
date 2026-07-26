package worker

import (
	"context"
	"testing"
	"time"

	"github.com/L1iith/cfx-escrow-service/internal/model"
	"github.com/L1iith/cfx-escrow-service/internal/store"
)

type processorStub struct{}

func (processorStub) Process(_ context.Context, job *model.Job, logf func(string)) ([]model.ResourceResult, error) {
	logf("processed")
	return []model.ResourceResult{{Path: job.Request.Resources[0], AssetID: 42}}, nil
}

func TestManagerProcessesSubmittedJob(t *testing.T) {
	jobStore, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	manager := NewManager(jobStore, processorStub{}, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	manager.Start(ctx)
	defer manager.Stop()

	job, err := manager.Submit(model.JobRequest{
		Repository: "owner/repo",
		Branch:     "main",
		Operation:  model.OperationUpload,
		Resources:  []string{"resources/example"},
	}, "key")
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		current, err := jobStore.Get(job.ID)
		if err != nil {
			t.Fatal(err)
		}
		if current.Status == model.StatusSucceeded {
			if len(current.Results) != 1 || current.Results[0].AssetID != 42 {
				t.Fatal("unexpected result")
			}
			if len(current.Logs) != 1 || current.Logs[0] != "processed" {
				t.Fatal("unexpected logs")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not finish")
}
