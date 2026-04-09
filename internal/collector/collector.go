package collector

import (
	"context"
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
	// 与 Identity 对齐，供控制面写入 nodes 表展示 OS/架构/版本
	Hostname        string `json:"hostname,omitempty"`
	Platform        string `json:"platform,omitempty"`
	PlatformVersion string `json:"platform_version,omitempty"`
	Arch            string `json:"arch,omitempty"`
	AgentVersion    string `json:"agent_version,omitempty"`
}

// Identity is the static node identity (日志/展示用；接入鉴权使用 agent_id + agent_secret)。
type Identity struct {
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
	nodeKey      string
	nodeName     string
	agentVersion string

	prevBytesRecv uint64
	prevBytesSent uint64
	prevAt        time.Time
}

// New creates a system collector.
func New(nodeKey, nodeName, agentVersion string) *SystemCollector {
	return &SystemCollector{
		nodeKey:      nodeKey,
		nodeName:     nodeName,
		agentVersion: agentVersion,
	}
}

// CollectIdentity collects static machine identity.
func (c *SystemCollector) CollectIdentity(context.Context) (*Identity, error) {
	info, err := host.Info()
	if err != nil {
		return nil, err
	}

	return &Identity{
		NodeKey:      c.nodeKey,
		Name:         c.nodeName,
		Hostname:     osutil.Hostname(),
		OS:           info.Platform,
		OSVersion:    info.PlatformVersion,
		Arch:         osutil.Arch(),
		PrivateIP:    osutil.PrivateIPv4(),
		PublicIP:     "",
		AgentVersion: c.agentVersion,
	}, nil
}

// CollectRuntimeState collects the latest runtime snapshot.
func (c *SystemCollector) CollectRuntimeState(ctx context.Context) (*RuntimeState, error) {
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
	if id, err := c.CollectIdentity(ctx); err == nil {
		result.Hostname = id.Hostname
		result.Platform = id.OS
		result.PlatformVersion = id.OSVersion
		result.Arch = id.Arch
		result.AgentVersion = id.AgentVersion
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
