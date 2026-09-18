package jobs

import (
	"context"
	"errors"
	"reconix-cloud/internal/model"
	"reconix-cloud/internal/policy"
	"reconix-cloud/internal/storage"
	"sync"
	"testing"
	"time"
)

type fakeRunner struct {
	err   error
	block bool
}

func (r fakeRunner) Run(ctx context.Context, _ model.Scan, _ string) ([]byte, error) {
	if r.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if r.err != nil {
		return nil, r.err
	}
	return []byte(`{"schema_version":"1.0","results":{"dns":[{"type":"dns","evidence":{"ip":"127.0.0.1"}}]}}`), nil
}

func testWorker(store storage.Store, runner fakeRunner, duration time.Duration) Worker {
	p, _ := policy.New("allowlist", nil, []string{"127.0.0.0/8"})
	return Worker{Store: store, Policy: p, Runner: runner, MaxConcurrent: 1, MaxDuration: duration, PollInterval: time.Millisecond, StaleAfter: time.Hour}
}

func queuedScan(t *testing.T, store *storage.Memory) model.Scan {
	t.Helper()
	now := time.Now().UTC()
	scan := model.Scan{ID: "scan_test", Target: "http://127.0.0.1", Profile: "safe", Status: model.Queued, CreatedAt: now, UpdatedAt: now}
	if err := store.Create(context.Background(), scan); err != nil {
		t.Fatal(err)
	}
	return scan
}

func waitFor(t *testing.T, store *storage.Memory, id string, status model.Status) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		scan, _ := store.Get(context.Background(), id)
		if scan.Status == status {
			return
		}
		time.Sleep(time.Millisecond * 5)
	}
	t.Fatalf("scan did not reach %s", status)
}

func TestJobCreation(t *testing.T) {
	store := storage.NewMemory()
	p, _ := policy.New("allowlist", nil, []string{"127.0.0.0/8"})
	manager := NewAPI(store, p)
	scan, err := manager.Create(context.Background(), "127.0.0.1", "standard")
	if err != nil || scan.Status != model.Queued {
		t.Fatalf("unexpected scan: %#v %v", scan, err)
	}
}

func TestTwoWorkersCannotClaimSameJob(t *testing.T) {
	store := storage.NewMemory()
	queuedScan(t, store)
	results := make(chan error, 2)
	var group sync.WaitGroup
	for i := 0; i < 2; i++ {
		group.Add(1)
		go func() { defer group.Done(); _, err := store.ClaimNext(context.Background(), time.Hour); results <- err }()
	}
	group.Wait()
	claimed := 0
	for i := 0; i < 2; i++ {
		if <-results == nil {
			claimed++
		}
	}
	if claimed != 1 {
		t.Fatalf("claimed %d jobs", claimed)
	}
}

func TestSuccessfulExecution(t *testing.T) {
	store := storage.NewMemory()
	scan := queuedScan(t, store)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go testWorker(store, fakeRunner{}, time.Second).Run(ctx)
	waitFor(t, store, scan.ID, model.Completed)
	result, err := store.Result(scan.ID)
	if err != nil || len(result.Findings) != 1 {
		t.Fatalf("unexpected result: %#v %v", result, err)
	}
}

func TestFailedExecution(t *testing.T) {
	store := storage.NewMemory()
	scan := queuedScan(t, store)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go testWorker(store, fakeRunner{err: errors.New("boom")}, time.Second).Run(ctx)
	waitFor(t, store, scan.ID, model.Failed)
}

func TestCancellation(t *testing.T) {
	store := storage.NewMemory()
	scan := queuedScan(t, store)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	worker := testWorker(store, fakeRunner{block: true}, time.Second)
	go worker.Run(ctx)
	waitFor(t, store, scan.ID, model.Running)
	if err := store.Cancel(context.Background(), scan.ID); err != nil {
		t.Fatal(err)
	}
	waitFor(t, store, scan.ID, model.Cancelled)
}

func TestTimeout(t *testing.T) {
	store := storage.NewMemory()
	scan := queuedScan(t, store)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go testWorker(store, fakeRunner{block: true}, time.Millisecond*20).Run(ctx)
	waitFor(t, store, scan.ID, model.Failed)
}

func TestStaleJobRecovery(t *testing.T) {
	store := storage.NewMemory()
	old := time.Now().UTC().Add(-time.Hour)
	scan := model.Scan{ID: "stale", Target: "http://127.0.0.1", Profile: "safe", Status: model.Running, CreatedAt: old, UpdatedAt: old}
	if err := store.Create(context.Background(), scan); err != nil {
		t.Fatal(err)
	}
	claimed, err := store.ClaimNext(context.Background(), time.Minute)
	if err != nil || claimed.ID != scan.ID || claimed.Status != model.Starting {
		t.Fatalf("stale job was not reclaimed: %#v %v", claimed, err)
	}
}

func TestWorkerShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	worker := testWorker(storage.NewMemory(), fakeRunner{}, time.Second)
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not shut down")
	}
}
