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
	enginecontainer "github.com/docker/docker/api/types/container"
	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
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
	log.Info("[checkAllContainers] health check begin")
	timeout := e.config.HealthCheckTimeout
	if timeout == 0 {
		timeout = 3
	}
	containers, err := e.listContainers(false)
	if err != nil {
		log.Errorf("[checkAllContainers] Error when list all containers with label \"ERU=1\": %v", err)
		return
	}

	for _, c := range containers {
		// 我只想 fuck docker
		// ContainerList 返回 enginetypes.Container
		// ContainerInspect 返回 enginetypes.ContainerJSON
		// 是不是有毛病啊, 不能返回一样的数据结构么我真是日了狗了... 艹他妹妹...
		container, err := e.detectContainer(c.ID, c.Labels)
		if err != nil {
			log.Errorf("[checkAllContainers] detect container failed %v", err)
			continue
		}
		go e.checkOneContainer(container, time.Duration(timeout)*time.Second)
	}
}

// 检查一个容器
func (e *Engine) checkOneContainer(container *types.Container, timeout time.Duration) int {
	// 不是running就不检查, 也没办法检查啊...
	if !container.Running {
		return healthNotRunning
	}

	portsStr, ok := container.Extend["healthcheck_ports"]
	if !ok {
		return healthNotFound
	}
	ports := strings.Split(portsStr, ",")

	// 拿老的数据出来
	url := container.Extend["healthcheck_url"]
	code, err := strconv.Atoi(container.Extend["healthcheck_expected_code"])
	if err != nil {
		log.Errorf("[checkOneContainer] Error when getting http check code %v", err)
		return healthNotFound
	}

	// 检查现在是不是还健康
	healthy := checkSingleContainerHealthy(container, ports, url, code, timeout)
	if healthy && !container.Healthy {
		// 如果健康并且之前是挂了, 那么修改成健康
		container.Healthy = true
		if err := e.store.DeployContainer(container, e.node); err != nil {
			log.Errorf("[checkOneContainer] update deploy status failed %v", err)
		}
		log.Infof("[checkOneContainer] Container %s resurges", container.ID[:common.SHORTID])
		return healthGood
	} else if !healthy && container.Healthy {
		// 如果挂了并且之前是健康, 那么修改成挂了
		container.Healthy = false
		if err := e.store.DeployContainer(container, e.node); err != nil {
			log.Errorf("[checkOneContainer] update deploy status failed %v", err)
		}
		log.Infof("[checkOneContainer] Container %s dies", container.ID[:common.SHORTID])
		return healthBad
	}
	return healthNotFound
}

func checkSingleContainerHealthy(container *types.Container, ports []string, url string, code int, timeout time.Duration) bool {
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
	id := container.ID[:common.SHORTID]
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
		log.Debugf("[checkHTTP] Check health via http: container %s, url %s, expect code %d", ID, backend, code)
		if !checkOneURL(backend, code, timeout) {
			log.Infof("[checkHTTP] Check health failed via http: container %s, url %s, expect code %d", ID, backend, code)
			return false
		}
	}
	return true
}

// 检查一个TCP
func checkTCP(ID string, backends []string, timeout time.Duration) bool {
	for _, backend := range backends {
		log.Debugf("[checkTCP] Check health via tcp: container %s, backend %s", ID, backend)
		_, err := net.DialTimeout("tcp", backend, timeout)
		if err != nil {
			return false
		}
	}
	return true
}

// 应用应该都是绑定到 0.0.0.0 的
// 所以拿哪个 IP 其实无所谓.
func getIPForContainer(container *types.Container) string {
	for name, endpoint := range container.Networks {
		networkmode := enginecontainer.NetworkMode(name)
		if networkmode.IsHost() {
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
		log.Errorf("[checkOneURL] Error when checking %s, %s", url, err.Error())
		return false
	}
	if expectedCode == 0 {
		return resp.StatusCode < 500 && resp.StatusCode >= 200
	}
	return resp.StatusCode == expectedCode
}
