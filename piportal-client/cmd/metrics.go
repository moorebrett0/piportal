package cmd

import (
	"os"
	"strconv"
	"strings"
	"syscall"
)

// CollectMetrics gathers system metrics from /proc and /sys.
// All reads are best-effort — returns -1 or 0 for unavailable values.
func CollectMetrics() MetricsMessage {
	return MetricsMessage{
		Type:      MessageTypeMetrics,
		CPUTemp:   readCPUTemp(),
		MemTotal:  readMemField("MemTotal"),
		MemFree:   readMemField("MemAvailable"),
		DiskTotal: readDiskTotal(),
		DiskFree:  readDiskFree(),
		Uptime:    readUptime(),
		LoadAvg:   readLoadAvg(),
	}
}

// readCPUTemp reads from thermal_zone0 (millidegrees -> celsius)
func readCPUTemp() float64 {
	data, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp")
	if err != nil {
		return -1
	}
	milliC, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return -1
	}
	return float64(milliC) / 1000.0
}

// readMemField reads a field from /proc/meminfo (returns bytes)
func readMemField(field string) uint64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, field+":") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				kb, err := strconv.ParseUint(parts[1], 10, 64)
				if err != nil {
					return 0
				}
				return kb * 1024 // kB to bytes
			}
		}
	}
	return 0
}

// readDiskTotal returns root partition total bytes
func readDiskTotal() uint64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		return 0
	}
	return stat.Blocks * uint64(stat.Bsize)
}

// readDiskFree returns root partition free bytes (available to non-root)
func readDiskFree() uint64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		return 0
	}
	return stat.Bavail * uint64(stat.Bsize)
}

// readUptime reads /proc/uptime and returns seconds
func readUptime() int64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	parts := strings.Fields(string(data))
	if len(parts) < 1 {
		return 0
	}
	seconds, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0
	}
	return int64(seconds)
}

// readLoadAvg reads 1-minute load average from /proc/loadavg
func readLoadAvg() float64 {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return -1
	}
	parts := strings.Fields(string(data))
	if len(parts) < 1 {
		return -1
	}
	load, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return -1
	}
	return load
}
