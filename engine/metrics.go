package engine

import (
	"fmt"
	"strings"

	statsdlib "github.com/CMGS/statsd"
	"github.com/projecteru2/agent/common"
	"github.com/projecteru2/agent/types"
	"github.com/projecteru2/core/cluster"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

// MetricsClient combine statsd and prometheus
type MetricsClient struct {
	statsd string
	prefix string
	data   map[string]float64

	cpuHostUsage     prometheus.Gauge
	cpuHostSysUsage  prometheus.Gauge
	cpuHostUserUsage prometheus.Gauge

	cpuContainerUsage     prometheus.Gauge
	cpuContainerSysUsage  prometheus.Gauge
	cpuContainerUserUsage prometheus.Gauge

	memUsage    prometheus.Gauge
	memMaxUsage prometheus.Gauge
	memRss      prometheus.Gauge
	memPercent  prometheus.Gauge

	bytesSent   *prometheus.GaugeVec
	bytesRecv   *prometheus.GaugeVec
	packetsSent *prometheus.GaugeVec
	packetsRecv *prometheus.GaugeVec
	errIn       *prometheus.GaugeVec
	errOut      *prometheus.GaugeVec
	dropIn      *prometheus.GaugeVec
	dropOut     *prometheus.GaugeVec
}

// NewMetricsClient new a metrics client
func NewMetricsClient(statsd, hostname string, container *types.Container) *MetricsClient {
	clables := []string{}
	for k, v := range container.Labels {
		l := fmt.Sprintf("%s=%s", k, v)
		clables = append(clables, l)
	}
	labels := map[string]string{
		"containerID":  container.ID,
		"hostname":     hostname,
		"appname":      container.Name,
		"entrypoint":   container.EntryPoint,
		"orchestrator": cluster.ERUMark,
		"labels":       strings.Join(clables, ","),
	}

	cpuHostUsage := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "cpu_host_usage",
		Help:        "cpu usage in host view.",
		ConstLabels: labels,
	})
	cpuHostSysUsage := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "cpu_host_sys_usage",
		Help:        "cpu sys usage in host view.",
		ConstLabels: labels,
	})
	cpuHostUserUsage := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "cpu_host_user_usage",
		Help:        "cpu user usage in host view.",
		ConstLabels: labels,
	})
	cpuContainerUsage := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "cpu_container_usage",
		Help:        "cpu usage in container view.",
		ConstLabels: labels,
	})
	cpuContainerSysUsage := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "cpu_container_sys_usage",
		Help:        "cpu sys usage in container view.",
		ConstLabels: labels,
	})
	cpuContainerUserUsage := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "cpu_container_user_usage",
		Help:        "cpu user usage in container view.",
		ConstLabels: labels,
	})
	memUsage := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "mem_usage",
		Help:        "memory usage.",
		ConstLabels: labels,
	})
	memMaxUsage := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "mem_max_usage",
		Help:        "memory max usage.",
		ConstLabels: labels,
	})
	memRss := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "mem_rss",
		Help:        "memory rss.",
		ConstLabels: labels,
	})
	memPercent := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "mem_percent",
		Help:        "memory percent.",
		ConstLabels: labels,
	})
	bytesSent := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:        "bytes_send",
		Help:        "bytes send.",
		ConstLabels: labels,
	}, []string{"nic"})
	bytesRecv := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:        "bytes_recv",
		Help:        "bytes recv.",
		ConstLabels: labels,
	}, []string{"nic"})
	packetsSent := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:        "packets_send",
		Help:        "packets send.",
		ConstLabels: labels,
	}, []string{"nic"})
	packetsRecv := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:        "packets_recv",
		Help:        "packets recv.",
		ConstLabels: labels,
	}, []string{"nic"})
	errIn := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:        "err_in",
		Help:        "err in.",
		ConstLabels: labels,
	}, []string{"nic"})
	errOut := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:        "err_out",
		Help:        "err out.",
		ConstLabels: labels,
	}, []string{"nic"})
	dropIn := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:        "drop_in",
		Help:        "drop in.",
		ConstLabels: labels,
	}, []string{"nic"})
	dropOut := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:        "drop_out",
		Help:        "drop out.",
		ConstLabels: labels,
	}, []string{"nic"})

	// TODO 这里已经没有了版本了
	tag := fmt.Sprintf("%s.%s", hostname, container.ID[:common.SHORTID])
	endpoint := fmt.Sprintf("%s.%s", container.Name, container.EntryPoint)
	prefix := fmt.Sprintf("%s.%s.%s", cluster.ERUMark, endpoint, tag)

	prometheus.MustRegister(
		cpuHostSysUsage, cpuHostUsage, cpuHostUserUsage,
		cpuContainerSysUsage, cpuContainerUsage, cpuContainerUserUsage,
		memMaxUsage, memRss, memUsage, memPercent,
		bytesRecv, bytesSent, packetsRecv, packetsSent,
		errIn, errOut, dropIn, dropOut,
	)

	return &MetricsClient{
		statsd: statsd,
		prefix: prefix,
		data:   map[string]float64{},

		cpuHostUsage:     cpuHostUsage,
		cpuHostSysUsage:  cpuHostSysUsage,
		cpuHostUserUsage: cpuHostUserUsage,

		cpuContainerUsage:     cpuContainerUsage,
		cpuContainerSysUsage:  cpuContainerSysUsage,
		cpuContainerUserUsage: cpuContainerUserUsage,

		memUsage:    memUsage,
		memMaxUsage: memMaxUsage,
		memRss:      memRss,
		memPercent:  memPercent,

		bytesSent:   bytesSent,
		bytesRecv:   bytesRecv,
		packetsSent: packetsSent,
		packetsRecv: packetsRecv,
		errIn:       errIn,
		errOut:      errOut,
		dropIn:      dropIn,
		dropOut:     dropOut,
	}
}

// Unregister unlink all prometheus things
func (m *MetricsClient) Unregister() {
	prometheus.Unregister(m.cpuHostSysUsage)
	prometheus.Unregister(m.cpuHostUsage)
	prometheus.Unregister(m.cpuHostUserUsage)

	prometheus.Unregister(m.cpuContainerUsage)
	prometheus.Unregister(m.cpuContainerSysUsage)
	prometheus.Unregister(m.cpuContainerUserUsage)

	prometheus.Unregister(m.memUsage)
	prometheus.Unregister(m.memMaxUsage)
	prometheus.Unregister(m.memRss)
	prometheus.Unregister(m.memPercent)

	prometheus.Unregister(m.bytesRecv)
	prometheus.Unregister(m.bytesSent)
	prometheus.Unregister(m.packetsRecv)
	prometheus.Unregister(m.packetsSent)
	prometheus.Unregister(m.errIn)
	prometheus.Unregister(m.errOut)
	prometheus.Unregister(m.dropIn)
	prometheus.Unregister(m.dropOut)
}

// CPUHostUsage set cpu usage in host view
func (m *MetricsClient) CPUHostUsage(i float64) {
	m.data["cpu_host_usage"] = i
	m.cpuHostUsage.Set(i)
}

// CPUHostSysUsage set cpu sys usage in host view
func (m *MetricsClient) CPUHostSysUsage(i float64) {
	m.data["cpu_host_sys_usage"] = i
	m.cpuHostSysUsage.Set(i)
}

// CPUHostUserUsage set cpu user usage in host view
func (m *MetricsClient) CPUHostUserUsage(i float64) {
	m.data["cpu_host_user_usage"] = i
	m.cpuHostUserUsage.Set(i)
}

// CPUContainerUsage set cpu usage in container view
func (m *MetricsClient) CPUContainerUsage(i float64) {
	m.data["cpu_container_usage"] = i
	m.cpuContainerUsage.Set(i)
}

// CPUContainerSysUsage set cpu sys usage in container view
func (m *MetricsClient) CPUContainerSysUsage(i float64) {
	m.data["cpu_container_sys_usage"] = i
	m.cpuContainerSysUsage.Set(i)
}

// CPUContainerUserUsage set cpu user usage in container view
func (m *MetricsClient) CPUContainerUserUsage(i float64) {
	m.data["cpu_container_user_usage"] = i
	m.cpuContainerUserUsage.Set(i)
}

// MemUsage set memory usage
func (m *MetricsClient) MemUsage(i float64) {
	m.data["mem_usage"] = i
	m.memUsage.Set(i)
}

// MemMaxUsage set memory max usage
func (m *MetricsClient) MemMaxUsage(i float64) {
	m.data["mem_max_usage"] = i
	m.memMaxUsage.Set(i)
}

// MemRss set memory rss
func (m *MetricsClient) MemRss(i float64) {
	m.data["mem_rss"] = i
	m.memRss.Set(i)
}

// MemPercent set memory percent
func (m *MetricsClient) MemPercent(i float64) {
	m.data["mem_percent"] = i
	m.memPercent.Set(i)
}

// BytesSent set bytes send
func (m *MetricsClient) BytesSent(nic string, i float64) {
	m.data[nic+".bytes.sent"] = i
	m.bytesSent.WithLabelValues(nic).Set(i)
}

// BytesRecv set bytes recv
func (m *MetricsClient) BytesRecv(nic string, i float64) {
	m.data[nic+".bytes.recv"] = i
	m.bytesRecv.WithLabelValues(nic).Set(i)
}

// PacketsSent set packets send
func (m *MetricsClient) PacketsSent(nic string, i float64) {
	m.data[nic+".packets.sent"] = i
	m.packetsSent.WithLabelValues(nic).Set(i)
}

// PacketsRecv set packets recv
func (m *MetricsClient) PacketsRecv(nic string, i float64) {
	m.data[nic+".packets.recv"] = i
	m.packetsRecv.WithLabelValues(nic).Set(i)
}

// ErrIn set inbound err count
func (m *MetricsClient) ErrIn(nic string, i float64) {
	m.data[nic+".err.in"] = i
	m.errIn.WithLabelValues(nic).Set(i)
}

// ErrOut set outbound err count
func (m *MetricsClient) ErrOut(nic string, i float64) {
	m.data[nic+".err.out"] = i
	m.errOut.WithLabelValues(nic).Set(i)
}

// DropIn set inbound drop count
func (m *MetricsClient) DropIn(nic string, i float64) {
	m.data[nic+".drop.in"] = i
	m.dropIn.WithLabelValues(nic).Set(i)
}

// DropOut set outbound drop count
func (m *MetricsClient) DropOut(nic string, i float64) {
	m.data[nic+".drop.out"] = i
	m.dropOut.WithLabelValues(nic).Set(i)
}

// Send to statsd
func (m *MetricsClient) Send() error {
	if m.statsd == "" {
		return nil
	}
	remote, err := statsdlib.New(m.statsd)
	if err != nil {
		log.Errorf("[statsd] Connect statsd failed: %v", err)
		return err
	}
	defer remote.Close()
	defer remote.Flush()
	for k, v := range m.data {
		key := fmt.Sprintf("%s.%s", m.prefix, k)
		remote.Gauge(key, v)
		delete(m.data, k)
	}
	return nil
}
