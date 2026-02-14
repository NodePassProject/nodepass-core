package server

import (
	"fmt"

	"github.com/NodePassProject/nph2"
	"github.com/NodePassProject/npws"
	"github.com/NodePassProject/pool"
	"github.com/NodePassProject/quic"
	"github.com/NodePassProject/nodepass/internal/common"
)

func (s *Server) initTunnelPool() error {
	switch s.PoolType {
	case "0":
		tcpPool := pool.NewServerPool(
			s.MaxPoolCapacity,
			s.ClientIP,
			s.TlsConfig,
			s.TunnelListener,
			common.ReportInterval)
		go tcpPool.ServerManager()
		s.TunnelPool = tcpPool
	case "1":
		quicPool := quic.NewServerPool(
			s.MaxPoolCapacity,
			s.ClientIP,
			s.TlsConfig,
			s.TunnelUDPAddr.String(),
			common.ReportInterval)
		go quicPool.ServerManager()
		s.TunnelPool = quicPool
	case "2":
		websocketPool := npws.NewServerPool(
			s.MaxPoolCapacity,
			"",
			s.TlsConfig,
			s.TunnelListener,
			common.ReportInterval)
		go websocketPool.ServerManager()
		s.TunnelPool = websocketPool
	case "3":
		http2Pool := nph2.NewServerPool(
			s.MaxPoolCapacity,
			s.ClientIP,
			s.TlsConfig,
			s.TunnelListener,
			common.ReportInterval)
		go http2Pool.ServerManager()
		s.TunnelPool = http2Pool
	default:
		return fmt.Errorf("initTunnelPool: unknown pool type: %s", s.PoolType)
	}
	return nil
}
