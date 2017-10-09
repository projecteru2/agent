package engine

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/docker/api/types"
	enginecontainer "github.com/docker/docker/api/types/container"
	enginefilters "github.com/docker/docker/api/types/filters"
	coretypes "github.com/projecteru2/core/types"
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
	portsStr, ok := container.Config.Labels["healthcheck_ports"]
	return !(ok && portsStr != "")
}

// 检查一个容器
func (e *Engine) checkOneContainer(container enginetypes.ContainerJSON, timeout time.Duration) int {
	// 不是running就不检查, 也没办法检查啊...
	if !container.State.Running {
		return healthNotRunning
	}

	portsStr, ok := container.Config.Labels["healthcheck_ports"]
	if !ok {
		return healthNotFound
	}
	ports := strings.Split(portsStr, ",")

	// 拿老的数据出来
	c, err := e.store.GetContainer(container.ID)
	if err != nil {
		log.Errorf("Error when retrieving data from etcd, Container ID: %s, error: %v", container.ID, err)
		return healthNotFound
	}

	url := container.Config.Labels["healthcheck_url"]
	code, err := strconv.Atoi(container.Config.Labels["healthcheck_expected_code"])
	if err != nil {
		log.Errorf("Error when getting http check code %v", err)
		return healthNotFound
	}

	healthy := checkSingleContainerHealthy(container, ports, url, code, timeout)
	// 检查现在是不是还健康
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

func checkSingleContainerHealthy(container enginetypes.ContainerJSON, ports []string, url string, code int, timeout time.Duration) bool {
	ip := getIPForContainer(container)
	tcpChecker := []string{}
	httpChecker := []string{}
	for _, port := range ports {
		p := coretypes.Port(port)
		if p.Proto() == "http" {
			httpChecker = append(httpChecker, fmt.Sprintf("http://%s:%s%s", ip, p.Port(), url))
		} else {
			tcpChecker = append(tcpChecker, fmt.Sprintf("%s:%s", ip, p.Port()))
		}
	}

	f1, f2 := true, false
	id := container.ID[:7]
	if len(httpChecker) > 0 {
		f1 = checkHTTP(id, httpChecker, code, timeout)
	}
	if len(tcpChecker) > 0 {
		f2 = checkTCP(id, tcpChecker, timeout)
	}
	return f1 && f2
}

// 检查一个容器的所有URL
// 事实上一般也就一个
func checkHTTP(ID string, backends []string, code int, timeout time.Duration) bool {
	for _, backend := range backends {
		log.Debugf("Check health via http: container %s, url %s, expect code %d", ID, backend, code)
		if !checkOneURL(backend, code, timeout) {
			log.Infof("Check health failed via http: container %s, url %s, expect code %d", ID, backend, code)
			return false
		}
	}
	return true
}

// 检查一个TCP
func checkTCP(ID string, backends []string, timeout time.Duration) bool {
	for _, backend := range backends {
		log.Debugf("Check health via tcp: container %s, backend %s", ID, backend)
		_, err := net.DialTimeout("tcp", backend, timeout)
		if err != nil {
			return false
		}
	}
	return true
}

// 应用应该都是绑定到 0.0.0.0 的
// 所以拿哪个 IP 其实无所谓.
func getIPForContainer(container enginetypes.ContainerJSON) string {
	for name, endpoint := range container.NetworkSettings.Networks {
		networkmode := enginecontainer.NetworkMode(name)
		if !networkmode.IsUserDefined() {
			return "127.0.0.1"
		}
		return endpoint.IPAddress
	}
	return ""
}

// 偷来的函数
// 谁要官方的context没有收录他 ¬ ¬
func get(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		select {
		case <-ctx.Done():
			err = ctx.Err()
		default:
		}
	}
	return resp, err
}

// 就先定义 [200, 500) 这个区间的 code 都算是成功吧
func checkOneURL(url string, expectedCode int, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resp, err := get(ctx, nil, url)
	if err != nil {
		log.Errorf("Error when checking %s, %s", url, err.Error())
		return false
	}
	if expectedCode == 0 {
		return resp.StatusCode < 500 && resp.StatusCode >= 200
	}
	return resp.StatusCode == expectedCode
}
