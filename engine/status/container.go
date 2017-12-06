package status

import (
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/docker/api/types"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
	coretypes "github.com/projecteru2/core/types"
)

//GenerateContainerMeta make meta obj
func GenerateContainerMeta(c enginetypes.ContainerJSON, version string, extend map[string]string) (*types.Container, error) {
	name, entrypoint, ident, err := utils.GetAppInfo(c.Name)
	if err != nil {
		return nil, err
	}

	if !c.State.Running {
		return &types.Container{
			ID:         c.ID,
			Name:       name,
			EntryPoint: entrypoint,
			Ident:      ident,
			Version:    version,
			Healthy:    false,
			Running:    false,
			Extend:     extend,
		}, nil
	}

	// 第一次上的容器可能没有设置health check
	// 那么我们认为这个容器一直是健康的, 并且不做检查
	// 需要告诉第一次上的时候这个容器是健康的, 还是不是
	_, checker := c.Config.Labels["healthcheck"]
	container := &types.Container{
		ID:          c.ID,
		Pid:         c.State.Pid,
		Running:     c.State.Running,
		Healthy:     !checker,
		Name:        name,
		EntryPoint:  entrypoint,
		Ident:       ident,
		Version:     version,
		CPUQuota:    c.HostConfig.Resources.CPUQuota,
		CPUPeriod:   c.HostConfig.Resources.CPUPeriod,
		CPUShares:   c.HostConfig.Resources.CPUShares,
		Memory:      c.HostConfig.Resources.Memory,
		HealthCheck: nil,
	}
	if checker {
		delete(extend, "healthcheck")
		tcpPorts, _ := c.Config.Labels["healthcheck_tcp"]
		httpPort, _ := c.Config.Labels["healthcheck_http"]
		httpURL, _ := c.Config.Labels["healthcheck_url"]
		var httpCode int
		if codeStr, ok := c.Config.Labels["healthcheck_code"]; ok {
			httpCode, err = strconv.Atoi(codeStr)
			if err != nil {
				log.Warnf("[GenerateContainerMeta] code invaild %s", codeStr)
			}
		}
		healthCheck := &coretypes.HealthCheck{
			TCPPorts: strings.Split(tcpPorts, ","),
			HTTPPort: httpPort,
			HTTPURL:  httpURL,
			HTTPCode: httpCode,
		}
		container.HealthCheck = healthCheck
	}
	container.Extend = extend
	log.Debugf("[GenerateContainerMeta] Generate container meta %v %v", container.Name, container.EntryPoint)
	return container, nil
}
