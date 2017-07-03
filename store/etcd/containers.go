package etcdstore

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coreos/etcd/client"
	"gitlab.ricebook.net/platform/agent/types"
	"golang.org/x/net/context"
)

func (c *Client) UpdateContainer(container *types.Container) error {
	path := fmt.Sprintf("%s/%s", c.containers, container.ID)
	b, err := json.Marshal(container)
	if err != nil {
		return err
	}
	_, err = c.etcd.Set(context.Background(), path, string(b), nil)
	return err
}

func (c *Client) GetContainer(cid string) (*types.Container, error) {
	path := fmt.Sprintf("%s/%s", c.containers, cid)
	container := &types.Container{}
	resp, err := c.etcd.Get(context.Background(), path, &client.GetOptions{})
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(resp.Node.Value), container); err != nil {
		return nil, err
	}
	return container, nil
}

func (c *Client) RemoveContainer(cid string) error {
	path := fmt.Sprintf("%s/%s", c.containers, cid)
	_, err := c.etcd.Delete(context.Background(), path, &client.DeleteOptions{})
	return err
}

func (c *Client) GetAllContainers() (containers []string, err error) {
	resp, err := c.etcd.Get(context.Background(), c.containers, &client.GetOptions{})
	if err != nil {
		return containers, err
	}
	for _, node := range resp.Node.Nodes {
		t := strings.Split(node.Key, "/")
		containerID := t[len(t)-1:][0]
		containers = append(containers, containerID)
	}
	return containers, nil
}
