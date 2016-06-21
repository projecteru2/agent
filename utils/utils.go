package utils

import (
	"fmt"

	"github.com/coreos/etcd/client"
	engineapi "github.com/docker/engine-api/client"
	"gitlab.ricebook.net/platform/agent/common"
	"gitlab.ricebook.net/platform/agent/types"
)

func CheckExistsError(err error) error {
	//FIXME indicate path exists
	if etcdError, ok := err.(client.Error); ok {
		if etcdError.Code == client.ErrorCodeNodeExist {
			return nil
		}
	}
	return err
}

func MakeDockerClient(config types.Config) (*engineapi.Client, error) {
	defaultHeaders := map[string]string{"User-Agent": fmt.Sprintf("eru-agent-%s", common.ERU_AGENT_VERSION)}
	return engineapi.NewClient(config.Docker.Endpoint, common.DOCKER_CLI_VERSION, nil, defaultHeaders)
}
