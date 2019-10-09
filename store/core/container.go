package corestore

import (
	"encoding/json"

	"github.com/projecteru2/agent/types"
	pb "github.com/projecteru2/core/rpc/gen"
	coretypes "github.com/projecteru2/core/types"
	"golang.org/x/net/context"
)

// DeployContainerStats deploy containers
func (c *CoreStore) DeployContainerStats(container *types.Container, node *coretypes.Node) error {
	client := c.client.GetRPCClient()
	bytes, err := json.Marshal(container)
	if err != nil {
		return err
	}
	opts := &pb.ContainerDeployedOptions{
		Id:         container.ID,
		Appname:    container.Name,
		Entrypoint: container.EntryPoint,
		Nodename:   node.Name,
		Data:       bytes,
	}
	_, err = client.ContainerDeployed(context.Background(), opts)
	return err
}
