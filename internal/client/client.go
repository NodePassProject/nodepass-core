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
			TCPBufferPool: &sync.Pool{
				New: func() any {
					buf := make([]byte, common.TCPDataBufSize)
					return &buf
				},
			},
			UDPBufferPool: &sync.Pool{
				New: func() any {
					buf := make([]byte, common.UDPDataBufSize)
					return &buf
				},
			},
		},
	}
	if err := client.InitConfig(); err != nil {
		return nil, fmt.Errorf("NewClient: initConfig failed: %w", err)
	}
	client.InitRateLimiter()
	return client, nil
}

func (c *Client) Run() {
	logInfo := func(prefix string) {
		c.Logger.Info("%v: client://%v@%v/%v?dns=%v&sni=%v&lbs=%v&min=%v&mode=%v&dial=%v&read=%v&rate=%v&slot=%v&proxy=%v&block=%v&notcp=%v&noudp=%v",
			prefix, c.TunnelKey, c.TunnelTCPAddr, c.GetTargetAddrsString(), c.DNSCacheTTL, c.ServerName, c.LBStrategy, c.MinPoolCapacity,
			c.RunMode, c.DialerIP, c.ReadTimeout, c.RateLimit/125000, c.SlotLimit,
			c.ProxyProtocol, c.BlockProtocol, c.DisableTCP, c.DisableUDP)
	}
	logInfo("Client started")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	go func() {
		for ctx.Err() == nil {
			if err := c.Start(); err != nil && err != io.EOF {
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
	if err := c.CommonShutdown(shutdownCtx, c.Stop); err != nil {
		c.Logger.Error("Client shutdown error: %v", err)
	} else {
		c.Logger.Info("Client shutdown complete")
	}
}

func (c *Client) Start() error {
	c.InitContext()

	switch c.RunMode {
	case "1":
		if err := c.InitTunnelListener(); err == nil {
			return c.SingleStart()
		} else {
			return fmt.Errorf("Start: initTunnelListener failed: %w", err)
		}
	case "2":
		return c.CommonStart()
	default:
		if err := c.InitTunnelListener(); err == nil {
			c.RunMode = "1"
			return c.SingleStart()
		} else {
			c.RunMode = "2"
			return c.CommonStart()
		}
	}
}

func (c *Client) SingleStart() error {
	if err := c.SingleControl(); err != nil {
		return fmt.Errorf("SingleStart: singleControl failed: %w", err)
	}
	return nil
}

func (c *Client) CommonStart() error {
	c.Logger.Info("Pending tunnel handshake...")
	c.HandshakeStart = time.Now()
	if err := c.TunnelHandshake(); err != nil {
		return fmt.Errorf("CommonStart: tunnelHandshake failed: %w", err)
	}

	if err := c.InitTunnelPool(); err != nil {
		return fmt.Errorf("CommonStart: initTunnelPool failed: %w", err)
	}

	c.Logger.Info("Getting tunnel pool ready...")
	if err := c.SetControlConn(); err != nil {
		return fmt.Errorf("CommonStart: setControlConn failed: %w", err)
	}

	if c.DataFlow == "+" {
		if err := c.InitTargetListener(); err != nil {
			return fmt.Errorf("CommonStart: initTargetListener failed: %w", err)
		}
		go c.TunnelLoop()
	}

	if err := c.CommonControl(); err != nil {
		return fmt.Errorf("CommonStart: commonControl failed: %w", err)
	}

	return nil
}
