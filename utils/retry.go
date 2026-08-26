package utils

import (
	"context"
	"time"

	"github.com/projecteru2/core/log"
)

// RetryFunc is one attempt of a retried operation.
type RetryFunc func() error

type RetryTask struct {
	ctx         context.Context
	cancel      context.CancelFunc
	fn          RetryFunc
	maxAttempts int
}

func NewRetryTask(ctx context.Context, maxAttempts int, f RetryFunc) *RetryTask {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	ctx, cancel := context.WithCancel(ctx)
	return &RetryTask{
		ctx:         ctx,
		cancel:      cancel,
		maxAttempts: maxAttempts,
		fn:          f,
	}
}

func (r *RetryTask) Run(ctx context.Context) error {
	logger := log.WithFunc("utils.Run")
	logger.Debug(ctx, "start")
	defer r.Stop()

	var err error
	interval := 1
	timer := time.NewTimer(0)
	defer timer.Stop()

	for range r.maxAttempts {
		select {
		case <-r.ctx.Done():
			logger.Debug(ctx, "abort")
			return r.ctx.Err()
		case <-timer.C:
			err = r.fn()
			if err == nil {
				return nil
			}
			logger.Debugf(ctx, "will retry after %v seconds", interval)
			timer.Reset(time.Duration(interval) * time.Second)
			interval *= 2
		}
	}
	return err
}

func (r *RetryTask) Stop() {
	r.cancel()
}

// BackoffRetry runs f up to maxAttempts times, doubling the wait after each failure.
func BackoffRetry(ctx context.Context, maxAttempts int, f RetryFunc) error {
	retryTask := NewRetryTask(ctx, maxAttempts, f)
	defer retryTask.Stop()
	return retryTask.Run(ctx)
}
