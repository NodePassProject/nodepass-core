package server

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/NodePassProject/nodepass/internal/common"
)

func (s *Server) tunnelHandshake() error {
	var clientIP string
	done := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Connection", "close")
			if r.URL.Path != "/" {
				http.Error(w, "Not Found", http.StatusNotFound)
				return
			}

			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") || !s.VerifyAuthToken(strings.TrimPrefix(auth, "Bearer ")) {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			clientIP = r.RemoteAddr
			if host, _, err := net.SplitHostPort(clientIP); err == nil {
				clientIP = host
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"flow": s.DataFlow,
				"max":  s.MaxPoolCapacity,
				"tls":  s.TlsCode,
				"type": s.PoolType,
			})

			s.Logger.Info("Sending tunnel config: FLOW=%v|MAX=%v|TLS=%v|TYPE=%v",
				s.DataFlow, s.MaxPoolCapacity, s.TlsCode, s.PoolType)

			close(done)
		case http.MethodConnect:
			if !s.VerifyPreAuth(r) {
				w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			s.HandlePreAuth(w, r)
		default:
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
	})

	tlsConfig := s.TlsConfig
	if tlsConfig == nil {
		tlsConfig, _ = common.NewTLSConfig()
	}

	if len(tlsConfig.Certificates) > 0 && len(tlsConfig.Certificates[0].Certificate) > 0 {
		fingerprint := s.FormatCertFingerprint(tlsConfig.Certificates[0].Certificate[0])
		s.Logger.Info("TLS cert fingerprint for authorization: %v", fingerprint)
	}

	server := &http.Server{
		Handler:   handler,
		TLSConfig: tlsConfig,
		ErrorLog:  s.Logger.StdLogger(),
	}
	go server.ServeTLS(s.TunnelListener, "", "")

	select {
	case <-done:
		server.Close()
		s.ClientIP = clientIP
		if s.TlsCode == "1" {
			if newTLSConfig, err := common.NewTLSConfig(); err == nil {
				newTLSConfig.MinVersion = tls.VersionTLS13
				s.TlsConfig = newTLSConfig
				s.Logger.Info("TLS code-1: RAM cert regenerated with TLS 1.3")
			} else {
				s.Logger.Warn("Failed to regenerate RAM cert: %v", err)
			}
		}

		s.TunnelListener, _ = net.ListenTCP("tcp", s.TunnelTCPAddr)
		return nil
	case <-s.Ctx.Done():
		server.Close()
		return fmt.Errorf("tunnelHandshake: context canceled")
	}
}
