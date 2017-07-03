package status

import (
	"gitlab.ricebook.net/platform/agent/types"
	"gitlab.ricebook.net/platform/agent/utils"
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
