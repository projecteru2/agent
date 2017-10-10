package engine

import (
	"context"
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/docker/api/types"
	enginecontainer "github.com/docker/docker/api/types/container"
	enginefilters "github.com/docker/docker/api/types/filters"
	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/engine/status"
	"github.com/projecteru2/agent/types"
)

func getFilter() enginefilters.Args {
	f := enginefilters.NewArgs()
	f.Add("label", fmt.Sprintf("%s=1", common.ERU_MARK))
	return f
}

func (e *Engine) listContainers(all bool) ([]enginetypes.Container, error) {
	f := getFilter()
	opts := enginetypes.ContainerListOptions{Filters: f, All: all}
	return e.docker.ContainerList(context.Background(), opts)
}

func (e *Engine) activated(f bool) error {
	e.node.Available = f
	return e.store.UpdateNode(e.node)
}

func (e *Engine) crash() error {
	log.Info("[crash] mark all containers unhealthy")
	containers, err := e.listContainers(false)
	if err != nil {
		return err
	}
	for _, c := range containers {
		container, err := e.detectContainer(c.ID, c.Labels)
		if err != nil {
			return err
		}
		container.Healthy = false
		if err := e.store.DeployContainer(container, e.node); err != nil {
			return err
		}
		log.Infof("[crash] mark %s unhealthy", container.ID[:common.SHORTID])
	}
	return e.activated(false)
}

func (e *Engine) detectContainer(ID string, label map[string]string) (*types.Container, error) {
	log.Debugf("[detectContainer] container label %v", label)
	if _, ok := label[common.ERU_MARK]; !ok {
		return nil, fmt.Errorf("not a eru container %s", ID[:common.SHORTID])
	}
	// 生成基准 meta
	delete(label, common.ERU_MARK)
	version := "UNKNOWN"
	if v, ok := label["version"]; ok {
		version = v
	}
	delete(label, "version")
	delete(label, "name")

	c, err := e.docker.ContainerInspect(context.Background(), ID)
	if err != nil {
		return nil, err
	}

	publish := map[string]string{}
	if v, ok := label["publish"]; ok {
		publish = e.makeContainerPublishInfo(c, strings.Split(v, ","))
		delete(label, "publish")
	}

	// 是否符合 eru pattern，如果一个容器又有 ERU_MARK 又是三段式的 name，那它就是个 ERU 容器
	container, err := status.GenerateContainerMeta(c, version, label)
	if err != nil {
		return nil, err
	}
	container.Publish = publish
	container.Networks = c.NetworkSettings.Networks

	return container, nil
}

func (e *Engine) makeContainerPublishInfo(c enginetypes.ContainerJSON, ports []string) map[string]string {
	result := map[string]string{}
	hostIP := e.node.GetIP()
	for nn, ns := range c.NetworkSettings.Networks {
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
