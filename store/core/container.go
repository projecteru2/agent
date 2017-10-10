package corestore

import (
	"encoding/json"

	pb "github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/types"
	"golang.org/x/net/context"
)

func (c *Client) GetContainer(cid string) (*types.Container, error) {
	client := pb.NewCoreRPCClient(c.conn)
	resp, err := client.GetContainerMeta(context.Background(), &pb.ContainerID{Id: cid})
	if err != nil {
		return nil, err
	}
	container := &types.Container{}
	if err := json.Unmarshal(resp.Meta, container); err != nil {
		return nil, err
	}
	return container, nil
}

func (c *Client) UpdateContainer(container *types.Container) error {
	client := pb.NewCoreRPCClient(c.conn)
	bytes, err := json.Marshal(container)
	if err != nil {
		return err
	}
	_, err = client.SetContainerMeta(context.Background(), &pb.ContainerMeta{Meta: bytes})
	if err != nil {
		return err
	}
	return nil
}
