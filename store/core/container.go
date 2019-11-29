package corestore

import (
	"encoding/json"

	"github.com/projecteru2/agent/types"
	pb "github.com/projecteru2/core/rpc/gen"
	coretypes "github.com/projecteru2/core/types"
	"golang.org/x/net/context"
)

// SetContainerStatus deploy containers
func (c *CoreStore) SetContainerStatus(ctx context.Context, container *types.Container, node *coretypes.Node) error {
	client := c.client.GetRPCClient()
	bytes, err := json.Marshal(container.Labels)
	if err != nil {
		return err
	}
	containerStatus := &pb.ContainerStatus{
		Id:        container.ID,
		Running:   container.Running,
		Healthy:   container.Healthy,
		Networks:  container.Networks,
		Extension: bytes,
		Ttl:       int64(2 * c.config.HealthCheckInterval),
	}

	opts := &pb.SetContainersStatusOptions{
		Status: []*pb.ContainerStatus{containerStatus},
	}
	_, err = client.SetContainersStatus(ctx, opts)
	return err
}
