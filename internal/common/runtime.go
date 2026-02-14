package common

import (
	"context"
	"fmt"
	"net"

	"github.com/NodePassProject/conn"
)

func (c *Common) InitRateLimiter() {
	if c.RateLimit > 0 {
		c.RateLimiter = conn.NewRateLimiter(int64(c.RateLimit), int64(c.RateLimit))
	}
}

func (c *Common) InitContext() {
	if c.Cancel != nil {
		c.Cancel()
	}
	c.Ctx, c.Cancel = context.WithCancel(context.Background())
}

func (c *Common) InitTunnelListener() error {
	if c.TunnelTCPAddr == nil && c.TunnelUDPAddr == nil {
		return fmt.Errorf("initTunnelListener: nil tunnel address")
	}

	if c.TunnelTCPAddr != nil && (c.DisableTCP != "1" || c.CoreType != "client") {
		tunnelListener, err := net.ListenTCP("tcp", c.TunnelTCPAddr)
		if err != nil {
			return fmt.Errorf("initTunnelListener: listenTCP failed: %w", err)
		}
		c.TunnelListener = tunnelListener
	}

	if c.TunnelUDPAddr != nil && (c.DisableUDP != "1" || c.CoreType != "client") {
		tunnelUDPConn, err := net.ListenUDP("udp", c.TunnelUDPAddr)
		if err != nil {
			return fmt.Errorf("initTunnelListener: listenUDP failed: %w", err)
		}
		c.TunnelUDPConn = &conn.StatConn{Conn: tunnelUDPConn, RX: &c.UdpRX, TX: &c.UdpTX, Rate: c.RateLimiter}
	}

	return nil
}

func (c *Common) InitTargetListener() error {
	if len(c.TargetAddrs) == 0 {
		return fmt.Errorf("initTargetListener: no target address")
	}

	if len(c.TargetTCPAddrs) > 0 && c.DisableTCP != "1" {
		targetListener, err := net.ListenTCP("tcp", c.TargetTCPAddrs[0])
		if err != nil {
			return fmt.Errorf("initTargetListener: listenTCP failed: %w", err)
		}
		c.TargetListener = targetListener
	}

	if len(c.TargetUDPAddrs) > 0 && c.DisableUDP != "1" {
		targetUDPConn, err := net.ListenUDP("udp", c.TargetUDPAddrs[0])
		if err != nil {
			return fmt.Errorf("initTargetListener: listenUDP failed: %w", err)
		}
		c.TargetUDPConn = &conn.StatConn{Conn: targetUDPConn, RX: &c.UdpRX, TX: &c.UdpTX, Rate: c.RateLimiter}
	}

	return nil
}

func (c *Common) Stop() {
	if c.Cancel != nil {
		c.Cancel()
	}

	if c.TunnelPool != nil {
		active := c.TunnelPool.Active()
		c.TunnelPool.Close()
		c.Logger.Debug("Tunnel connection closed: pool active %v", active)
	}

	c.TargetUDPSession.Range(func(key, value any) bool {
		if conn, ok := value.(*net.UDPConn); ok {
			conn.Close()
		}
		c.TargetUDPSession.Delete(key)
		return true
	})

	if c.TargetUDPConn != nil {
		c.TargetUDPConn.Close()
		c.Logger.Debug("Target connection closed: %v", c.TargetUDPConn.LocalAddr())
	}

	if c.TunnelUDPConn != nil {
		c.TunnelUDPConn.Close()
		c.Logger.Debug("Tunnel connection closed: %v", c.TunnelUDPConn.LocalAddr())
	}

	if c.ControlConn != nil {
		c.ControlConn.Close()
		c.Logger.Debug("Control connection closed: %v", c.ControlConn.LocalAddr())
	}

	if c.TargetListener != nil {
		c.TargetListener.Close()
		c.Logger.Debug("Target listener closed: %v", c.TargetListener.Addr())
	}

	if c.TunnelListener != nil {
		c.TunnelListener.Close()
		c.Logger.Debug("Tunnel listener closed: %v", c.TunnelListener.Addr())
	}

	Drain(c.SignalChan)
	Drain(c.WriteChan)
	Drain(c.VerifyChan)

	if c.RateLimiter != nil {
		c.RateLimiter.Reset()
	}

	c.ClearCache()
}

func (c *Common) Shutdown(ctx context.Context, stopFunc func()) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		stopFunc()
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("shutdown: context error: %w", ctx.Err())
	case <-done:
		return nil
	}
}
