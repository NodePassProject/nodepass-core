package server

import (
	"context"
	"crypto/tls"
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

type Server struct{ common.Common }

func NewServer(parsedURL *url.URL, tlsCode string, tlsConfig *tls.Config, logger *logs.Logger) (*Server, error) {
	server := &Server{
		Common: common.Common{
			ParsedURL:  parsedURL,
			TlsCode:    tlsCode,
			TlsConfig:  tlsConfig,
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
	if err := server.InitConfig(); err != nil {
		return nil, fmt.Errorf("newServer: initConfig failed: %w", err)
	}
	server.InitRateLimiter()
	return server, nil
}

func (s *Server) Run() {
	logInfo := func(prefix string) {
		s.Logger.Info("%v: server://%v@%v/%v?dns=%v&lbs=%v&max=%v&mode=%v&type=%v&dial=%v&read=%v&rate=%v&slot=%v&proxy=%v&block=%v&notcp=%v&noudp=%v",
			prefix, s.TunnelKey, s.TunnelTCPAddr, s.GetTargetAddrsString(), s.DnsCacheTTL, s.LbStrategy, s.MaxPoolCapacity,
			s.RunMode, s.PoolType, s.DialerIP, s.ReadTimeout, s.RateLimit/125000, s.SlotLimit,
			s.ProxyProtocol, s.BlockProtocol, s.DisableTCP, s.DisableUDP)
	}
	logInfo("Server started")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	go func() {
		for ctx.Err() == nil {
			if err := s.start(); err != nil && err != io.EOF {
				s.Logger.Error("Server error: %v", err)
				s.Stop()
				select {
				case <-ctx.Done():
					return
				case <-time.After(common.ServiceCooldown):
				}
				logInfo("Server restart")
			}
		}
	}()

	<-ctx.Done()
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), common.ShutdownTimeout)
	defer cancel()
	if err := s.Shutdown(shutdownCtx, s.Stop); err != nil {
		s.Logger.Error("Server shutdown error: %v", err)
	} else {
		s.Logger.Info("Server shutdown complete")
	}
}

func (s *Server) start() error {
	s.InitContext()

	if err := s.InitTunnelListener(); err != nil {
		return fmt.Errorf("start: initTunnelListener failed: %w", err)
	}

	if s.TunnelUDPConn != nil {
		s.TunnelUDPConn.Close()
	}

	switch s.RunMode {
	case "1":
		if err := s.InitTargetListener(); err != nil {
			return fmt.Errorf("start: initTargetListener failed: %w", err)
		}
		s.DataFlow = "-"
	case "2":
		s.DataFlow = "+"
	default:
		if err := s.InitTargetListener(); err == nil {
			s.RunMode = "1"
			s.DataFlow = "-"
		} else {
			s.RunMode = "2"
			s.DataFlow = "+"
		}
	}

	s.Logger.Info("Pending tunnel handshake...")
	s.HandshakeStart = time.Now()
	if err := s.tunnelHandshake(); err != nil {
		return fmt.Errorf("start: tunnelHandshake failed: %w", err)
	}

	if err := s.initTunnelPool(); err != nil {
		return fmt.Errorf("start: initTunnelPool failed: %w", err)
	}

	s.Logger.Info("Getting tunnel pool ready...")
	if err := s.SetControlConn(); err != nil {
		return fmt.Errorf("start: setControlConn failed: %w", err)
	}

	if s.DataFlow == "-" {
		go s.TunnelLoop()
	}

	if err := s.CommonControl(); err != nil {
		return fmt.Errorf("start: commonControl failed: %w", err)
	}
	return nil
}
