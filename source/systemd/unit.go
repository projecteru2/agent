package systemd

import (
	"regexp"

	"github.com/projecteru2/agent/source/meta"
)

const (
	unitPrefix  = "eru-"
	unitSuffix  = ".service"
	unitPattern = unitPrefix + "*" + unitSuffix
)

// workloadUnit excludes what else eru runs under the prefix, eru-agent.service and eru-core.service.
var workloadUnit = regexp.MustCompile("^" + unitPrefix + "(" + meta.IDPattern + ")" + regexp.QuoteMeta(unitSuffix) + "$")

func unitOf(ID string) string {
	return unitPrefix + ID + unitSuffix
}

func workloadIDFromUnit(name string) (string, bool) {
	match := workloadUnit.FindStringSubmatch(name)
	if match == nil {
		return "", false
	}
	return match[1], true
}
