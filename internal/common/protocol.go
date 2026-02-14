package common

import (
	"bufio"
	"fmt"
	"net"
)

func (c *Common) SendProxyV1Header(ip string, conn net.Conn) error {
	if c.ProxyProtocol != "1" {
		return nil
	}

	clientAddr, err := net.ResolveTCPAddr("tcp", ip)
	if err != nil {
		return fmt.Errorf("sendProxyV1Header: resolveTCPAddr failed: %w", err)
	}
	remoteAddr, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok {
		return fmt.Errorf("sendProxyV1Header: remote address is not TCPAddr")
	}

	var protocol string
	switch {
	case clientAddr.IP.To4() != nil && remoteAddr.IP.To4() != nil:
		protocol = "TCP4"
	case clientAddr.IP.To16() != nil && remoteAddr.IP.To16() != nil:
		protocol = "TCP6"
	default:
		return fmt.Errorf("sendProxyV1Header: unsupported IP protocol for PROXY v1")
	}

	if _, err = fmt.Fprintf(conn, "PROXY %s %s %s %d %d\r\n",
		protocol,
		clientAddr.IP.String(),
		remoteAddr.IP.String(),
		clientAddr.Port,
		remoteAddr.Port); err != nil {
		return fmt.Errorf("sendProxyV1Header: fprintf failed: %w", err)
	}

	return nil
}

func (c *Common) DetectBlockProtocol(conn net.Conn) (string, net.Conn) {
	if !c.BlockSOCKS && !c.BlockHTTP && !c.BlockTLS {
		return "", conn
	}

	reader := bufio.NewReader(conn)
	b, err := reader.Peek(8)
	if err != nil || len(b) < 1 {
		return "", &ReaderConn{Conn: conn, Reader: reader}
	}

	if c.BlockSOCKS && len(b) >= 2 {
		if b[0] == 0x04 && (b[1] == 0x01 || b[1] == 0x02) {
			return "SOCKS4", &ReaderConn{Conn: conn, Reader: reader}
		}
		if b[0] == 0x05 && b[1] >= 0x01 && b[1] <= 0x03 {
			return "SOCKS5", &ReaderConn{Conn: conn, Reader: reader}
		}
	}

	if c.BlockHTTP && len(b) >= 4 && b[0] >= 'A' && b[0] <= 'Z' {
		for i, c := range b[1:] {
			if c == ' ' {
				return "HTTP", &ReaderConn{Conn: conn, Reader: reader}
			}
			if c < 'A' || c > 'Z' || i >= 7 {
				break
			}
		}
	}

	if c.BlockTLS && b[0] == 0x16 {
		return "TLS", &ReaderConn{Conn: conn, Reader: reader}
	}

	return "", &ReaderConn{Conn: conn, Reader: reader}
}
