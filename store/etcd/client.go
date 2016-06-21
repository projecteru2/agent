package etcdstore

import (
	"fmt"

	"github.com/coreos/etcd/client"
	"gitlab.ricebook.net/platform/agent/types"
)

type Client struct {
	etcd       client.KeysAPI
	config     types.Config
	root       string
	containers string
	stats      string
}

func NewClient(config types.Config) (*Client, error) {
	if len(config.Etcd.EtcdMachines) == 0 {
		return nil, fmt.Errorf("ETCD must be set")
	}

	cli, err := client.New(client.Config{Endpoints: config.Etcd.EtcdMachines})
	if err != nil {
		return nil, err
	}

	etcd := client.NewKeysAPI(cli)
	root := fmt.Sprintf("/%s/%s", config.Etcd.Prefix, config.HostName)
	stats := fmt.Sprintf("%s/stats", root)
	containers := fmt.Sprintf("%s/containers", root)
	return &Client{etcd: etcd, config: config, root: root, stats: stats, containers: containers}, nil
}
