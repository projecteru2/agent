package collector

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/log"
	coreutils "github.com/projecteru2/core/utils"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/projecteru2/agent/source"
)

const (
	labelNIC = "nic"
	labelDev = "dev"

	metricMemMaxUsage = "mem_max_usage"
)

var (
	clientsMutex sync.Mutex
	clients      = map[string]*MetricsClient{}

	metricSpecs = []metricSpec{
		{"cpu_host_usage", "cpu usage in host view.", "", "cpu_host_usage"},
		{"cpu_host_sys_usage", "cpu sys usage in host view.", "", "cpu_host_sys_usage"},
		{"cpu_host_user_usage", "cpu user usage in host view.", "", "cpu_host_user_usage"},
		{"cpu_container_usage", "cpu usage in container view.", "", "cpu_container_usage"},
		{"cpu_container_sys_usage", "cpu sys usage in container view.", "", "cpu_container_sys_usage"},
		{"cpu_container_user_usage", "cpu user usage in container view.", "", "cpu_container_user_usage"},
		{"mem_usage", "memory usage.", "", "mem_usage"},
		{metricMemMaxUsage, "memory max usage.", "", metricMemMaxUsage},
		{"mem_rss", "memory rss.", "", "mem_rss"},
		{"mem_percent", "memory percent.", "", "mem_percent"},
		{"mem_rss_percent", "memory rss percent.", "", "mem_rss_percent"},

		{"bytes_send", "bytes send.", labelNIC, "bytes.sent"},
		{"bytes_recv", "bytes recv.", labelNIC, "bytes.recv"},
		{"packets_send", "packets send.", labelNIC, "packets.sent"},
		{"packets_recv", "packets recv.", labelNIC, "packets.recv"},
		{"err_in", "err in.", labelNIC, "err.in"},
		{"err_out", "err out.", labelNIC, "err.out"},
		{"drop_in", "drop in.", labelNIC, "drop.in"},
		{"drop_out", "drop out.", labelNIC, "drop.out"},

		{"io_service_bytes_read", "number of bytes read to the disk by the group.", labelDev, "io_service_bytes_read"},
		{"io_service_bytes_write", "number of bytes write to the disk by the group.", labelDev, "io_service_bytes_write"},
		{"io_serviced_read", "number of read IOs to the disk by the group.", labelDev, "io_serviced_read"},
		{"io_serviced_write", "number of write IOs to the disk by the group.", labelDev, "io_serviced_write"},
		{"io_service_bytes_read_per_second", "number of bytes read per second to the disk by the group.", labelDev, "io_service_bytes_read_per_second"},
		{"io_service_bytes_write_per_second", "number of bytes write per second to the disk by the group.", labelDev, "io_service_bytes_write_per_second"},
		{"io_serviced_read_per_second", "number of read IOs per second to the disk by the group.", labelDev, "io_serviced_read_per_second"},
		{"io_serviced_write_per_second", "number of write IOs per second to the disk by the group.", labelDev, "io_serviced_write_per_second"},
	}
)

type metricSpec struct {
	name   string
	help   string
	label  string
	statsd string
}

type gauge struct {
	plain  prometheus.Gauge
	vector *prometheus.GaugeVec
	statsd string
}

type MetricsClient struct {
	statsd       string
	statsdClient *coreutils.Statsd
	prefix       string
	data         map[string]float64

	gauges     map[string]gauge
	collectors []prometheus.Collector
}

func NewMetricsClient(statsd, hostname string, w *source.Workload, unsupported []string) *MetricsClient {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	if metricsClient, ok := clients[w.ID]; ok {
		return metricsClient
	}

	labelPairs := make([]string, 0, len(w.Meta.Labels))
	for k, v := range w.Meta.Labels {
		if strings.HasPrefix(k, cluster.ERUMark) || strings.HasPrefix(k, "eru.") {
			continue
		}
		labelPairs = append(labelPairs, fmt.Sprintf("%s=%s", k, v))
	}
	labels := map[string]string{
		"containerID":  w.ID,
		"hostname":     hostname,
		"appname":      w.Meta.Appname,
		"entrypoint":   w.Meta.Entrypoint,
		"orchestrator": cluster.ERUMark,
		"labels":       strings.Join(slices.Sorted(slices.Values(labelPairs)), ","),
	}

	tag := fmt.Sprintf("%s.%s", hostname, coreutils.ShortID(w.ID))
	endpoint := fmt.Sprintf("%s.%s", coreutils.CleanStatsdMetrics(w.Meta.Appname), coreutils.CleanStatsdMetrics(w.Meta.Entrypoint))

	metricsClient := &MetricsClient{
		statsd: statsd,
		prefix: fmt.Sprintf("%s.%s.%s", cluster.ERUMark, endpoint, tag),
		data:   map[string]float64{},
		gauges: make(map[string]gauge, len(metricSpecs)),
	}
	for _, spec := range metricSpecs {
		if slices.Contains(unsupported, spec.name) {
			continue
		}
		opts := prometheus.GaugeOpts{Name: spec.name, Help: spec.help, ConstLabels: labels}
		g := gauge{statsd: spec.statsd}
		if spec.label == "" {
			g.plain = prometheus.NewGauge(opts)
			metricsClient.collectors = append(metricsClient.collectors, g.plain)
		} else {
			g.vector = prometheus.NewGaugeVec(opts, []string{spec.label})
			metricsClient.collectors = append(metricsClient.collectors, g.vector)
		}
		metricsClient.gauges[spec.name] = g
	}
	prometheus.MustRegister(metricsClient.collectors...)

	clients[w.ID] = metricsClient
	return metricsClient
}

func (m *MetricsClient) Unregister() {
	for _, collector := range m.collectors {
		prometheus.Unregister(collector)
	}
}

func (m *MetricsClient) CPUHostUsage(i float64) { m.set("cpu_host_usage", i) }

func (m *MetricsClient) CPUHostSysUsage(i float64) { m.set("cpu_host_sys_usage", i) }

func (m *MetricsClient) CPUHostUserUsage(i float64) { m.set("cpu_host_user_usage", i) }

func (m *MetricsClient) CPUContainerUsage(i float64) { m.set("cpu_container_usage", i) }

func (m *MetricsClient) CPUContainerSysUsage(i float64) { m.set("cpu_container_sys_usage", i) }

func (m *MetricsClient) CPUContainerUserUsage(i float64) { m.set("cpu_container_user_usage", i) }

func (m *MetricsClient) MemUsage(i float64) { m.set("mem_usage", i) }

func (m *MetricsClient) MemMaxUsage(i float64) { m.set(metricMemMaxUsage, i) }

func (m *MetricsClient) MemRss(i float64) { m.set("mem_rss", i) }

func (m *MetricsClient) MemPercent(i float64) { m.set("mem_percent", i) }

func (m *MetricsClient) MemRSSPercent(i float64) { m.set("mem_rss_percent", i) }

func (m *MetricsClient) BytesSent(nic string, i float64) { m.setVec("bytes_send", nic, i) }

func (m *MetricsClient) BytesRecv(nic string, i float64) { m.setVec("bytes_recv", nic, i) }

func (m *MetricsClient) PacketsSent(nic string, i float64) { m.setVec("packets_send", nic, i) }

func (m *MetricsClient) PacketsRecv(nic string, i float64) { m.setVec("packets_recv", nic, i) }

func (m *MetricsClient) ErrIn(nic string, i float64) { m.setVec("err_in", nic, i) }

func (m *MetricsClient) ErrOut(nic string, i float64) { m.setVec("err_out", nic, i) }

func (m *MetricsClient) DropIn(nic string, i float64) { m.setVec("drop_in", nic, i) }

func (m *MetricsClient) DropOut(nic string, i float64) { m.setVec("drop_out", nic, i) }

func (m *MetricsClient) IOServiceBytesRead(dev string, i float64) {
	m.setVec("io_service_bytes_read", dev, i)
}

func (m *MetricsClient) IOServiceBytesWrite(dev string, i float64) {
	m.setVec("io_service_bytes_write", dev, i)
}

func (m *MetricsClient) IOServicedRead(dev string, i float64) { m.setVec("io_serviced_read", dev, i) }

func (m *MetricsClient) IOServicedWrite(dev string, i float64) {
	m.setVec("io_serviced_write", dev, i)
}

func (m *MetricsClient) IOServiceBytesReadPerSecond(dev string, i float64) {
	m.setVec("io_service_bytes_read_per_second", dev, i)
}

func (m *MetricsClient) IOServiceBytesWritePerSecond(dev string, i float64) {
	m.setVec("io_service_bytes_write_per_second", dev, i)
}

func (m *MetricsClient) IOServicedReadPerSecond(dev string, i float64) {
	m.setVec("io_serviced_read_per_second", dev, i)
}

func (m *MetricsClient) IOServicedWritePerSecond(dev string, i float64) {
	m.setVec("io_serviced_write_per_second", dev, i)
}

func (m *MetricsClient) Send(ctx context.Context) error {
	if m.statsd == "" {
		return nil
	}
	if err := m.checkConn(ctx); err != nil {
		return err
	}
	for k, v := range m.data {
		m.statsdClient.Gauge(m.prefix+"."+k, v)
		delete(m.data, k)
	}
	return nil
}

func (m *MetricsClient) set(name string, value float64) {
	g, ok := m.gauges[name]
	if !ok {
		return
	}
	if m.statsd != "" {
		m.data[g.statsd] = value
	}
	g.plain.Set(value)
}

func (m *MetricsClient) setVec(name, label string, value float64) {
	g, ok := m.gauges[name]
	if !ok {
		return
	}
	if m.statsd != "" {
		m.data[coreutils.CleanStatsdMetrics(label)+"."+g.statsd] = value
	}
	g.vector.WithLabelValues(label).Set(value)
}

func (m *MetricsClient) checkConn(ctx context.Context) error {
	if m.statsdClient != nil {
		return nil
	}
	// statsd speaks UDP only, so a client never has to be renewed
	var err error
	if m.statsdClient, err = coreutils.NewStatsd(m.statsd); err != nil {
		log.WithFunc("collector.checkConn").Error(ctx, err, "failed to connect statsd")
		return err
	}
	return nil
}

func removeMetricsClient(ID string) {
	clientsMutex.Lock()
	defer clientsMutex.Unlock()
	if metricsClient, ok := clients[ID]; ok {
		metricsClient.Unregister()
		if metricsClient.statsdClient != nil {
			_ = metricsClient.statsdClient.Close()
		}
		delete(clients, ID)
	}
}
