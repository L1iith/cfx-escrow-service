package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/L1iith/cfx-escrow-service/internal/model"
	"github.com/L1iith/cfx-escrow-service/internal/store"
)

type Submitter interface {
	Submit(model.JobRequest, string) (*model.Job, error)
}

type Server struct {
	store        *store.Store
	submitter    Submitter
	maxBodyBytes int64
}

func New(store *store.Store, submitter Submitter, maxBodyBytes int64) *Server {
	return &Server{
		store:        store,
		submitter:    submitter,
		maxBodyBytes: maxBodyBytes,
	}
}

func (s *Server) Handler(auth func(http.Handler) http.Handler) http.Handler {
	public := http.NewServeMux()
	private := http.NewServeMux()
	public.HandleFunc("GET /healthz", s.health)
	private.HandleFunc("POST /v1/jobs", s.createJob)
	private.HandleFunc("GET /v1/jobs", s.listJobs)
	private.HandleFunc("GET /v1/jobs/{id}", s.getJob)
	public.Handle("/v1/", auth(private))
	return public
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) createJob(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.maxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request model.JobRequest
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if err := ensureEOF(decoder); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	request.Repository = strings.TrimSpace(request.Repository)
	request.Branch = strings.TrimSpace(request.Branch)
	request.Commit = strings.TrimSpace(request.Commit)
	for index := range request.Resources {
		request.Resources[index] = strings.TrimSpace(request.Resources[index])
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if len(key) > 200 {
		writeError(w, http.StatusBadRequest, "idempotency key is too long")
		return
	}
	job, err := s.submitter.Submit(request, key)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) getJob(w http.ResponseWriter, r *http.Request) {
	job, err := s.store.Get(r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load job")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) listJobs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.store.List())
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return errors.New("extra data")
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(value)
}
