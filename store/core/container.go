package corestore

import (
	"context"
	"encoding/json"

	"github.com/projecteru2/agent/types"
	pb "github.com/projecteru2/core/rpc/gen"
	coretypes "github.com/projecteru2/core/types"
)

// a hour to expire
const defaultContainerStatusTTL = 3600

// SetContainerStatus calls api of core to set status for a container
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
		Ttl:       0,
	}
	pbs := containerStatus.String()
	v, ok := c.cache.Get(container.ID)
	if ok && pbs == v {
		return nil
	}

	if !ok || pbs != v {
		c.cache.Put(container.ID, pbs, defaultContainerStatusTTL)
	}

	opts := &pb.SetWorkloadsStatusOptions{
		Status: []*pb.WorkloadStatus{containerStatus},
	}
	_, err = client.SetWorkloadsStatus(ctx, opts)
	return err
}
