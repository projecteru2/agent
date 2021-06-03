package engine

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strings"

	enginetypes "github.com/docker/docker/api/types"
	enginecontainer "github.com/docker/docker/api/types/container"
	enginefilters "github.com/docker/docker/api/types/filters"
	"github.com/pkg/errors"
	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/engine/status"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/core/cluster"
	coreutils "github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

var (
	ipv4Pattern *regexp.Regexp
)

func init() {
	ipv4Pattern = regexp.MustCompile(`\d+\.\d+\.\d+\.\d+`)
}

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

	if useCNI(label) {
		cniIPv4, err := fetchCNIIPv4(c.State.Pid)
		if err == nil && cniIPv4 != "" {
			container.Networks = map[string]string{
				"cni": cniIPv4,
			}
		}
	}

	return container, nil
}

func useCNI(label map[string]string) bool {
	for k, v := range label {
		if k == "cni" && v == "1" {
			return true
		}
	}
	return false
}

func fetchCNIIPv4(pid int) (ipv4 string, err error) {
	if pid == 0 {
		return
	}

	args := []string{"ip", "-4", "a", "sh", "eth0"}
	var out []byte
	if err = WithNetns(fmt.Sprintf("/proc/%d/ns/net", pid), func() error {
		out, err = exec.Command(args[0], args[1:]...).Output()
		return errors.WithStack(err)
	}); err != nil {
		return
	}
	return string(ipv4Pattern.Find(out)), nil
}

func WithNetns(netnsPath string, f func() error) (err error) {
	file, err := os.Open(netnsPath)
	if err != nil {
		return errors.WithStack(err)
	}

	origin, err := os.Open("/proc/self/ns/net")
	if err != nil {
		return errors.WithStack(err)
	}

	if err = unix.Setns(int(file.Fd()), unix.CLONE_NEWNET); err != nil {
		return errors.WithStack(err)
	}
	defer func() {
		if e := unix.Setns(int(origin.Fd()), unix.CLONE_NEWNET); e != nil {
			log.Errorf("failed to recover netns: %+v", e)
		}
	}()
	return f()
}
