package etcdstore

import (
	"encoding/json"

	"github.com/coreos/etcd/client"
	"gitlab.ricebook.net/platform/agent/types"
	"gitlab.ricebook.net/platform/agent/utils"
	"golang.org/x/net/context"
)

func (c *Client) UpdateStats(node *types.Node) error {
	b, err := json.Marshal(node)
	if err != nil {
		return err
	}
	_, err = c.etcd.Set(context.Background(), c.stats, string(b), nil)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) RegisterNode(node *types.Node) error {
	_, err := c.etcd.Set(context.Background(), c.containers, "", &client.SetOptions{Dir: true, PrevExist: client.PrevNoExist})
	if utils.CheckExistsError(err) != nil {
		return err
	}
	return c.UpdateStats(node)
}

func (c *Client) Crash() error {
	resp, err := c.etcd.Get(context.Background(), c.containers, &client.GetOptions{})
	if err != nil {
		return err
	}
	for _, n := range resp.Node.Nodes {
		container := &types.Container{}
		err := json.Unmarshal([]byte(n.Value), container)
		if err != nil {
			return err
		}
		container.Alive = false
		if err := c.UpdateContainer(container); err != nil {
			return err
		}
	}
	resp, err = c.etcd.Get(context.Background(), c.stats, &client.GetOptions{})
	if err != nil {
		return err
	}
	node := &types.Node{}
	err = json.Unmarshal([]byte(resp.Node.Value), node)
	if err != nil {
		return err
	}
	node.Alive = false
	return c.UpdateStats(node)
}
