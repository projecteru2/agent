package core

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	pb "github.com/projecteru2/core/rpc/gen"

	"github.com/projecteru2/agent/types"
)

func (c *Store) ListRunningWorkloadIDs(ctx context.Context) ([]string, error) {
	workloads, err := call(ctx, c, func(ctx context.Context) (*pb.Workloads, error) {
		return c.GetClient().ListNodeWorkloads(ctx, &pb.GetNodeOptions{Nodename: c.config.HostName})
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

func (c *Store) SetWorkloadStatus(ctx context.Context, status *types.WorkloadStatus) error {
	workloadStatus := fmt.Sprintf("%+v", status)
	if cached, ok := c.cache.Get(status.ID); ok && cached == workloadStatus {
		return nil
	}

	// core's selfmon owns status expiry, so the reported ttl stays zero
	statusPb := &pb.WorkloadStatus{
		Id:        status.ID,
		Running:   status.Running,
		Healthy:   status.Healthy,
		Networks:  status.Networks,
		Extension: status.Extension,

		Appname:    status.Appname,
		Entrypoint: status.Entrypoint,
		Nodename:   c.config.HostName,
	}

	opts := &pb.SetWorkloadsStatusOptions{
		Status: []*pb.WorkloadStatus{statusPb},
	}

	_, err := call(ctx, c, func(ctx context.Context) (*pb.WorkloadsStatus, error) {
		return c.GetClient().SetWorkloadsStatus(ctx, opts)
	})
	if err != nil {
		c.cache.Delete(status.ID)
	} else {
		c.cache.Set(status.ID, workloadStatus, getCacheTTL(c.config.HealthCheck.CacheTTL))
	}
	return err
}

func getCacheTTL(ttl int64) time.Duration {
	delta := rand.Int64N(max(ttl, 1)) / 4 //nolint:gosec // cache ttl jitter needs no csprng
	return time.Duration(ttl-ttl/8+delta) * time.Second
}
