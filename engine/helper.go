package engine

import (
	"context"
	"fmt"
	"strings"

	enginetypes "github.com/docker/docker/api/types"
	enginecontainer "github.com/docker/docker/api/types/container"
	enginefilters "github.com/docker/docker/api/types/filters"
	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/engine/status"
	"github.com/projecteru2/agent/types"
)

func getFilter(extend map[string]string) enginefilters.Args {
	f := enginefilters.NewArgs()
	f.Add("label", fmt.Sprintf("%s=1", common.ERU_MARK))
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

func (e *Engine) detectContainer(ID string, label map[string]string) (*types.Container, error) {
	if _, ok := label[common.ERU_MARK]; !ok {
		return nil, fmt.Errorf("not a eru container %s", ID[:common.SHORTID])
	}

	// 标准化为 inspect 的数据
	c, err := e.docker.ContainerInspect(context.Background(), ID)
	if err != nil {
		return nil, err
	}
	label = c.Config.Labels

	// 生成基准 meta
	delete(label, common.ERU_MARK)
	version := "UNKNOWN"
	if v, ok := label["version"]; ok {
		version = v
	}
	delete(label, "version")
	delete(label, "name")

	pubStr, _ := label["publish"]
	delete(label, "publish")

	// 是否符合 eru pattern，如果一个容器又有 ERU_MARK 又是三段式的 name，那它就是个 ERU 容器
	container, err := status.GenerateContainerMeta(c, version, label)
	if err != nil {
		return container, err
	}
	// 活着才有发布必要
	if c.NetworkSettings != nil {
		if container.Running && pubStr != "" {
			container.Publish = e.makeContainerPublishInfo(c.NetworkSettings, strings.Split(pubStr, ","))
		}
		container.Networks = c.NetworkSettings.Networks
	}

	return container, nil
}

func (e *Engine) makeContainerPublishInfo(nss *enginetypes.NetworkSettings, ports []string) map[string]string {
	result := map[string]string{}
	hostIP := e.node.GetIP()
	for nn, ns := range nss.Networks {
		ip := ns.IPAddress
		if enginecontainer.NetworkMode(nn).IsHost() {
			ip = hostIP
		}

		data := []string{}
		for _, port := range ports {
			data = append(data, fmt.Sprintf("%s:%s", ip, port))
		}
		result[nn] = strings.Join(data, ",")
	}
	return result
}
