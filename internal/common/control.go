package common

import (
	"bufio"
	"encoding/json"
	"fmt"
	"time"

	"github.com/NodePassProject/conn"
)

func (c *Common) SetControlConn() error {
	start := time.Now()
	for c.Ctx.Err() == nil {
		if c.TunnelPool.Ready() && c.TunnelPool.Active() > 0 {
			break
		}
		if time.Since(start) > HandshakeTimeout {
			return fmt.Errorf("SetControlConn: handshake timeout")
		}
		select {
		case <-c.Ctx.Done():
			return fmt.Errorf("SetControlConn: context error: %w", c.Ctx.Err())
		case <-time.After(ContextCheckInterval):
		}
	}

	poolConn, err := c.TunnelPool.OutgoingGet("00000000", PoolGetTimeout)
	if err != nil {
		return fmt.Errorf("SetControlConn: outgoingGet failed: %w", err)
	}
	c.ControlConn = poolConn
	c.BufReader = bufio.NewReader(&conn.TimeoutReader{Conn: c.ControlConn, Timeout: 3 * ReportInterval})
	c.Logger.Info("Marking tunnel handshake as complete in %vms", time.Since(c.HandshakeStart).Milliseconds())

	go func() {
		for {
			select {
			case <-c.Ctx.Done():
				return
			case data := <-c.WriteChan:
				_, err := c.ControlConn.Write(data)
				if err != nil {
					c.Logger.Error("SetControlConn: write failed: %v", err)
				}
			}
		}
	}()

	if c.TLSCode == "1" {
		c.Logger.Info("TLS code-1: RAM cert fingerprint verifying...")
	}
	return nil
}

func (c *Common) CommonControl() error {
	errChan := make(chan error, 3)

	go func() { errChan <- c.CommonOnce() }()
	go func() { errChan <- c.CommonQueue() }()
	go func() { errChan <- c.HealthCheck() }()

	select {
	case <-c.Ctx.Done():
		return fmt.Errorf("commonControl: context error: %w", c.Ctx.Err())
	case err := <-errChan:
		return fmt.Errorf("commonControl: %w", err)
	}
}

func (c *Common) CommonQueue() error {
	for c.Ctx.Err() == nil {
		rawSignal, err := c.BufReader.ReadBytes('\n')
		if err != nil {
			return fmt.Errorf("CommonQueue: readBytes failed: %w", err)
		}

		signalData, err := c.Decode(rawSignal)
		if err != nil {
			c.Logger.Error("CommonQueue: decode signal failed: %v", err)
			select {
			case <-c.Ctx.Done():
				return fmt.Errorf("CommonQueue: context error: %w", c.Ctx.Err())
			case <-time.After(ContextCheckInterval):
			}
			continue
		}

		var signal Signal
		if err := json.Unmarshal(signalData, &signal); err != nil {
			c.Logger.Error("CommonQueue: unmarshal signal failed: %v", err)
			select {
			case <-c.Ctx.Done():
				return fmt.Errorf("CommonQueue: context error: %w", c.Ctx.Err())
			case <-time.After(ContextCheckInterval):
			}
			continue
		}

		select {
		case c.SignalChan <- signal:
		default:
			c.Logger.Error("CommonQueue: queue limit reached: %v", SemaphoreLimit)
			select {
			case <-c.Ctx.Done():
				return fmt.Errorf("CommonQueue: context error: %w", c.Ctx.Err())
			case <-time.After(ContextCheckInterval):
			}
		}
	}

	return fmt.Errorf("CommonQueue: context error: %w", c.Ctx.Err())
}

func (c *Common) HealthCheck() error {
	ticker := time.NewTicker(ReportInterval)
	defer ticker.Stop()

	if c.TLSCode == "1" {
		go func() {
			select {
			case <-c.Ctx.Done():
			case <-time.After(ReportInterval):
				c.IncomingVerify()
			}
		}()
	}

	for c.Ctx.Err() == nil {
		if c.TunnelPool.ErrorCount() > c.TunnelPool.Active()/2 {
			if c.Ctx.Err() == nil && c.ControlConn != nil {
				signalData, _ := json.Marshal(Signal{ActionType: "flush"})
				c.WriteChan <- c.Encode(signalData)
			}
			c.TunnelPool.Flush()
			c.TunnelPool.ResetError()

			select {
			case <-c.Ctx.Done():
				return fmt.Errorf("HealthCheck: context error: %w", c.Ctx.Err())
			case <-ticker.C:
			}

			c.Logger.Debug("Tunnel pool flushed: %v active connections", c.TunnelPool.Active())
		}

		if c.LBStrategy == "1" && len(c.TargetTCPAddrs) > 1 {
			c.ProbeBestTarget()
		}

		c.CheckPoint = time.Now()
		if c.Ctx.Err() == nil && c.ControlConn != nil {
			signalData, _ := json.Marshal(Signal{ActionType: "ping"})
			c.WriteChan <- c.Encode(signalData)
		}
		select {
		case <-c.Ctx.Done():
			return fmt.Errorf("HealthCheck: context error: %w", c.Ctx.Err())
		case <-ticker.C:
		}
	}

	return fmt.Errorf("HealthCheck: context error: %w", c.Ctx.Err())
}

func (c *Common) SingleControl() error {
	errChan := make(chan error, 3)

	if len(c.TargetTCPAddrs) > 0 {
		go func() { errChan <- c.SingleEventLoop() }()
	}
	if c.TunnelListener != nil || c.DisableTCP != "1" {
		go func() { errChan <- c.SingleTCPLoop() }()
	}
	if c.TunnelUDPConn != nil || c.DisableUDP != "1" {
		go func() { errChan <- c.SingleUDPLoop() }()
	}

	select {
	case <-c.Ctx.Done():
		return fmt.Errorf("SingleControl: context error: %w", c.Ctx.Err())
	case err := <-errChan:
		return fmt.Errorf("SingleControl: %w", err)
	}
}
