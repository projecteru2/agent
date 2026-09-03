package utils

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/projecteru2/core/log"
)

// CheckHTTP reports whether url answers with the expected status code; an empty url passes.
func CheckHTTP(ctx context.Context, ID, url string, code int, timeout time.Duration) bool {
	if url == "" {
		return true
	}
	logger := log.WithFunc("utils.CheckHTTP").WithField("ID", ID).WithField("url", url).WithField("code", code)
	logger.Debug(ctx, "checking health via http")

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		logger.Error(ctx, err, "failed to build request")
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Debugf(ctx, "http health check failed: %v", err)
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	if code == 0 {
		if resp.StatusCode >= 500 {
			logger.Debugf(ctx, "http health check failed with %d", resp.StatusCode)
		}
		return resp.StatusCode < 500 && resp.StatusCode >= 200
	}
	if resp.StatusCode != code {
		logger.Warnf(ctx, "unexpected status, want %d, got %d", code, resp.StatusCode)
	}
	return resp.StatusCode == code
}

// CheckTCP reports whether every backend accepts a TCP connection.
func CheckTCP(ctx context.Context, ID string, backends []string, timeout time.Duration) bool {
	logger := log.WithFunc("utils.CheckTCP").WithField("ID", ID).WithField("backends", backends)
	for _, backend := range backends {
		logger.Debug(ctx, "checking health via tcp")
		conn, err := net.DialTimeout("tcp", backend, timeout)
		if err != nil {
			logger.Debugf(ctx, "tcp health check failed: %v", err)
			return false
		}
		_ = conn.Close()
	}
	return true
}
