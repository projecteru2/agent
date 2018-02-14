package engine

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"
	enginecontainer "github.com/docker/docker/api/types/container"
	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
)

const (
	healthNotRunning = 0
	healthNotFound   = 1
	healthGood       = 2
	healthBad        = 3
)

func (e *Engine) healthCheck() {
	interval := e.config.HealthCheckInterval
	tick := time.NewTicker(time.Duration(interval) * time.Second)
	defer tick.Stop()
	for ; ; <-tick.C {
		go e.checkAllContainers()
	}
}

// 检查全部 label 为ERU=1的容器
// 这里需要 list all，原因是 monitor 检测到 die 的时候已经标记为 false 了
// 但是这时候 health check 刚返回 true 回来并写入 core
// 为了保证最终数据一致性这里也要检测
func (e *Engine) checkAllContainers() {
	log.Debug("[checkAllContainers] health check begin")
	timeout := time.Duration(e.config.HealthCheckTimeout) * time.Second
	containers, err := e.listContainers(true, map[string]string{"label": "healthcheck"})
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

		go e.checkOneContainer(container, timeout)
	}
}

// 检查一个容器
func (e *Engine) checkOneContainer(container *types.Container, timeout time.Duration) int {
	// 理论上这里都是 running 的容器，因为 listContainers 标记为 all=false 了
	// 并且都有 healthcheck 标记
	// 检查现在是不是还健康
	// for safe
	if container.HealthCheck == nil {
		return healthNotFound
	}

	healthy := checkSingleContainerHealthy(container, timeout)
	prevHealthy := e.checker.Get(container.ID)
	defer e.checker.Set(container.ID, healthy)
	if healthy && !prevHealthy {
		// 如果健康并且之前是挂了, 那么修改成健康
		container.Healthy = true
		if err := e.store.DeployContainer(container, e.node); err != nil {
			log.Errorf("[checkOneContainer] update deploy status failed %v", err)
		}
		log.Infof("[checkOneContainer] Container %s resurges", container.ID[:common.SHORTID])
		return healthGood
	} else if !healthy && prevHealthy {
		// 如果挂了并且之前是健康, 那么修改成挂了
		container.Healthy = false
		if err := e.store.DeployContainer(container, e.node); err != nil {
			log.Errorf("[checkOneContainer] update deploy status failed %v", err)
		}
		log.Infof("[continerDie] Container %s dies", container.ID[:common.SHORTID])
		return healthBad
	}
	return healthNotFound
}

func checkSingleContainerHealthy(container *types.Container, timeout time.Duration) bool {
	ip := getIPForContainer(container)
	tcpChecker := []string{}
	httpChecker := []string{}

	for _, port := range container.HealthCheck.TCPPorts {
		tcpChecker = append(tcpChecker, fmt.Sprintf("%s:%s", ip, port))
	}
	if container.HealthCheck.HTTPPort != "" {
		httpChecker = append(httpChecker, fmt.Sprintf("http://%s:%s%s", ip, container.HealthCheck.HTTPPort, container.HealthCheck.HTTPURL))
	}

	id := container.ID[:common.SHORTID]
	f1 := checkHTTP(id, httpChecker, container.HealthCheck.HTTPCode, timeout)
	f2 := checkTCP(id, tcpChecker, timeout)
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
		log.Warnf("[checkOneURL] Error when checking %s, %s", url, err.Error())
		return false
	}
	if expectedCode == 0 {
		return resp.StatusCode < 500 && resp.StatusCode >= 200
	}
	if resp.StatusCode != expectedCode {
		log.Infof("[checkOneURL] Error when checking %s, expect %d, got %d", url, expectedCode, resp.StatusCode)
	}
	return resp.StatusCode == expectedCode
}
