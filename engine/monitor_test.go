package engine

import (
	"io"
	"os"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
	// "github.com/docker/docker/api/types/network"

	coretypes "github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestInitMonitor(t *testing.T) {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)

	e := mockNewEngine()
	eventChan, errChan := e.initMonitor()

	go func() {
		for {
			select {
			case err := <-errChan:
				assert.Equal(t, err, io.ErrClosedPipe)
				return
			case event := <-eventChan:
				testlogF("ID: %s, Action: %s, Status: %s", event.ID, event.Action, event.Status)
			}
		}
	}()

	time.Sleep(3 * time.Second)
}

func TestMonitor(t *testing.T) {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)

	e := mockNewEngine()
	eventChan, _ := e.initMonitor()

	n := new(coretypes.Node)
	mockStore.On("GetNode", mock.AnythingOfType("string")).Return(n, nil)
	mockStore.On("UpdateNode", mock.Anything).Return(nil)
	mockStore.On("DeployContainer", mock.Anything, mock.Anything).Return(nil)

	go e.monitor(eventChan)
	time.Sleep(3 * time.Second)
}
