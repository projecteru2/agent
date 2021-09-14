package utils

import (
	"context"
	"net"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

// CheckHTTP 检查一个workload的所有URL
// CheckHTTP 事实上一般也就一个
func CheckHTTP(ctx context.Context, ID string, backends []string, code int, timeout time.Duration) bool {
	for _, backend := range backends {
		log.Debugf("[checkHTTP] Check health via http: workload %s, url %s, expect code %d", ID, backend, code)
		if !checkOneURL(ctx, backend, code, timeout) {
			log.Infof("[checkHTTP] Check health failed via http: workload %s, url %s, expect code %d", ID, backend, code)
			return false
		}
	}
	return true
}

// CheckTCP 检查一个TCP
// 这里不支持ctx?
func CheckTCP(ID string, backends []string, timeout time.Duration) bool {
	for _, backend := range backends {
		log.Debugf("[checkTCP] Check health via tcp: workload %s, backend %s", ID, backend)
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
func checkOneURL(ctx context.Context, url string, expectedCode int, timeout time.Duration) bool {
	var resp *http.Response
	var err error
	WithTimeout(ctx, timeout, func(ctx context.Context) {
		resp, err = get(ctx, nil, url) // nolint
	})
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
