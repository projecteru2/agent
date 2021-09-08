package utils

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCheck(t *testing.T) {
	go http.ListenAndServe(":12306", http.NotFoundHandler())
	time.Sleep(time.Second)
	ctx := context.Background()
	assert.Equal(t, CheckHTTP(ctx, "", []string{"http://127.0.0.1:12306"}, 404, time.Second), true)
	assert.Equal(t, CheckHTTP(ctx, "", []string{"http://127.0.0.1:12306"}, 0, time.Second), true)
	assert.Equal(t, CheckHTTP(ctx, "", []string{"http://127.0.0.1:12306"}, 200, time.Second), false)
	assert.Equal(t, CheckHTTP(ctx, "", []string{"http://127.0.0.1:12307"}, 200, time.Second), false)

	assert.Equal(t, CheckTCP("", []string{"127.0.0.1:12306"}, time.Second), true)
	assert.Equal(t, CheckTCP("", []string{"127.0.0.1:12307"}, time.Second), false)
}
