package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/L1iith/cfx-escrow-service/internal/model"
)

var ErrNotFound = errors.New("job not found")

type Store struct {
	mu   sync.RWMutex
	path string
	jobs map[string]*model.Job
}

func Open(directory string) (*Store, error) {
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return nil, err
	}
	s := &Store{
		path: filepath.Join(directory, "jobs.json"),
		jobs: map[string]*model.Job{},
	}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(data, &s.jobs); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Create(job *model.Job) (*model.Job, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job.IdempotencyKey != "" {
		for _, existing := range s.jobs {
			if existing.IdempotencyKey == job.IdempotencyKey {
				copy := clone(existing)
				return copy, false, nil
			}
		}
	}
	s.jobs[job.ID] = clone(job)
	if err := s.saveLocked(); err != nil {
		delete(s.jobs, job.ID)
		return nil, false, err
	}
	return clone(job), true, nil
}

func (s *Store) Get(id string) (*model.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	if !ok {
		return nil, ErrNotFound
	}
	return clone(job), nil
}

func (s *Store) List() []*model.Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := make([]*model.Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, clone(job))
	}
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].CreatedAt.After(jobs[j].CreatedAt)
	})
	return jobs
}

func (s *Store) Update(id string, apply func(*model.Job)) (*model.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return nil, ErrNotFound
	}
	before := clone(job)
	apply(job)
	if err := s.saveLocked(); err != nil {
		s.jobs[id] = before
		return nil, err
	}
	return clone(job), nil
}

func (s *Store) AppendLog(id, line string) error {
	_, err := s.Update(id, func(job *model.Job) {
		job.Logs = append(job.Logs, line)
		if len(job.Logs) > 1000 {
			job.Logs = append([]string(nil), job.Logs[len(job.Logs)-1000:]...)
		}
	})
	return err
}

func (s *Store) saveLocked() error {
	data, err := json.MarshalIndent(s.jobs, "", "  ")
	if err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(s.path), "jobs-*.json")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	ok := false
	defer func() {
		temp.Close()
		if !ok {
			os.Remove(tempPath)
		}
	}()
	if _, err := temp.Write(data); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, s.path); err != nil {
		if removeErr := os.Remove(s.path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return err
		}
		if err := os.Rename(tempPath, s.path); err != nil {
			return err
		}
	}
	ok = true
	return nil
}

func clone(job *model.Job) *model.Job {
	copy := *job
	copy.Request.Resources = append([]string(nil), job.Request.Resources...)
	copy.Results = append([]model.ResourceResult(nil), job.Results...)
	copy.Logs = append([]string(nil), job.Logs...)
	return &copy
}
