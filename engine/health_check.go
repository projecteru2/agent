package engine

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/engine-api/types"
	enginefilters "github.com/docker/engine-api/types/filters"
	"gitlab.ricebook.net/platform/agent/common"
	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
)

func (e *Engine) healthCheck() {
	// 默认用一分钟
	interval := e.config.HealthCheckInterval
	if interval == 0 {
		interval = 60
	}

	tick := time.NewTicker(time.Duration(interval) * time.Second)
	for {
		select {
		case <-tick.C:
			go e.checkAllContainers()
		}
	}
}

// 检查全部label为ERU=1的容器
// 这些容器是被ERU标记管理的
// 似乎是高版本的docker才能用, 但是看起来1.11.2已经有了
func (e *Engine) checkAllContainers() {
	f := enginefilters.NewArgs()
	f.Add("label", "ERU=1")
	containers, err := e.docker.ContainerList(context.Background(), enginetypes.ContainerListOptions{Filter: f})
	if err != nil {
		log.Errorf("Error when list all containers with label \"ERU=1\": %s", err.Error())
		return
	}

	for _, container := range containers {
		if getStatus(container.Status) != common.STATUS_START {
			continue
		}

		// 拿老的数据出来
		c, err := e.store.GetContainer(container.ID)
		if err != nil {
			log.Errorf("Error when retrieving data from etcd, Container ID: %s, error: %s", container.ID, err.Error())
			continue
		}

		// 检查现在是不是还健康
		// 如果健康并且之前是挂了, 那么修改成健康
		// 如果挂了并且之前是健康, 那么修改成挂了
		status := checkSingleContainer(container)
		if status && !c.Alive {
			c.Alive = true
			e.store.UpdateContainer(c)
		} else if !status && c.Alive {
			c.Alive = false
			e.store.UpdateContainer(c)
		}
	}
}

// 检查一个容器的所有URL
// 事实上一般也就一个
func checkSingleContainer(container enginetypes.Container) bool {
	backends := getContainerBackends(container)
	total := len(backends)

	ch := make(chan bool, total)
	wg := sync.WaitGroup{}
	wg.Add(total)

	for _, backend := range backends {
		go func(backend string) {
			defer wg.Done()
			url := fmt.Sprintf("http://%s/healthcheck", backend)
			ch <- checkOneURL(url)
		}(backend)
	}
	wg.Wait()

	defer close(ch)
	for i := 0; i < total; i++ {
		r, ok := <-ch
		if !r || !ok {
			return false
		}
	}
	return true
}

// 获取一个容器的全部后端
// 其实就是IP跟端口笛卡儿积, 然后IP拿一个就够了
func getContainerBackends(container enginetypes.Container) []string {
	backends := []string{}

	// 拿端口的 label
	// 没有的话就算了
	portsLabel, ok := container.Labels["ports"]
	if !ok {
		log.Warnf("Container %s has no ports defined, ignore", container.ID)
		return backends
	}

	// 拿端口列表
	// 现在只检测 tcp 的端口
	ports := []string{}
	for _, part := range strings.Split(portsLabel, ",") {
		// 必须是 port/protocol 的格式, 不是就不管
		// protocol 不是 tcp 也不管
		ps := strings.SplitN(part, "/", 2)
		if len(ps) != 2 || ps[1] != "tcp" {
			continue
		}
		ports = append(ports, ps[0])
	}

	// 拿容器的IP, 没有分配IP就不检查了
	ip := getIPForContainer(container)
	if ip == "" {
		return backends
	}

	for _, port := range ports {
		backends = append(backends, net.JoinHostPort(ip, port))
	}
	return backends
}

// 应用应该都是绑定到 0.0.0.0 的
// 所以拿哪个 IP 其实无所谓.
func getIPForContainer(container enginetypes.Container) string {
	for _, endpoint := range container.NetworkSettings.Networks {
		return endpoint.IPAddress
	}
	return ""
}

// 就先定义 [200, 500) 这个区间的 code 都算是成功吧
func checkOneURL(url string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := ctxhttp.Get(ctx, nil, url)
	if err != nil {
		log.Errorf("Error when checking %s, %s", url, err.Error())
		return false
	}

	return resp.StatusCode < 500 && resp.StatusCode >= 200
}
