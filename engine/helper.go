package engine

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/engine/status"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/core/cluster"
	coreutils "github.com/projecteru2/core/utils"

	enginetypes "github.com/docker/docker/api/types"
	enginecontainer "github.com/docker/docker/api/types/container"
	enginefilters "github.com/docker/docker/api/types/filters"
)

func useLabelAsFilter() bool {
	return os.Getenv("ERU_AGENT_EXPERIMENTAL_FILTER") == "label"
}

func (e *Engine) getFilter(extend map[string]string) enginefilters.Args {
	f := enginefilters.NewArgs()
	f.Add("label", fmt.Sprintf("%s=1", cluster.ERUMark))

	if e.config.CheckOnlyMine && useLabelAsFilter() {
		f.Add("label", fmt.Sprintf("eru.nodename=%s", e.config.HostName))
		if e.coreIdentifier != "" {
			f.Add("label", fmt.Sprintf("eru.coreid=%s", e.coreIdentifier))
		}
	}

	for k, v := range extend {
		f.Add(k, v)
	}
	return f
}

func (e *Engine) listContainers(all bool, extend map[string]string) ([]enginetypes.Container, error) {
	f := e.getFilter(extend)
	opts := enginetypes.ContainerListOptions{Filters: f, All: all}

	ctx, cancel := context.WithTimeout(context.Background(), e.config.GlobalConnectionTimeout)
	defer cancel()
	return e.docker.ContainerList(ctx, opts)
}

func (e *Engine) activated(f bool) error {
	e.node.Available = f
	return e.store.UpdateNode(e.node)
}

// check if ERU_NODE_NAME env in container is the hostname of this agent
// TODO should be removed in the future, should always use label to filter
func checkHostname(env []string, hostname string) bool {
	for _, e := range env {
		ps := strings.SplitN(e, "=", 2)
		if len(ps) != 2 {
			continue
		}
		if ps[0] == "ERU_NODE_NAME" && ps[1] == hostname {
			return true
		}
	}
	return false
}

func (e *Engine) detectContainer(id string) (*types.Container, error) {
	// 标准化为 inspect 的数据
	ctx, cancel := context.WithTimeout(context.Background(), e.config.GlobalConnectionTimeout)
	defer cancel()
	c, err := e.docker.ContainerInspect(ctx, id)
	if err != nil {
		return nil, err
	}
	label := c.Config.Labels

	if _, ok := label[cluster.ERUMark]; !ok {
		return nil, fmt.Errorf("not a eru container %s", coreutils.ShortID(id))
	}

	// TODO should be removed in the future
	if e.config.CheckOnlyMine && !useLabelAsFilter() && !checkHostname(c.Config.Env, e.config.HostName) {
		return nil, fmt.Errorf("should ignore this container")
	}

	// 生成基准 meta
	meta := coreutils.DecodeMetaInLabel(context.TODO(), label)

	// 是否符合 eru pattern，如果一个容器又有 ERUMark 又是三段式的 name，那它就是个 ERU 容器
	container, err := status.GenerateContainerMeta(c, meta, label)
	if err != nil {
		return container, err
	}
	// 计算容器用了多少 CPU
	container = status.CalcuateCPUNum(container, c, e.cpuCore)
	if container.Memory == 0 || container.Memory == math.MaxInt64 {
		container.Memory = e.memory
	}
	// 活着才有发布必要
	if c.NetworkSettings != nil && container.Running {
		networks := map[string]string{}
		for name, endpoint := range c.NetworkSettings.Networks {
			networkmode := enginecontainer.NetworkMode(name)
			if networkmode.IsHost() {
				container.LocalIP = common.LocalIP
				networks[name] = e.nodeIP
			} else {
				container.LocalIP = endpoint.IPAddress
				networks[name] = endpoint.IPAddress
			}
			break
		}
		container.Networks = networks
	}

	return container, nil
}

func getMaxAttemptsByTTL(ttl int64) int {
	if ttl <= 1 {
		return 1
	}
	maxAttempts := int(math.Floor(math.Log2((float64(ttl) - 1) / 2)))
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	return maxAttempts
}

// replaceNonUtf8 replaces non-utf8 characters in \x format.
func replaceNonUtf8(str string) string {
	if str == "" {
		return str
	}

	// deal with "legal" error rune in utf8
	if strings.ContainsRune(str, utf8.RuneError) {
		str = strings.ReplaceAll(str, string(utf8.RuneError), "\\xff\\xfd")
	}

	if utf8.ValidString(str) {
		return str
	}

	v := make([]rune, 0, len(str))
	for i, r := range str {
		switch {
		case r == utf8.RuneError:
			_, size := utf8.DecodeRuneInString(str[i:])
			if size > 0 {
				v = append(v, []rune(fmt.Sprintf("\\x%02x", str[i:i+size]))...)
			}
		case unicode.IsControl(r) && r != '\r' && r != '\n':
			v = append(v, []rune(fmt.Sprintf("\\x%02x", r))...)
		default:
			v = append(v, r)
		}
	}
	return string(v)
}
