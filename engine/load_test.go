package engine

import (
	"os"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/stringid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gitlab.ricebook.net/platform/agent/types"
	atypes "gitlab.ricebook.net/platform/agent/types"
)

func TestBind(t *testing.T) {
	e := mockNewEngine()
	mockStore.On("UpdateContainer", mock.Anything).Return(nil)

	container := &types.Container{
		ID: stringid.GenerateRandomID(),
	}

	err := e.bind(container, true)
	assert.NoError(t, err)
}

func TestLoad(t *testing.T) {
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)

	e := mockNewEngine()

	c := new(atypes.Container)
	c.ID = stringid.GenerateRandomID()
	mockStore.On("GetContainer", mock.AnythingOfType("string")).Return(c, nil)
	mockStore.On("UpdateContainer", mock.Anything).Return(nil)
	mockStore.On("RemoveContainer", mock.Anything).Return(nil)
	mockStore.On("GetAllContainers").Return([]string{c.ID}, nil)

	err := e.load()
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)
}
