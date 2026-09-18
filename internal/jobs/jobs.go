package jobs

import (
	"context"
	"errors"
	"fmt"
	"reconix-cloud/internal/model"
	"reconix-cloud/internal/normalize"
	"reconix-cloud/internal/policy"
	"reconix-cloud/internal/runner"
	"reconix-cloud/internal/storage"
	"sync"
	"time"
)

// Manager owns API-side validation and job creation only. It never executes Reconix.
type Manager struct {
	Store  storage.Store
	Policy policy.Policy
}

func NewAPI(store storage.Store, p policy.Policy) *Manager {
	return &Manager{Store: store, Policy: p}
}

func (m *Manager) Create(ctx context.Context, target, profile string) (model.Scan, error) {
	target, err := m.Policy.Validate(ctx, target)
	if err != nil {
		return model.Scan{}, err
	}
	now := time.Now().UTC()
	scan := model.Scan{ID: fmt.Sprintf("scan_%d", now.UnixNano()), Target: target, Profile: profile, Status: model.Queued, CreatedAt: now, UpdatedAt: now}
	if err := m.Store.Create(ctx, scan); err != nil {
		return model.Scan{}, err
	}
	return scan, nil
}

func (m *Manager) Cancel(ctx context.Context, id string) error { return m.Store.Cancel(ctx, id) }

// Worker polls the persistent queue and limits Reconix processes to MaxConcurrent.
type Worker struct {
	Store         storage.Store
	Policy        policy.Policy
	Runner        runner.ReconRunner
	MaxConcurrent int
	MaxDuration   time.Duration
	PollInterval  time.Duration
	StaleAfter    time.Duration
}

func (w Worker) Run(ctx context.Context) error {
	if w.MaxConcurrent < 1 {
		return fmt.Errorf("max concurrent workers must be positive")
	}
	if w.PollInterval <= 0 {
		w.PollInterval = time.Second
	}
	if w.StaleAfter <= 0 {
		w.StaleAfter = w.MaxDuration * 2
	}
	if w.StaleAfter <= w.MaxDuration {
		w.StaleAfter = w.MaxDuration * 2
	}
	var group sync.WaitGroup
	for i := 0; i < w.MaxConcurrent; i++ {
		group.Add(1)
		go func() { defer group.Done(); w.loop(ctx) }()
	}
	group.Wait()
	return nil
}

func (w Worker) loop(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		scan, err := w.Store.ClaimNext(ctx, w.StaleAfter)
		if err != nil {
			if errors.Is(err, storage.ErrNoJob) {
				w.wait(ctx)
				continue
			}
			w.wait(ctx)
			continue
		}
		w.process(ctx, scan)
	}
}

func (w Worker) wait(ctx context.Context) {
	timer := time.NewTimer(w.PollInterval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func (w Worker) process(parent context.Context, scan model.Scan) {
	ctx, cancel := context.WithTimeout(parent, w.MaxDuration)
	defer cancel()
	go w.watchCancellation(ctx, scan.ID, cancel)

	scan.Status = model.Running
	scan.Stage = "reconix"
	scan.UpdatedAt = time.Now().UTC()
	if err := w.Store.Update(ctx, scan, nil, nil); err != nil {
		return
	}
	target, err := w.Policy.Validate(ctx, scan.Target)
	if err != nil {
		w.fail(scan)
		return
	}
	raw, err := w.Runner.Run(ctx, scan, target)
	if err != nil {
		if ctx.Err() == context.Canceled {
			scan.Status = model.Cancelled
		} else {
			scan.Status = model.Failed
			scan.Error = "scan execution failed"
		}
		now := time.Now().UTC()
		scan.CompletedAt = &now
		scan.UpdatedAt = now
		_ = w.Store.Update(context.Background(), scan, nil, nil)
		return
	}
	normalized, err := normalize.Result(scan, raw)
	if err != nil {
		w.fail(scan)
		return
	}
	now := time.Now().UTC()
	scan.Status = model.Completed
	scan.CompletedAt = &now
	scan.UpdatedAt = now
	_ = w.Store.Update(context.Background(), scan, raw, &normalized)
}

func (w Worker) watchCancellation(ctx context.Context, id string, cancel context.CancelFunc) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			scan, err := w.Store.Get(ctx, id)
			if err == nil && scan.Status == model.Cancelled {
				cancel()
				return
			}
		}
	}
}

func (w Worker) fail(scan model.Scan) {
	now := time.Now().UTC()
	scan.Status = model.Failed
	scan.Error = "scan execution failed"
	scan.CompletedAt = &now
	scan.UpdatedAt = now
	_ = w.Store.Update(context.Background(), scan, nil, nil)
}
