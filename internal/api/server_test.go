package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/L1iith/cfx-escrow-service/internal/model"
	"github.com/L1iith/cfx-escrow-service/internal/store"
)

type submitterStub struct {
	store *store.Store
}

func (s submitterStub) Submit(request model.JobRequest, key string) (*model.Job, error) {
	job := &model.Job{
		ID:             "job-1",
		IdempotencyKey: key,
		Request:        request,
		Status:         model.StatusQueued,
		CreatedAt:      time.Now().UTC(),
	}
	created, _, err := s.store.Create(job)
	return created, err
}

func TestCreateAndGetJob(t *testing.T) {
	jobStore, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	server := New(jobStore, submitterStub{store: jobStore}, 1024)
	handler := server.Handler(func(next http.Handler) http.Handler { return next })
	payload, err := json.Marshal(model.JobRequest{
		Repository: "owner/repo",
		Branch:     "main",
		Operation:  model.OperationUpload,
		Resources:  []string{"server-files/resources/example"},
	})
	if err != nil {
		t.Fatal(err)
	}
	createRequest := httptest.NewRequest(http.MethodPost, "/v1/jobs", bytes.NewReader(payload))
	createRequest.Header.Set("Idempotency-Key", "key-1")
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusAccepted {
		t.Fatalf("expected %d, got %d", http.StatusAccepted, createResponse.Code)
	}

	getRequest := httptest.NewRequest(http.MethodGet, "/v1/jobs/job-1", nil)
	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, getResponse.Code)
	}
	var job model.Job
	if err := json.Unmarshal(getResponse.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	if job.ID != "job-1" || job.IdempotencyKey != "key-1" {
		t.Fatal("unexpected job response")
	}
}
