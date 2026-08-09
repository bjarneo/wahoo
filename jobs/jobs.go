// Package jobs provides a bounded pull-based job runner.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"
)

const (
	// DefaultWorkers is used when Config.Workers is zero.
	DefaultWorkers = 1
	// MaxWorkers prevents a configuration error from creating unbounded workers.
	MaxWorkers = 1024
	// DefaultIdleDelay is used when Config.IdleDelay is zero.
	DefaultIdleDelay = time.Second
	// DefaultJobTimeout is used when Config.JobTimeout is zero.
	DefaultJobTimeout = 5 * time.Minute
	// MaxJobTimeout prevents a configuration error from retaining workers forever.
	MaxJobTimeout = 24 * time.Hour
)

var (
	// ErrNoJob tells a Runner to wait for its idle delay before pulling again.
	ErrNoJob = errors.New("no job available")
	// ErrNilContext is returned when Run receives a nil context.
	ErrNilContext = errors.New("nil context")
	// ErrInvalidRunner reports an uninitialized runner or dependency.
	ErrInvalidRunner = errors.New("invalid job runner")
)

// Job is an opaque unit of work supplied by a Puller.
type Job any

// Puller obtains one job when a worker is ready to process it. It must return
// ErrNoJob when no work is currently available and honor context cancellation.
type Puller interface {
	Pull(context.Context) (Job, error)
}

// PullFunc adapts a function to Puller.
type PullFunc func(context.Context) (Job, error)

// Pull calls f.
func (f PullFunc) Pull(ctx context.Context) (Job, error) {
	return f(ctx)
}

// Handler processes one job. A Runner never calls Handle concurrently more
// often than its configured worker count.
type Handler interface {
	Handle(context.Context, Job) error
}

// HandlerFunc adapts a function to Handler.
type HandlerFunc func(context.Context, Job) error

// Handle calls f.
func (f HandlerFunc) Handle(ctx context.Context, job Job) error {
	return f(ctx, job)
}

// Config bounds a Runner's concurrent pull and handle operations.
type Config struct {
	Workers    int
	IdleDelay  time.Duration
	JobTimeout time.Duration
}

// Runner pulls work only when one of its workers is free. It intentionally has
// no in-memory queue or prefetch buffer.
type Runner struct {
	puller     Puller
	handler    Handler
	workers    int
	idleDelay  time.Duration
	jobTimeout time.Duration
}

// New creates a Runner with validated bounded configuration.
func New(puller Puller, handler Handler, config Config) (*Runner, error) {
	if nilInterface(puller) {
		return nil, ErrInvalidRunner
	}
	if nilInterface(handler) {
		return nil, ErrInvalidRunner
	}
	if config.Workers == 0 {
		config.Workers = DefaultWorkers
	}
	if config.Workers < 1 || config.Workers > MaxWorkers {
		return nil, fmt.Errorf("workers must be between 1 and %d", MaxWorkers)
	}
	if config.IdleDelay == 0 {
		config.IdleDelay = DefaultIdleDelay
	}
	if config.IdleDelay < 0 {
		return nil, errors.New("idle delay must not be negative")
	}
	if config.JobTimeout == 0 {
		config.JobTimeout = DefaultJobTimeout
	}
	if config.JobTimeout < time.Millisecond || config.JobTimeout > MaxJobTimeout {
		return nil, fmt.Errorf("job timeout must be from %s to %s", time.Millisecond, MaxJobTimeout)
	}
	return &Runner{
		puller:     puller,
		handler:    handler,
		workers:    config.Workers,
		idleDelay:  config.IdleDelay,
		jobTimeout: config.JobTimeout,
	}, nil
}

// Run pulls and handles work until ctx is canceled or a puller or handler
// returns an error other than ErrNoJob. It returns the first runner error, or
// ctx.Err when the caller stops the runner.
func (r *Runner) Run(ctx context.Context) error {
	if ctx == nil {
		return ErrNilContext
	}
	if r == nil || nilInterface(r.puller) || nilInterface(r.handler) || r.workers < 1 || r.workers > MaxWorkers || r.idleDelay < 0 || r.jobTimeout < time.Millisecond || r.jobTimeout > MaxJobTimeout {
		return ErrInvalidRunner
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		workers sync.WaitGroup
		failed  sync.Once
		runErr  error
	)
	fail := func(err error) {
		failed.Do(func() {
			runErr = err
			cancel()
		})
	}

	workers.Add(r.workers)
	for range r.workers {
		go func() {
			defer workers.Done()
			for {
				if runCtx.Err() != nil {
					return
				}
				job, err := r.puller.Pull(runCtx)
				if err != nil {
					if errors.Is(err, ErrNoJob) {
						if !wait(runCtx, r.idleDelay) {
							return
						}
						continue
					}
					if runCtx.Err() != nil {
						return
					}
					fail(fmt.Errorf("pull job: %w", err))
					return
				}
				jobCtx, cancelJob := context.WithTimeout(runCtx, r.jobTimeout)
				err = r.handler.Handle(jobCtx, job)
				cancelJob()
				if err != nil {
					if runCtx.Err() != nil {
						return
					}
					fail(fmt.Errorf("handle job: %w", err))
					return
				}
			}
		}()
	}
	workers.Wait()
	if runErr != nil {
		return runErr
	}
	return ctx.Err()
}

func wait(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
