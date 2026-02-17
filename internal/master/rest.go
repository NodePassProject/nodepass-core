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

func (m *Master) HandleInstances(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		instances := []*Instance{}
		m.Instances.Range(func(_, value any) bool {
			instances = append(instances, value.(*Instance))
			return true
		})
		WriteJSON(w, http.StatusOK, instances)

	case http.MethodPost:
		var reqData struct {
			Alias string `json:"alias"`
			URL   string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil || reqData.URL == "" {
			HTTPError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		parsedURL, err := url.Parse(reqData.URL)
		if err != nil {
			HTTPError(w, "Invalid URL format", http.StatusBadRequest)
			return
		}

		instanceRole := parsedURL.Scheme
		if instanceRole != "client" && instanceRole != "server" {
			HTTPError(w, "Invalid URL scheme", http.StatusBadRequest)
			return
		}

		id := GenerateID()
		if _, exists := m.Instances.Load(id); exists {
			HTTPError(w, "Instance ID already exists", http.StatusConflict)
			return
		}

		instance := &Instance{
			ID:      id,
			Alias:   reqData.Alias,
			Type:    instanceRole,
			URL:     m.EnhanceURL(reqData.URL, instanceRole),
			Status:  "stopped",
			Restart: true,
			Meta:    Meta{Tags: make(map[string]string)},
			stopped: make(chan struct{}),
		}

		instance.Config = m.GenerateConfigURL(instance)
		m.Instances.Store(id, instance)

		go m.StartInstance(instance)

		go func() {
			time.Sleep(BaseDuration)
			m.SaveState()
		}()
		WriteJSON(w, http.StatusCreated, instance)

		m.SendSSEEvent("create", instance)

	default:
		HTTPError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Master) HandleInstanceDetail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, fmt.Sprintf("%s/instances/", m.Prefix))
	if id == "" || id == "/" {
		HTTPError(w, "Instance ID is required", http.StatusBadRequest)
		return
	}

	instance, ok := m.FindInstance(id)
	if !ok {
		HTTPError(w, "Instance not found", http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodGet:
		m.HandleGetInstance(w, instance)
	case http.MethodPatch:
		m.HandlePatchInstance(w, r, id, instance)
	case http.MethodPut:
		m.HandlePutInstance(w, r, id, instance)
	case http.MethodDelete:
		m.HandleDeleteInstance(w, id, instance)
	default:
		HTTPError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Master) HandleGetInstance(w http.ResponseWriter, instance *Instance) {
	WriteJSON(w, http.StatusOK, instance)
}

func (m *Master) HandlePatchInstance(w http.ResponseWriter, r *http.Request, id string, instance *Instance) {
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
		if id == APIKeyID {
			if reqData.Action == "restart" {
				m.ReGenerateAPIKey(instance)
				m.SendSSEEvent("update", instance)
			}
		} else {
			if reqData.Alias != "" && instance.Alias != reqData.Alias {
				if len(reqData.Alias) > MaxValueLen {
					HTTPError(w, fmt.Sprintf("Instance alias exceeds maximum length %d", MaxValueLen), http.StatusBadRequest)
					return
				}
				instance.Alias = reqData.Alias
				m.Instances.Store(id, instance)
				go m.SaveState()
				m.Logger.Info("Alias updated: %v [%v]", reqData.Alias, instance.ID)

				m.SendSSEEvent("update", instance)
			}

			if reqData.Action != "" {
				validActions := map[string]bool{
					"start":   true,
					"stop":    true,
					"restart": true,
					"reset":   true,
				}
				if !validActions[reqData.Action] {
					HTTPError(w, fmt.Sprintf("Invalid action: %s", reqData.Action), http.StatusBadRequest)
					return
				}

				if reqData.Action == "reset" {
					instance.tcpRXReset = instance.TCPRX - instance.tcpRXBase
					instance.tcpTXReset = instance.TCPTX - instance.tcpTXBase
					instance.udpRXReset = instance.UDPRX - instance.udpRXBase
					instance.udpTXReset = instance.UDPTX - instance.udpTXBase
					instance.TCPRX = 0
					instance.TCPTX = 0
					instance.UDPRX = 0
					instance.UDPTX = 0
					instance.tcpRXBase = 0
					instance.tcpTXBase = 0
					instance.udpRXBase = 0
					instance.udpTXBase = 0
					m.Instances.Store(id, instance)
					go m.SaveState()
					m.Logger.Info("Traffic stats reset: 0 [%v]", instance.ID)

					m.SendSSEEvent("update", instance)
				} else {
					m.ProcessInstanceAction(instance, reqData.Action)
				}
			}

			if reqData.Restart != nil && instance.Restart != *reqData.Restart {
				instance.Restart = *reqData.Restart
				m.Instances.Store(id, instance)
				go m.SaveState()
				m.Logger.Info("Restart policy updated: %v [%v]", *reqData.Restart, instance.ID)

				m.SendSSEEvent("update", instance)
			}

			if reqData.Meta != nil {
				if reqData.Meta.Peer != nil {
					if len(reqData.Meta.Peer.SID) > MaxValueLen {
						HTTPError(w, fmt.Sprintf("Meta peer.sid exceeds maximum length %d", MaxValueLen), http.StatusBadRequest)
						return
					}
					if len(reqData.Meta.Peer.Type) > MaxValueLen {
						HTTPError(w, fmt.Sprintf("Meta peer.type exceeds maximum length %d", MaxValueLen), http.StatusBadRequest)
						return
					}
					if len(reqData.Meta.Peer.Alias) > MaxValueLen {
						HTTPError(w, fmt.Sprintf("Meta peer.alias exceeds maximum length %d", MaxValueLen), http.StatusBadRequest)
						return
					}
					instance.Meta.Peer = *reqData.Meta.Peer
				}

				if reqData.Meta.Tags != nil {
					seen := make(map[string]bool)
					for key, value := range reqData.Meta.Tags {
						if len(key) > MaxValueLen {
							HTTPError(w, fmt.Sprintf("Meta tag key exceeds maximum length %d", MaxValueLen), http.StatusBadRequest)
							return
						}
						if len(value) > MaxValueLen {
							HTTPError(w, fmt.Sprintf("Meta tag value exceeds maximum length %d", MaxValueLen), http.StatusBadRequest)
							return
						}
						if seen[key] {
							HTTPError(w, fmt.Sprintf("Duplicate meta tag key: %s", key), http.StatusBadRequest)
							return
						}
						seen[key] = true
					}
					instance.Meta.Tags = reqData.Meta.Tags
				}

				m.Instances.Store(id, instance)
				go m.SaveState()
				m.Logger.Info("Meta updated [%v]", instance.ID)
				m.SendSSEEvent("update", instance)
			}

		}
	}
	WriteJSON(w, http.StatusOK, instance)
}

func (m *Master) HandlePutInstance(w http.ResponseWriter, r *http.Request, id string, instance *Instance) {
	if id == APIKeyID {
		HTTPError(w, "Forbidden: API Key", http.StatusForbidden)
		return
	}

	var reqData struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil || reqData.URL == "" {
		HTTPError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	parsedURL, err := url.Parse(reqData.URL)
	if err != nil {
		HTTPError(w, "Invalid URL format", http.StatusBadRequest)
		return
	}

	instanceRole := parsedURL.Scheme
	if instanceRole != "client" && instanceRole != "server" {
		HTTPError(w, "Invalid URL scheme", http.StatusBadRequest)
		return
	}

	enhancedURL := m.EnhanceURL(reqData.URL, instanceRole)

	if instance.URL == enhancedURL {
		HTTPError(w, "Instance URL conflict", http.StatusConflict)
		return
	}

	if instance.Status != "stopped" {
		m.StopInstance(instance)
		time.Sleep(BaseDuration)
	}

	instance.URL = enhancedURL
	instance.Type = instanceRole
	instance.Config = m.GenerateConfigURL(instance)

	instance.Status = "stopped"
	m.Instances.Store(id, instance)

	go m.StartInstance(instance)

	go func() {
		time.Sleep(BaseDuration)
		m.SaveState()
	}()
	WriteJSON(w, http.StatusOK, instance)

	m.Logger.Info("Instance URL updated: %v [%v]", instance.URL, instance.ID)
}

func (m *Master) HandleDeleteInstance(w http.ResponseWriter, id string, instance *Instance) {
	if id == APIKeyID {
		HTTPError(w, "Forbidden: API Key", http.StatusForbidden)
		return
	}

	instance.deleted = true
	m.Instances.Store(id, instance)

	if instance.Status != "stopped" {
		m.StopInstance(instance)
	}
	m.Instances.Delete(id)
	go m.SaveState()
	w.WriteHeader(http.StatusNoContent)
	m.SendSSEEvent("delete", instance)
}

func (m *Master) HandleInfo(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		WriteJSON(w, http.StatusOK, m.GetMasterInfo())

	case http.MethodPost:
		var reqData struct {
			Alias string `json:"alias"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqData); err != nil {
			HTTPError(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if len(reqData.Alias) > MaxValueLen {
			HTTPError(w, fmt.Sprintf("Master alias exceeds maximum length %d", MaxValueLen), http.StatusBadRequest)
			return
		}
		m.Alias = reqData.Alias

		if apiKey, ok := m.FindInstance(APIKeyID); ok {
			apiKey.Alias = m.Alias
			m.Instances.Store(APIKeyID, apiKey)
			go m.SaveState()
		}

		WriteJSON(w, http.StatusOK, m.GetMasterInfo())

	default:
		HTTPError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Master) HandleTCPing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		HTTPError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	target := r.URL.Query().Get("target")
	if target == "" {
		HTTPError(w, "Target address required", http.StatusBadRequest)
		return
	}

	result := m.PerformTCPing(target)
	WriteJSON(w, http.StatusOK, result)
}

func (m *Master) PerformTCPing(target string) *TCPingResult {
	result := &TCPingResult{
		Target:    target,
		Connected: false,
		Latency:   0,
		Error:     nil,
	}

	select {
	case m.TCPingSem <- struct{}{}:
		defer func() { <-m.TCPingSem }()
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

func (m *Master) HandleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	SetCorsHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(m.GenerateOpenAPISpec()))
}

func (m *Master) HandleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	SetCorsHeaders(w)
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, SwaggerUIHTML, m.GenerateOpenAPISpec())
}
