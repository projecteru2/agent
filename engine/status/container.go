package status

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/docker/api/types"
	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
)

//GenerateContainerMeta make meta obj
func GenerateContainerMeta(c enginetypes.ContainerJSON, version string, extend map[string]string) (*types.Container, error) {
	if !c.State.Running {
		return nil, fmt.Errorf("container %s [%s] not running", c.Name, c.ID[:common.SHORTID])
	}

	name, entrypoint, ident, err := utils.GetAppInfo(c.Name)
	if err != nil {
		return nil, err
	}

	// 第一次上的容器可能没有设置health check
	// 那么我们认为这个容器一直是健康的, 并且不做检查
	// 需要告诉第一次上的时候这个容器是健康的, 还是不是
	portsStr, ok := c.Config.Labels["healthcheck_ports"]
	container := &types.Container{
		ID:         c.ID,
		Pid:        c.State.Pid,
		Running:    c.State.Running,
		Healthy:    !(ok && portsStr != ""),
		Name:       name,
		EntryPoint: entrypoint,
		Ident:      ident,
		Version:    version,
		CPUQuota:   c.HostConfig.Resources.CPUQuota,
		CPUPeriod:  c.HostConfig.Resources.CPUPeriod,
		CPUShares:  c.HostConfig.Resources.CPUShares,
		Memory:     c.HostConfig.Resources.Memory,
		Extend:     extend,
	}
	log.Debugf("[GenerateContainerMeta] Generate container meta %v", container.Name)
	return container, nil
}
