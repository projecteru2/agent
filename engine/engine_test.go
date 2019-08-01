package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/stringid"
	"github.com/projecteru2/agent/store/mocks"
	agenttypes "github.com/projecteru2/agent/types"
	agentutils "github.com/projecteru2/agent/utils"
	"github.com/stretchr/testify/assert"
)

const (
	apiVersion = "v1.25"
	mockID     = "f1f9da344e8f8f90f73899ddad02da6cdf2218bbe52413af2bcfef4fba2d22de"
)

var (
	i         int
	err       error
	mockStore *mocks.Store
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

func mockEvents(ctx context.Context) (*http.Response, error) {
	pr, pw := io.Pipe()
	w := ioutils.NewWriteFlusher(pw)
	msgChan := make(chan []byte)

	filters := filters.NewArgs()
	filters.Add("type", events.ContainerEventType)

	eventsCases := struct {
		options types.EventsOptions
		events  []events.Message
	}{
		options: types.EventsOptions{
			Filters: filters,
		},
		events: []events.Message{
			{
				Type:   "container",
				ID:     stringid.GenerateRandomID(),
				Action: "create",
				Status: "state create",
			},
			{
				Type:   "container",
				ID:     stringid.GenerateRandomID(),
				Action: "die",
				Status: "state die",
			},
			{
				Type:   "container",
				ID:     stringid.GenerateRandomID(),
				Action: "destroy",
				Status: "state destroy",
			},
		},
	}
	go func() {
		for _, e := range eventsCases.events {
			b, _ := json.Marshal(e)
			msgChan <- b
			time.Sleep(1000 * time.Millisecond)
		}

	}()
	go func() {
		for {
			select {
			case <-ctx.Done():
				testlogF("Context canceld")
				w.Close()
				pw.Close()
				pr.Close()
				return
			case msg := <-msgChan:
				w.Write(msg)
			}
		}
	}()

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       pr,
	}, nil
}

func mockPing() (*http.Response, error) {
	header := http.Header{}
	header.Add("OSType", "Linux")
	header.Add("API-Version", apiVersion)
	header.Add("Docker-Experimental", "true")
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     header,
	}, nil
}

func mockDockerDoer(r *http.Request) (*http.Response, error) {
	var b []byte
	prefix := fmt.Sprintf("/%s", apiVersion)
	path := strings.TrimPrefix(r.URL.Path, prefix)

	// get container id
	containerID := ""
	if strings.HasPrefix(path, "/containers/") {
		cid := strings.TrimPrefix(path, "/containers/")
		containerID = strings.Split(cid, "/")[0]
		if containerID == "" {
			containerID = "_"
		}
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
		return mockPing()
	case fmt.Sprintf("/containers/%s/json", containerID):
		testlogF("inspect container %s", containerID)
		b, _ = json.Marshal(types.ContainerJSON{
			ContainerJSONBase: &types.ContainerJSONBase{
				ID:    containerID,
				Image: "image:latest",
				Name:  "name_entry_ident",
				State: &types.ContainerState{
					Running: true,
				},
				HostConfig: &container.HostConfig{
					Resources: container.Resources{
						CPUQuota:  9999,
						CPUPeriod: 9999,
						Memory:    99999,
					},
				},
			},
			Config: &container.Config{
				Labels: map[string]string{
					"ERU":              "1",
					"healthcheck":      "1",
					"healthcheck_http": "80",
					"healthcheck_code": "404",
					"healthcheck_url":  "/",
				},
				Image: "image:latest",
			},
		})
	case "/networks/bridge/disconnect":
		var disconnect types.NetworkDisconnect
		json.NewDecoder(r.Body).Decode(&disconnect)
		testlogF("disconnect container %s from bridge network", disconnect.Container)
		b = []byte("body")
	case "/networks":
		b, _ = json.Marshal([]types.NetworkResource{
			{
				Name:   "mock_network",
				Driver: "bridge",
			},
		})
	case "/events":
		testlogF("mock docker events")
		return mockEvents(r.Context())
	case "/containers/json":
		testlogF("mock docker ps")
		b, _ = json.Marshal([]types.Container{
			{
				ID:      stringid.GenerateRandomID(),
				Names:   []string{"hello_docker_ident"},
				Image:   "test:image",
				ImageID: stringid.GenerateNonCryptoID(),
				Command: "top",
				Labels:  map[string]string{"ERU": "1"},
			},
		})
	default:
		errMsg := fmt.Sprintf("Server Error, unknown path: %s", path)
		return errorMock(500, errMsg)
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       ioutil.NopCloser(bytes.NewReader(b)),
	}, nil
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
	}, fmt.Errorf(message)
}

// transportFunc allows us to inject a mock transport for testing. We define it
// here so we can detect the tlsconfig and return nil for only this type.
type transportFunc func(*http.Request) (*http.Response, error)

func (tf transportFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return tf(req)
}

func mockNewEngine() *Engine {
	engine := new(Engine)
	mockStore = new(mocks.Store)

	docker, err := client.NewClient("http://127.0.0.1", "1.25", mockDockerHTTPClient(), nil)
	if err != nil {
		panic(err)
	}

	engine.config = &agenttypes.Config{}
	engine.checker = agenttypes.NewPrevCheck(engine.config)
	engine.store = mockStore
	engine.docker = docker
	engine.cpuCore = float64(runtime.NumCPU())
	engine.transfers = agentutils.NewHashBackends([]string{"127.0.0.1:8125"})
	engine.forwards = agentutils.NewHashBackends([]string{"udp://127.0.0.1:5144"})

	return engine
}

func TestPing(t *testing.T) {
	e := mockNewEngine()
	_, err := e.docker.Ping(context.Background())
	assert.NoError(t, err)
}

func TestEvents(t *testing.T) {
	docker, err := client.NewClient("http://10.0.0.1", "1.25", mockDockerHTTPClient(), nil)
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	eventChan, errChan := docker.Events(ctx, types.EventsOptions{})

	for {
		select {
		case err := <-errChan:
			assert.Equal(t, err, io.ErrClosedPipe)
			return
		case event := <-eventChan:
			testlogF("ID: %s, Action: %s, Status: %s", event.ID, event.Action, event.Status)
		}
	}
}
