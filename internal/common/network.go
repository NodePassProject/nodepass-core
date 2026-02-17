package common

import (
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"time"
)

func (c *Common) Resolve(network, address string) (any, error) {
	now := time.Now()

	if val, ok := c.DNSCacheEntries.Load(address); ok {
		entry := val.(*DnsCacheEntry)
		if now.Before(entry.ExpiredAt) {
			if network == "tcp" {
				return entry.TCPAddr, nil
			}
			return entry.UDPAddr, nil
		}
		c.DNSCacheEntries.Delete(address)
	}

	tcpAddr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("Resolve: resolveTCPAddr failed: %w", err)
	}

	udpAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, fmt.Errorf("Resolve: resolveUDPAddr failed: %w", err)
	}

	entry := &DnsCacheEntry{
		TCPAddr:   tcpAddr,
		UDPAddr:   udpAddr,
		ExpiredAt: now.Add(c.DNSCacheTTL),
	}
	c.DNSCacheEntries.LoadOrStore(address, entry)

	if network == "tcp" {
		return tcpAddr, nil
	}
	return udpAddr, nil
}

func (c *Common) ClearCache() {
	c.DNSCacheEntries.Range(func(key, value any) bool {
		c.DNSCacheEntries.Delete(key)
		return true
	})
}

func (c *Common) ResolveAddr(network, address string) (any, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("ResolveAddr: invalid address %s: %w", address, err)
	}

	if host == "" || net.ParseIP(host) != nil {
		if network == "tcp" {
			return net.ResolveTCPAddr("tcp", address)
		}
		return net.ResolveUDPAddr("udp", address)
	}

	return c.Resolve(network, address)
}

func (c *Common) ResolveTarget(network string, idx int) (any, error) {
	if idx < 0 || idx >= len(c.TargetAddrs) {
		return nil, fmt.Errorf("ResolveTarget: index %d out of range", idx)
	}

	addr, err := c.ResolveAddr(network, c.TargetAddrs[idx])
	if err != nil {
		if network == "tcp" {
			return c.TargetTCPAddrs[idx], err
		}
		return c.TargetUDPAddrs[idx], err
	}
	return addr, nil
}

func (c *Common) GetTunnelTCPAddr() (*net.TCPAddr, error) {
	addr, err := c.ResolveAddr("tcp", c.TunnelAddr)
	if err != nil {
		return c.TunnelTCPAddr, err
	}
	return addr.(*net.TCPAddr), nil
}

func (c *Common) GetTunnelUDPAddr() (*net.UDPAddr, error) {
	addr, err := c.ResolveAddr("udp", c.TunnelAddr)
	if err != nil {
		return c.TunnelUDPAddr, err
	}
	return addr.(*net.UDPAddr), nil
}

func (c *Common) GetTargetAddrsString() string {
	addrs := make([]string, len(c.TargetTCPAddrs))
	for i, addr := range c.TargetTCPAddrs {
		addrs[i] = addr.String()
	}
	return strings.Join(addrs, ",")
}

func (c *Common) NextTargetIdx() int {
	if len(c.TargetTCPAddrs) <= 1 {
		return 0
	}
	return int((atomic.AddUint64(&c.TargetIdx, 1) - 1) % uint64(len(c.TargetTCPAddrs)))
}

func (c *Common) ProbeBestTarget() int {
	count := len(c.TargetTCPAddrs)
	if count == 0 {
		return 0
	}

	type result struct{ idx, lat int }
	results := make(chan result, count)
	for i := range count {
		go func(idx int) { results <- result{idx, c.TcpPing(idx)} }(i)
	}

	bestIdx, bestLat := 0, 0
	for range count {
		if r := <-results; r.lat > 0 && (bestLat == 0 || r.lat < bestLat) {
			bestIdx, bestLat = r.idx, r.lat
		}
	}

	if bestLat > 0 {
		atomic.StoreUint64(&c.TargetIdx, uint64(bestIdx))
		atomic.StoreInt32(&c.BestLatency, int32(bestLat))
	}
	return bestLat
}

func (c *Common) TcpPing(idx int) int {
	addr, _ := c.ResolveTarget("tcp", idx)
	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		start := time.Now()
		if conn, err := net.DialTimeout("tcp", tcpAddr.String(), ReportInterval); err == nil {
			conn.Close()
			return int(time.Since(start).Milliseconds())
		}
	}
	return 0
}

func (c *Common) GetDialFunc(network string, timeout time.Duration) func(string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: timeout}
	if c.DialerIP != DefaultDialerIP && atomic.LoadUint32(&c.DialerFallback) == 0 {
		if network == "tcp" {
			dialer.LocalAddr = &net.TCPAddr{IP: net.ParseIP(c.DialerIP)}
		} else {
			dialer.LocalAddr = &net.UDPAddr{IP: net.ParseIP(c.DialerIP)}
		}
	}

	return func(addr string) (net.Conn, error) {
		conn, err := dialer.Dial(network, addr)
		if err != nil && dialer.LocalAddr != nil && atomic.CompareAndSwapUint32(&c.DialerFallback, 0, 1) {
			c.Logger.Error("GetDialFunc: fallback to system auto due to dialer failure: %v", err)
			dialer.LocalAddr = nil
			return dialer.Dial(network, addr)
		}
		return conn, err
	}
}

func (c *Common) DialWithRotation(network string, timeout time.Duration) (net.Conn, error) {
	addrCount := len(c.TargetAddrs)

	getAddr := func(i int) string {
		addr, _ := c.ResolveTarget(network, i)
		if tcpAddr, ok := addr.(*net.TCPAddr); ok {
			return tcpAddr.String()
		}
		if udpAddr, ok := addr.(*net.UDPAddr); ok {
			return udpAddr.String()
		}
		return ""
	}

	tryDial := c.GetDialFunc(network, timeout)

	if addrCount == 1 {
		if addr := getAddr(0); addr != "" {
			return tryDial(addr)
		}
		return nil, fmt.Errorf("DialWithRotation: invalid target address")
	}

	var startIdx int
	switch c.LBStrategy {
	case "1":
		startIdx = int(atomic.LoadUint64(&c.TargetIdx) % uint64(addrCount))
	case "2":
		now := uint64(time.Now().UnixNano())
		last := atomic.LoadUint64(&c.LastFallback)
		if now-last > uint64(FallbackInterval) {
			atomic.StoreUint64(&c.LastFallback, now)
			atomic.StoreUint64(&c.TargetIdx, 0)
		}
		startIdx = int(atomic.LoadUint64(&c.TargetIdx) % uint64(addrCount))
	default:
		startIdx = c.NextTargetIdx()
	}

	var lastErr error
	for i := range addrCount {
		targetIdx := (startIdx + i) % addrCount
		addr := getAddr(targetIdx)
		if addr == "" {
			continue
		}
		conn, err := tryDial(addr)
		if err == nil {
			if i > 0 && (c.LBStrategy == "1" || c.LBStrategy == "2") {
				atomic.StoreUint64(&c.TargetIdx, uint64(targetIdx))
			}
			return conn, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("DialWithRotation: all %d targets failed: %w", addrCount, lastErr)
}
