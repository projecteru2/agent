package corestore

import (
	"encoding/json"

	"context"

	"github.com/projecteru2/agent/types"
	pb "github.com/projecteru2/core/rpc/gen"
	coretypes "github.com/projecteru2/core/types"
)

// SetContainerStatus deploy containers
func (c *CoreStore) SetContainerStatus(ctx context.Context, container *types.Container, node *coretypes.Node) error {
	client := c.client.GetRPCClient()
	bytes, err := json.Marshal(container.Labels)
	if err != nil {
		return err
	}
	containerStatus := &pb.WorkloadStatus{
		Id:        container.ID,
		Running:   container.Running,
		Healthy:   container.Healthy,
		Networks:  container.Networks,
		Extension: bytes,
		Ttl:       int64(2*c.config.HealthCheck.StatusTTL + c.config.HealthCheck.StatusTTL/2),
	}

	opts := &pb.SetWorkloadsStatusOptions{
		Status: []*pb.WorkloadStatus{containerStatus},
	}
	_, err = client.SetWorkloadsStatus(ctx, opts)
	return err
}
