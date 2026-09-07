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

	for _, tc := range []struct {
		name string
		url  string
		code int
		want bool
	}{
		{"the expected code passes", server.URL, 404, true},
		{"any non-error code passes when none is expected", server.URL, 0, true},
		{"an unexpected code fails", server.URL, 200, false},
		{"a closed port fails", "http://127.0.0.1:1", 200, false},
		{"an empty url passes", "", 200, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, CheckHTTP(ctx, "", tc.url, tc.code, time.Second))
		})
	}

	cancel()
	assert.Equal(t, CheckHTTP(ctx, "", server.URL, 404, time.Second), false)

	assert.Equal(t, CheckTCP(ctx, "", []string{addr}, time.Second), true)
	assert.Equal(t, CheckTCP(ctx, "", []string{"127.0.0.1:1"}, time.Second), false)
}
