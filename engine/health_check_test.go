package engine

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/pkg/stringid"
	"github.com/projecteru2/agent/store/mocks"
	"github.com/projecteru2/agent/types"
	coretypes "github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCheckSingleContainerHealthy(t *testing.T) {
	go http.ListenAndServe(":10236", http.NotFoundHandler())
	time.Sleep(100 * time.Millisecond)
	go http.ListenAndServe(":10237", http.NotFoundHandler())
	time.Sleep(100 * time.Millisecond)
	container := &types.Container{
		StatusMeta: coretypes.StatusMeta{
			ID:      stringid.GenerateRandomID(),
			Running: true,
		},
		Pid:        12349,
		Name:       "test",
		EntryPoint: "t1",
		HealthCheck: &coretypes.HealthCheck{
			TCPPorts: []string{"10236"},
			HTTPPort: "10237",
			HTTPURL:  "/",
			HTTPCode: 404,
		},
	}
	state := checkSingleContainerHealthy(container, 3*time.Second)
	assert.True(t, state)
}

func TestCheckAllContainers(t *testing.T) {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)

	e := mockNewEngine()
	mockStore := e.store.(*mocks.Store)
	mockStore.On("SetContainerStatus", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	e.checkAllContainers(context.TODO())

	time.Sleep(1 * time.Second)
}

func TestCheckMethodTCP(t *testing.T) {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)

	assert.False(t, checkTCP(stringid.GenerateRandomID(), []string{"192.168.233.233:10234"}, 2*time.Second))
	go http.ListenAndServe(":10235", http.NotFoundHandler())
	time.Sleep(100 * time.Millisecond)
	assert.True(t, checkTCP(stringid.GenerateRandomID(), []string{"127.0.0.1:10235"}, 2*time.Second))
}

func TestCheckMethodHTTP(t *testing.T) {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)

	// server
	go http.ListenAndServe(":10234", http.NotFoundHandler())
	time.Sleep(100 * time.Millisecond)
	assert.True(t, checkHTTP(stringid.GenerateRandomID(), []string{"http://127.0.0.1:10234/"}, 404, 5*time.Second))
}
