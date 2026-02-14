package master

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/NodePassProject/nodepass/internal/common"
)

func (m *Master) handleInstances(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		instances := []*Instance{}
		m.instances.Range(func(_, value any) bool {
			instances = append(instances, value.(*Instance))
			return true
		})
		writeJSON(w, http.StatusOK, instances)

	case http.MethodPost:
		var reqData struct {
			Alias string `json:"alias"`
			URL   string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil || reqData.URL == "" {
			httpError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		parsedURL, err := url.Parse(reqData.URL)
		if err != nil {
			httpError(w, "Invalid URL format", http.StatusBadRequest)
			return
		}

		instanceRole := parsedURL.Scheme
		if instanceRole != "client" && instanceRole != "server" {
			httpError(w, "Invalid URL scheme", http.StatusBadRequest)
			return
		}

		id := generateID()
		if _, exists := m.instances.Load(id); exists {
			httpError(w, "Instance ID already exists", http.StatusConflict)
			return
		}

		instance := &Instance{
			ID:      id,
			Alias:   reqData.Alias,
			Type:    instanceRole,
			URL:     m.enhanceURL(reqData.URL, instanceRole),
			Status:  "stopped",
			Restart: true,
			Meta:    Meta{Tags: make(map[string]string)},
			Stopped: make(chan struct{}),
		}

		instance.Config = m.generateConfigURL(instance)
		m.instances.Store(id, instance)

		go m.startInstance(instance)

		go func() {
			time.Sleep(baseDuration)
			m.saveState()
		}()
		writeJSON(w, http.StatusCreated, instance)

		m.sendSSEEvent("create", instance)

	default:
		httpError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Master) handleInstanceDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, fmt.Sprintf("%s/instances/", m.prefix))
	if id == "" || id == "/" {
		httpError(w, "Instance ID is required", http.StatusBadRequest)
		return
	}

	instance, ok := m.findInstance(id)
	if !ok {
		httpError(w, "Instance not found", http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		m.handleGetInstance(w, instance)
	case http.MethodPatch:
		m.handlePatchInstance(w, r, id, instance)
	case http.MethodPut:
		m.handlePutInstance(w, r, id, instance)
	case http.MethodDelete:
		m.handleDeleteInstance(w, id, instance)
	default:
		httpError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Master) handleGetInstance(w http.ResponseWriter, instance *Instance) {
	writeJSON(w, http.StatusOK, instance)
}

func (m *Master) handlePatchInstance(w http.ResponseWriter, r *http.Request, id string, instance *Instance) {
	var reqData struct {
		Alias   string `json:"alias,omitempty"`
		Action  string `json:"action,omitempty"`
		Restart *bool  `json:"restart,omitempty"`
		Meta    *struct {
			Peer *Peer             `json:"peer,omitempty"`
			Tags map[string]string `json:"tags,omitempty"`
		} `json:"meta,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqData); err == nil {
		if id == apiKeyID {
			if reqData.Action == "restart" {
				m.regenerateAPIKey(instance)
				m.sendSSEEvent("update", instance)
			}
		} else {
			if reqData.Alias != "" && instance.Alias != reqData.Alias {
				if len(reqData.Alias) > maxValueLen {
					httpError(w, fmt.Sprintf("Instance alias exceeds maximum length %d", maxValueLen), http.StatusBadRequest)
					return
				}
				instance.Alias = reqData.Alias
				m.instances.Store(id, instance)
				go m.saveState()
				m.Logger.Info("Alias updated: %v [%v]", reqData.Alias, instance.ID)

				m.sendSSEEvent("update", instance)
			}

			if reqData.Action != "" {
				validActions := map[string]bool{
					"start":   true,
					"stop":    true,
					"restart": true,
					"reset":   true,
				}
				if !validActions[reqData.Action] {
					httpError(w, fmt.Sprintf("Invalid action: %s", reqData.Action), http.StatusBadRequest)
					return
				}

				if reqData.Action == "reset" {
					instance.TCPRXReset = instance.TCPRX - instance.TCPRXBase
					instance.TCPTXReset = instance.TCPTX - instance.TCPTXBase
					instance.UDPRXReset = instance.UDPRX - instance.UDPRXBase
					instance.UDPTXReset = instance.UDPTX - instance.UDPTXBase
					instance.TCPRX = 0
					instance.TCPTX = 0
					instance.UDPRX = 0
					instance.UDPTX = 0
					instance.TCPRXBase = 0
					instance.TCPTXBase = 0
					instance.UDPRXBase = 0
					instance.UDPTXBase = 0
					m.instances.Store(id, instance)
					go m.saveState()
					m.Logger.Info("Traffic stats reset: 0 [%v]", instance.ID)

					m.sendSSEEvent("update", instance)
				} else {
					m.processInstanceAction(instance, reqData.Action)
				}
			}

			if reqData.Restart != nil && instance.Restart != *reqData.Restart {
				instance.Restart = *reqData.Restart
				m.instances.Store(id, instance)
				go m.saveState()
				m.Logger.Info("Restart policy updated: %v [%v]", *reqData.Restart, instance.ID)

				m.sendSSEEvent("update", instance)
			}

			if reqData.Meta != nil {
				if reqData.Meta.Peer != nil {
					if len(reqData.Meta.Peer.SID) > maxValueLen {
						httpError(w, fmt.Sprintf("Meta peer.sid exceeds maximum length %d", maxValueLen), http.StatusBadRequest)
						return
					}
					if len(reqData.Meta.Peer.Type) > maxValueLen {
						httpError(w, fmt.Sprintf("Meta peer.type exceeds maximum length %d", maxValueLen), http.StatusBadRequest)
						return
					}
					if len(reqData.Meta.Peer.Alias) > maxValueLen {
						httpError(w, fmt.Sprintf("Meta peer.alias exceeds maximum length %d", maxValueLen), http.StatusBadRequest)
						return
					}
					instance.Meta.Peer = *reqData.Meta.Peer
				}

				if reqData.Meta.Tags != nil {
					seen := make(map[string]bool)
					for key, value := range reqData.Meta.Tags {
						if len(key) > maxValueLen {
							httpError(w, fmt.Sprintf("Meta tag key exceeds maximum length %d", maxValueLen), http.StatusBadRequest)
							return
						}
						if len(value) > maxValueLen {
							httpError(w, fmt.Sprintf("Meta tag value exceeds maximum length %d", maxValueLen), http.StatusBadRequest)
							return
						}
						if seen[key] {
							httpError(w, fmt.Sprintf("Duplicate meta tag key: %s", key), http.StatusBadRequest)
							return
						}
						seen[key] = true
					}
					instance.Meta.Tags = reqData.Meta.Tags
				}

				m.instances.Store(id, instance)
				go m.saveState()
				m.Logger.Info("Meta updated [%v]", instance.ID)
				m.sendSSEEvent("update", instance)
			}

		}
	}
	writeJSON(w, http.StatusOK, instance)
}

func (m *Master) handlePutInstance(w http.ResponseWriter, r *http.Request, id string, instance *Instance) {
	if id == apiKeyID {
		httpError(w, "Forbidden: API Key", http.StatusForbidden)
		return
	}

	var reqData struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil || reqData.URL == "" {
		httpError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	parsedURL, err := url.Parse(reqData.URL)
	if err != nil {
		httpError(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	instanceRole := parsedURL.Scheme
	if instanceRole != "client" && instanceRole != "server" {
		httpError(w, "Invalid URL scheme", http.StatusBadRequest)
		return
	}

	enhancedURL := m.enhanceURL(reqData.URL, instanceRole)

	if instance.URL == enhancedURL {
		httpError(w, "Instance URL conflict", http.StatusConflict)
		return
	}

	if instance.Status != "stopped" {
		m.stopInstance(instance)
		time.Sleep(baseDuration)
	}

	instance.URL = enhancedURL
	instance.Type = instanceRole
	instance.Config = m.generateConfigURL(instance)

	instance.Status = "stopped"
	m.instances.Store(id, instance)

	go m.startInstance(instance)

	go func() {
		time.Sleep(baseDuration)
		m.saveState()
	}()
	writeJSON(w, http.StatusOK, instance)

	m.Logger.Info("Instance URL updated: %v [%v]", instance.URL, instance.ID)
}

func (m *Master) handleDeleteInstance(w http.ResponseWriter, id string, instance *Instance) {
	if id == apiKeyID {
		httpError(w, "Forbidden: API Key", http.StatusForbidden)
		return
	}

	instance.Deleted = true
	m.instances.Store(id, instance)

	if instance.Status != "stopped" {
		m.stopInstance(instance)
	}
	m.instances.Delete(id)
	go m.saveState()
	w.WriteHeader(http.StatusNoContent)
	m.sendSSEEvent("delete", instance)
}

func (m *Master) handleInfo(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, m.getMasterInfo())

	case http.MethodPost:
		var reqData struct {
			Alias string `json:"alias"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
			httpError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if len(reqData.Alias) > maxValueLen {
			httpError(w, fmt.Sprintf("Master alias exceeds maximum length %d", maxValueLen), http.StatusBadRequest)
			return
		}
		m.alias = reqData.Alias

		if apiKey, ok := m.findInstance(apiKeyID); ok {
			apiKey.Alias = m.alias
			m.instances.Store(apiKeyID, apiKey)
			go m.saveState()
		}

		writeJSON(w, http.StatusOK, m.getMasterInfo())

	default:
		httpError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Master) handleTCPing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	target := r.URL.Query().Get("target")
	if target == "" {
		httpError(w, "Target address required", http.StatusBadRequest)
		return
	}

	result := m.performTCPing(target)
	writeJSON(w, http.StatusOK, result)
}

func (m *Master) performTCPing(target string) *TCPingResult {
	result := &TCPingResult{
		Target:    target,
		Connected: false,
		Latency:   0,
		Error:     nil,
	}

	select {
	case m.tcpingSem <- struct{}{}:
		defer func() { <-m.tcpingSem }()
	case <-time.After(time.Second):
		errMsg := "too many requests"
		result.Error = &errMsg
		return result
	}

	start := time.Now()
	conn, err := net.DialTimeout("tcp", target, common.ReportInterval)
	if err != nil {
		errMsg := err.Error()
		result.Error = &errMsg
		return result
	}

	result.Connected = true
	result.Latency = time.Since(start).Milliseconds()
	conn.Close()
	return result
}

func (m *Master) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	setCorsHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(m.generateOpenAPISpec()))
}

func (m *Master) handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	setCorsHeaders(w)
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, swaggerUIHTML, m.generateOpenAPISpec())
}
