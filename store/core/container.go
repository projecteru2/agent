package corestore

import (
	"encoding/json"

	"github.com/projecteru2/agent/types"
	pb "github.com/projecteru2/core/rpc/gen"
	coretypes "github.com/projecteru2/core/types"
	"golang.org/x/net/context"
)

func (c *Client) DeployContainer(container *types.Container, node *coretypes.Node) error {
	client := pb.NewCoreRPCClient(c.conn)
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
