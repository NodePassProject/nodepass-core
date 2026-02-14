package common

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/NodePassProject/conn"
)

func NewTLSConfig() (*tls.Config, error) {
	private, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(1, 0, 0),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	crtBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &private.PublicKey, private)
	if err != nil {
		return nil, err
	}

	keyBytes, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return nil, err
	}

	crtPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: crtBytes})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})

	cert, err := tls.X509KeyPair(crtPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
}

func (c *Common) FormatCertFingerprint(certRaw []byte) string {
	hash := sha256.Sum256(certRaw)
	return "sha256:" + hex.EncodeToString(hash[:])
}

func (c *Common) Xor(data []byte) []byte {
	for i := range data {
		data[i] ^= c.TunnelKey[i%len(c.TunnelKey)]
	}
	return data
}

func (c *Common) GenerateAuthToken() string {
	return hex.EncodeToString(hmac.New(sha256.New, []byte(c.TunnelKey)).Sum(nil))
}

func (c *Common) VerifyAuthToken(token string) bool {
	return hmac.Equal([]byte(token), []byte(c.GenerateAuthToken()))
}

func (c *Common) VerifyPreAuth(r *http.Request) bool {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(r.Header.Get("Proxy-Authorization"), "Basic "))
	return err == nil && strings.HasPrefix(string(decoded), c.TunnelKey+":")
}

func (c *Common) HandlePreAuth(w http.ResponseWriter, r *http.Request) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		return
	}
	clientConn, _, err := hj.Hijack()
	if err != nil {
		return
	}
	defer clientConn.Close()

	targetConn, err := net.DialTimeout("tcp", r.URL.Host, TcpDialTimeout)
	if err != nil {
		return
	}
	defer targetConn.Close()

	clientConn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))

	buffer1 := c.GetTCPBuffer()
	buffer2 := c.GetTCPBuffer()
	defer func() {
		c.PutTCPBuffer(buffer1)
		c.PutTCPBuffer(buffer2)
	}()

	conn.DataExchange(clientConn, targetConn, c.ReadTimeout, buffer1, buffer2)
}

func (c *Common) Encode(data []byte) []byte {
	return append([]byte(base64.StdEncoding.EncodeToString(c.Xor(data))), '\n')
}

func (c *Common) Decode(data []byte) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(string(bytes.TrimSuffix(data, []byte{'\n'})))
	if err != nil {
		return nil, fmt.Errorf("decode: base64 decode failed: %w", err)
	}
	return c.Xor(decoded), nil
}

func (c *Common) IncomingVerify() {
	for c.Ctx.Err() == nil {
		if c.TunnelPool.Ready() && c.TunnelPool.Active() > 0 {
			break
		}
		select {
		case <-c.Ctx.Done():
			continue
		case <-time.After(ContextCheckInterval):
		}
	}

	id, testConn, err := c.TunnelPool.IncomingGet(PoolGetTimeout)
	if err != nil {
		c.Logger.Error("incomingVerify: incomingGet failed: %v", err)
		c.Cancel()
		return
	}
	defer testConn.Close()

	var fingerprint string
	switch c.CoreType {
	case "server":
		if c.TlsConfig != nil && len(c.TlsConfig.Certificates) > 0 {
			cert := c.TlsConfig.Certificates[0]
			if len(cert.Certificate) > 0 {
				fingerprint = c.FormatCertFingerprint(cert.Certificate[0])
			}
		}
	case "client":
		if conn, ok := testConn.(interface{ ConnectionState() tls.ConnectionState }); ok {
			state := conn.ConnectionState()
			if len(state.PeerCertificates) > 0 {
				fingerprint = c.FormatCertFingerprint(state.PeerCertificates[0].Raw)
			}
		}
	}

	if c.Ctx.Err() == nil && c.ControlConn != nil {
		signalData, _ := json.Marshal(Signal{
			ActionType:  "verify",
			PoolConnID:  id,
			Fingerprint: fingerprint,
		})
		c.WriteChan <- c.Encode(signalData)
	}

	c.Logger.Debug("TLS code-1: verify signal: cid %v -> %v", id, c.ControlConn.RemoteAddr())
}

func (c *Common) OutgoingVerify(signal Signal) {
	for c.Ctx.Err() == nil {
		if c.TunnelPool.Ready() {
			break
		}
		select {
		case <-c.Ctx.Done():
			continue
		case <-time.After(ContextCheckInterval):
		}
	}

	fingerPrint := signal.Fingerprint
	if fingerPrint == "" {
		c.Logger.Error("outgoingVerify: no fingerprint in signal")
		c.Cancel()
		return
	}

	id := signal.PoolConnID
	c.Logger.Debug("TLS verify signal: cid %v <- %v", id, c.ControlConn.RemoteAddr())

	testConn, err := c.TunnelPool.OutgoingGet(id, PoolGetTimeout)
	if err != nil {
		c.Logger.Error("outgoingVerify: request timeout: %v", err)
		c.Cancel()
		return
	}
	defer testConn.Close()

	var serverFingerprint, clientFingerprint string
	switch c.CoreType {
	case "server":
		if c.TlsConfig == nil || len(c.TlsConfig.Certificates) == 0 {
			c.Logger.Error("outgoingVerify: no local certificate")
			c.Cancel()
			return
		}

		cert := c.TlsConfig.Certificates[0]
		if len(cert.Certificate) == 0 {
			c.Logger.Error("outgoingVerify: empty local certificate")
			c.Cancel()
			return
		}

		serverFingerprint = c.FormatCertFingerprint(cert.Certificate[0])
		clientFingerprint = fingerPrint
	case "client":
		conn, ok := testConn.(interface{ ConnectionState() tls.ConnectionState })
		if !ok {
			return
		}
		state := conn.ConnectionState()

		if len(state.PeerCertificates) == 0 {
			c.Logger.Error("outgoingVerify: no peer certificates found")
			c.Cancel()
			return
		}

		clientFingerprint = c.FormatCertFingerprint(state.PeerCertificates[0].Raw)
		serverFingerprint = fingerPrint
	}

	if serverFingerprint != clientFingerprint {
		c.Logger.Error("outgoingVerify: certificate fingerprint mismatch: server: %v - client: %v", serverFingerprint, clientFingerprint)
		c.Cancel()
		return
	}

	c.Logger.Info("TLS code-1: RAM cert fingerprint verified: %v", fingerPrint)

	c.VerifyChan <- struct{}{}
}
