package systemd

import (
	"strings"

	"github.com/projecteru2/agent/source/meta"
)

const (
	unitPrefix  = "eru-"
	unitSuffix  = ".service"
	unitPattern = unitPrefix + "*" + unitSuffix
)

func unitOf(ID string) string {
	return unitPrefix + ID + unitSuffix
}

// workloadIDFromUnit rejects what else eru runs under the prefix, eru-agent.service and eru-core.service.
func workloadIDFromUnit(name string) (string, bool) {
	ID, ok := strings.CutPrefix(name, unitPrefix)
	if !ok {
		return "", false
	}
	ID, ok = strings.CutSuffix(ID, unitSuffix)
	if !ok || !meta.IsID(ID) {
		return "", false
	}
	return ID, true
}
