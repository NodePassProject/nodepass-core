package master

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (m *Master) HandleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		HTTPError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		m.WriteMCPError(w, nil, -32700, "Parse error", err.Error())
		return
	}

	if req.JSONRPC != "2.0" {
		m.WriteMCPError(w, req.ID, -32600, "Invalid Request", "jsonrpc must be 2.0")
		return
	}

	switch req.Method {
	case "initialize":
		m.HandleMCPInitialize(w, req)
	case "tools/list":
		m.HandleMCPToolsList(w, req)
	case "tools/call":
		m.HandleMCPToolsCall(w, req)
	default:
		m.WriteMCPError(w, req.ID, -32601, "Method not found", req.Method)
	}
}

func (m *Master) HandleMCPInitialize(w http.ResponseWriter, req MCPRequest) {
	result := map[string]any{
		"protocolVersion": MCPVersion,
		"capabilities": map[string]any{
			"tools": map[string]any{},
		},
		"serverInfo": map[string]any{
			"name":    "NodePass Master",
			"version": m.Version,
		},
	}
	m.WriteMCPResponse(w, req.ID, result)
}

func (m *Master) HandleMCPToolsList(w http.ResponseWriter, req MCPRequest) {
	commonParams := map[string]map[string]any{
		"id": {
			"type":        "string",
			"description": "Instance ID",
		},
		"alias": {
			"type":        "string",
			"description": "Instance alias",
		},
		"role": {
			"type":        "string",
			"description": "Instance role: server or client",
			"enum":        []string{"server", "client"},
		},
		"tunnel_address": {
			"type":        "string",
			"description": "Tunnel address",
		},
		"tunnel_port": {
			"type":        "string",
			"description": "Tunnel port",
		},
		"target_address": {
			"type":        "string",
			"description": "Target address",
		},
		"target_port": {
			"type":        "string",
			"description": "Target port",
		},
		"targets": {
			"type":        "string",
			"description": "Multiple targets: host1:port1,host2:port2...",
		},
		"password": {
			"type":        "string",
			"description": "Connection password",
		},
		"log": {
			"type":        "string",
			"description": "Log level: none, debug, info, warn, error, event",
			"enum":        []string{"none", "debug", "info", "warn", "error", "event"},
		},
		"tls": {
			"type":        "string",
			"description": "TLS mode: 0=none, 1=self-signed, 2=custom",
			"enum":        []string{"0", "1", "2"},
		},
		"crt": {
			"type":        "string",
			"description": "Certificate file path (for tls=2)",
		},
		"key": {
			"type":        "string",
			"description": "Key file path (for tls=2)",
		},
		"sni": {
			"type":        "string",
			"description": "SNI hostname (client dual-end mode)",
		},
		"dns": {
			"type":        "string",
			"description": "DNS cache TTL (e.g., 5m, 1h)",
		},
		"lbs": {
			"type":        "string",
			"description": "Load balancing: 0=round-robin, 1=optimal-latency, 2=primary-backup",
			"enum":        []string{"0", "1", "2"},
		},
		"mode": {
			"type":        "string",
			"description": "Connection mode: 0=auto, 1=reverse/single-end, 2=forward/dual-end",
			"enum":        []string{"0", "1", "2"},
		},
		"type": {
			"type":        "string",
			"description": "Pool type: 0=TCP, 1=QUIC, 2=WebSocket, 3=HTTP2 (server only)",
			"enum":        []string{"0", "1", "2", "3"},
		},
		"min": {
			"type":        "string",
			"description": "Minimum pool size (client dual-end mode)",
		},
		"max": {
			"type":        "string",
			"description": "Maximum pool size (dual-end mode)",
		},
		"dial": {
			"type":        "string",
			"description": "Outbound source IP (default: auto)",
		},
		"read": {
			"type":        "string",
			"description": "Read timeout (e.g., 30s, 5m, 1h)",
		},
		"rate": {
			"type":        "string",
			"description": "Bandwidth limit in Mbps (0=unlimited)",
		},
		"slot": {
			"type":        "string",
			"description": "Connection slots (0=unlimited)",
		},
		"proxy": {
			"type":        "string",
			"description": "PROXY protocol v1: 0=disabled, 1=enabled",
			"enum":        []string{"0", "1"},
		},
		"block": {
			"type":        "string",
			"description": "Block protocols: 1=SOCKS, 2=HTTP, 3=TLS, combine like '123'",
		},
		"notcp": {
			"type":        "string",
			"description": "Disable TCP: 0=enabled, 1=disabled",
			"enum":        []string{"0", "1"},
		},
		"noudp": {
			"type":        "string",
			"description": "Disable UDP: 0=enabled, 1=disabled",
			"enum":        []string{"0", "1"},
		},
	}

	tools := []map[string]any{
		{
			"name":        "list_instances",
			"description": "List all NodePass instances",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			"name":        "get_instance",
			"description": "Get details of a specific instance",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"id": commonParams["id"]},
				"required":   []string{"id"},
			},
		},
		{
			"name":        "create_instance",
			"description": "Create a new NodePass instance with structured configuration",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"alias":          commonParams["alias"],
					"role":           commonParams["role"],
					"tunnel_address": commonParams["tunnel_address"],
					"tunnel_port":    commonParams["tunnel_port"],
					"target_address": commonParams["target_address"],
					"target_port":    commonParams["target_port"],
					"targets":        commonParams["targets"],
					"password":       commonParams["password"],
					"log":            commonParams["log"],
					"tls":            commonParams["tls"],
					"crt":            commonParams["crt"],
					"key":            commonParams["key"],
					"sni":            commonParams["sni"],
					"dns":            commonParams["dns"],
					"lbs":            commonParams["lbs"],
					"mode":           commonParams["mode"],
					"type":           commonParams["type"],
					"min":            commonParams["min"],
					"max":            commonParams["max"],
					"dial":           commonParams["dial"],
					"read":           commonParams["read"],
					"rate":           commonParams["rate"],
					"slot":           commonParams["slot"],
					"proxy":          commonParams["proxy"],
					"block":          commonParams["block"],
					"notcp":          commonParams["notcp"],
					"noudp":          commonParams["noudp"],
				},
				"required": []string{"role", "tunnel_port", "target_port"},
			},
		},
		{
			"name":        "update_instance",
			"description": "Update instance metadata (alias, restart policy, peer, tags)",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":    commonParams["id"],
					"alias": commonParams["alias"],
					"restart": map[string]any{
						"type":        "boolean",
						"description": "Auto-restart policy",
					},
					"peer": map[string]any{
						"type":        "object",
						"description": "service and peer information",
						"properties": map[string]any{
							"sid": map[string]any{
								"type":        "string",
								"description": "Service ID",
							},
							"type": map[string]any{
								"type":        "string",
								"description": "Service type",
							},
							"alias": map[string]any{
								"type":        "string",
								"description": "Service alias",
							},
						},
					},
					"tags": map[string]any{
						"type":        "object",
						"description": "Metadata tags",
					},
				},
				"required": []string{"id"},
			},
		},
		{
			"name":        "control_instance",
			"description": "Control instance state (start, stop, restart, reset)",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": commonParams["id"],
					"action": map[string]any{
						"type":        "string",
						"description": "Control action",
						"enum":        []string{"start", "stop", "restart", "reset"},
					},
				},
				"required": []string{"id", "action"},
			},
		},
		{
			"name":        "set_instance_basic",
			"description": "Set instance basic configuration (role, tunnel/target addresses, log level)",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":             commonParams["id"],
					"role":           commonParams["role"],
					"tunnel_address": commonParams["tunnel_address"],
					"tunnel_port":    commonParams["tunnel_port"],
					"target_address": commonParams["target_address"],
					"target_port":    commonParams["target_port"],
					"targets":        commonParams["targets"],
					"log":            commonParams["log"],
				},
				"required": []string{"id"},
			},
		},
		{
			"name":        "set_instance_security",
			"description": "Set instance security and encryption settings (password, TLS mode, certificates, SNI)",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":       commonParams["id"],
					"password": commonParams["password"],
					"tls":      commonParams["tls"],
					"crt":      commonParams["crt"],
					"key":      commonParams["key"],
					"sni":      commonParams["sni"],
				},
				"required": []string{"id"},
			},
		},
		{
			"name":        "set_instance_connection",
			"description": "Set instance connection pool settings (mode, type, pool size limits)",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":   commonParams["id"],
					"mode": commonParams["mode"],
					"type": commonParams["type"],
					"min":  commonParams["min"],
					"max":  commonParams["max"],
				},
				"required": []string{"id"},
			},
		},
		{
			"name":        "set_instance_network",
			"description": "Set instance network tuning parameters (DNS TTL, source IP, timeouts, connection limits)",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":   commonParams["id"],
					"dns":  commonParams["dns"],
					"dial": commonParams["dial"],
					"read": commonParams["read"],
					"rate": commonParams["rate"],
					"slot": commonParams["slot"],
				},
				"required": []string{"id"},
			},
		},
		{
			"name":        "set_instance_protocol",
			"description": "Set instance protocol control settings (TCP/UDP enable/disable, PROXY protocol v1)",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":    commonParams["id"],
					"notcp": commonParams["notcp"],
					"noudp": commonParams["noudp"],
					"proxy": commonParams["proxy"],
				},
				"required": []string{"id"},
			},
		},
		{
			"name":        "set_instance_traffic",
			"description": "Set instance traffic control and load balancing (Protocol blocking, load balancing)",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":    commonParams["id"],
					"block": commonParams["block"],
					"lbs":   commonParams["lbs"],
				},
				"required": []string{"id"},
			},
		},
		{
			"name":        "set_instance_advanced",
			"description": "Set instance advanced/custom parameters (any additional URL parameters)",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": commonParams["id"],
					"parameters": map[string]any{
						"type":        "object",
						"description": "Custom URL query parameters (key-value pairs)",
					},
				},
				"required": []string{"id", "parameters"},
			},
		},
		{
			"name":        "get_instance_config",
			"description": "Get instance configuration (structured config object)",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"id": commonParams["id"]},
				"required":   []string{"id"},
			},
		},
		{
			"name":        "delete_instance",
			"description": "Delete an instance (stop and remove)",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"id": commonParams["id"]},
				"required":   []string{"id"},
			},
		},
		{
			"name":        "export_instances",
			"description": "Export all instances configuration to nodepass.json",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			"name":        "import_instances",
			"description": "Import instances configuration from nodepass.json",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			"name":        "tcping_target",
			"description": "Test TCP connectivity (host:port latency probe)",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"target": map[string]any{
						"type":        "string",
						"description": "Target address (host:port)",
					},
				},
				"required": []string{"target"},
			},
		},
		{
			"name":        "get_master_info",
			"description": "Get master information (system stats, uptime, version)",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
		{
			"name":        "update_master_info",
			"description": "Update master alias (server name or label)",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"alias": map[string]any{
						"type":        "string",
						"description": "New alias for master (optional)",
					},
				},
				"required": []string{"alias"},
			},
		},
	}
	m.WriteMCPResponse(w, req.ID, map[string]any{"tools": tools})
}

func (m *Master) HandleMCPToolsCall(w http.ResponseWriter, req MCPRequest) {
	var params MCPToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		m.WriteMCPError(w, req.ID, -32602, "Invalid params", err.Error())
		return
	}

	switch params.Name {
	case "list_instances":
		var instances []*Instance
		m.Instances.Range(func(_, value any) bool {
			instance := value.(*Instance)
			if instance.ID != APIKeyID {
				instances = append(instances, instance)
			}
			return true
		})
		m.WriteMCPResponse(w, req.ID, map[string]any{
			"content":   []map[string]any{{"type": "text", "text": fmt.Sprintf("Found %d instances", len(instances))}},
			"instances": instances,
		})
	case "get_instance":
		id, _ := params.Arguments["id"].(string)
		if id == "" {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "id is required")
			return
		}
		instance, ok := m.FindInstance(id)
		if !ok {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "instance not found")
			return
		}
		m.WriteMCPResponse(w, req.ID, map[string]any{
			"content":  []map[string]any{{"type": "text", "text": fmt.Sprintf("Instance: %s (%s)", instance.Alias, instance.Status)}},
			"instance": instance,
		})
	case "create_instance":
		instanceRole, _ := params.Arguments["role"].(string)
		if instanceRole != "server" && instanceRole != "client" {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "role must be 'server' or 'client'")
			return
		}

		tunnelAddr, _ := params.Arguments["tunnel_address"].(string)
		if tunnelAddr == "" {
			tunnelAddr = map[string]string{"server": "", "client": ""}[instanceRole]
		}

		tunnelPort, _ := params.Arguments["tunnel_port"].(string)
		targetPort, _ := params.Arguments["target_port"].(string)

		var targetPath string
		if targets, ok := params.Arguments["targets"].(string); ok && targets != "" {
			targetPath = targets
		} else {
			targetAddr, _ := params.Arguments["target_address"].(string)
			targetPath = fmt.Sprintf("%s:%s", targetAddr, targetPort)
		}

		var instanceURL string
		if password, ok := params.Arguments["password"].(string); ok && password != "" {
			instanceURL = fmt.Sprintf("%s://%s@%s:%s/%s", instanceRole, password, tunnelAddr, tunnelPort, targetPath)
		} else {
			instanceURL = fmt.Sprintf("%s://%s:%s/%s", instanceRole, tunnelAddr, tunnelPort, targetPath)
		}

		parsedURL, err := url.Parse(instanceURL)
		if err != nil {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "failed to construct URL")
			return
		}

		query := parsedURL.Query()
		skipKeys := map[string]bool{"alias": true, "role": true, "tunnel_address": true, "tunnel_port": true, "target_address": true, "target_port": true, "targets": true, "password": true}
		for key, val := range params.Arguments {
			if skipKeys[key] {
				continue
			}
			if strVal, ok := val.(string); ok && strVal != "" {
				query.Set(key, strVal)
			}
		}

		parsedURL.RawQuery = query.Encode()
		instanceURL = parsedURL.String()

		id := GenerateID()
		if _, exists := m.Instances.Load(id); exists {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "instance ID already exists")
			return
		}

		alias, _ := params.Arguments["alias"].(string)
		instance := &Instance{
			ID:      id,
			Alias:   alias,
			Type:    instanceRole,
			URL:     m.EnhanceURL(instanceURL, instanceRole),
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

		m.SendSSEEvent("create", instance)
		m.WriteMCPResponse(w, req.ID, map[string]any{
			"content":  []map[string]any{{"type": "text", "text": fmt.Sprintf("Created instance %s", id)}},
			"instance": instance,
		})
	case "update_instance":
		id, _ := params.Arguments["id"].(string)
		if id == "" {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "id is required")
			return
		}
		if id == APIKeyID {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "forbidden: API Key")
			return
		}

		instance, ok := m.FindInstance(id)
		if !ok {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "instance not found")
			return
		}

		if alias, ok := params.Arguments["alias"].(string); ok && alias != "" && instance.Alias != alias {
			if len(alias) > MaxValueLen {
				m.WriteMCPError(w, req.ID, -32602, "Invalid params", fmt.Sprintf("alias exceeds max length %d", MaxValueLen))
				return
			}
			instance.Alias = alias
			m.Instances.Store(id, instance)
			go m.SaveState()
			m.SendSSEEvent("update", instance)
		}

		if restart, ok := params.Arguments["restart"].(bool); ok && instance.Restart != restart {
			instance.Restart = restart
			m.Instances.Store(id, instance)
			go m.SaveState()
			m.SendSSEEvent("update", instance)
		}

		if peer, ok := params.Arguments["peer"].(map[string]any); ok {
			if sid, ok := peer["sid"].(string); ok {
				if len(sid) > MaxValueLen {
					m.WriteMCPError(w, req.ID, -32602, "Invalid params", fmt.Sprintf("peer.sid exceeds max length %d", MaxValueLen))
					return
				}
				instance.Meta.Peer.SID = sid
			}
			if peerType, ok := peer["type"].(string); ok {
				if len(peerType) > MaxValueLen {
					m.WriteMCPError(w, req.ID, -32602, "Invalid params", fmt.Sprintf("peer.type exceeds max length %d", MaxValueLen))
					return
				}
				instance.Meta.Peer.Type = peerType
			}
			if peerAlias, ok := peer["alias"].(string); ok {
				if len(peerAlias) > MaxValueLen {
					m.WriteMCPError(w, req.ID, -32602, "Invalid params", fmt.Sprintf("peer.alias exceeds max length %d", MaxValueLen))
					return
				}
				instance.Meta.Peer.Alias = peerAlias
			}
			m.Instances.Store(id, instance)
			go m.SaveState()
			m.SendSSEEvent("update", instance)
		}

		if tags, ok := params.Arguments["tags"].(map[string]any); ok {
			tagsMap := make(map[string]string)
			for k, v := range tags {
				if len(k) > MaxValueLen {
					m.WriteMCPError(w, req.ID, -32602, "Invalid params", fmt.Sprintf("tag key exceeds max length %d", MaxValueLen))
					return
				}
				valStr := fmt.Sprintf("%v", v)
				if len(valStr) > MaxValueLen {
					m.WriteMCPError(w, req.ID, -32602, "Invalid params", fmt.Sprintf("tag value exceeds max length %d", MaxValueLen))
					return
				}
				tagsMap[k] = valStr
			}
			instance.Meta.Tags = tagsMap
			m.Instances.Store(id, instance)
			go m.SaveState()
			m.SendSSEEvent("update", instance)
		}

		m.WriteMCPResponse(w, req.ID, map[string]any{
			"content":  []map[string]any{{"type": "text", "text": fmt.Sprintf("Updated instance %s", id)}},
			"instance": instance,
		})
	case "control_instance":
		id, _ := params.Arguments["id"].(string)
		action, _ := params.Arguments["action"].(string)
		if id == "" || action == "" {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "id and action are required")
			return
		}
		if id == APIKeyID {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "forbidden: API Key")
			return
		}

		instance, ok := m.FindInstance(id)
		if !ok {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "instance not found")
			return
		}

		validActions := map[string]bool{"start": true, "stop": true, "restart": true, "reset": true}
		if !validActions[action] {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "invalid action")
			return
		}

		if action == "reset" {
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
			m.SendSSEEvent("update", instance)
		} else {
			m.ProcessInstanceAction(instance, action)
		}

		m.WriteMCPResponse(w, req.ID, map[string]any{
			"content":  []map[string]any{{"type": "text", "text": fmt.Sprintf("Controlled instance %s: %s", id, action)}},
			"instance": instance,
		})
	case "set_instance_basic":
		id, _ := params.Arguments["id"].(string)
		if id == "" {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "id is required")
			return
		}
		if id == APIKeyID {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "forbidden: API Key")
			return
		}

		instance, ok := m.FindInstance(id)
		if !ok {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "instance not found")
			return
		}

		updates := make(map[string]string)
		if instanceRole, ok := params.Arguments["role"].(string); ok && instanceRole != "" {
			updates["type"] = instanceRole
		}
		if tunnelAddr, ok := params.Arguments["tunnel_address"].(string); ok && tunnelAddr != "" {
			updates["tunnel_address"] = tunnelAddr
		}
		if tunnelPort, ok := params.Arguments["tunnel_port"].(string); ok && tunnelPort != "" {
			updates["tunnel_port"] = tunnelPort
		}
		if targetAddr, ok := params.Arguments["target_address"].(string); ok && targetAddr != "" {
			updates["target_address"] = targetAddr
		}
		if targetPort, ok := params.Arguments["target_port"].(string); ok && targetPort != "" {
			updates["target_port"] = targetPort
		}
		if targets, ok := params.Arguments["targets"].(string); ok && targets != "" {
			updates["targets"] = targets
		}
		if logLevel, ok := params.Arguments["log"].(string); ok && logLevel != "" {
			updates["log"] = logLevel
		}

		if len(updates) == 0 {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "no updates provided")
			return
		}

		if err := m.SetInstanceURL(instance, updates); err != nil {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", err.Error())
			return
		}

		m.WriteMCPResponse(w, req.ID, map[string]any{
			"content":  []map[string]any{{"type": "text", "text": fmt.Sprintf("Set basic configuration for instance %s", id)}},
			"instance": instance,
		})
	case "set_instance_security":
		id, _ := params.Arguments["id"].(string)
		if id == "" {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "id is required")
			return
		}
		if id == APIKeyID {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "forbidden: API Key")
			return
		}

		instance, ok := m.FindInstance(id)
		if !ok {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "instance not found")
			return
		}

		updates := make(map[string]string)
		if password, ok := params.Arguments["password"].(string); ok {
			updates["password"] = password
		}
		if tls, ok := params.Arguments["tls"].(string); ok && tls != "" {
			updates["tls"] = tls
		}
		if crt, ok := params.Arguments["crt"].(string); ok {
			updates["crt"] = crt
		}
		if key, ok := params.Arguments["key"].(string); ok {
			updates["key"] = key
		}
		if sni, ok := params.Arguments["sni"].(string); ok {
			updates["sni"] = sni
		}

		if len(updates) == 0 {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "no updates provided")
			return
		}

		if err := m.SetInstanceURL(instance, updates); err != nil {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", err.Error())
			return
		}

		m.WriteMCPResponse(w, req.ID, map[string]any{
			"content":  []map[string]any{{"type": "text", "text": fmt.Sprintf("Set security configuration for instance %s", id)}},
			"instance": instance,
		})
	case "set_instance_connection":
		id, _ := params.Arguments["id"].(string)
		if id == "" {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "id is required")
			return
		}
		if id == APIKeyID {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "forbidden: API Key")
			return
		}

		instance, ok := m.FindInstance(id)
		if !ok {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "instance not found")
			return
		}

		updates := make(map[string]string)
		if mode, ok := params.Arguments["mode"].(string); ok && mode != "" {
			updates["mode"] = mode
		}
		if poolType, ok := params.Arguments["type"].(string); ok && poolType != "" {
			updates["type"] = poolType
		}
		if min, ok := params.Arguments["min"].(string); ok && min != "" {
			updates["min"] = min
		}
		if max, ok := params.Arguments["max"].(string); ok && max != "" {
			updates["max"] = max
		}

		if len(updates) == 0 {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "no updates provided")
			return
		}

		if err := m.SetInstanceURL(instance, updates); err != nil {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", err.Error())
			return
		}

		m.WriteMCPResponse(w, req.ID, map[string]any{
			"content":  []map[string]any{{"type": "text", "text": fmt.Sprintf("Set connection configuration for instance %s", id)}},
			"instance": instance,
		})
	case "set_instance_network":
		id, _ := params.Arguments["id"].(string)
		if id == "" {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "id is required")
			return
		}
		if id == APIKeyID {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "forbidden: API Key")
			return
		}

		instance, ok := m.FindInstance(id)
		if !ok {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "instance not found")
			return
		}

		updates := make(map[string]string)
		if dns, ok := params.Arguments["dns"].(string); ok {
			updates["dns"] = dns
		}
		if dial, ok := params.Arguments["dial"].(string); ok {
			updates["dial"] = dial
		}
		if read, ok := params.Arguments["read"].(string); ok {
			updates["read"] = read
		}
		if rate, ok := params.Arguments["rate"].(string); ok {
			updates["rate"] = rate
		}
		if slot, ok := params.Arguments["slot"].(string); ok {
			updates["slot"] = slot
		}

		if len(updates) == 0 {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "no updates provided")
			return
		}

		if err := m.SetInstanceURL(instance, updates); err != nil {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", err.Error())
			return
		}

		m.WriteMCPResponse(w, req.ID, map[string]any{
			"content":  []map[string]any{{"type": "text", "text": fmt.Sprintf("Set network tuning for instance %s", id)}},
			"instance": instance,
		})
	case "set_instance_protocol":
		id, _ := params.Arguments["id"].(string)
		if id == "" {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "id is required")
			return
		}
		if id == APIKeyID {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "forbidden: API Key")
			return
		}

		instance, ok := m.FindInstance(id)
		if !ok {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "instance not found")
			return
		}

		updates := make(map[string]string)
		if notcp, ok := params.Arguments["notcp"].(string); ok {
			updates["notcp"] = notcp
		}
		if noudp, ok := params.Arguments["noudp"].(string); ok {
			updates["noudp"] = noudp
		}
		if proxy, ok := params.Arguments["proxy"].(string); ok {
			updates["proxy"] = proxy
		}

		if len(updates) == 0 {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "no updates provided")
			return
		}

		if err := m.SetInstanceURL(instance, updates); err != nil {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", err.Error())
			return
		}

		m.WriteMCPResponse(w, req.ID, map[string]any{
			"content":  []map[string]any{{"type": "text", "text": fmt.Sprintf("Set protocol control for instance %s", id)}},
			"instance": instance,
		})
	case "set_instance_traffic":
		id, _ := params.Arguments["id"].(string)
		if id == "" {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "id is required")
			return
		}
		if id == APIKeyID {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "forbidden: API Key")
			return
		}

		instance, ok := m.FindInstance(id)
		if !ok {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "instance not found")
			return
		}

		updates := make(map[string]string)
		if block, ok := params.Arguments["block"].(string); ok {
			updates["block"] = block
		}
		if lbs, ok := params.Arguments["lbs"].(string); ok {
			updates["lbs"] = lbs
		}

		if len(updates) == 0 {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "no updates provided")
			return
		}

		if err := m.SetInstanceURL(instance, updates); err != nil {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", err.Error())
			return
		}

		m.WriteMCPResponse(w, req.ID, map[string]any{
			"content":  []map[string]any{{"type": "text", "text": fmt.Sprintf("Set traffic control for instance %s", id)}},
			"instance": instance,
		})
	case "set_instance_advanced":
		id, _ := params.Arguments["id"].(string)
		if id == "" {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "id is required")
			return
		}
		if id == APIKeyID {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "forbidden: API Key")
			return
		}

		instance, ok := m.FindInstance(id)
		if !ok {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "instance not found")
			return
		}

		params, ok := params.Arguments["parameters"].(map[string]any)
		if !ok || len(params) == 0 {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "parameters object is required")
			return
		}

		updates := make(map[string]string)
		for key, value := range params {
			updates[key] = fmt.Sprintf("%v", value)
		}

		if err := m.SetInstanceURL(instance, updates); err != nil {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", err.Error())
			return
		}

		m.WriteMCPResponse(w, req.ID, map[string]any{
			"content":  []map[string]any{{"type": "text", "text": fmt.Sprintf("Set advanced parameters for instance %s", id)}},
			"instance": instance,
		})
	case "get_instance_config":
		id, _ := params.Arguments["id"].(string)
		if id == "" {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "id is required")
			return
		}

		instance, ok := m.FindInstance(id)
		if !ok {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "instance not found")
			return
		}

		parsedURL, err := url.Parse(instance.Config)
		if err != nil {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "failed to parse config URL")
			return
		}

		query := parsedURL.Query()
		config := map[string]any{
			"role": parsedURL.Scheme,
		}

		if parsedURL.User != nil {
			if password, ok := parsedURL.User.Password(); ok {
				config["password"] = password
			} else if username := parsedURL.User.Username(); username != "" {
				config["password"] = username
			}
		}

		config["tunnel_address"] = parsedURL.Hostname()
		config["tunnel_port"] = parsedURL.Port()

		path := strings.Trim(parsedURL.Path, "/")
		if strings.Contains(path, ",") {
			config["targets"] = path
		} else if path != "" {
			parts := strings.Split(path, ":")
			if len(parts) >= 1 {
				config["target_address"] = parts[0]
			}
			if len(parts) >= 2 {
				config["target_port"] = parts[1]
			}
		}

		params := make(map[string]string)
		for key := range query {
			params[key] = query.Get(key)
		}
		if len(params) > 0 {
			config["parameters"] = params
		}

		m.WriteMCPResponse(w, req.ID, map[string]any{
			"content": []map[string]any{{"type": "text", "text": fmt.Sprintf("Instance %s configuration retrieved", id)}},
			"config":  config,
		})
	case "delete_instance":
		id, _ := params.Arguments["id"].(string)
		if id == "" {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "id is required")
			return
		}
		if id == APIKeyID {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "forbidden: API Key")
			return
		}

		instance, ok := m.FindInstance(id)
		if !ok {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "instance not found")
			return
		}

		instance.deleted = true
		m.Instances.Store(id, instance)

		if instance.Status != "stopped" {
			m.StopInstance(instance)
		}
		m.Instances.Delete(id)
		go m.SaveState()
		m.SendSSEEvent("delete", instance)

		m.WriteMCPResponse(w, req.ID, map[string]any{
			"content": []map[string]any{{"type": "text", "text": fmt.Sprintf("Deleted instance %s", id)}},
		})
	case "export_instances":
		exportPath := filepath.Join(filepath.Dir(m.StatePath), ExportFileName)
		var exports []map[string]any
		m.Instances.Range(func(_, value any) bool {
			instance := value.(*Instance)
			if instance.ID != APIKeyID {
				exports = append(exports, map[string]any{
					"alias":   instance.Alias,
					"url":     instance.URL,
					"restart": instance.Restart,
					"meta":    instance.Meta,
					"tcprx":   instance.TCPRX,
					"tcptx":   instance.TCPTX,
					"udprx":   instance.UDPRX,
					"udptx":   instance.UDPTX,
				})
			}
			return true
		})

		data, err := json.MarshalIndent(exports, "", "  ")
		if err != nil {
			m.WriteMCPError(w, req.ID, -32603, "Internal error", err.Error())
			return
		}

		if err := os.WriteFile(exportPath, data, 0644); err != nil {
			m.WriteMCPError(w, req.ID, -32603, "Internal error", err.Error())
			return
		}

		m.WriteMCPResponse(w, req.ID, map[string]any{
			"content": []map[string]any{{"type": "text", "text": fmt.Sprintf("Exported %d instances to %s", len(exports), exportPath)}},
		})
	case "import_instances":
		importPath := filepath.Join(filepath.Dir(m.StatePath), ExportFileName)
		data, err := os.ReadFile(importPath)
		if err != nil {
			m.WriteMCPError(w, req.ID, -32603, "Internal error", err.Error())
			return
		}

		var imports []map[string]any
		if err := json.Unmarshal(data, &imports); err != nil {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", err.Error())
			return
		}

		var created int
		for _, imp := range imports {
			instanceURL, _ := imp["url"].(string)
			if instanceURL == "" {
				continue
			}

			parsedURL, err := url.Parse(instanceURL)
			if err != nil {
				continue
			}

			instanceRole := parsedURL.Scheme
			if instanceRole != "client" && instanceRole != "server" {
				continue
			}

			id := GenerateID()
			instance := &Instance{
				ID:      id,
				Alias:   "",
				Type:    instanceRole,
				URL:     m.EnhanceURL(instanceURL, instanceRole),
				Status:  "stopped",
				Restart: true,
				Meta:    Meta{Tags: make(map[string]string)},
				stopped: make(chan struct{}),
			}

			if alias, ok := imp["alias"].(string); ok {
				instance.Alias = alias
			}
			if restart, ok := imp["restart"].(bool); ok {
				instance.Restart = restart
			}
			if meta, ok := imp["meta"].(map[string]any); ok {
				if metaBytes, err := json.Marshal(meta); err == nil {
					json.Unmarshal(metaBytes, &instance.Meta)
				}
			}
			if tcprx, ok := imp["tcprx"].(float64); ok {
				instance.TCPRX = uint64(tcprx)
			}
			if tcptx, ok := imp["tcptx"].(float64); ok {
				instance.TCPTX = uint64(tcptx)
			}
			if udprx, ok := imp["udprx"].(float64); ok {
				instance.UDPRX = uint64(udprx)
			}
			if udptx, ok := imp["udptx"].(float64); ok {
				instance.UDPTX = uint64(udptx)
			}

			instance.Config = m.GenerateConfigURL(instance)
			m.Instances.Store(id, instance)
			if instance.Restart {
				go m.StartInstance(instance)
			}
			created++
		}
		go func() {
			time.Sleep(BaseDuration)
			m.SaveState()
		}()

		m.WriteMCPResponse(w, req.ID, map[string]any{
			"content": []map[string]any{{"type": "text", "text": fmt.Sprintf("Imported %d instances from %s", created, importPath)}},
		})
	case "tcping_target":
		target, _ := params.Arguments["target"].(string)
		if target == "" {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "target is required")
			return
		}

		result := m.PerformTCPing(target)
		statusText := "failed"
		if result.Connected {
			statusText = fmt.Sprintf("connected in %dms", result.Latency)
		} else if result.Error != nil {
			statusText = fmt.Sprintf("failed: %s", *result.Error)
		}

		m.WriteMCPResponse(w, req.ID, map[string]any{
			"content": []map[string]any{{"type": "text", "text": fmt.Sprintf("TCPing %s: %s", target, statusText)}},
			"result":  result,
		})
	case "get_master_info":
		m.WriteMCPResponse(w, req.ID, map[string]any{
			"content": []map[string]any{{
				"type": "text",
				"text": "Master information retrieved",
			}},
			"info": m.GetMasterInfo(),
		})
	case "update_master_info":
		alias, _ := params.Arguments["alias"].(string)
		if alias == "" {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", "alias is required")
			return
		}

		if len(alias) > MaxValueLen {
			m.WriteMCPError(w, req.ID, -32602, "Invalid params", fmt.Sprintf("alias exceeds max length %d", MaxValueLen))
			return
		}

		m.Alias = alias
		if apiKey, ok := m.FindInstance(APIKeyID); ok {
			apiKey.Alias = m.Alias
			m.Instances.Store(APIKeyID, apiKey)
			go m.SaveState()
		}

		m.WriteMCPResponse(w, req.ID, map[string]any{
			"content": []map[string]any{{
				"type": "text",
				"text": fmt.Sprintf("Master alias updated to: %s", alias),
			}},
			"info": m.GetMasterInfo(),
		})
	default:
		m.WriteMCPError(w, req.ID, -32602, "Invalid params", "unknown tool")
	}
}

func (m *Master) WriteMCPResponse(w http.ResponseWriter, id any, result any) {
	response := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (m *Master) WriteMCPError(w http.ResponseWriter, id any, code int, message string, data any) {
	response := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    any    `json:"data,omitempty"`
		}{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
