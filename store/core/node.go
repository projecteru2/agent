package corestore

import (
	pb "github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/types"
	"golang.org/x/net/context"
)

//GetNode return a node by core
func (c *Client) GetNode(nodename string) (*types.Node, error) {
	client := pb.NewCoreRPCClient(c.conn)
	resp, err := client.GetNodeByName(context.Background(), &pb.GetNodeOptions{Nodename: nodename})
	if err != nil {
		return nil, err
	}
	node := &types.Node{
		Name:      resp.Name,
		Podname:   resp.Podname,
		Available: resp.Available,
	}
	return node, nil
}

//UpdateNode update node status
func (c *Client) UpdateNode(node *types.Node) error {
	client := pb.NewCoreRPCClient(c.conn)
	opts := &pb.NodeAvailable{
		Podname:   node.Podname,
		Nodename:  node.Name,
		Available: node.Available,
	}
	_, err := client.SetNodeAvailable(context.Background(), opts)
	return err
}

//Crash update node and it's containers status
func (c *Client) Crash(node *types.Node) error {
	//TODO 标记所有管辖的容器 health 和 alive
	node.Available = false
	return c.UpdateNode(node)
}
