package core

import (
	"context"
	"fmt"
	"maps"
	"math/rand/v2"
	"slices"
	"strings"
	"time"

	pb "github.com/projecteru2/core/rpc/gen"
	coretypes "github.com/projecteru2/core/types"

	"github.com/projecteru2/agent/types"
)

func (s *Store) ListRunningWorkloadIDs(ctx context.Context) ([]string, error) {
	workloads, err := call(ctx, s, func(ctx context.Context) (*pb.Workloads, error) {
		return s.client().ListNodeWorkloads(ctx, &pb.GetNodeOptions{Nodename: s.config.HostName})
	})
	if err != nil {
		return nil, err
	}

	IDs := make([]string, 0, len(workloads.GetWorkloads()))
	for _, workload := range workloads.GetWorkloads() {
		if workload.GetStatus().GetRunning() {
			IDs = append(IDs, workload.GetId())
		}
	}
	return IDs, nil
}

// WorkloadExists reports whether core still owns the workload.
func (s *Store) WorkloadExists(ctx context.Context, ID string) (bool, error) {
	_, err := call(ctx, s, func(ctx context.Context) (*pb.Workload, error) {
		return s.client().GetWorkload(ctx, &pb.WorkloadID{Id: ID})
	})
	switch {
	case err == nil:
		return true, nil
	case strings.Contains(err.Error(), coretypes.ErrInvaildCount.Error()), strings.Contains(err.Error(), coretypes.ErrWorkloadNotExists.Error()):
		return false, nil
	}
	return false, err
}

func (s *Store) SetWorkloadStatus(ctx context.Context, status *types.WorkloadStatus) error {
	workloadStatus := statusKey(status)
	if cached, ok := s.cache.Get(status.ID); ok && cached == workloadStatus {
		return nil
	}

	// core's selfmon owns status expiry
	statusPb := &pb.WorkloadStatus{
		Id:        status.ID,
		Running:   status.Running,
		Healthy:   status.Healthy,
		Networks:  status.Networks,
		Extension: status.Extension,

		Appname:    status.Appname,
		Entrypoint: status.Entrypoint,
		Nodename:   s.config.HostName,
	}

	opts := &pb.SetWorkloadsStatusOptions{
		Status: []*pb.WorkloadStatus{statusPb},
	}

	_, err := call(ctx, s, func(ctx context.Context) (*pb.WorkloadsStatus, error) {
		return s.client().SetWorkloadsStatus(ctx, opts)
	})
	if err != nil {
		s.cache.Delete(status.ID)
	} else {
		s.cache.Set(status.ID, workloadStatus, getCacheTTL(s.config.HealthCheck.CacheTTL))
	}
	return err
}

func statusKey(status *types.WorkloadStatus) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%t\x00%t\x00%s\x00%s\x00", status.Running, status.Healthy, status.Appname, status.Entrypoint)
	b.Write(status.Extension)
	for _, k := range slices.Sorted(maps.Keys(status.Networks)) {
		fmt.Fprintf(&b, "\x00%s=%s", k, status.Networks[k])
	}
	return b.String()
}

func getCacheTTL(ttl int64) time.Duration {
	delta := rand.Int64N(max(ttl, 1)) / 4 //nolint:gosec // cache ttl jitter needs no csprng
	return time.Duration(ttl-ttl/8+delta) * time.Second
}
