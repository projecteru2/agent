package status

import (
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
)

func GenerateContainerMeta(ID, cname, version string, attrs map[string]string) (*types.Container, error) {
	name, entrypoint, ident, err := utils.GetAppInfo(cname)
	if err != nil {
		return nil, err
	}

	container := &types.Container{}
	container.ID = ID
	container.Name = name
	container.EntryPoint = entrypoint
	container.Ident = ident
	container.Version = version
	container.Extend = attrs
	return container, nil
}
