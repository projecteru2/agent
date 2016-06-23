package utils

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/coreos/etcd/client"
	engineapi "github.com/docker/engine-api/client"
	"gitlab.ricebook.net/platform/agent/common"
	"gitlab.ricebook.net/platform/agent/types"
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

func GetAppInfo(containerName string) (name string, entrypoint string, ident string, err error) {
	containerName = strings.TrimLeft(containerName, "/")
	appinfo := strings.Split(containerName, "_")
	if len(appinfo) < common.CNAME_NUM {
		return "", "", "", errors.New("container name is not eru pattern")
	}
	l := len(appinfo)
	return strings.Join(appinfo[:l-2], "_"), appinfo[l-2], appinfo[l-1], nil
}
