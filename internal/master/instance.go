package master

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/NodePassProject/nodepass/internal/common"
)

func (m *Master) findInstance(id string) (*Instance, bool) {
	value, exists := m.instances.Load(id)
	if !exists {
		return nil, false
	}
	return value.(*Instance), true
}

func (m *Master) startInstance(instance *Instance) {
	if value, exists := m.instances.Load(instance.ID); exists {
		instance = value.(*Instance)
		if instance.Status != "stopped" {
			return
		}
	}

	instance.TCPRXBase = instance.TCPRX
	instance.TCPTXBase = instance.TCPTX
	instance.UDPRXBase = instance.UDPRX
	instance.UDPTXBase = instance.UDPTX

	execPath, err := os.Executable()
	if err != nil {
		m.Logger.Error("startInstance: get path failed: %v [%v]", err, instance.ID)
		instance.Status = "error"
		m.instances.Store(instance.ID, instance)
		m.sendSSEEvent("update", instance)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, execPath, instance.URL)
	instance.CancelFunc = cancel

	writer := NewInstanceLogWriter(instance.ID, instance, os.Stdout, m)
	cmd.Stdout, cmd.Stderr = writer, writer

	m.Logger.Info("Instance starting: %v [%v]", instance.URL, instance.ID)

	if err := cmd.Start(); err != nil || cmd.Process == nil || cmd.Process.Pid <= 0 {
		if err != nil {
			m.Logger.Error("startInstance: instance error: %v [%v]", err, instance.ID)
		} else {
			m.Logger.Error("startInstance: instance start failed [%v]", instance.ID)
		}
		instance.Status = "error"
		m.instances.Store(instance.ID, instance)
		m.sendSSEEvent("update", instance)
		cancel()
		return
	}

	instance.Cmd = cmd
	instance.Status = "running"
	go m.monitorInstance(instance, cmd)

	m.instances.Store(instance.ID, instance)

	m.sendSSEEvent("update", instance)
}

func (m *Master) monitorInstance(instance *Instance, cmd *exec.Cmd) {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	for {
		select {
		case <-instance.Stopped:
			return
		case err := <-done:
			if value, exists := m.instances.Load(instance.ID); exists {
				instance = value.(*Instance)
				if instance.Status == "running" {
					if err != nil {
						m.Logger.Error("monitorInstance: instance error: %v [%v]", err, instance.ID)
						instance.Status = "error"
					} else {
						instance.Status = "stopped"
					}
					m.instances.Store(instance.ID, instance)
					m.sendSSEEvent("update", instance)
				}
			}
			return
		case <-time.After(common.ReportInterval):
			if !instance.LastCheckPoint.IsZero() && time.Since(instance.LastCheckPoint) > 3*common.ReportInterval {
				instance.Status = "error"
				m.instances.Store(instance.ID, instance)
				m.sendSSEEvent("update", instance)
			}
		}
	}
}

func (m *Master) stopInstance(instance *Instance) {
	if instance.Status == "stopped" {
		return
	}

	if instance.Cmd == nil || instance.Cmd.Process == nil {
		instance.Status = "stopped"
		m.instances.Store(instance.ID, instance)
		m.sendSSEEvent("update", instance)
		return
	}

	select {
	case <-instance.Stopped:
	default:
		close(instance.Stopped)
	}

	process := instance.Cmd.Process
	if runtime.GOOS == "windows" {
		process.Signal(os.Interrupt)
	} else {
		process.Signal(syscall.SIGTERM)
	}

	if instance.CancelFunc != nil {
		instance.CancelFunc()
	}

	done := make(chan struct{})
	go func() {
		process.Wait()
		close(done)
	}()

	select {
	case <-done:
		m.Logger.Info("Instance stopped [%v]", instance.ID)
	case <-time.After(gracefulTimeout):
		process.Kill()
		<-done
		m.Logger.Warn("Instance force killed [%v]", instance.ID)
	}

	instance.Status = "stopped"
	instance.Stopped = make(chan struct{})
	instance.CancelFunc = nil
	instance.Ping = 0
	instance.Pool = 0
	instance.TCPS = 0
	instance.UDPS = 0
	m.instances.Store(instance.ID, instance)

	go m.saveState()

	m.sendSSEEvent("update", instance)
}

func (m *Master) processInstanceAction(instance *Instance, action string) {
	switch action {
	case "start":
		if instance.Status == "stopped" {
			go m.startInstance(instance)
		}
	case "stop":
		if instance.Status != "stopped" {
			go m.stopInstance(instance)
		}
	case "restart":
		go func() {
			m.stopInstance(instance)
			time.Sleep(baseDuration)
			m.startInstance(instance)
		}()
	}
}

func (m *Master) regenerateAPIKey(instance *Instance) {
	instance.URL = generateAPIKey()
	m.instances.Store(apiKeyID, instance)
	fmt.Printf("%s  \033[32mINFO\033[0m  API Key regenerated: %v\n", time.Now().Format("2006-01-02 15:04:05.000"), instance.URL)
	go m.saveState()
	go m.shutdownSSEConnections()
}

func (m *Master) enhanceURL(instanceURL string, instanceRole string) string {
	parsedURL, err := url.Parse(instanceURL)
	if err != nil {
		m.Logger.Error("enhanceURL: invalid URL format: %v", err)
		return instanceURL
	}

	query := parsedURL.Query()

	if m.logLevel != "" && query.Get("log") == "" {
		query.Set("log", m.logLevel)
	}

	if instanceRole == "server" && m.TlsCode != "0" {
		if query.Get("tls") == "" {
			query.Set("tls", m.TlsCode)
		}

		if m.TlsCode == "2" {
			if m.crtPath != "" && query.Get("crt") == "" {
				query.Set("crt", m.crtPath)
			}
			if m.keyPath != "" && query.Get("key") == "" {
				query.Set("key", m.keyPath)
			}
		}
	}

	parsedURL.RawQuery = query.Encode()
	return parsedURL.String()
}

func (m *Master) generateConfigURL(instance *Instance) string {
	parsedURL, err := url.Parse(instance.URL)
	if err != nil {
		m.Logger.Error("generateConfigURL: invalid URL format: %v", err)
		return instance.URL
	}

	query := parsedURL.Query()

	if m.logLevel != "" && query.Get("log") == "" {
		query.Set("log", m.logLevel)
	}

	if instance.Type == "server" && m.TlsCode != "0" {
		if query.Get("tls") == "" {
			query.Set("tls", m.TlsCode)
		}

		if m.TlsCode == "2" {
			if m.crtPath != "" && query.Get("crt") == "" {
				query.Set("crt", m.crtPath)
			}
			if m.keyPath != "" && query.Get("key") == "" {
				query.Set("key", m.keyPath)
			}
		}
	}

	switch instance.Type {
	case "client":
		if query.Get("dns") == "" {
			query.Set("dns", common.DefaultDNSTTL.String())
		}
		if query.Get("sni") == "" {
			query.Set("sni", common.DefaultServerName)
		}
		if query.Get("lbs") == "" {
			query.Set("lbs", common.DefaultLBStrategy)
		}
		if query.Get("min") == "" {
			query.Set("min", strconv.Itoa(common.DefaultMinPool))
		}
		if query.Get("mode") == "" {
			query.Set("mode", common.DefaultRunMode)
		}
		if query.Get("dial") == "" {
			query.Set("dial", common.DefaultDialerIP)
		}
		if query.Get("read") == "" {
			query.Set("read", common.DefaultReadTimeout.String())
		}
		if query.Get("rate") == "" {
			query.Set("rate", strconv.Itoa(common.DefaultRateLimit))
		}
		if query.Get("slot") == "" {
			query.Set("slot", strconv.Itoa(common.DefaultSlotLimit))
		}
		if query.Get("proxy") == "" {
			query.Set("proxy", common.DefaultProxyProtocol)
		}
		if query.Get("block") == "" {
			query.Set("block", common.DefaultBlockProtocol)
		}
		if query.Get("notcp") == "" {
			query.Set("notcp", common.DefaultTCPStrategy)
		}
		if query.Get("noudp") == "" {
			query.Set("noudp", common.DefaultUDPStrategy)
		}
	case "server":
		if query.Get("dns") == "" {
			query.Set("dns", common.DefaultDNSTTL.String())
		}
		if query.Get("lbs") == "" {
			query.Set("lbs", common.DefaultLBStrategy)
		}
		if query.Get("max") == "" {
			query.Set("max", strconv.Itoa(common.DefaultMaxPool))
		}
		if query.Get("mode") == "" {
			query.Set("mode", common.DefaultRunMode)
		}
		if query.Get("type") == "" {
			query.Set("type", common.DefaultPoolType)
		}
		if query.Get("dial") == "" {
			query.Set("dial", common.DefaultDialerIP)
		}
		if query.Get("read") == "" {
			query.Set("read", common.DefaultReadTimeout.String())
		}
		if query.Get("rate") == "" {
			query.Set("rate", strconv.Itoa(common.DefaultRateLimit))
		}
		if query.Get("slot") == "" {
			query.Set("slot", strconv.Itoa(common.DefaultSlotLimit))
		}
		if query.Get("proxy") == "" {
			query.Set("proxy", common.DefaultProxyProtocol)
		}
		if query.Get("block") == "" {
			query.Set("block", common.DefaultBlockProtocol)
		}
		if query.Get("notcp") == "" {
			query.Set("notcp", common.DefaultTCPStrategy)
		}
		if query.Get("noudp") == "" {
			query.Set("noudp", common.DefaultUDPStrategy)
		}
	}

	parsedURL.RawQuery = query.Encode()
	return parsedURL.String()
}

func (m *Master) setInstanceURL(instance *Instance, updates map[string]string) error {
	parsedURL, err := url.Parse(instance.URL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	query := parsedURL.Query()

	for key, value := range updates {
		switch key {
		case "type":
			if value != "client" && value != "server" {
				return fmt.Errorf("invalid type: must be 'client' or 'server'")
			}
			parsedURL.Scheme = value
			instance.Type = value
		case "password":
			if value == "" {
				parsedURL.User = nil
			} else {
				parsedURL.User = url.User(value)
			}
		case "tunnel_address":
			parsedURL.Host = value + ":" + parsedURL.Port()
		case "tunnel_port":
			parsedURL.Host = parsedURL.Hostname() + ":" + value
		case "target_address", "target_port":
			pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), ":")
			if key == "target_address" {
				pathParts[0] = value
			} else if len(pathParts) > 1 {
				pathParts[1] = value
			} else {
				pathParts = append(pathParts, value)
			}
			parsedURL.Path = "/" + strings.Join(pathParts, ":")
		case "targets":
			parsedURL.Path = "/" + value
		default:
			if value == "" {
				query.Del(key)
			} else {
				query.Set(key, value)
			}
		}
	}

	parsedURL.RawQuery = query.Encode()
	newURL := parsedURL.String()

	if newURL == instance.URL {
		return fmt.Errorf("no changes detected")
	}

	if instance.Status != "stopped" {
		m.stopInstance(instance)
		time.Sleep(baseDuration)
	}

	instance.URL = newURL
	instance.Config = m.generateConfigURL(instance)
	instance.Status = "stopped"
	m.instances.Store(instance.ID, instance)

	go m.startInstance(instance)
	go func() {
		time.Sleep(baseDuration)
		m.saveState()
	}()

	return nil
}
