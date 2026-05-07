package listener

import (
	"fmt"
	"syscall"

	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

// ScanListeners scans all network connections and returns a list of
// ListenerInfo for each TCP socket in LISTEN state.
func ScanListeners() ([]ListenerInfo, error) {
	conns, err := net.Connections("inet")
	if err != nil {
		return nil, fmt.Errorf("failed to get network connections: %w", err)
	}

	var listeners []ListenerInfo
	for _, conn := range conns {
		// Filter: only TCP sockets in LISTEN state
		if conn.Type != uint32(syscall.SOCK_STREAM) || conn.Status != "LISTEN" {
			continue
		}

		info, err := gatherProcessInfo(conn)
		if err != nil {
			// Log and skip processes we can't inspect (e.g., kernel-owned or permissions)
			continue
		}
		listeners = append(listeners, *info)
	}

	return listeners, nil
}

// gatherProcessInfo enriches a net.ConnectionStat with process details
// using gopsutil/process.
func gatherProcessInfo(conn net.ConnectionStat) (*ListenerInfo, error) {
	proc, err := process.NewProcess(conn.Pid)
	if err != nil {
		return nil, fmt.Errorf("process %d: %w", conn.Pid, err)
	}

	name, _ := proc.Name()
	cmdline, _ := proc.Cmdline()
	exe, _ := proc.Exe()
	username, _ := proc.Username()

	// CPU percent — use duration 0 to get cached value (avoids 1s blocking call)
	cpuPercent, _ := proc.Percent(0)

	// Memory info
	memInfo, err := proc.MemoryInfo()
	var rssBytes uint64
	if err == nil && memInfo != nil {
		rssBytes = memInfo.RSS
	}

	// Uptime
	createTime, _ := proc.CreateTime()
	uptimeStr, uptimeSecs := FormatUptime(createTime)

	// Bound address (prefer IPv4 string; IPv6 addresses come from /proc as hex)
	address := conn.Laddr.IP
	if address == "" {
		address = "*"
	}

	return &ListenerInfo{
		PID:           conn.Pid,
		Name:          name,
		Cmdline:       cmdline,
		Exe:           exe,
		Username:      username,
		Port:          conn.Laddr.Port,
		Address:       address,
		Protocol:      "TCP",
		Uptime:        uptimeStr,
		UptimeSeconds: uptimeSecs,
		CPUPercent:    cpuPercent,
		RSSBytes:      rssBytes,
		RSSHuman:      FormatBytes(rssBytes),
	}, nil
}