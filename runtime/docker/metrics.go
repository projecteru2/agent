package docker

import (
	"fmt"
	"strings"
	"sync"

	"github.com/projecteru2/core/cluster"
	coreutils "github.com/projecteru2/core/utils"

	statsdlib "github.com/CMGS/statsd"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

// MetricsClient combine statsd and prometheus
type MetricsClient struct {
	statsd       string
	statsdClient *statsdlib.Client
	prefix       string
	data         map[string]float64

	cpuHostUsage     prometheus.Gauge
	cpuHostSysUsage  prometheus.Gauge
	cpuHostUserUsage prometheus.Gauge

	cpuContainerUsage     prometheus.Gauge
	cpuContainerSysUsage  prometheus.Gauge
	cpuContainerUserUsage prometheus.Gauge

	memUsage      prometheus.Gauge
	memMaxUsage   prometheus.Gauge
	memRss        prometheus.Gauge
	memPercent    prometheus.Gauge
	memRSSPercent prometheus.Gauge

	bytesSent   *prometheus.GaugeVec
	bytesRecv   *prometheus.GaugeVec
	packetsSent *prometheus.GaugeVec
	packetsRecv *prometheus.GaugeVec
	errIn       *prometheus.GaugeVec
	errOut      *prometheus.GaugeVec
	dropIn      *prometheus.GaugeVec
	dropOut     *prometheus.GaugeVec

	// diskio stats
	ioServiceBytesRead  *prometheus.GaugeVec
	ioServiceBytesWrite *prometheus.GaugeVec
	ioServicedRead      *prometheus.GaugeVec
	ioServicedWrite     *prometheus.GaugeVec
	// io/byte per second
	ioServiceBytesReadPerSecond  *prometheus.GaugeVec
	ioServiceBytesWritePerSecond *prometheus.GaugeVec
	ioServicedReadPerSecond      *prometheus.GaugeVec
	ioServicedWritePerSecond     *prometheus.GaugeVec
}

var clients sync.Map

// NewMetricsClient new a metrics client
func NewMetricsClient(statsd, hostname string, container *Container) *MetricsClient {
	if metricsClient, ok := clients.Load(container.ID); ok {
		return metricsClient.(*MetricsClient)
	}

	clables := []string{}
	for k, v := range container.Labels {
		if strings.HasPrefix(k, cluster.ERUMark) || strings.HasPrefix(k, cluster.LabelMeta) {
			continue
		}
		clables = append(clables, fmt.Sprintf("%s=%s", k, v))
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
	memRSSPercent := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:        "mem_rss_percent",
		Help:        "memory rss percent.",
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
	ioServiceBytesRead := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:        "io_service_bytes_read",
		Help:        "number of bytes read to the disk by the group.",
		ConstLabels: labels,
	}, []string{"dev"})
	ioServiceBytesWrite := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:        "io_service_bytes_write",
		Help:        "number of bytes write to the disk by the group.",
		ConstLabels: labels,
	}, []string{"dev"})
	ioServicedRead := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:        "io_serviced_read",
		Help:        "number of read IOs to the disk by the group.",
		ConstLabels: labels,
	}, []string{"dev"})
	ioServicedWrite := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:        "io_serviced_write",
		Help:        "number of write IOs to the disk by the group.",
		ConstLabels: labels,
	}, []string{"dev"})
	ioServiceBytesReadPerSecond := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:        "io_service_bytes_read_per_second",
		Help:        "number of bytes read per second to the disk by the group.",
		ConstLabels: labels,
	}, []string{"dev"})
	ioServiceBytesWritePerSecond := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:        "io_service_bytes_write_per_second",
		Help:        "number of bytes write per second to the disk by the group.",
		ConstLabels: labels,
	}, []string{"dev"})
	ioServicedReadPerSecond := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:        "io_serviced_read_per_second",
		Help:        "number of read IOs per second to the disk by the group.",
		ConstLabels: labels,
	}, []string{"dev"})
	ioServicedWritePerSecond := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:        "io_serviced_write_per_second",
		Help:        "number of write IOs per second to the disk by the group.",
		ConstLabels: labels,
	}, []string{"dev"})
	// TODO 这里已经没有了版本了
	tag := fmt.Sprintf("%s.%s", hostname, coreutils.ShortID(container.ID))
	endpoint := fmt.Sprintf("%s.%s", container.Name, container.EntryPoint)
	prefix := fmt.Sprintf("%s.%s.%s", cluster.ERUMark, endpoint, tag)

	prometheus.MustRegister(
		cpuHostSysUsage, cpuHostUsage, cpuHostUserUsage,
		cpuContainerSysUsage, cpuContainerUsage, cpuContainerUserUsage,
		memMaxUsage, memRss, memUsage, memPercent, memRSSPercent,
		bytesRecv, bytesSent, packetsRecv, packetsSent,
		errIn, errOut, dropIn, dropOut, ioServiceBytesRead, ioServiceBytesWrite, ioServicedRead, ioServicedWrite, ioServiceBytesReadPerSecond, ioServiceBytesWritePerSecond, ioServicedReadPerSecond, ioServicedWritePerSecond,
	)

	metricsClient := &MetricsClient{
		statsd: statsd,
		prefix: prefix,
		data:   map[string]float64{},

		cpuHostUsage:     cpuHostUsage,
		cpuHostSysUsage:  cpuHostSysUsage,
		cpuHostUserUsage: cpuHostUserUsage,

		cpuContainerUsage:     cpuContainerUsage,
		cpuContainerSysUsage:  cpuContainerSysUsage,
		cpuContainerUserUsage: cpuContainerUserUsage,

		memUsage:      memUsage,
		memMaxUsage:   memMaxUsage,
		memRss:        memRss,
		memPercent:    memPercent,
		memRSSPercent: memRSSPercent,

		bytesSent:   bytesSent,
		bytesRecv:   bytesRecv,
		packetsSent: packetsSent,
		packetsRecv: packetsRecv,
		errIn:       errIn,
		errOut:      errOut,
		dropIn:      dropIn,
		dropOut:     dropOut,

		ioServiceBytesRead:  ioServiceBytesRead,
		ioServiceBytesWrite: ioServiceBytesWrite,
		ioServicedRead:      ioServicedRead,
		ioServicedWrite:     ioServicedWrite,

		ioServiceBytesReadPerSecond:  ioServiceBytesReadPerSecond,
		ioServiceBytesWritePerSecond: ioServiceBytesWritePerSecond,
		ioServicedReadPerSecond:      ioServicedReadPerSecond,
		ioServicedWritePerSecond:     ioServicedWritePerSecond,
	}
	clients.Store(container.ID, metricsClient)
	return metricsClient
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
	prometheus.Unregister(m.memRSSPercent)

	prometheus.Unregister(m.bytesRecv)
	prometheus.Unregister(m.bytesSent)
	prometheus.Unregister(m.packetsRecv)
	prometheus.Unregister(m.packetsSent)
	prometheus.Unregister(m.errIn)
	prometheus.Unregister(m.errOut)
	prometheus.Unregister(m.dropIn)
	prometheus.Unregister(m.dropOut)

	prometheus.Unregister(m.ioServiceBytesRead)
	prometheus.Unregister(m.ioServiceBytesWrite)
	prometheus.Unregister(m.ioServicedRead)
	prometheus.Unregister(m.ioServicedWrite)

	prometheus.Unregister(m.ioServiceBytesReadPerSecond)
	prometheus.Unregister(m.ioServiceBytesWritePerSecond)
	prometheus.Unregister(m.ioServicedReadPerSecond)
	prometheus.Unregister(m.ioServicedWritePerSecond)
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

// MemRSSPercent set memory percent
func (m *MetricsClient) MemRSSPercent(i float64) {
	m.data["mem_rss_percent"] = i
	m.memRSSPercent.Set(i)
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

// IOServiceBytesRead .
func (m *MetricsClient) IOServiceBytesRead(dev string, i float64) {
	m.data[dev+".io_service_bytes_read"] = i
	m.ioServiceBytesRead.WithLabelValues(dev).Set(i)
}

// IOServiceBytesWrite .
func (m *MetricsClient) IOServiceBytesWrite(dev string, i float64) {
	m.data[dev+".io_service_bytes_write"] = i
	m.ioServiceBytesWrite.WithLabelValues(dev).Set(i)
}

// IOServicedRead .
func (m *MetricsClient) IOServicedRead(dev string, i float64) {
	m.data[dev+".io_serviced_read"] = i
	m.ioServicedRead.WithLabelValues(dev).Set(i)
}

// IOServicedWrite .
func (m *MetricsClient) IOServicedWrite(dev string, i float64) {
	m.data[dev+".io_serviced_write"] = i
	m.ioServicedWrite.WithLabelValues(dev).Set(i)
}

// IOServiceBytesReadPerSecond .
func (m *MetricsClient) IOServiceBytesReadPerSecond(dev string, i float64) {
	m.data[dev+".io_service_bytes_read_per_second"] = i
	m.ioServiceBytesReadPerSecond.WithLabelValues(dev).Set(i)
}

// IOServiceBytesWritePerSecond .
func (m *MetricsClient) IOServiceBytesWritePerSecond(dev string, i float64) {
	m.data[dev+".io_service_bytes_write_per_second"] = i
	m.ioServiceBytesWritePerSecond.WithLabelValues(dev).Set(i)
}

// IOServicedReadPerSecond .
func (m *MetricsClient) IOServicedReadPerSecond(dev string, i float64) {
	m.data[dev+".io_serviced_read_per_second"] = i
	m.ioServicedReadPerSecond.WithLabelValues(dev).Set(i)
}

// IOServicedWritePerSecond .
func (m *MetricsClient) IOServicedWritePerSecond(dev string, i float64) {
	m.data[dev+".io_serviced_write_per_second"] = i
	m.ioServicedWritePerSecond.WithLabelValues(dev).Set(i)
}

// Lazy connecting
func (m *MetricsClient) checkConn() error {
	if m.statsdClient != nil {
		return nil
	}
	// We needn't try to renew/reconnect because of only supporting UDP protocol now
	// We should add an `errorCount` to reconnect when implementing TCP protocol
	var err error
	if m.statsdClient, err = statsdlib.New(m.statsd, statsdlib.WithErrorHandler(func(err error) {
		log.Errorf("[statsd] Sending statsd failed: %v", err)
	})); err != nil {
		log.Errorf("[statsd] Connect statsd failed: %v", err)
		return err
	}
	return nil
}

// Send to statsd
func (m *MetricsClient) Send() error {
	if m.statsd == "" {
		return nil
	}
	if err := m.checkConn(); err != nil {
		return err
	}
	for k, v := range m.data {
		key := fmt.Sprintf("%s.%s", m.prefix, k)
		m.statsdClient.Gauge(key, v)
		delete(m.data, k)
	}
	return nil
}
