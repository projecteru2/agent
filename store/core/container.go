package corestore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/projecteru2/agent/types"
	pb "github.com/projecteru2/core/rpc/gen"
	coretypes "github.com/projecteru2/core/types"
)

// SetContainerStatus deploy containers
func (c *CoreStore) SetContainerStatus(ctx context.Context, container *types.Container, node *coretypes.Node, ttl int64) error {
	status := fmt.Sprintf("%s|%v|%v", container.ID, container.Running, container.Healthy)
	if ttl == 0 {
		cached, ok := c.cache.Get(container.ID)
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
		Ttl:       ttl,

		Appname:    container.Name,
		Entrypoint: container.EntryPoint,
		Nodename:   c.config.HostName,
	}

	opts := &pb.SetWorkloadsStatusOptions{
		Status: []*pb.WorkloadStatus{containerStatus},
	}

	_, err = c.GetClient().SetWorkloadsStatus(ctx, opts)

	if ttl == 0 {
		if err != nil {
			c.cache.Delete(container.ID)
		} else {
			c.cache.Set(container.ID, status, time.Duration(c.config.HealthCheck.CacheTTL)*time.Second)
		}
	}

	return err
}
