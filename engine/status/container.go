package status

import (
	log "github.com/Sirupsen/logrus"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
	coretypes "github.com/projecteru2/core/types"
)

func GenerateContainerMeta(c *coretypes.Container, version string, attrs map[string]string) (*types.Container, error) {
	name, entrypoint, ident, err := utils.GetAppInfo(c.Name)
	if err != nil {
		return nil, err
	}

	container := &types.Container{}
	container.ID = c.ID
	container.Name = name
	container.EntryPoint = entrypoint
	container.Ident = ident
	container.Version = version
	container.Extend = attrs
	log.Debugf("Generate container meta %v", container)
	return container, nil
}
