package master

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/NodePassProject/logs"
	"github.com/NodePassProject/nodepass/internal/common"
)

const (
	defaultAPIPath  = "/api"
	openAPIVersion  = "v1"
	nextMCPVersion  = "v2"
	mcpVersion      = "2025-11-25"
	stateFilePath   = "gob"
	stateFileName   = "nodepass.gob"
	exportFileName  = "nodepass.json"
	sseRetryTime    = 3000
	apiKeyID        = "********"
	tcpingSemLimit  = 10
	baseDuration    = 100 * time.Millisecond
	gracefulTimeout = 5 * time.Second
	maxValueLen     = 256
)

func NewMaster(parsedURL *url.URL, tlsCode string, tlsConfig *tls.Config, logger *logs.Logger, version string) (*Master, error) {
	host, err := net.ResolveTCPAddr("tcp", parsedURL.Host)
	if err != nil {
		return nil, fmt.Errorf("newMaster: resolve host failed: %w", err)
	}

	var hostname string
	if tlsConfig != nil && tlsConfig.ServerName != "" {
		hostname = tlsConfig.ServerName
	} else {
		hostname = parsedURL.Hostname()
	}

	prefix := parsedURL.Path
	if prefix == "" || prefix == "/" {
		prefix = defaultAPIPath
	} else {
		prefix = strings.TrimRight(prefix, "/")
	}

	execPath, _ := os.Executable()
	baseDir := filepath.Dir(execPath)

	master := &Master{
		Common: common.Common{
			TlsCode: tlsCode,
			Logger:  logger,
		},
		prefix:        fmt.Sprintf("%s/%s", prefix, openAPIVersion),
		version:       version,
		logLevel:      parsedURL.Query().Get("log"),
		crtPath:       parsedURL.Query().Get("crt"),
		keyPath:       parsedURL.Query().Get("key"),
		hostname:      hostname,
		tlsConfig:     tlsConfig,
		masterURL:     parsedURL,
		statePath:     filepath.Join(baseDir, stateFilePath, stateFileName),
		notifyChannel: make(chan *InstanceEvent, common.SemaphoreLimit),
		tcpingSem:     make(chan struct{}, tcpingSemLimit),
		startTime:     time.Now(),
		periodicDone:  make(chan struct{}),
	}
	master.TunnelTCPAddr = host

	master.loadState()

	go master.startEventDispatcher()

	return master, nil
}

func (m *Master) Run() {
	m.Logger.Info("Master started: %v%v", m.TunnelTCPAddr, m.prefix)

	apiKey, ok := m.findInstance(apiKeyID)
	if !ok {
		apiKey = &Instance{
			ID:     apiKeyID,
			URL:    generateAPIKey(),
			Config: generateMID(),
			Meta:   Meta{Tags: make(map[string]string)},
		}
		m.instances.Store(apiKeyID, apiKey)
		m.saveState()
		fmt.Printf("%s  \033[32mINFO\033[0m  API Key created: %v\n", time.Now().Format("2006-01-02 15:04:05.000"), apiKey.URL)
	} else {
		m.alias = apiKey.Alias

		if apiKey.Config == "" {
			apiKey.Config = generateMID()
			m.instances.Store(apiKeyID, apiKey)
			m.saveState()
			m.Logger.Info("Master ID created: %v", apiKey.Config)
		}
		m.mid = apiKey.Config

		fmt.Printf("%s  \033[32mINFO\033[0m  API Key loaded: %v\n", time.Now().Format("2006-01-02 15:04:05.000"), apiKey.URL)
	}

	mux := http.NewServeMux()

	protectedEndpoints := map[string]http.HandlerFunc{
		fmt.Sprintf("%s/instances", m.prefix):  m.handleInstances,
		fmt.Sprintf("%s/instances/", m.prefix): m.handleInstanceDetail,
		fmt.Sprintf("%s/events", m.prefix):     m.handleSSE,
		fmt.Sprintf("%s/info", m.prefix):       m.handleInfo,
		fmt.Sprintf("%s/tcping", m.prefix):     m.handleTCPing,

		strings.TrimSuffix(m.prefix, "/"+openAPIVersion) + "/" + nextMCPVersion: m.handleMCP,
	}

	publicEndpoints := map[string]http.HandlerFunc{
		fmt.Sprintf("%s/openapi.json", m.prefix): m.handleOpenAPISpec,
		fmt.Sprintf("%s/docs", m.prefix):         m.handleSwaggerUI,
	}

	apiKeyMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			setCorsHeaders(w)
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			apiKeyInstance, keyExists := m.findInstance(apiKeyID)
			if keyExists && apiKeyInstance.URL != "" {
				reqAPIKey := r.Header.Get("X-API-Key")
				if reqAPIKey == "" {
					httpError(w, "Unauthorized: API key required", http.StatusUnauthorized)
					return
				}

				if reqAPIKey != apiKeyInstance.URL {
					httpError(w, "Unauthorized: Invalid API key", http.StatusUnauthorized)
					return
				}
			}

			next(w, r)
		}
	}

	corsMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			setCorsHeaders(w)
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next(w, r)
		}
	}

	for path, handler := range protectedEndpoints {
		mux.HandleFunc(path, apiKeyMiddleware(handler))
	}

	for path, handler := range publicEndpoints {
		mux.HandleFunc(path, corsMiddleware(handler))
	}

	m.server = &http.Server{
		Addr:      m.TunnelTCPAddr.String(),
		ErrorLog:  m.Logger.StdLogger(),
		Handler:   mux,
		TLSConfig: m.tlsConfig,
	}

	go func() {
		var err error
		if m.tlsConfig != nil {
			err = m.server.ListenAndServeTLS("", "")
		} else {
			err = m.server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			m.Logger.Error("run: listen failed: %v", err)
		}
	}()

	go m.startPeriodicTasks()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	<-ctx.Done()
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), common.ShutdownTimeout)
	defer cancel()
	if err := m.MasterShutdown(shutdownCtx); err != nil {
		m.Logger.Error("Master shutdown error: %v", err)
	} else {
		m.Logger.Info("Master shutdown complete")
	}
}

func (m *Master) MasterShutdown(ctx context.Context) error {
	return m.Shutdown(ctx, func() {
		m.shutdownSSEConnections()

		var wg sync.WaitGroup
		m.instances.Range(func(key, value any) bool {
			instance := value.(*Instance)
			if instance.Status != "stopped" && instance.cmd != nil && instance.cmd.Process != nil {
				wg.Add(1)
				go func(inst *Instance) {
					defer wg.Done()
					m.stopInstance(inst)
				}(instance)
			}
			return true
		})

		wg.Wait()

		close(m.periodicDone)

		close(m.notifyChannel)

		if err := m.saveState(); err != nil {
			m.Logger.Error("shutdown: save gob failed: %v", err)
		} else {
			m.Logger.Info("Instances saved: %v", m.statePath)
		}

		if err := m.server.Shutdown(ctx); err != nil {
			m.Logger.Error("shutdown: api shutdown error: %v", err)
		}
	})
}
