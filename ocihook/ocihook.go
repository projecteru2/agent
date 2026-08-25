package ocihook

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	cni "github.com/containerd/go-cni"
	"github.com/containernetworking/cni/libcni"
	"github.com/urfave/cli/v3"

	"github.com/projecteru2/agent/common"
)

const (
	defaultConfDir = "/etc/cni/net.d"
	defaultBinDir  = "/opt/cni/bin"

	netnsPathFormat = "/proc/%d/ns/net"
	statusStopped   = "stopped"

	annotationNamespace = "eru.namespace"

	timeout = time.Minute
)

type options struct {
	network string
	confDir string
	binDir  string
	socket  string
}

// Command returns the cni hook mode core places in a container's spec, as createRuntime and poststop.
func Command() *cli.Command {
	return &cli.Command{
		Name:  "oci-hook",
		Usage: "attach or detach a container's cni network, as an oci createRuntime and poststop hook",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "network",
				Usage:    "name of the cni network to attach, as the conflist in --conf-dir names it",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "conf-dir",
				Usage: "cni configuration directory",
				Value: defaultConfDir,
			},
			&cli.StringFlag{
				Name:  "bin-dir",
				Usage: "cni plugin directory",
				Value: defaultBinDir,
			},
			&cli.StringFlag{
				Name:  "socket",
				Usage: "local containerd grpc socket, where the address is written back",
				Value: common.ContainerdSocket,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			// the hook runs inside runc's create, which waits for it however long an ipam plugin takes
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			return run(ctx, os.Stdin, options{
				network: cmd.String("network"),
				confDir: cmd.String("conf-dir"),
				binDir:  cmd.String("bin-dir"),
				socket:  cmd.String("socket"),
			})
		},
	}
}

func run(ctx context.Context, reader io.Reader, opts options) error {
	s, err := readState(reader)
	if err != nil {
		return err
	}
	network, err := newCNI(opts)
	if err != nil {
		return err
	}

	if !s.adding() {
		return network.Remove(ctx, s.ID, s.netns())
	}
	result, err := network.Setup(ctx, s.ID, s.netns())
	if err != nil {
		return err
	}
	return publish(ctx, s, opts, addressOf(result))
}

type state struct {
	ID          string            `json:"id"`
	Pid         int               `json:"pid"`
	Status      string            `json:"status"`
	Annotations map[string]string `json:"annotations"`
}

// adding reports whether the hook runs at createRuntime, where runc reports "creating" and a live pid.
func (s *state) adding() bool {
	return s.Pid > 0 && s.Status != statusStopped
}

// netns is empty once the process is gone, which is what cni expects of a delete.
func (s *state) netns() string {
	if s.Pid <= 0 {
		return ""
	}
	return fmt.Sprintf(netnsPathFormat, s.Pid)
}

func (s *state) namespace() string {
	return cmp.Or(s.Annotations[annotationNamespace], common.ContainerdNamespace)
}

func readState(reader io.Reader) (*state, error) {
	s := &state{}
	if err := json.NewDecoder(reader).Decode(s); err != nil {
		return nil, fmt.Errorf("read the oci state: %w", err)
	}
	if s.ID == "" {
		return nil, errors.New("the oci state carries no container id")
	}
	return s, nil
}

// newCNI loads the configuration that names itself network, so the label the agent reads back matches the flag.
func newCNI(opts options) (cni.CNI, error) {
	conf, err := libcni.LoadNetworkConf(opts.confDir, opts.network)
	if err != nil {
		return nil, fmt.Errorf("load the cni network %s from %s: %w", opts.network, opts.confDir, err)
	}
	return cni.New(
		cni.WithPluginConfDir(opts.confDir),
		cni.WithPluginDir([]string{opts.binDir}),
		cni.WithConfListBytes(conf.Bytes),
	)
}

// addressOf prefers the ipv4 core publishes, and falls back to ipv6 on a single stack network.
func addressOf(result *cni.Result) string {
	fallback := ""
	for _, name := range slices.Sorted(maps.Keys(result.Interfaces)) {
		for _, config := range result.Interfaces[name].IPConfigs {
			if ipv4 := config.IP.To4(); ipv4 != nil {
				return ipv4.String()
			}
			if fallback == "" && config.IP != nil {
				fallback = config.IP.String()
			}
		}
	}
	return fallback
}

// publish writes the address back as a container label, which is where core and the agent read it.
func publish(ctx context.Context, s *state, opts options, address string) error {
	if address == "" {
		return fmt.Errorf("cni gave %s no address on %s", s.ID, opts.network)
	}

	client, err := containerd.New(opts.socket, containerd.WithDefaultNamespace(s.namespace()))
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	container, err := client.LoadContainer(ctx, s.ID)
	if err != nil {
		return err
	}
	_, err = container.SetLabels(ctx, map[string]string{common.NetworkLabelPrefix + opts.network: address})
	return err
}
