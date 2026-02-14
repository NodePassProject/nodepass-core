package client

import (
	"fmt"
	"net"

	"github.com/NodePassProject/nph2"
	"github.com/NodePassProject/npws"
	"github.com/NodePassProject/pool"
	"github.com/NodePassProject/quic"
	"github.com/NodePassProject/nodepass/internal/common"
)

func (c *Client) initTunnelPool() error {
	switch c.PoolType {
	case "0":
		tcpPool := pool.NewClientPool(
			c.MinPoolCapacity,
			c.MaxPoolCapacity,
			common.MinPoolInterval,
			common.MaxPoolInterval,
			common.ReportInterval,
			c.TlsCode,
			c.ServerName,
			func() (net.Conn, error) {
				tcpAddr, err := c.GetTunnelTCPAddr()
				if err != nil {
					return nil, err
				}
				return net.DialTimeout("tcp", tcpAddr.String(), common.TcpDialTimeout)
			})
		go tcpPool.ClientManager()
		c.TunnelPool = tcpPool
	case "1":
		quicPool := quic.NewClientPool(
			c.MinPoolCapacity,
			c.MaxPoolCapacity,
			common.MinPoolInterval,
			common.MaxPoolInterval,
			common.ReportInterval,
			c.TlsCode,
			c.ServerName,
			func() (string, error) {
				udpAddr, err := c.GetTunnelUDPAddr()
				if err != nil {
					return "", err
				}
				return udpAddr.String(), nil
			})
		go quicPool.ClientManager()
		c.TunnelPool = quicPool
	case "2":
		websocketPool := npws.NewClientPool(
			c.MinPoolCapacity,
			c.MaxPoolCapacity,
			common.MinPoolInterval,
			common.MaxPoolInterval,
			common.ReportInterval,
			c.TlsCode,
			c.TunnelAddr)
		go websocketPool.ClientManager()
		c.TunnelPool = websocketPool
	case "3":
		http2Pool := nph2.NewClientPool(
			c.MinPoolCapacity,
			c.MaxPoolCapacity,
			common.MinPoolInterval,
			common.MaxPoolInterval,
			common.ReportInterval,
			c.TlsCode,
			c.ServerName,
			func() (string, error) {
				tcpAddr, err := c.GetTunnelTCPAddr()
				if err != nil {
					return "", err
				}
				return tcpAddr.String(), nil
			})
		go http2Pool.ClientManager()
		c.TunnelPool = http2Pool
	default:
		return fmt.Errorf("initTunnelPool: unknown pool type: %s", c.PoolType)
	}
	return nil
}
