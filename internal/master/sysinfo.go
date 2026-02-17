package master

import (
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func (m *Master) GetMasterInfo() map[string]any {
	info := map[string]any{
		"mid":        m.MID,
		"alias":      m.Alias,
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"noc":        runtime.NumCPU(),
		"cpu":        -1,
		"mem_total":  uint64(0),
		"mem_used":   uint64(0),
		"swap_total": uint64(0),
		"swap_used":  uint64(0),
		"netrx":      uint64(0),
		"nettx":      uint64(0),
		"diskr":      uint64(0),
		"diskw":      uint64(0),
		"sysup":      uint64(0),
		"ver":        m.Version,
		"name":       m.Hostname,
		"uptime":     uint64(time.Since(m.StartTime).Seconds()),
		"log":        m.LogLevel,
		"tls":        m.TLSConfig,
		"crt":        m.CrtPath,
		"key":        m.KeyPath,
	}

	if runtime.GOOS == "linux" {
		sysInfo := GetLinuxSysInfo()
		info["cpu"] = sysInfo.CPU
		info["mem_total"] = sysInfo.MemTotal
		info["mem_used"] = sysInfo.MemUsed
		info["swap_total"] = sysInfo.SwapTotal
		info["swap_used"] = sysInfo.SwapUsed
		info["netrx"] = sysInfo.NetRX
		info["nettx"] = sysInfo.NetTX
		info["diskr"] = sysInfo.DiskR
		info["diskw"] = sysInfo.DiskW
		info["sysup"] = sysInfo.SysUp
	}

	return info
}

func GetLinuxSysInfo() SystemInfo {
	info := SystemInfo{
		CPU:       -1,
		MemTotal:  0,
		MemUsed:   0,
		SwapTotal: 0,
		SwapUsed:  0,
		NetRX:     0,
		NetTX:     0,
		DiskR:     0,
		DiskW:     0,
		SysUp:     0,
	}

	if runtime.GOOS != "linux" {
		return info
	}

	readStat := func() (idle, total uint64) {
		data, err := os.ReadFile("/proc/stat")
		if err != nil {
			return
		}
		for line := range strings.SplitSeq(string(data), "\n") {
			if strings.HasPrefix(line, "cpu ") {
				fields := strings.Fields(line)
				for i, v := range fields[1:] {
					val, _ := strconv.ParseUint(v, 10, 64)
					total += val
					if i == 3 {
						idle = val
					}
				}
				break
			}
		}
		return
	}
	idle1, total1 := readStat()
	time.Sleep(BaseDuration)
	idle2, total2 := readStat()
	if deltaIdle, deltaTotal := idle2-idle1, total2-total1; deltaTotal > 0 {
		info.CPU = min(int((deltaTotal-deltaIdle)*100/deltaTotal), 100)
	}

	if data, err := os.ReadFile("/proc/meminfo"); err == nil {
		var memTotal, memAvailable, swapTotal, swapFree uint64
		for line := range strings.SplitSeq(string(data), "\n") {
			if fields := strings.Fields(line); len(fields) >= 2 {
				if val, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
					val *= 1024
					switch fields[0] {
					case "MemTotal:":
						memTotal = val
					case "MemAvailable:":
						memAvailable = val
					case "SwapTotal:":
						swapTotal = val
					case "SwapFree:":
						swapFree = val
					}
				}
			}
		}
		info.MemTotal = memTotal
		info.MemUsed = memTotal - memAvailable
		info.SwapTotal = swapTotal
		info.SwapUsed = swapTotal - swapFree
	}

	if data, err := os.ReadFile("/proc/net/dev"); err == nil {
		for _, line := range strings.Split(string(data), "\n")[2:] {
			if fields := strings.Fields(line); len(fields) >= 10 {
				ifname := strings.TrimSuffix(fields[0], ":")
				if strings.HasPrefix(ifname, "lo") || strings.HasPrefix(ifname, "veth") ||
					strings.HasPrefix(ifname, "docker") || strings.HasPrefix(ifname, "podman") ||
					strings.HasPrefix(ifname, "br-") || strings.HasPrefix(ifname, "virbr") {
					continue
				}
				if val, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
					info.NetRX += val
				}
				if val, err := strconv.ParseUint(fields[9], 10, 64); err == nil {
					info.NetTX += val
				}
			}
		}
	}

	if data, err := os.ReadFile("/proc/diskstats"); err == nil {
		for line := range strings.SplitSeq(string(data), "\n") {
			if fields := strings.Fields(line); len(fields) >= 14 {
				deviceName := fields[2]
				if strings.Contains(deviceName, "loop") || strings.Contains(deviceName, "ram") ||
					strings.HasPrefix(deviceName, "dm-") || strings.HasPrefix(deviceName, "md") {
					continue
				}
				if matched, _ := regexp.MatchString(`\d+$`, deviceName); matched {
					continue
				}
				if val, err := strconv.ParseUint(fields[5], 10, 64); err == nil {
					info.DiskR += val * 512
				}
				if val, err := strconv.ParseUint(fields[9], 10, 64); err == nil {
					info.DiskW += val * 512
				}
			}
		}
	}

	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		if fields := strings.Fields(string(data)); len(fields) > 0 {
			if uptime, err := strconv.ParseFloat(fields[0], 64); err == nil {
				info.SysUp = uint64(uptime)
			}
		}
	}

	return info
}
