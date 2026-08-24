package utils

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/projecteru2/core/log"
)

// CheckHTTP reports whether every backend answers with the expected status code.
func CheckHTTP(ctx context.Context, ID string, backends []string, code int, timeout time.Duration) bool {
	logger := log.WithFunc("CheckHTTP").WithField("ID", ID).WithField("backends", backends).WithField("code", code)
	for _, backend := range backends {
		logger.Debug(ctx, "Check health via http")
		if !checkOneURL(ctx, backend, code, timeout) {
			logger.Info(ctx, "Check health failed via http")
			return false
		}
	}
	return true
}

// CheckTCP reports whether every backend accepts a TCP connection.
func CheckTCP(ctx context.Context, ID string, backends []string, timeout time.Duration) bool {
	logger := log.WithFunc("CheckTCP").WithField("ID", ID).WithField("backends", backends)
	for _, backend := range backends {
		logger.Debug(ctx, "Check health via tcp")
		conn, err := net.DialTimeout("tcp", backend, timeout)
		if err != nil {
			logger.Debug(ctx, "Check health failed via tcp")
			return false
		}
		conn.Close()
	}
	return true
}

func get(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		select {
		case <-ctx.Done():
			err = ctx.Err()
		default:
		}
	}
	return resp, err
}

func checkOneURL(ctx context.Context, url string, expectedCode int, timeout time.Duration) bool {
	logger := log.WithFunc("checkOneURL").WithField("url", url)
	var resp *http.Response
	var err error
	WithTimeout(ctx, timeout, func(ctx context.Context) {
		resp, err = get(ctx, nil, url) //nolint
	})
	if err != nil {
		logger.Error(ctx, err, "Error when checking")
		return false
	}
	defer resp.Body.Close()
	if expectedCode == 0 {
		return resp.StatusCode < 500 && resp.StatusCode >= 200
	}
	if resp.StatusCode != expectedCode {
		logger.Warnf(ctx, "Error when checking, expect %d, got %d", expectedCode, resp.StatusCode)
	}
	return resp.StatusCode == expectedCode
}
