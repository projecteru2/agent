package status

import (
	"gitlab.ricebook.net/platform/agent/types"
	"gitlab.ricebook.net/platform/agent/utils"
)

func GenerateContainerMeta(ID string, attrs map[string]string) (*types.Container, error) {
	name, entrypoint, ident, err := utils.GetAppInfo(attrs["name"])
	if err != nil {
		return nil, err
	}
	delete(attrs, "name")

	container := &types.Container{}
	container.ID = ID
	container.Name = name
	container.EntryPoint = entrypoint
	container.Ident = ident
	if v, ok := attrs["version"]; ok {
		container.Version = v
		delete(attrs, "version")
	} else {
		container.Version = "UNKNOWN"
	}

	container.Extend = attrs

	return container, nil
}
