package common

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/NodePassProject/conn"
)

func (c *Common) SingleEventLoop() error {
	ticker := time.NewTicker(ReportInterval)
	defer ticker.Stop()

	for c.Ctx.Err() == nil {
		c.Logger.Event("CHECK_POINT|MODE=%v|PING=%vms|POOL=0|TCPS=%v|UDPS=%v|TCPRX=%v|TCPTX=%v|UDPRX=%v|UDPTX=%v", c.RunMode, c.ProbeBestTarget(),
			atomic.LoadInt32(&c.TCPSlot), atomic.LoadInt32(&c.UDPSlot),
			atomic.LoadUint64(&c.TCPRX), atomic.LoadUint64(&c.TCPTX),
			atomic.LoadUint64(&c.UDPRX), atomic.LoadUint64(&c.UDPTX))

		select {
		case <-c.Ctx.Done():
			return fmt.Errorf("SingleEventLoop: context error: %w", c.Ctx.Err())
		case <-ticker.C:
		}
	}

	return fmt.Errorf("SingleEventLoop: context error: %w", c.Ctx.Err())
}

func (c *Common) SingleTCPLoop() error {
	for c.Ctx.Err() == nil {
		tunnelConn, err := c.TunnelListener.Accept()
		if err != nil {
			if c.Ctx.Err() != nil || err == net.ErrClosed {
				return fmt.Errorf("SingleTCPLoop: context error: %w", c.Ctx.Err())
			}
			c.Logger.Error("SingleTCPLoop: accept failed: %v", err)

			select {
			case <-c.Ctx.Done():
				return fmt.Errorf("SingleTCPLoop: context error: %w", c.Ctx.Err())
			case <-time.After(ContextCheckInterval):
			}
			continue
		}

		tunnelConn = &conn.StatConn{Conn: tunnelConn, RX: &c.TCPRX, TX: &c.TCPTX, Rate: c.RateLimiter}
		c.Logger.Debug("Tunnel connection: %v <-> %v", tunnelConn.LocalAddr(), tunnelConn.RemoteAddr())

		go func(tunnelConn net.Conn) {
			defer func() {
				if tunnelConn != nil {
					tunnelConn.Close()
				}
			}()

			if !c.TryAcquireSlot(false) {
				c.Logger.Error("SingleTCPLoop: TCP slot limit reached: %v/%v", c.TCPSlot, c.SlotLimit)
				return
			}

			defer c.ReleaseSlot(false)

			protocol, wrappedConn := c.DetectBlockProtocol(tunnelConn)
			if protocol != "" {
				c.Logger.Warn("SingleTCPLoop: blocked %v protocol from %v", protocol, tunnelConn.RemoteAddr())
				return
			}
			tunnelConn = wrappedConn

			targetConn, err := c.DialWithRotation("tcp", TCPDialTimeout)
			if err != nil {
				c.Logger.Error("SingleTCPLoop: dialWithRotation failed: %v", err)
				return
			}

			defer func() {
				if targetConn != nil {
					targetConn.Close()
				}
			}()

			c.Logger.Debug("Target connection: %v <-> %v", targetConn.LocalAddr(), targetConn.RemoteAddr())

			if err := c.SendProxyV1Header(tunnelConn.RemoteAddr().String(), targetConn); err != nil {
				c.Logger.Error("SingleTCPLoop: sendProxyV1Header failed: %v", err)
				return
			}

			buffer1 := c.GetTCPBuffer()
			buffer2 := c.GetTCPBuffer()
			defer func() {
				c.PutTCPBuffer(buffer1)
				c.PutTCPBuffer(buffer2)
			}()

			c.Logger.Info("Starting exchange: %v <-> %v", tunnelConn.RemoteAddr(), targetConn.RemoteAddr())
			c.Logger.Info("Exchange complete: %v", conn.DataExchange(tunnelConn, targetConn, c.ReadTimeout, buffer1, buffer2))
		}(tunnelConn)
	}

	return fmt.Errorf("SingleTCPLoop: context error: %w", c.Ctx.Err())
}

func (c *Common) SingleUDPLoop() error {
	for c.Ctx.Err() == nil {
		buffer := c.GetUDPBuffer()

		x, clientAddr, err := c.TunnelUDPConn.ReadFromUDP(buffer)
		if err != nil {
			if c.Ctx.Err() != nil || err == net.ErrClosed {
				c.PutUDPBuffer(buffer)
				return fmt.Errorf("SingleUDPLoop: context error: %w", c.Ctx.Err())
			}
			c.Logger.Error("SingleUDPLoop: ReadFromUDP failed: %v", err)

			c.PutUDPBuffer(buffer)
			select {
			case <-c.Ctx.Done():
				return fmt.Errorf("SingleUDPLoop: context error: %w", c.Ctx.Err())
			case <-time.After(ContextCheckInterval):
			}
			continue
		}

		c.Logger.Debug("Tunnel connection: %v <-> %v", c.TunnelUDPConn.LocalAddr(), clientAddr)

		var targetConn net.Conn
		sessionKey := clientAddr.String()

		if session, ok := c.TargetUDPSession.Load(sessionKey); ok {
			targetConn = session.(net.Conn)
			c.Logger.Debug("Using UDP session: %v <-> %v", targetConn.LocalAddr(), targetConn.RemoteAddr())
		} else {
			if !c.TryAcquireSlot(true) {
				c.Logger.Error("SingleUDPLoop: UDP slot limit reached: %v/%v", c.UDPSlot, c.SlotLimit)
				c.PutUDPBuffer(buffer)
				continue
			}

			newSession, err := c.DialWithRotation("udp", UDPDialTimeout)
			if err != nil {
				c.Logger.Error("SingleUDPLoop: dialWithRotation failed: %v", err)
				c.ReleaseSlot(true)
				c.PutUDPBuffer(buffer)
				continue
			}
			targetConn = newSession
			c.TargetUDPSession.Store(sessionKey, newSession)
			c.Logger.Debug("Target connection: %v <-> %v", targetConn.LocalAddr(), targetConn.RemoteAddr())

			go func(targetConn net.Conn, clientAddr *net.UDPAddr, sessionKey string) {
				defer func() {
					if targetConn != nil {
						targetConn.Close()
					}
					c.ReleaseSlot(true)
				}()

				buffer := c.GetUDPBuffer()
				defer c.PutUDPBuffer(buffer)
				reader := &conn.TimeoutReader{Conn: targetConn, Timeout: UDPReadTimeout}

				for c.Ctx.Err() == nil {
					x, err := reader.Read(buffer)
					if err != nil {
						if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
							c.Logger.Debug("UDP session abort: %v", err)
						} else if err.Error() != "EOF" {
							c.Logger.Error("SingleUDPLoop: read from target failed: %v", err)
						}
						c.TargetUDPSession.Delete(sessionKey)
						if targetConn != nil {
							targetConn.Close()
						}
						return
					}

					_, err = c.TunnelUDPConn.WriteToUDP(buffer[:x], clientAddr)
					if err != nil {
						if err.Error() != "EOF" {
							c.Logger.Error("SingleUDPLoop: writeToUDP failed: %v", err)
						}
						c.TargetUDPSession.Delete(sessionKey)
						if targetConn != nil {
							targetConn.Close()
						}
						return
					}
					c.Logger.Debug("Transfer complete: %v <-> %v", c.TunnelUDPConn.LocalAddr(), targetConn.LocalAddr())
				}
			}(targetConn, clientAddr, sessionKey)
		}

		c.Logger.Debug("Starting transfer: %v <-> %v", targetConn.LocalAddr(), c.TunnelUDPConn.LocalAddr())
		_, err = targetConn.Write(buffer[:x])
		if err != nil {
			if err.Error() != "EOF" {
				c.Logger.Error("SingleUDPLoop: write to target failed: %v", err)
			}
			c.TargetUDPSession.Delete(sessionKey)
			if targetConn != nil {
				targetConn.Close()
			}
			c.PutUDPBuffer(buffer)
			return fmt.Errorf("SingleUDPLoop: write to target failed: %w", err)
		}

		c.Logger.Debug("Transfer complete: %v <-> %v", targetConn.LocalAddr(), c.TunnelUDPConn.LocalAddr())
		c.PutUDPBuffer(buffer)
	}

	return fmt.Errorf("SingleUDPLoop: context error: %w", c.Ctx.Err())
}
