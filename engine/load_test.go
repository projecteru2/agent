package engine

import (
	"os"
	"testing"
	"time"

	coretypes "github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestLoad(t *testing.T) {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)

	e := mockNewEngine()

	n := new(coretypes.Node)
	mockStore.On("GetNode", mock.AnythingOfType("string")).Return(n, nil)
	mockStore.On("UpdateNode", mock.Anything).Return(nil)
	mockStore.On("SetContainerStatus", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	err := e.load()
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)
}
