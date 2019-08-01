package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/coreos/etcd/client"
	engineapi "github.com/docker/docker/client"
	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
	coreutils "github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

// CheckExistsError check etcd data exist
func CheckExistsError(err error) error {
	if etcdError, ok := err.(client.Error); ok {
		if etcdError.Code == client.ErrorCodeNodeExist {
			return nil
		}
	}
	return err
}

// MakeDockerClient make a docker client
func MakeDockerClient(config *types.Config) (*engineapi.Client, error) {
	defaultHeaders := map[string]string{"User-Agent": fmt.Sprintf("eru-agent-%s", common.EruAgentVersion)}
	return engineapi.NewClient(config.Docker.Endpoint, common.DockerCliVersion, nil, defaultHeaders)
}

// WritePid write pid
func WritePid(path string) {
	if err := ioutil.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0755); err != nil {
		log.Panicf("Save pid file failed %s", err)
	}
}

// GetAppInfo return app info
func GetAppInfo(containerName string) (name, entrypoint, ident string, err error) {
	return coreutils.ParseContainerName(containerName)
}
