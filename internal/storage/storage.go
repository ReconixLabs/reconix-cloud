package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"reconix-cloud/internal/model"
	"sync"
	"time"
)

var ErrNoJob = fmt.Errorf("no queued job")

type Store interface {
	Create(context.Context, model.Scan) error
	Get(context.Context, string) (model.Scan, error)
	List(context.Context) ([]model.Scan, error)
	Claim(context.Context, string) error
	ClaimNext(context.Context, time.Duration) (model.Scan, error)
	Update(context.Context, model.Scan, []byte, *model.NormalizedResult) error
	Cancel(context.Context, string) error
}
type ResultStore interface {
	Result(string) (model.NormalizedResult, error)
}

type Memory struct {
	mu      sync.Mutex
	scans   map[string]model.Scan
	raw     map[string][]byte
	results map[string]model.NormalizedResult
}

func NewMemory() *Memory {
	return &Memory{scans: map[string]model.Scan{}, raw: map[string][]byte{}, results: map[string]model.NormalizedResult{}}
}
func (s *Memory) Create(_ context.Context, scan model.Scan) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scans[scan.ID] = scan
	return nil
}
func (s *Memory) Get(_ context.Context, id string) (model.Scan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	scan, ok := s.scans[id]
	if !ok {
		return scan, fmt.Errorf("scan not found")
	}
	return scan, nil
}
func (s *Memory) List(_ context.Context) ([]model.Scan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]model.Scan, 0, len(s.scans))
	for _, scan := range s.scans {
		out = append(out, scan)
	}
	return out, nil
}
func (s *Memory) Claim(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	scan, ok := s.scans[id]
	if !ok {
		return fmt.Errorf("scan not found")
	}
	if scan.Status != model.Queued {
		return fmt.Errorf("scan is not queued")
	}
	now := scan.UpdatedAt
	scan.Status = model.Starting
	scan.Stage = "reconix"
	scan.StartedAt = &now
	scan.UpdatedAt = now
	s.scans[id] = scan
	return nil
}
func (s *Memory) ClaimNext(_ context.Context, staleAfter time.Duration) (model.Scan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for id, scan := range s.scans {
		stale := (scan.Status == model.Starting || scan.Status == model.Running) && now.Sub(scan.UpdatedAt) >= staleAfter
		if scan.Status != model.Queued && !stale {
			continue
		}
		scan.Status = model.Starting
		scan.Stage = "reconix"
		scan.StartedAt = &now
		scan.UpdatedAt = now
		s.scans[id] = scan
		return scan, nil
	}
	return model.Scan{}, ErrNoJob
}
func (s *Memory) Update(_ context.Context, scan model.Scan, raw []byte, result *model.NormalizedResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scans[scan.ID] = scan
	s.raw[scan.ID] = raw
	if result != nil {
		s.results[scan.ID] = *result
	}
	return nil
}
func (s *Memory) Cancel(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	scan, ok := s.scans[id]
	if !ok {
		return fmt.Errorf("scan not found")
	}
	if scan.Status != model.Queued && scan.Status != model.Starting && scan.Status != model.Running {
		return fmt.Errorf("scan cannot be cancelled")
	}
	scan.Status = model.Cancelled
	s.scans[id] = scan
	return nil
}
func (s *Memory) Result(id string) (model.NormalizedResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, ok := s.results[id]
	if !ok {
		return result, fmt.Errorf("results not found")
	}
	return result, nil
}
func JSON(v any) []byte { data, _ := json.Marshal(v); return data }
