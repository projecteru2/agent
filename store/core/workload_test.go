package core

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

func TestWorkloadLookupResult(t *testing.T) {
	transport := errors.New("dial core: connection refused")
	legacy := grpcstatus.Error(1051, "workload not exists")
	tests := []struct {
		name    string
		err     error
		exists  bool
		wantErr error
	}{
		{"found", nil, true, nil},
		{"not found", grpcstatus.Error(codes.NotFound, "workload not exists"), false, nil},
		{"per-rpc code", legacy, false, legacy},
		{"transport", transport, false, transport},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, err := workloadLookupResult(tt.err)
			assert.Equal(t, tt.exists, exists)
			assert.Equal(t, tt.wantErr, err)
		})
	}
}
