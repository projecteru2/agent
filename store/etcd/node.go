package etcdstore

import (
	"encoding/json"

	"github.com/coreos/etcd/client"
	"gitlab.ricebook.net/platform/agent/types"
	"gitlab.ricebook.net/platform/agent/utils"
	"golang.org/x/net/context"
)

func (c *Client) UpdateStats(stats *types.NodeStats) error {
	b, err := json.Marshal(stats)
	if err != nil {
		return err
	}
	_, err = c.etcd.Set(context.Background(), c.stats, string(b), nil)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) RegisterNode(stats *types.NodeStats) error {
	_, err := c.etcd.Set(context.Background(), c.containers, "", &client.SetOptions{Dir: true, PrevExist: client.PrevNoExist})
	if utils.CheckExistsError(err) != nil {
		return err
	}
	return c.UpdateStats(stats)
}

func (c *Client) Crash() error {
	resp, err := c.etcd.Get(context.Background(), c.containers, &client.GetOptions{})
	if err != nil {
		return err
	}
	for _, node := range resp.Node.Nodes {
		stats := types.ContainerStats{}
		err := json.Unmarshal([]byte(node.Value), &stats)
		if err != nil {
			return err
		}
		stats.Alive = false
		if err := c.UpdateContainer(&stats); err != nil {
			return err
		}
	}
	resp, err = c.etcd.Get(context.Background(), c.stats, &client.GetOptions{})
	if err != nil {
		return err
	}
	stats := types.NodeStats{}
	err = json.Unmarshal([]byte(resp.Node.Value), &stats)
	if err != nil {
		return err
	}
	stats.Alive = false
	return c.UpdateStats(&stats)
}
