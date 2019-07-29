package engine

import (
	"context"
	"fmt"

	enginetypes "github.com/docker/docker/api/types"
	enginecontainer "github.com/docker/docker/api/types/container"
	enginefilters "github.com/docker/docker/api/types/filters"
	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/engine/status"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/core/cluster"
	engine "github.com/projecteru2/core/engine/docker"
	coreutils "github.com/projecteru2/core/utils"
)

func getFilter(extend map[string]string) enginefilters.Args {
	f := enginefilters.NewArgs()
	f.Add("label", fmt.Sprintf("%s=1", cluster.ERUMark))
	for k, v := range extend {
		f.Add(k, v)
	}
	return f
}

func (e *Engine) listContainers(all bool, extend map[string]string) ([]enginetypes.Container, error) {
	f := getFilter(extend)
	opts := enginetypes.ContainerListOptions{Filters: f, All: all}
	return e.docker.ContainerList(context.Background(), opts)
}

func (e *Engine) activated(f bool) error {
	e.node.Available = f
	return e.store.UpdateNode(e.node)
}

func (e *Engine) detectContainer(ID string) (*types.Container, error) {
	// 标准化为 inspect 的数据
	c, err := e.docker.ContainerInspect(context.Background(), ID)
	if err != nil {
		return nil, err
	}
	label := c.Config.Labels

	if _, ok := label[cluster.ERUMark]; !ok {
		return nil, fmt.Errorf("not a eru container %s", ID[:common.SHORTID])
	}
	delete(label, cluster.ERUMark)

	// 生成基准 meta
	meta := coreutils.DecodeMetaInLabel(label)
	delete(label, cluster.ERUMeta)

	// 是否符合 eru pattern，如果一个容器又有 ERUMark 又是三段式的 name，那它就是个 ERU 容器
	container, err := status.GenerateContainerMeta(c, meta, label)
	if err != nil {
		return container, err
	}
	// 计算容器用了多少 CPU
	container = status.CalcuateCPUNum(container, c, e.cpuCore)
	// 活着才有发布必要
	if c.NetworkSettings != nil && container.Running {
		networks := map[string]string{}
		for name, endpoint := range c.NetworkSettings.Networks {
			networkmode := enginecontainer.NetworkMode(name)
			if networkmode.IsHost() {
				container.LocalIP = engine.GetIP(e.node.Endpoint)
				networks[name] = container.LocalIP
			} else {
				container.LocalIP = endpoint.IPAddress
				networks[name] = endpoint.IPAddress
			}
			break
		}
		container.Networks = networks
	}

	return container, nil
}
