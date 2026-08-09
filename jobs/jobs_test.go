package jobs

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewValidatesConfiguration(t *testing.T) {
	t.Parallel()
	puller := PullFunc(func(context.Context) (Job, error) { return nil, ErrNoJob })
	handler := HandlerFunc(func(context.Context, Job) error { return nil })

	runner, err := New(puller, handler, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if runner.workers != DefaultWorkers || runner.idleDelay != DefaultIdleDelay || runner.jobTimeout != DefaultJobTimeout {
		t.Fatalf("default config = workers %d, idle delay %s, job timeout %s", runner.workers, runner.idleDelay, runner.jobTimeout)
	}

	for _, config := range []Config{
		{Workers: -1},
		{Workers: MaxWorkers + 1},
		{IdleDelay: -time.Second},
		{JobTimeout: time.Nanosecond},
		{JobTimeout: MaxJobTimeout + time.Second},
	} {
		if _, err := New(puller, handler, config); err == nil {
			t.Fatalf("New(%+v) succeeded, want error", config)
		}
	}
	if _, err := New(nil, handler, Config{}); err == nil {
		t.Fatal("New(nil, handler) succeeded, want error")
	}
	if _, err := New(puller, nil, Config{}); err == nil {
		t.Fatal("New(puller, nil) succeeded, want error")
	}
	var nilPuller PullFunc
	if _, err := New(nilPuller, handler, Config{}); !errors.Is(err, ErrInvalidRunner) {
		t.Fatalf("New(typed nil puller) error = %v, want %v", err, ErrInvalidRunner)
	}
}

func TestRunnerAppliesJobTimeout(t *testing.T) {
	t.Parallel()
	runner, err := New(
		PullFunc(func(context.Context) (Job, error) { return "job", nil }),
		HandlerFunc(func(ctx context.Context, _ Job) error {
			<-ctx.Done()
			return ctx.Err()
		}),
		Config{JobTimeout: time.Millisecond},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Run(t.Context()); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v, want deadline exceeded", err)
	}
}

func TestRunnerPullsOnlyForAvailableWorkers(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var calls atomic.Int64
	started := make(chan struct{})
	runner, err := New(
		PullFunc(func(context.Context) (Job, error) {
			return calls.Add(1), nil
		}),
		HandlerFunc(func(ctx context.Context, _ Job) error {
			started <- struct{}{}
			<-ctx.Done()
			return ctx.Err()
		}),
		Config{Workers: 2},
	)
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- runner.Run(ctx) }()
	for range 2 {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("workers did not start")
		}
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("pull calls while workers are busy = %d, want 2", got)
	}

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run() error = %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not stop after cancellation")
	}
}

func TestRunnerWaitsAfterNoJob(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var (
		calls  atomic.Int64
		pulled = make(chan struct{})
		once   sync.Once
	)
	runner, err := New(
		PullFunc(func(context.Context) (Job, error) {
			calls.Add(1)
			once.Do(func() { close(pulled) })
			return nil, ErrNoJob
		}),
		HandlerFunc(func(context.Context, Job) error { return nil }),
		Config{IdleDelay: time.Hour},
	)
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- runner.Run(ctx) }()
	select {
	case <-pulled:
	case <-time.After(time.Second):
		t.Fatal("Pull was not called")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("pull calls before idle wait = %d, want 1", got)
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context cancellation", err)
	}
}

func TestRunnerReturnsHandlerError(t *testing.T) {
	t.Parallel()
	want := errors.New("handle failed")
	runner, err := New(
		PullFunc(func(context.Context) (Job, error) { return "job", nil }),
		HandlerFunc(func(context.Context, Job) error { return want }),
		Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Run(t.Context()); !errors.Is(err, want) {
		t.Fatalf("Run() error = %v, want %v", err, want)
	}
}

func TestRunRejectsNilContext(t *testing.T) {
	t.Parallel()
	runner, err := New(
		PullFunc(func(context.Context) (Job, error) { return nil, ErrNoJob }),
		HandlerFunc(func(context.Context, Job) error { return nil }),
		Config{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Run(nil); !errors.Is(err, ErrNilContext) {
		t.Fatalf("Run(nil) error = %v, want %v", err, ErrNilContext)
	}
}

func TestZeroRunnerReturnsError(t *testing.T) {
	t.Parallel()
	var runner Runner
	if err := runner.Run(t.Context()); !errors.Is(err, ErrInvalidRunner) {
		t.Fatalf("zero Runner.Run() error = %v, want %v", err, ErrInvalidRunner)
	}
}
