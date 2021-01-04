package corestore

import (
	"encoding/json"
	"fmt"
	"time"

	"context"

	"github.com/projecteru2/agent/types"
	pb "github.com/projecteru2/core/rpc/gen"
	coretypes "github.com/projecteru2/core/types"
)

// SetContainerStatus deploy containers
func (c *CoreStore) SetContainerStatus(ctx context.Context, container *types.Container, node *coretypes.Node) error {
	if c.config.HealthCheck.StatusTTL == 0 {
		status := fmt.Sprintf("%s|%v|%v", container.ID, container.Running, container.Healthy)
		cached, ok := c.cache.Get(container.ID)
		c.cache.Set(container.ID, status, time.Duration(c.config.HealthCheck.CacheTTL)*time.Second)
		if ok {
			str := cached.(string)
			if str == status {
				return nil
			}
		}
	}

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
	_, err = c.client.GetRPCClient().SetWorkloadsStatus(ctx, opts)
	return err
}
