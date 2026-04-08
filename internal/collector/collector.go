package collector

import (
	"context"
	"strings"
	"time"

	"github.com/nexctl/agent/pkg/osutil"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	gopsnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

// RuntimeState is the current machine runtime snapshot.
type RuntimeState struct {
	CPUPercent    float64   `json:"cpu_percent"`
	MemoryPercent float64   `json:"memory_percent"`
	DiskPercent   float64   `json:"disk_percent"`
	NetworkRxBps  uint64    `json:"network_rx_bps"`
	NetworkTxBps  uint64    `json:"network_tx_bps"`
	Load1         float64   `json:"load_1"`
	Load5         float64   `json:"load_5"`
	Load15        float64   `json:"load_15"`
	UptimeSeconds uint64    `json:"uptime_seconds"`
	ProcessCount  uint32    `json:"process_count"`
	Timestamp     time.Time `json:"timestamp"`
}

// Identity is the static node identity used during registration.
type Identity struct {
	InstallToken     string `json:"install_token"`
	EnrollmentToken string `json:"enrollment_token,omitempty"`
	NodeKey      string `json:"node_key"`
	Name         string `json:"name"`
	Hostname     string `json:"hostname"`
	OS           string `json:"os"`
	OSVersion    string `json:"os_version"`
	Arch         string `json:"arch"`
	PrivateIP    string `json:"private_ip"`
	PublicIP     string `json:"public_ip"`
	AgentVersion string `json:"agent_version"`
}

// Collector collects runtime state and machine identity.
type Collector interface {
	CollectRuntimeState(ctx context.Context) (*RuntimeState, error)
	CollectIdentity(ctx context.Context) (*Identity, error)
}

// SystemCollector is the default runtime collector.
type SystemCollector struct {
	nodeKey           string
	nodeName          string
	installToken      string
	enrollmentToken   string
	agentVersion      string

	prevBytesRecv uint64
	prevBytesSent uint64
	prevAt        time.Time
}

// New creates a system collector.
func New(nodeKey, nodeName, installToken, enrollmentToken, agentVersion string) *SystemCollector {
	return &SystemCollector{
		nodeKey:          nodeKey,
		nodeName:         nodeName,
		installToken:     installToken,
		enrollmentToken:  enrollmentToken,
		agentVersion:     agentVersion,
	}
}

// CollectIdentity collects static machine identity.
func (c *SystemCollector) CollectIdentity(context.Context) (*Identity, error) {
	info, err := host.Info()
	if err != nil {
		return nil, err
	}

	id := &Identity{
		InstallToken: c.installToken,
		NodeKey:      c.nodeKey,
		Name:         c.nodeName,
		Hostname:     osutil.Hostname(),
		OS:           info.Platform,
		OSVersion:    info.PlatformVersion,
		Arch:         osutil.Arch(),
		PrivateIP:    osutil.PrivateIPv4(),
		PublicIP:     "",
		AgentVersion: c.agentVersion,
	}
	if strings.TrimSpace(c.enrollmentToken) != "" {
		id.EnrollmentToken = strings.TrimSpace(c.enrollmentToken)
		id.InstallToken = ""
	}
	return id, nil
}

// CollectRuntimeState collects the latest runtime snapshot.
func (c *SystemCollector) CollectRuntimeState(context.Context) (*RuntimeState, error) {
	now := time.Now().UTC()
	result := &RuntimeState{Timestamp: now}

	if values, err := cpu.Percent(0, false); err == nil && len(values) > 0 {
		result.CPUPercent = values[0]
	}
	if info, err := mem.VirtualMemory(); err == nil {
		result.MemoryPercent = info.UsedPercent
	}
	if info, err := disk.Usage(rootPath()); err == nil {
		result.DiskPercent = info.UsedPercent
	}
	if avg, err := load.Avg(); err == nil {
		result.Load1 = avg.Load1
		result.Load5 = avg.Load5
		result.Load15 = avg.Load15
	}
	if info, err := host.Info(); err == nil {
		result.UptimeSeconds = info.Uptime
	}
	if pids, err := process.Pids(); err == nil {
		result.ProcessCount = uint32(len(pids))
	}
	if counters, err := gopsnet.IOCounters(false); err == nil && len(counters) > 0 {
		result.NetworkRxBps, result.NetworkTxBps = c.networkRate(now, counters[0].BytesRecv, counters[0].BytesSent)
	}
	return result, nil
}

func (c *SystemCollector) networkRate(now time.Time, currentRecv, currentSent uint64) (uint64, uint64) {
	if c.prevAt.IsZero() {
		c.prevAt = now
		c.prevBytesRecv = currentRecv
		c.prevBytesSent = currentSent
		return 0, 0
	}

	elapsed := now.Sub(c.prevAt).Seconds()
	if elapsed <= 0 {
		return 0, 0
	}

	rx := uint64(float64(currentRecv-c.prevBytesRecv) / elapsed)
	tx := uint64(float64(currentSent-c.prevBytesSent) / elapsed)
	c.prevAt = now
	c.prevBytesRecv = currentRecv
	c.prevBytesSent = currentSent
	return rx, tx
}
