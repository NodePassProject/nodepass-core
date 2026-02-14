package common

import (
	"fmt"
	"hash/fnv"
	"encoding/hex"
	"net"
	"strconv"
	"strings"
	"time"
)

func (c *Common) GetAddress() error {
	tunnelAddr := c.ParsedURL.Host
	if tunnelAddr == "" {
		return fmt.Errorf("getAddress: no valid tunnel address found")
	}

	c.TunnelAddr = tunnelAddr
	if name, port, err := net.SplitHostPort(tunnelAddr); err == nil {
		c.ServerName, c.ServerPort = name, port
	}

	tcpAddr, err := c.ResolveAddr("tcp", tunnelAddr)
	if err != nil {
		return fmt.Errorf("getAddress: resolveTCPAddr failed: %w", err)
	}
	c.TunnelTCPAddr = tcpAddr.(*net.TCPAddr)

	udpAddr, err := c.ResolveAddr("udp", tunnelAddr)
	if err != nil {
		return fmt.Errorf("getAddress: resolveUDPAddr failed: %w", err)
	}
	c.TunnelUDPAddr = udpAddr.(*net.UDPAddr)

	targetAddr := strings.TrimPrefix(c.ParsedURL.Path, "/")
	if targetAddr == "" {
		return fmt.Errorf("getAddress: no valid target address found")
	}

	addrList := strings.Split(targetAddr, ",")
	tempTCPAddrs := make([]*net.TCPAddr, 0, len(addrList))
	tempUDPAddrs := make([]*net.UDPAddr, 0, len(addrList))
	tempRawAddrs := make([]string, 0, len(addrList))

	for _, addr := range addrList {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}

		tcpAddr, err := c.ResolveAddr("tcp", addr)
		if err != nil {
			return fmt.Errorf("getAddress: resolveTCPAddr failed for %s: %w", addr, err)
		}

		udpAddr, err := c.ResolveAddr("udp", addr)
		if err != nil {
			return fmt.Errorf("getAddress: resolveUDPAddr failed for %s: %w", addr, err)
		}

		tempTCPAddrs = append(tempTCPAddrs, tcpAddr.(*net.TCPAddr))
		tempUDPAddrs = append(tempUDPAddrs, udpAddr.(*net.UDPAddr))
		tempRawAddrs = append(tempRawAddrs, addr)
	}

	if len(tempTCPAddrs) == 0 || len(tempUDPAddrs) == 0 || len(tempTCPAddrs) != len(tempUDPAddrs) {
		return fmt.Errorf("getAddress: no valid target address found")
	}

	c.TargetAddrs = tempRawAddrs
	c.TargetTCPAddrs = tempTCPAddrs
	c.TargetUDPAddrs = tempUDPAddrs
	c.TargetIdx = 0

	tunnelPort := c.TunnelTCPAddr.Port
	for _, targetAddr := range c.TargetTCPAddrs {
		if targetAddr.Port == tunnelPort && (targetAddr.IP.IsLoopback() || c.TunnelTCPAddr.IP.IsUnspecified()) {
			return fmt.Errorf("getAddress: tunnel port %d conflicts with target address %s", tunnelPort, targetAddr.String())
		}
	}

	return nil
}

func (c *Common) getCoreType() {
	c.CoreType = c.ParsedURL.Scheme
}

func (c *Common) getTunnelKey() {
	if key := c.ParsedURL.User.Username(); key != "" {
		c.TunnelKey = key
	} else {
		hash := fnv.New32a()
		hash.Write([]byte(c.ParsedURL.Port()))
		c.TunnelKey = hex.EncodeToString(hash.Sum(nil))
	}
}

func (c *Common) getDNSTTL() {
	if dns := c.ParsedURL.Query().Get("dns"); dns != "" {
		if ttl, err := time.ParseDuration(dns); err == nil && ttl > 0 {
			c.DnsCacheTTL = ttl
		}
	} else {
		c.DnsCacheTTL = DefaultDNSTTL
	}
}

func (c *Common) getServerName() {
	if serverName := c.ParsedURL.Query().Get("sni"); serverName != "" {
		c.ServerName = serverName
		return
	}
	if c.ServerName == "" || net.ParseIP(c.ServerName) != nil {
		c.ServerName = DefaultServerName
	}
}

func (c *Common) getLBStrategy() {
	if lbStrategy := c.ParsedURL.Query().Get("lbs"); lbStrategy != "" {
		c.LbStrategy = lbStrategy
	} else {
		c.LbStrategy = DefaultLBStrategy
	}
}

func (c *Common) getPoolCapacity() {
	if min := c.ParsedURL.Query().Get("min"); min != "" {
		if value, err := strconv.Atoi(min); err == nil && value > 0 {
			c.MinPoolCapacity = value
		}
	} else {
		c.MinPoolCapacity = DefaultMinPool
	}

	if max := c.ParsedURL.Query().Get("max"); max != "" {
		if value, err := strconv.Atoi(max); err == nil && value > 0 {
			c.MaxPoolCapacity = value
		}
	} else {
		c.MaxPoolCapacity = DefaultMaxPool
	}
}

func (c *Common) getRunMode() {
	if mode := c.ParsedURL.Query().Get("mode"); mode != "" {
		c.RunMode = mode
	} else {
		c.RunMode = DefaultRunMode
	}
}

func (c *Common) getPoolType() {
	if poolType := c.ParsedURL.Query().Get("type"); poolType != "" {
		c.PoolType = poolType
	} else {
		c.PoolType = DefaultPoolType
	}
	if c.PoolType == "1" && c.TlsCode == "0" {
		c.TlsCode = "1"
	}
}

func (c *Common) getDialerIP() {
	if dialerIP := c.ParsedURL.Query().Get("dial"); dialerIP != "" && dialerIP != "auto" {
		if ip := net.ParseIP(dialerIP); ip != nil {
			c.DialerIP = dialerIP
			return
		} else {
			c.Logger.Error("getDialerIP: fallback to system auto due to invalid IP address: %v", dialerIP)
		}
	}
	c.DialerIP = DefaultDialerIP
}

func (c *Common) getReadTimeout() {
	if timeout := c.ParsedURL.Query().Get("read"); timeout != "" {
		if value, err := time.ParseDuration(timeout); err == nil && value > 0 {
			c.ReadTimeout = value
		}
	} else {
		c.ReadTimeout = DefaultReadTimeout
	}
}

func (c *Common) getRateLimit() {
	if limit := c.ParsedURL.Query().Get("rate"); limit != "" {
		if value, err := strconv.Atoi(limit); err == nil && value > 0 {
			c.RateLimit = value * 125000
		}
	} else {
		c.RateLimit = DefaultRateLimit
	}
}

func (c *Common) getSlotLimit() {
	if slot := c.ParsedURL.Query().Get("slot"); slot != "" {
		if value, err := strconv.Atoi(slot); err == nil && value > 0 {
			c.SlotLimit = int32(value)
		}
	} else {
		c.SlotLimit = DefaultSlotLimit
	}
}

func (c *Common) getProxyProtocol() {
	if protocol := c.ParsedURL.Query().Get("proxy"); protocol != "" {
		c.ProxyProtocol = protocol
	} else {
		c.ProxyProtocol = DefaultProxyProtocol
	}
}

func (c *Common) getBlockProtocol() {
	if protocol := c.ParsedURL.Query().Get("block"); protocol != "" {
		c.BlockProtocol = protocol
	} else {
		c.BlockProtocol = DefaultBlockProtocol
	}
	c.BlockSOCKS = strings.Contains(c.BlockProtocol, "1")
	c.BlockHTTP = strings.Contains(c.BlockProtocol, "2")
	c.BlockTLS = strings.Contains(c.BlockProtocol, "3")
}

func (c *Common) getTCPStrategy() {
	if tcpStrategy := c.ParsedURL.Query().Get("notcp"); tcpStrategy != "" {
		c.DisableTCP = tcpStrategy
	} else {
		c.DisableTCP = DefaultTCPStrategy
	}
}

func (c *Common) getUDPStrategy() {
	if udpStrategy := c.ParsedURL.Query().Get("noudp"); udpStrategy != "" {
		c.DisableUDP = udpStrategy
	} else {
		c.DisableUDP = DefaultUDPStrategy
	}
}

func (c *Common) InitConfig() error {
	if err := c.GetAddress(); err != nil {
		return err
	}

	c.getCoreType()
	c.getDNSTTL()
	c.getTunnelKey()
	c.getPoolCapacity()
	c.getServerName()
	c.getLBStrategy()
	c.getRunMode()
	c.getPoolType()
	c.getDialerIP()
	c.getReadTimeout()
	c.getRateLimit()
	c.getSlotLimit()
	c.getProxyProtocol()
	c.getBlockProtocol()
	c.getTCPStrategy()
	c.getUDPStrategy()

	return nil
}
