package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	engineapi "github.com/docker/docker/client"
	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/version"
	coreutils "github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

// MakeDockerClient make a docker client
func MakeDockerClient(config *types.Config) (*engineapi.Client, error) {
	defaultHeaders := map[string]string{"User-Agent": fmt.Sprintf("eru-agent-%s", version.VERSION)}
	return engineapi.NewClient(config.Docker.Endpoint, common.DockerCliVersion, nil, defaultHeaders)
}

// WritePid write pid
func WritePid(path string) {
	if err := ioutil.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		log.Panicf("Save pid file failed %s", err)
	}
}

// GetAppInfo return app info
func GetAppInfo(containerName string) (name, entrypoint, ident string, err error) {
	return coreutils.ParseWorkloadName(containerName)
}

// Max return max value
func Max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
