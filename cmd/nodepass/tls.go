package main

import (
	"crypto/tls"
	"net/url"
	"time"

	"github.com/NodePassProject/logs"
	"github.com/NodePassProject/nodepass/internal/common"
)

func getTLSProtocol(parsedURL *url.URL, logger *logs.Logger) (string, *tls.Config) {
	tlsConfig, err := common.NewTLSConfig()
	if err != nil {
		logger.Error("Generate TLS config failed: %v", err)
		logger.Warn("TLS code-0: nil cert")
		return "0", nil
	}

	tlsConfig.MinVersion = tls.VersionTLS13

	switch parsedURL.Query().Get("tls") {
	case "1":
		logger.Info("TLS code-1: RAM cert with TLS 1.3")
		return "1", tlsConfig
	case "2":
		crtFile, keyFile := parsedURL.Query().Get("crt"), parsedURL.Query().Get("key")
		cert, err := tls.LoadX509KeyPair(crtFile, keyFile)
		if err != nil {
			logger.Error("Certificate load failed: %v", err)
			logger.Warn("TLS code-1: RAM cert with TLS 1.3")
			return "1", tlsConfig
		}

		cachedCert := cert
		lastReload := time.Now()
		tlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS13,
			GetCertificate: func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
				if time.Since(lastReload) >= common.ReloadInterval {
					newCert, err := tls.LoadX509KeyPair(crtFile, keyFile)
					if err != nil {
						logger.Error("Certificate reload failed: %v", err)
					} else {
						logger.Debug("TLS cert reloaded: %v", crtFile)
						cachedCert = newCert
					}
					lastReload = time.Now()
				}
				return &cachedCert, nil
			},
		}

		if cert.Leaf != nil {
			logger.Info("TLS code-2: %v with TLS 1.3", cert.Leaf.Subject.CommonName)
		} else {
			logger.Warn("TLS code-2: unknown cert name with TLS 1.3")
		}
		return "2", tlsConfig
	default:
		if poolType := parsedURL.Query().Get("type"); poolType == "1" || poolType == "3" {
			logger.Info("TLS code-1: RAM cert with TLS 1.3 for stream pool")
			return "1", tlsConfig
		}
		logger.Warn("TLS code-0: unencrypted")
		return "0", nil
	}
}
