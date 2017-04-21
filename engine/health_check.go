package engine

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/docker/api/types"
	enginefilters "github.com/docker/docker/api/types/filters"
	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
)

const (
	healthNotRunning = 0
	healthNotFound   = 1
	healthGood       = 2
	healthBad        = 3
)

func (e *Engine) healthCheck() {
	interval := e.config.HealthCheckInterval
	if interval == 0 {
		interval = 3
	}

	tick := time.NewTicker(time.Duration(interval) * time.Second)
	defer tick.Stop()
	for ; ; <-tick.C {
		go e.checkAllContainers()
	}
}

// 检查全部label为ERU=1的容器
// 这些容器是被ERU标记管理的
// 似乎是高版本的docker才能用, 但是看起来1.11.2已经有了
func (e *Engine) checkAllContainers() {
	timeout := e.config.HealthCheckTimeout
	if timeout == 0 {
		timeout = 3
	}
	f := enginefilters.NewArgs()
	f.Add("label", "ERU=1")
	containers, err := e.docker.ContainerList(context.Background(), enginetypes.ContainerListOptions{Filters: f})
	if err != nil {
		log.Errorf("Error when list all containers with label \"ERU=1\": %s", err.Error())
		return
	}

	for _, c := range containers {
		// 我只想 fuck docker
		// ContainerList 返回 enginetypes.Container
		// ContainerInspect 返回 enginetypes.ContainerJSON
		// 是不是有毛病啊, 不能返回一样的数据结构么我真是日了狗了... 艹他妹妹...
		container, err := e.docker.ContainerInspect(context.Background(), c.ID)
		if err != nil {
			log.Errorf("Error when inspect container %s in check container health", c.ID)
			continue
		}
		go e.checkOneContainer(container, time.Duration(timeout)*time.Second)
	}
}

// 第一次上的容器可能没有设置health check
// 那么我们认为这个容器一直是健康的, 并且不做检查
// 需要告诉第一次上的时候这个容器是健康的, 还是不是
func (e *Engine) judgeContainerHealth(container enginetypes.ContainerJSON) bool {
	// 如果找不到健康检查, 那么就认为不需要检查, 一直是健康的
	checkMethod, ok := container.Config.Labels["healthcheck"]
	return !(ok && (checkMethod == "tcp" || checkMethod == "http"))
}

// 检查一个容器
func (e *Engine) checkOneContainer(container enginetypes.ContainerJSON, timeout time.Duration) int {
	// 不是running就不检查, 也没办法检查啊...
	if !container.State.Running {
		return healthNotRunning
	}
	// 拿下检查方法, 暂时只支持tcp和http
	checkMethod, ok := container.Config.Labels["healthcheck"]
	if !(ok && (checkMethod == "tcp" || checkMethod == "http")) {
		return healthNotFound
	}

	// 拿老的数据出来
	c, err := e.store.GetContainer(container.ID)
	if err != nil {
		log.Errorf("Error when retrieving data from etcd, Container ID: %s, error: %s", container.ID, err.Error())
		return healthNotFound
	}

	// 检查现在是不是还健康
	healthy := checkSingleContainerHealthy(container, checkMethod, timeout)
	if healthy {
		// 如果健康并且之前是挂了, 那么修改成健康
		if !c.Healthy {
			c.Healthy = true
			e.store.UpdateContainer(c)
			log.Infof("Container %s resurges", container.ID)
		}
		return healthGood
	}
	// 如果挂了并且之前是健康, 那么修改成挂了
	if c.Healthy {
		c.Healthy = false
		e.store.UpdateContainer(c)
		log.Infof("Container %s dies", container.ID)
	}
	return healthBad
}

func checkSingleContainerHealthy(container enginetypes.ContainerJSON, checkMethod string, timeout time.Duration) bool {
	if checkMethod == "tcp" {
		return checkTCP(container, timeout)
	} else if checkMethod == "http" {
		return checkHTTP(container, timeout)
	}
	return false
}

// 检查一个容器的所有URL
// 事实上一般也就一个
func checkHTTP(container enginetypes.ContainerJSON, timeout time.Duration) bool {
	backends := getContainerBackends(container)
	expectedCodeStr, ok := container.Config.Labels["healthcheck_expected_code"]
	if !ok {
		expectedCodeStr = "0"
	}
	expectedCode, err := strconv.Atoi(expectedCodeStr)
	if err != nil {
		expectedCode = 0
	}
	healthcheckURL, ok := container.Config.Labels["healthcheck_url"]
	if !ok {
		healthcheckURL = "/healthcheck"
	}
	if !strings.HasPrefix(healthcheckURL, "/") {
		healthcheckURL = "/" + healthcheckURL
	}

	for _, backend := range backends {
		url := fmt.Sprintf("http://%s%s", backend, healthcheckURL)
		log.Debugf("Check health via http: container %s, url %s, expect code %d", container.ID, url, expectedCode)
		if !checkOneURL(url, expectedCode, timeout) {
			log.Infof("Check health failed via http: container %s, url %s, expect code %d", container.ID, url, expectedCode)
			return false
		}
	}
	return true
}

// 检查一个TCP
func checkTCP(container enginetypes.ContainerJSON, timeout time.Duration) bool {
	backends := getContainerBackends(container)
	for _, backend := range backends {
		log.Debugf("Check health via tcp: container %s, backend %s", container.ID, backend)
		_, err := net.DialTimeout("tcp", backend, timeout)
		if err != nil {
			return false
		}
	}
	return true
}

// 获取一个容器的全部后端
// 其实就是IP跟端口笛卡儿积, 然后IP拿一个就够了
func getContainerBackends(container enginetypes.ContainerJSON) []string {
	backends := []string{}

	// 拿端口的 label
	// 没有的话就算了
	portsLabel, ok := container.Config.Labels["ports"]
	if !ok {
		log.Warnf("Container %s has no ports defined, ignore", container.ID)
		return backends
	}

	// 拿端口列表
	ports := []string{}
	for _, part := range strings.Split(portsLabel, ",") {
		// 必须是 port/protocol 的格式, 不是就不管
		ps := strings.SplitN(part, "/", 2)
		if len(ps) != 2 {
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
func getIPForContainer(container enginetypes.ContainerJSON) string {
	for _, endpoint := range container.NetworkSettings.Networks {
		return endpoint.IPAddress
	}
	return ""
}

// 就先定义 [200, 500) 这个区间的 code 都算是成功吧
func checkOneURL(url string, expectedCode int, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resp, err := ctxhttp.Get(ctx, nil, url)
	if err != nil {
		log.Errorf("Error when checking %s, %s", url, err.Error())
		return false
	}
	if expectedCode == 0 {
		return resp.StatusCode < 500 && resp.StatusCode >= 200
	}
	return resp.StatusCode == expectedCode
}
