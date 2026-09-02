package containerd

import (
	"context"
	"fmt"
	"strings"

	tasks "github.com/containerd/containerd/api/services/tasks/v1"
	tasktypes "github.com/containerd/containerd/api/types/task"
	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/errdefs"
	"github.com/containerd/errdefs/pkg/errgrpc"
	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/log"

	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/source"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/agent/utils"
)

var _ source.Source = (*Containerd)(nil)

// Containerd is the view of the workloads one local containerd daemon runs in one namespace.
type Containerd struct {
	client *containerd.Client
	config *types.Config

	nodeIP string
	filter string
}

func New(ctx context.Context, config *types.Config, nodeIP, storeIdentifier string) (*Containerd, error) {
	runtime := config.Runtimes.Containerd
	logger := log.WithFunc("containerd.New").WithField("socket", runtime.Socket)
	logger.Infof(ctx, "containerd source starting in namespace %s", runtime.Namespace)

	client, err := containerd.New(runtime.Socket, containerd.WithDefaultNamespace(runtime.Namespace))
	if err != nil {
		logger.Error(ctx, err, "failed to dial containerd")
		return nil, err
	}
	return &Containerd{client: client, config: config, nodeIP: nodeIP, filter: newFilter(config, storeIdentifier)}, nil
}

func (c *Containerd) List(ctx context.Context) ([]*source.Workload, error) {
	logger := log.WithFunc("containerd.List")

	var listed []containerd.Container
	var processes map[string]*tasktypes.Process
	var err error
	utils.WithTimeout(ctx, c.config.GlobalConnectionTimeout, func(ctx context.Context) {
		if listed, err = c.client.Containers(ctx, c.filter); err != nil {
			return
		}
		processes, err = c.tasks(ctx)
	})
	if err != nil {
		logger.Error(ctx, err, "failed to list workloads")
		return nil, err
	}

	workloads := make([]*source.Workload, 0, len(listed))
	for _, container := range listed {
		w, err := c.inspect(ctx, container, processes[container.ID()])
		if err != nil {
			logger.WithField("ID", container.ID()).Debugf(ctx, "skipping workload: %v", err)
			continue
		}
		workloads = append(workloads, w)
	}
	return workloads, nil
}

func (c *Containerd) Get(ctx context.Context, ID string) (*source.Workload, error) {
	var w *source.Workload
	var err error
	utils.WithTimeout(ctx, c.config.GlobalConnectionTimeout, func(ctx context.Context) {
		var container containerd.Container
		if container, err = c.client.LoadContainer(ctx, ID); err != nil {
			return
		}
		var process *tasktypes.Process
		if process, err = c.task(ctx, ID); err != nil && !errdefs.IsNotFound(err) {
			return
		}
		w, err = c.inspect(ctx, container, process)
	})
	return w, err
}

func (c *Containerd) Events(ctx context.Context) (<-chan *types.WorkloadEventMessage, <-chan error) {
	envelopes, errs := c.client.Subscribe(ctx, eventFilters(c.config.Runtimes.Containerd.Namespace)...)
	return relay(ctx, envelopes, errs)
}

func (c *Containerd) Alive(ctx context.Context) bool {
	var serving bool
	var err error
	utils.WithTimeout(ctx, c.config.GlobalConnectionTimeout, func(ctx context.Context) {
		serving, err = c.client.IsServing(ctx)
	})
	if err != nil {
		log.WithFunc("containerd.Alive").Error(ctx, err, "connect to containerd failed")
		return false
	}
	return serving
}

func (c *Containerd) inspect(ctx context.Context, container containerd.Container, process *tasktypes.Process) (*source.Workload, error) {
	info, err := container.Info(ctx, containerd.WithoutRefreshedMetadata)
	if err != nil {
		return nil, err
	}
	if _, ok := info.Labels[cluster.ERUMark]; !ok {
		return nil, common.ErrInvalidContainer
	}
	if c.config.CheckOnlyMine && !utils.UseLabelAsFilter() && info.Labels[cluster.LabelNodeName] != c.config.HostName {
		return nil, common.ErrInvalidContainer
	}

	s, err := readSpec(info.Spec)
	if err != nil {
		return nil, err
	}
	w, err := c.workload(ctx, info.ID, info.Labels, s)
	if err != nil {
		return nil, err
	}

	if process == nil {
		return w, nil
	}
	w.Running = process.Status == tasktypes.Status_RUNNING
	if !w.Running {
		return w, nil
	}
	w.NetnsPID = int(process.Pid)
	w.CgroupPath = cgroupPath(ctx, w.NetnsPID)
	return w, nil
}

func (c *Containerd) task(ctx context.Context, ID string) (*tasktypes.Process, error) {
	resp, err := c.client.TaskService().Get(ctx, &tasks.GetRequest{ContainerID: ID})
	if err != nil {
		return nil, errgrpc.ToNative(err)
	}
	return resp.Process, nil
}

func (c *Containerd) tasks(ctx context.Context) (map[string]*tasktypes.Process, error) {
	resp, err := c.client.TaskService().List(ctx, &tasks.ListTasksRequest{})
	if err != nil {
		return nil, errgrpc.ToNative(err)
	}
	processes := make(map[string]*tasktypes.Process, len(resp.Tasks))
	for _, process := range resp.Tasks {
		processes[process.ContainerID] = process
	}
	return processes, nil
}

// newFilter builds one containerd filter term, whose comma separated conditions are anded.
func newFilter(config *types.Config, storeIdentifier string) string {
	conditions := []string{fmt.Sprintf("labels.%q==%q", cluster.ERUMark, "1")}
	if config.CheckOnlyMine && utils.UseLabelAsFilter() {
		conditions = append(conditions, fmt.Sprintf("labels.%q==%q", cluster.LabelNodeName, config.HostName))
		if storeIdentifier != "" {
			conditions = append(conditions, fmt.Sprintf("labels.%q==%q", cluster.LabelCoreID, storeIdentifier))
		}
	}
	return strings.Join(conditions, ",")
}

func cgroupPath(ctx context.Context, pid int) string {
	path, err := utils.CgroupPath(utils.CgroupRoot, utils.ProcRoot(), pid)
	if err != nil {
		log.WithFunc("containerd.cgroupPath").Warnf(ctx, "no cgroup v2 path for pid %d, this node reports no metrics: %v", pid, err)
		return ""
	}
	return path
}
