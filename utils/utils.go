package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/client"
	engineapi "github.com/docker/docker/client"
	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
	coreutils "github.com/projecteru2/core/utils"
)

func CheckExistsError(err error) error {
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

func WritePid(path string) {
	if err := ioutil.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0755); err != nil {
		log.Panicf("Save pid file failed %s", err)
	}
}

func GetAppInfo(containerName string) (name, entrypoint, ident string, err error) {
	containerName = strings.TrimLeft(containerName, "/")
	return coreutils.ParseContainerName(containerName)
}
