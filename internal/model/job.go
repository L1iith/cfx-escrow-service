package model

import "time"

type Operation string

const (
	OperationUpload          Operation = "upload"
	OperationMirror          Operation = "mirror"
	OperationUploadAndMirror Operation = "upload_and_mirror"
)

type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

type JobRequest struct {
	Repository string    `json:"repository"`
	Branch     string    `json:"branch"`
	Commit     string    `json:"commit"`
	Operation  Operation `json:"operation"`
	Resources  []string  `json:"resources"`
}

type ResourceResult struct {
	Path    string `json:"path"`
	AssetID int64  `json:"asset_id,omitempty"`
}

type Job struct {
	ID             string           `json:"id"`
	IdempotencyKey string           `json:"idempotency_key,omitempty"`
	Request        JobRequest       `json:"request"`
	Status         Status           `json:"status"`
	Error          string           `json:"error,omitempty"`
	Results        []ResourceResult `json:"results,omitempty"`
	Logs           []string         `json:"logs,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
	StartedAt      *time.Time       `json:"started_at,omitempty"`
	FinishedAt     *time.Time       `json:"finished_at,omitempty"`
}
