package engine

import (
	"net/http"
	"os"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stringid"
	"github.com/stretchr/testify/assert"
)

func TestCheckAllContainers(t *testing.T) {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)

	e := mockNewEngine()
	e.checkAllContainers()

	time.Sleep(1 * time.Second)
}

func TestCheckMethodTCP(t *testing.T) {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)

	// checkTCP(container enginetypes.ContainerJSON, timeout time.Duration) bool
	container := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID: stringid.GenerateRandomID(),
		},
		Config: &container.Config{
			Labels: map[string]string{
				"ports": "10234/tcp",
			},
		},
		NetworkSettings: &types.NetworkSettings{
			Networks: map[string]*network.EndpointSettings{
				"x": &network.EndpointSettings{
					IPAddress: "192.168.233.233",
				},
			},
		},
	}

	assert.False(t, checkTCP(container, 2*time.Second))
}

func TestCheckMethodHTTP(t *testing.T) {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)

	// server
	go http.ListenAndServe(":10234", http.NotFoundHandler())
	time.Sleep(100 * time.Millisecond)
	// checkHTTP(container enginetypes.ContainerJSON, timeout time.Duration) bool
	container := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID: stringid.GenerateRandomID(),
		},
		Config: &container.Config{
			Labels: map[string]string{
				"ports":                     "10234/http",
				"healthcheck_expected_code": "404",
			},
		},
		NetworkSettings: &types.NetworkSettings{
			Networks: map[string]*network.EndpointSettings{
				"x": &network.EndpointSettings{
					IPAddress: "127.0.0.1",
				},
			},
		},
	}

	assert.True(t, checkHTTP(container, 5*time.Second))
}
