package worker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/L1iith/cfx-escrow-service/internal/model"
	"github.com/L1iith/cfx-escrow-service/internal/store"
)

type Processor interface {
	Process(context.Context, *model.Job, func(string)) ([]model.ResourceResult, error)
}

type Manager struct {
	store     *store.Store
	processor Processor
	timeout   time.Duration
	queue     chan string
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func NewManager(store *store.Store, processor Processor, timeout time.Duration) *Manager {
	return &Manager{
		store:     store,
		processor: processor,
		timeout:   timeout,
		queue:     make(chan string, 256),
	}
}

func (m *Manager) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	m.cancel = cancel
	m.wg.Add(1)
	go m.loop(ctx)
	for _, job := range m.store.List() {
		if job.Status == model.StatusQueued || job.Status == model.StatusRunning {
			m.store.Update(job.ID, func(current *model.Job) {
				current.Status = model.StatusQueued
				current.StartedAt = nil
				current.FinishedAt = nil
				current.Error = ""
			})
			m.queue <- job.ID
		}
	}
}

func (m *Manager) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	m.wg.Wait()
}

func (m *Manager) Submit(request model.JobRequest, idempotencyKey string) (*model.Job, error) {
	id, err := newID()
	if err != nil {
		return nil, err
	}
	job := &model.Job{
		ID:             id,
		IdempotencyKey: idempotencyKey,
		Request:        request,
		Status:         model.StatusQueued,
		CreatedAt:      time.Now().UTC(),
	}
	created, fresh, err := m.store.Create(job)
	if err != nil {
		return nil, err
	}
	if fresh {
		select {
		case m.queue <- created.ID:
		default:
			m.store.Update(created.ID, func(current *model.Job) {
				current.Status = model.StatusFailed
				current.Error = "job queue is full"
				now := time.Now().UTC()
				current.FinishedAt = &now
			})
			return nil, errors.New("job queue is full")
		}
	}
	return created, nil
}

func (m *Manager) loop(ctx context.Context) {
	defer m.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case id := <-m.queue:
			m.run(ctx, id)
		}
	}
}

func (m *Manager) run(parent context.Context, id string) {
	job, err := m.store.Get(id)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	job, err = m.store.Update(id, func(current *model.Job) {
		current.Status = model.StatusRunning
		current.StartedAt = &now
		current.FinishedAt = nil
		current.Error = ""
	})
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(parent, m.timeout)
	defer cancel()
	results, processErr := m.processor.Process(ctx, job, func(line string) {
		m.store.AppendLog(id, line)
	})
	finished := time.Now().UTC()
	m.store.Update(id, func(current *model.Job) {
		current.FinishedAt = &finished
		current.Results = results
		if processErr != nil {
			current.Status = model.StatusFailed
			current.Error = processErr.Error()
			return
		}
		current.Status = model.StatusSucceeded
	})
}

func newID() (string, error) {
	data := make([]byte, 16)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}
