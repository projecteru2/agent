package etcdstore

import (
	"encoding/json"
	"fmt"

	"gitlab.ricebook.net/platform/agent/types"
	"golang.org/x/net/context"
)

func (c *Client) UpdateContainer(stats *types.ContainerStats) error {
	path := fmt.Sprintf("%s/%s", c.containers, stats.Cid)
	b, err := json.Marshal(stats)
	if err != nil {
		return err
	}
	_, err = c.etcd.Set(context.Background(), path, string(b), nil)
	return err
}
