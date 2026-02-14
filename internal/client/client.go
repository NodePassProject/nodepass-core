package client

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/NodePassProject/logs"
	"github.com/NodePassProject/nodepass/internal/common"
)

type Client struct{ common.Common }

func NewClient(parsedURL *url.URL, logger *logs.Logger) (*Client, error) {
	client := &Client{
		Common: common.Common{
			ParsedURL:  parsedURL,
			Logger:     logger,
			SignalChan: make(chan common.Signal, common.SemaphoreLimit),
			WriteChan:  make(chan []byte, common.SemaphoreLimit),
			VerifyChan: make(chan struct{}),
			TcpBufferPool: &sync.Pool{
				New: func() any {
					buf := make([]byte, common.TcpDataBufSize)
					return &buf
				},
			},
			UdpBufferPool: &sync.Pool{
				New: func() any {
					buf := make([]byte, common.UdpDataBufSize)
					return &buf
				},
			},
		},
	}
	if err := client.InitConfig(); err != nil {
		return nil, fmt.Errorf("newClient: initConfig failed: %w", err)
	}
	client.InitRateLimiter()
	return client, nil
}

func (c *Client) Run() {
	logInfo := func(prefix string) {
		c.Logger.Info("%v: client://%v@%v/%v?dns=%v&sni=%v&lbs=%v&min=%v&mode=%v&dial=%v&read=%v&rate=%v&slot=%v&proxy=%v&block=%v&notcp=%v&noudp=%v",
			prefix, c.TunnelKey, c.TunnelTCPAddr, c.GetTargetAddrsString(), c.DnsCacheTTL, c.ServerName, c.LbStrategy, c.MinPoolCapacity,
			c.RunMode, c.DialerIP, c.ReadTimeout, c.RateLimit/125000, c.SlotLimit,
			c.ProxyProtocol, c.BlockProtocol, c.DisableTCP, c.DisableUDP)
	}
	logInfo("Client started")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	go func() {
		for ctx.Err() == nil {
			if err := c.start(); err != nil && err != io.EOF {
				c.Logger.Error("Client error: %v", err)
				c.Stop()
				select {
				case <-ctx.Done():
					return
				case <-time.After(common.ServiceCooldown):
				}
				logInfo("Client restart")
			}
		}
	}()

	<-ctx.Done()
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), common.ShutdownTimeout)
	defer cancel()
	if err := c.Shutdown(shutdownCtx, c.Stop); err != nil {
		c.Logger.Error("Client shutdown error: %v", err)
	} else {
		c.Logger.Info("Client shutdown complete")
	}
}

func (c *Client) start() error {
	c.InitContext()

	switch c.RunMode {
	case "1":
		if err := c.InitTunnelListener(); err == nil {
			return c.singleStart()
		} else {
			return fmt.Errorf("start: initTunnelListener failed: %w", err)
		}
	case "2":
		return c.commonStart()
	default:
		if err := c.InitTunnelListener(); err == nil {
			c.RunMode = "1"
			return c.singleStart()
		} else {
			c.RunMode = "2"
			return c.commonStart()
		}
	}
}

func (c *Client) singleStart() error {
	if err := c.SingleControl(); err != nil {
		return fmt.Errorf("singleStart: singleControl failed: %w", err)
	}
	return nil
}

func (c *Client) commonStart() error {
	c.Logger.Info("Pending tunnel handshake...")
	c.HandshakeStart = time.Now()
	if err := c.tunnelHandshake(); err != nil {
		return fmt.Errorf("commonStart: tunnelHandshake failed: %w", err)
	}

	if err := c.initTunnelPool(); err != nil {
		return fmt.Errorf("commonStart: initTunnelPool failed: %w", err)
	}

	c.Logger.Info("Getting tunnel pool ready...")
	if err := c.SetControlConn(); err != nil {
		return fmt.Errorf("commonStart: setControlConn failed: %w", err)
	}

	if c.DataFlow == "+" {
		if err := c.InitTargetListener(); err != nil {
			return fmt.Errorf("commonStart: initTargetListener failed: %w", err)
		}
		go c.TunnelLoop()
	}

	if err := c.CommonControl(); err != nil {
		return fmt.Errorf("commonStart: commonControl failed: %w", err)
	}

	return nil
}
