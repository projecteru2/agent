package engine

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/engine-api/types"
	enginefilters "github.com/docker/engine-api/types/filters"
	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
)

const (
	timeout            = 5 * time.Second
	HEALTH_NOT_RUNNING = 0
	HEALTH_NOT_FOUND   = 1
	HEALTH_GOOD        = 2
	HEALTH_BAD         = 3
)

func (e *Engine) healthCheck() {
	// 默认用一分钟
	interval := e.config.HealthCheckInterval
	if interval == 0 {
		interval = 60
	}

	tick := time.NewTicker(time.Duration(interval) * time.Second)
	defer tick.Stop()
	for ; ; <-tick.C {
		log.Debugf("Start to check all containers...")
		go e.checkAllContainers()
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
		e.checkOneContainer(container)
	}
}

// 检查新上线的容器, 目前是检查10分钟, 10秒钟一次.
// 也就是一共检查最多60次.
// 在容器健康后或者10分钟时间到停止.
func (e *Engine) checkNewContainerHealth(container enginetypes.ContainerJSON) {
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	timeout := time.After(10 * time.Minute)
	for {
		select {
		case <-timeout:
			log.Infof("Timeout when check new container %s", container.ID)
			return
		case <-tick.C:
			r := e.checkOneContainer(container)
			if r == HEALTH_GOOD || r == HEALTH_NOT_FOUND {
				log.Infof("Check new container %s health stop", container.ID)
				return
			}
		}
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
func (e *Engine) checkOneContainer(container enginetypes.ContainerJSON) int {
	// 不是running就不检查, 也没办法检查啊...
	if !container.State.Running {
		return HEALTH_NOT_RUNNING
	}
	// 拿下检查方法, 暂时只支持tcp和http
	checkMethod, ok := container.Config.Labels["healthcheck"]
	if !(ok && (checkMethod == "tcp" || checkMethod == "http")) {
		return HEALTH_NOT_FOUND
	}

	// 拿老的数据出来
	c, err := e.store.GetContainer(container.ID)
	if err != nil {
		log.Errorf("Error when retrieving data from etcd, Container ID: %s, error: %s", container.ID, err.Error())
		return HEALTH_NOT_FOUND
	}

	// 检查现在是不是还健康
	healthy := checkSingleContainerHealthy(container, checkMethod)
	if healthy {
		// 如果健康并且之前是挂了, 那么修改成健康
		if !c.Healthy {
			c.Healthy = true
			e.store.UpdateContainer(c)
			log.Infof("Container %s resurges", container.ID)
		}
		return HEALTH_GOOD
	} else {
		// 如果挂了并且之前是健康, 那么修改成挂了
		if c.Healthy {
			c.Healthy = false
			e.store.UpdateContainer(c)
			log.Infof("Container %s dies", container.ID)
		}
		return HEALTH_BAD
	}
}

func checkSingleContainerHealthy(container enginetypes.ContainerJSON, checkMethod string) bool {
	if checkMethod == "tcp" {
		return checkTCP(container)
	} else if checkMethod == "http" {
		return checkHTTP(container)
	}
	return false
}

// 检查一个容器的所有URL
// 事实上一般也就一个
func checkHTTP(container enginetypes.ContainerJSON) bool {
	backends := getContainerBackends(container)
	expected_code_str, ok := container.Config.Labels["healthcheck_expected_code"]
	if !ok {
		expected_code_str = "0"
	}
	expected_code, err := strconv.Atoi(expected_code_str)
	if err != nil {
		expected_code = 0
	}
	healthcheck_url, ok := container.Config.Labels["healthcheck_url"]
	if !ok {
		healthcheck_url = "/healthcheck"
	}
	if !strings.HasPrefix(healthcheck_url, "/") {
		healthcheck_url = "/" + healthcheck_url
	}

	for _, backend := range backends {
		url := fmt.Sprintf("http://%s%s", backend, healthcheck_url)
		if !checkOneURL(url, expected_code) {
			return false
		}
	}
	return true
}

// 检查一个TCP
func checkTCP(container enginetypes.ContainerJSON) bool {
	backends := getContainerBackends(container)
	for _, backend := range backends {
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
func getIPForContainer(container enginetypes.ContainerJSON) string {
	for _, endpoint := range container.NetworkSettings.Networks {
		return endpoint.IPAddress
	}
	return ""
}

// 就先定义 [200, 500) 这个区间的 code 都算是成功吧
func checkOneURL(url string, expected_code int) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resp, err := ctxhttp.Get(ctx, nil, url)
	if err != nil {
		log.Errorf("Error when checking %s, %s", url, err.Error())
		return false
	}
	if expected_code == 0 {
		return resp.StatusCode < 500 && resp.StatusCode >= 200
	}
	return resp.StatusCode == expected_code
}
