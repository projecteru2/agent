package docker

import (
	"testing"

	coretypes "github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func TestSetMetricsContainerRefreshesTheCachedMeta(t *testing.T) {
	container := containerMeta("refreshes-the-cached-meta", 1)
	mClient := NewMetricsClient("", "node", container)
	defer removeMetricsClient(container.ID)
	assert.Equal(t, 1.0, mClient.container.Load().CPUNum)

	setMetricsContainer(container.ID, containerMeta(container.ID, 4))
	assert.Equal(t, 4.0, mClient.container.Load().CPUNum)
}

func TestSetMetricsContainerIgnoresAWorkloadWithoutAClient(t *testing.T) {
	setMetricsContainer("without-a-client", containerMeta("without-a-client", 4))
	assert.NotContains(t, clients, "without-a-client")
}

func TestNewMetricsClientRefreshesTheMetaOfAReusedClient(t *testing.T) {
	container := containerMeta("refreshes-a-reused-client", 1)
	mClient := NewMetricsClient("", "node", container)
	defer removeMetricsClient(container.ID)

	reused := NewMetricsClient("", "node", containerMeta(container.ID, 4))
	assert.Same(t, mClient, reused)
	assert.Equal(t, 4.0, mClient.container.Load().CPUNum)
}

func containerMeta(ID string, cpuNum float64) *Container {
	return &Container{
		StatusMeta: coretypes.StatusMeta{ID: ID},
		Name:       "app",
		EntryPoint: "web",
		CPUNum:     cpuNum,
	}
}
