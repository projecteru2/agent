package utils

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCheck(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	addr := server.Listener.Addr().String()
	ctx, cancel := context.WithCancel(t.Context())

	assert.Equal(t, CheckHTTP(ctx, "", server.URL, 404, time.Second), true)
	assert.Equal(t, CheckHTTP(ctx, "", server.URL, 0, time.Second), true)
	assert.Equal(t, CheckHTTP(ctx, "", server.URL, 200, time.Second), false)
	assert.Equal(t, CheckHTTP(ctx, "", "http://127.0.0.1:1", 200, time.Second), false)
	assert.Equal(t, CheckHTTP(ctx, "", "", 200, time.Second), true)

	cancel()
	assert.Equal(t, CheckHTTP(ctx, "", server.URL, 404, time.Second), false)

	assert.Equal(t, CheckTCP(ctx, "", []string{addr}, time.Second), true)
	assert.Equal(t, CheckTCP(ctx, "", []string{"127.0.0.1:1"}, time.Second), false)
}
