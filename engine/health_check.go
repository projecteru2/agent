package engine

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/projecteru2/agent/types"
	coreutils "github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

func (e *Engine) healthCheck(ctx context.Context) {
	tick := time.NewTicker(time.Duration(e.config.HealthCheck.Interval) * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-tick.C:
			go e.checkAllContainers()
		case <-ctx.Done():
			return
		}
	}
}

// 检查全部 label 为ERU=1的容器
// 这里需要 list all，原因是 monitor 检测到 die 的时候已经标记为 false 了
// 但是这时候 health check 刚返回 true 回来并写入 core
// 为了保证最终数据一致性这里也要检测
func (e *Engine) checkAllContainers() {
	log.Debug("[checkAllContainers] health check begin")
	containers, err := e.listContainers(true, nil)
	if err != nil {
		log.Errorf("[checkAllContainers] Error when list all containers with label \"ERU=1\": %v", err)
		return
	}

	for _, c := range containers {
		// 我只想 fuck docker
		// ContainerList 返回 enginetypes.Container
		// ContainerInspect 返回 enginetypes.ContainerJSON
		// 是不是有毛病啊, 不能返回一样的数据结构么我真是日了狗了... 艹他妹妹...
		container, err := e.detectContainer(c.ID)
		if err != nil {
			log.Errorf("[checkAllContainers] detect container failed %v", err)
			continue
		}

		go e.checkOneContainer(container)
	}
}

// 检查一个容器
func (e *Engine) checkOneContainer(container *types.Container) {
	free, acquired := e.cas.Acquire(container.ID)
	if !acquired {
		return
	}
	defer free()

	// 理论上这里都是 running 的容器，因为 listContainers 标记为 all=false 了
	// 并且都有 healthcheck 标记
	// 检查现在是不是还健康
	// for safe
	container.Healthy = container.Running
	if container.HealthCheck != nil {
		timeout := time.Duration(e.config.HealthCheck.Timeout) * time.Second
		container.Healthy = checkSingleContainerHealthy(container, timeout)
	}

	if err := e.store.SetContainerStatus(context.Background(), container, e.node); err != nil {
		log.Errorf("[checkOneContainer] update deploy status failed %v", err)
	}
}

func checkSingleContainerHealthy(container *types.Container, timeout time.Duration) bool {
	tcpChecker := []string{}
	httpChecker := []string{}

	for _, port := range container.HealthCheck.TCPPorts {
		tcpChecker = append(tcpChecker, fmt.Sprintf("%s:%s", container.LocalIP, port))
	}
	if container.HealthCheck.HTTPPort != "" {
		httpChecker = append(httpChecker, fmt.Sprintf("http://%s:%s%s", container.LocalIP, container.HealthCheck.HTTPPort, container.HealthCheck.HTTPURL))
	}

	id := coreutils.ShortID(container.ID)
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
		conn, err := net.DialTimeout("tcp", backend, timeout)
		if err != nil {
			return false
		}
		conn.Close()
	}
	return true
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
	defer resp.Body.Close()
	if expectedCode == 0 {
		return resp.StatusCode < 500 && resp.StatusCode >= 200
	}
	if resp.StatusCode != expectedCode {
		log.Infof("[checkOneURL] Error when checking %s, expect %d, got %d", url, expectedCode, resp.StatusCode)
	}
	return resp.StatusCode == expectedCode
}
