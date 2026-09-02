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

func (r *RetryTask) Run() error {
	logger := log.WithFunc("utils.Run")
	logger.Debug(r.ctx, "start")
	defer r.Stop()

	interval := time.Second
	var err error
	for attempt := range r.maxAttempts {
		if attempt > 0 {
			logger.Debugf(r.ctx, "will retry after %v", interval)
			select {
			case <-r.ctx.Done():
				logger.Debug(r.ctx, "abort")
				return r.ctx.Err()
			case <-time.After(interval):
			}
			interval *= 2
		}
		if err = r.fn(); err == nil {
			return nil
		}
	}
	return err
}

func (r *RetryTask) Stop() {
	r.cancel()
}

// BackoffRetry runs f up to maxAttempts times, doubling the wait after each failure.
func BackoffRetry(ctx context.Context, maxAttempts int, f RetryFunc) error {
	return NewRetryTask(ctx, maxAttempts, f).Run()
}
