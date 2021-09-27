package utils

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
)

// RetryTask .
type RetryTask struct {
	ctx         context.Context
	cancel      context.CancelFunc
	Func        func() error
	MaxAttempts int
}

// NewRetryTask .
func NewRetryTask(ctx context.Context, maxAttempts int, f func() error) *RetryTask {
	// make sure to execute at least once
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	ctx, cancel := context.WithCancel(ctx)
	return &RetryTask{
		ctx:         ctx,
		cancel:      cancel,
		MaxAttempts: maxAttempts,
		Func:        f,
	}
}

// Run start running retry task
func (r *RetryTask) Run() error {
	log.Debug("[RetryTask] start")
	defer r.Stop()

	var err error
	interval := 1
	timer := time.NewTimer(0)
	defer timer.Stop()

	for i := 0; i < r.MaxAttempts; i++ {
		select {
		case <-r.ctx.Done():
			log.Debug("[RetryTask] abort")
			return r.ctx.Err()
		case <-timer.C:
			err = r.Func()
			if err == nil {
				return nil
			}
			log.Debugf("[RetryTask] will retry after %v seconds", interval)
			timer.Reset(time.Duration(interval) * time.Second)
			interval *= 2
		}
	}
	return err
}

// Stop stops running task
func (r *RetryTask) Stop() {
	log.Debug("[RetryTask] stop")
	r.cancel()
}

// BackoffRetry retries up to `maxAttempts` times, and the interval will grow exponentially
func BackoffRetry(ctx context.Context, maxAttempts int, f func() error) error {
	retryTask := NewRetryTask(ctx, maxAttempts, f)
	defer retryTask.Stop()
	return retryTask.Run()
}
