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
	DefaultAPIPath  = "/api"
	OpenAPIVersion  = "v1"
	NextMCPVersion  = "v2"
	MCPVersion      = "2025-11-25"
	StateFilePath   = "gob"
	StateFileName   = "nodepass.gob"
	ExportFileName  = "nodepass.json"
	SSERetryTime    = 3000
	APIKeyID        = "********"
	PingSemLimit    = 10
	BaseDuration    = 100 * time.Millisecond
	GracefulTimeout = 5 * time.Second
	MaxValueLen     = 256
)

func NewMaster(parsedURL *url.URL, tlsCode string, tlsConfig *tls.Config, logger *logs.Logger, version string) (*Master, error) {
	host, err := net.ResolveTCPAddr("tcp", parsedURL.Host)
	if err != nil {
		return nil, fmt.Errorf("NewMaster: resolve host failed: %w", err)
	}

	var hostname string
	if tlsConfig != nil && tlsConfig.ServerName != "" {
		hostname = tlsConfig.ServerName
	} else {
		hostname = parsedURL.Hostname()
	}

	prefix := parsedURL.Path
	if prefix == "" || prefix == "/" {
		prefix = DefaultAPIPath
	} else {
		prefix = strings.TrimRight(prefix, "/")
	}

	execPath, _ := os.Executable()
	baseDir := filepath.Dir(execPath)

	master := &Master{
		Common: common.Common{
			TLSCode: tlsCode,
			Logger:  logger,
		},
		Prefix:        fmt.Sprintf("%s/%s", prefix, OpenAPIVersion),
		Version:       version,
		LogLevel:      parsedURL.Query().Get("log"),
		CrtPath:       parsedURL.Query().Get("crt"),
		KeyPath:       parsedURL.Query().Get("key"),
		Hostname:      hostname,
		MTLSConfig:    tlsConfig,
		MasterURL:     parsedURL,
		StatePath:     filepath.Join(baseDir, StateFilePath, StateFileName),
		NotifyChannel: make(chan *InstanceEvent, common.SemaphoreLimit),
		TCPingSem:     make(chan struct{}, PingSemLimit),
		StartTime:     time.Now(),
		PeriodicDone:  make(chan struct{}),
	}
	master.TunnelTCPAddr = host

	master.LoadState()

	go master.StartEventDispatcher()

	return master, nil
}

func (m *Master) Run() {
	m.Logger.Info("Master started: %v%v", m.TunnelTCPAddr, m.Prefix)
	apiKey, ok := m.FindInstance(APIKeyID)
	if !ok {
		apiKey = &Instance{
			ID:     APIKeyID,
			URL:    GenerateAPIKey(),
			Config: GenerateMID(),
			Meta:   Meta{Tags: make(map[string]string)},
		}
		m.Instances.Store(APIKeyID, apiKey)
		m.SaveState()
		fmt.Printf("%s  \033[32mINFO\033[0m  API Key created: %v\n", time.Now().Format("2006-01-02 15:04:05.000"), apiKey.URL)
	} else {
		m.Alias = apiKey.Alias

		if apiKey.Config == "" {
			apiKey.Config = GenerateMID()
			m.Instances.Store(APIKeyID, apiKey)
			m.SaveState()
			m.Logger.Info("Master ID created: %v", apiKey.Config)
		}
		m.MID = apiKey.Config

		fmt.Printf("%s  \033[32mINFO\033[0m  API Key loaded: %v\n", time.Now().Format("2006-01-02 15:04:05.000"), apiKey.URL)
	}

	mux := http.NewServeMux()

	protectedEndpoints := map[string]http.HandlerFunc{
		fmt.Sprintf("%s/instances", m.Prefix):  m.HandleInstances,
		fmt.Sprintf("%s/instances/", m.Prefix): m.HandleInstanceDetail,
		fmt.Sprintf("%s/events", m.Prefix):     m.HandleSSE,
		fmt.Sprintf("%s/info", m.Prefix):       m.HandleInfo,
		fmt.Sprintf("%s/tcping", m.Prefix):     m.HandleTCPing,

		strings.TrimSuffix(m.Prefix, "/"+OpenAPIVersion) + "/" + NextMCPVersion: m.HandleMCP,
	}

	publicEndpoints := map[string]http.HandlerFunc{
		fmt.Sprintf("%s/openapi.json", m.Prefix): m.HandleOpenAPISpec,
		fmt.Sprintf("%s/docs", m.Prefix):         m.HandleSwaggerUI,
	}

	apiKeyMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			SetCorsHeaders(w)
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			apiKeyInstance, keyExists := m.FindInstance(APIKeyID)
			if keyExists && apiKeyInstance.URL != "" {
				reqAPIKey := r.Header.Get("X-API-Key")
				if reqAPIKey == "" {
					HTTPError(w, "Unauthorized: API key required", http.StatusUnauthorized)
					return
				}

				if reqAPIKey != apiKeyInstance.URL {
					HTTPError(w, "Unauthorized: Invalid API key", http.StatusUnauthorized)
					return
				}
			}

			next(w, r)
		}
	}

	corsMiddleware := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			SetCorsHeaders(w)
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

	m.Server = &http.Server{
		Addr:      m.TunnelTCPAddr.String(),
		ErrorLog:  m.Logger.StdLogger(),
		Handler:   mux,
		TLSConfig: m.MTLSConfig,
	}

	go func() {
		var err error
		if m.MTLSConfig != nil {
			err = m.Server.ListenAndServeTLS("", "")
		} else {
			err = m.Server.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			m.Logger.Error("Run: listen failed: %v", err)
		}
	}()

	go m.StartPeriodicTasks()

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
	return m.CommonShutdown(ctx, func() {
		m.ShutdownSSEConnections()

		var wg sync.WaitGroup
		m.Instances.Range(func(key, value any) bool {
			instance := value.(*Instance)
			if instance.Status != "stopped" && instance.cmd != nil && instance.cmd.Process != nil {
				wg.Add(1)
				go func(inst *Instance) {
					defer wg.Done()
					m.StopInstance(inst)
				}(instance)
			}
			return true
		})

		wg.Wait()

		close(m.PeriodicDone)

		close(m.NotifyChannel)

		if err := m.SaveState(); err != nil {
			m.Logger.Error("MasterShutdown: save gob failed: %v", err)
		} else {
			m.Logger.Info("Instances saved: %v", m.StatePath)
		}

		if err := m.Server.Shutdown(ctx); err != nil {
			m.Logger.Error("MasterShutdown: api shutdown error: %v", err)
		}
	})
}
