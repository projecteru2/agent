package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"runtime"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

const (
	podname    = "dev_pod"
	desc       = "dev pod with one node"
	nodename   = "node1"
	image      = "hub.testhub.com/base/alpine:base-2017.03.14"
	APIVersion = "v1.29"
	mockMemory = int64(8589934592)
	mockID     = "f1f9da344e8f8f90f73899ddad02da6cdf2218bbe52413af2bcfef4fba2d22de"
)

var (
	err error
)

func testlogF(format interface{}, a ...interface{}) {
	var (
		caller string
		main   string
	)
	_, fn, line, _ := runtime.Caller(1)
	caller = fmt.Sprintf("%s:%d", fn, line)
	s := strings.Split(caller, "/")
	caller = s[len(s)-1]

	switch format.(type) {
	case string:
		main = fmt.Sprintf(format.(string), a...)
	default:
		main = fmt.Sprintf("%v", format)
	}
	fmt.Printf("%s: %s \n", caller, main)
}

func mockDockerDoer(r *http.Request) (*http.Response, error) {
	var b []byte
	prefix := fmt.Sprintf("/%s", APIVersion)
	path := strings.TrimPrefix(r.URL.Path, prefix)

	// get container id
	containerID := ""
	if strings.HasPrefix(path, "/containers/") {
		cid := strings.TrimPrefix(path, "/containers/")
		containerID = strings.Split(cid, "/")[0]
	}

	// mock docker responses
	switch path {
	case "/info": // docker info
		testlogF("mock docker info response")
		info := &types.Info{
			ID:         "daemonID",
			Containers: 3,
		}
		b, _ = json.Marshal(info)
	case "/_ping": // just ping
		testlogF("mock docker ping response")
		header := http.Header{}
		header.Add("OSType", "Linux")
		header.Add("API-Version", APIVersion)
		header.Add("Docker-Experimental", "true")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
		}, nil
	case fmt.Sprintf("/containers/%s/json", containerID):
		testlogF("inspect container %s", containerID)
		b, _ = json.Marshal(types.ContainerJSON{
			ContainerJSONBase: &types.ContainerJSONBase{
				ID:    containerID,
				Image: "image:latest",
				Name:  "name",
			},
			Config: &container.Config{
				Labels: nil,
				Image:  "image:latest",
			},
		})
	case "/networks/bridge/disconnect":
		var disconnect types.NetworkDisconnect
		if err := json.NewDecoder(r.Body).Decode(&disconnect); err != nil {
			return errorMock(500, err.Error())
		}
		testlogF("disconnect container %s from bridge network", disconnect.Container)
		b = []byte("body")
	case "/networks":
		b, _ = json.Marshal([]types.NetworkResource{
			{
				Name:   "mock_network",
				Driver: "bridge",
			},
		})
	}

	if len(b) != 0 {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       ioutil.NopCloser(bytes.NewReader(b)),
		}, nil
	}

	errMsg := fmt.Sprintf("Server Error, unknown path: %s", path)
	return errorMock(500, errMsg)
}

func newMockClient(doer func(*http.Request) (*http.Response, error)) *http.Client {
	r := &http.Client{
		Transport: transportFunc(doer),
	}
	return r
}

func mockDockerHTTPClient() *http.Client {
	return newMockClient(mockDockerDoer)
}

func errorMock(statusCode int, message string) (*http.Response, error) {
	header := http.Header{}
	header.Set("Content-Type", "application/json")

	body, err := json.Marshal(&types.ErrorResponse{
		Message: message,
	})
	if err != nil {
		return nil, err
	}

	return &http.Response{
		StatusCode: statusCode,
		Body:       ioutil.NopCloser(bytes.NewReader(body)),
		Header:     header,
	}, nil
}

// transportFunc allows us to inject a mock transport for testing. We define it
// here so we can detect the tlsconfig and return nil for only this type.
type transportFunc func(*http.Request) (*http.Response, error)

func (tf transportFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return tf(req)
}

// func initMockConfig() {

// 	clnt, err := client.NewClient("http://127.0.0.1", "v1.29", mockDockerHTTPClient(), nil)
// 	if err != nil {
// 		panic(err)
// 	}

// }
