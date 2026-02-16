# MCP Integration

## Overview

NodePass Master supports the [Model Context Protocol (MCP)](https://modelcontextprotocol.io) version **2025-11-25**, enabling AI assistants to directly manage NodePass instances through a standardized JSON-RPC 2.0 interface. MCP provides a more structured and extensible way for AI agents to interact with NodePass, complementing the existing REST API.

| Feature | REST API (v1) | MCP Protocol (v2) |
|---------|---------------|-------------------|
| **Path** | `/api/v1/*` | `/api/v2` |
| **Protocol** | HTTP Methods | JSON-RPC 2.0 |
| **Schema** | OpenAPI 3.0 | MCP Tools |
| **Events** | SSE (Server-Sent Events) | Stateless (no events) |
| **Client** | Browsers, curl, scripts | AI assistants (Claude, GPT, etc.) |
| **Authentication** | API Key header | API Key header |
| **Discovery** | `/openapi.json` | `tools/list` |

## Getting Started

### Enable MCP

MCP is automatically enabled when you start Master mode:

```bash
# HTTP
nodepass "master://0.0.0.0:8080?log=info"
# MCP endpoint: http://localhost:8080/api/v2

# HTTPS with TLS
nodepass "master://0.0.0.0:8443?log=info&tls=1"
# MCP endpoint: https://localhost:8443/api/v2
```

### Authentication

Use the same API key as REST API (displayed on startup):

```bash
API Key: abc123def456...
```

Include in HTTP header:
```
X-API-Key: abc123def456...
```

## MCP Protocol

### Endpoint

```
POST /api/v2
Content-Type: application/json
X-API-Key: <your-api-key>
```

### JSON-RPC 2.0 Format

**Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "method_name",
  "params": { }
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": { }
}
```

**Error**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32602,
    "message": "Invalid params",
    "data": "error details"
  }
}
```

## MCP Methods

### 1. Initialize Session

**Method**: `initialize`

Establish connection and negotiate protocol version.

**Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2025-11-25",
    "capabilities": {},
    "clientInfo": {
      "name": "YourClient",
      "version": "1.0.0"
    }
  }
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2025-11-25",
    "capabilities": {
      "tools": {}
    },
    "serverInfo": {
      "name": "NodePass Master",
      "version": "x.x.x"
    }
  }
}
```

### 2. List Tools

**Method**: `tools/list`

Get all available MCP tools.

**Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/list"
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "tools": [
      {
        "name": "list_instances",
        "description": "List all NodePass instances",
        "inputSchema": {
          "type": "object",
          "properties": {}
        }
      },
      ...
    ]
  }
}
```

### 3. Call Tool

**Method**: `tools/call`

Execute a specific tool.

**Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "tool_name",
    "arguments": { }
  }
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Result description"
      }
    ]
  }
}
```

## Available Tools

NodePass provides 19 MCP tools covering all instance and master management operations:

### Instance Management

#### 1. list_instances

List all NodePass instances (excludes API Key instance).

**Arguments**: None

**Example**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "list_instances",
    "arguments": {}
  }
}
```

#### 2. get_instance

Get details of a specific instance.

**Arguments**:
- `id` (string, required): Instance ID

**Example**:
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "get_instance",
    "arguments": {
      "id": "abc123"
    }
  }
}
```

#### 3. create_instance

Create a new NodePass instance with structured configuration.

**Required Arguments**:
- `role` (string): Instance role - `server` or `client`
- `tunnel_port` (string): Tunnel port number
- `target_port` (string): Target port number

**Optional Arguments**:
- `alias` (string): Instance alias
- `tunnel_address` (string): Tunnel address (defaults: `0.0.0.0` for server, `localhost` for client)
- `target_address` (string): Target address (default: `localhost`)
- `targets` (string): Multiple targets in format `host1:port1,host2:port2,...`
- `password` (string): Connection password
- `log` (string): Log level - `none`, `debug`, `info`, `warn`, `error`, `event`
- `tls` (string): TLS mode - `0` (none), `1` (self-signed), `2` (custom, server only)
- `crt` (string): Certificate file path (for tls=2)
- `key` (string): Key file path (for tls=2)
- `sni` (string): SNI hostname (client dual-end mode)
- `dns` (string): DNS cache TTL (e.g., `5m`, `1h`)
- `lbs` (string): Load balancing - `0` (round-robin), `1` (optimal-latency), `2` (primary-backup)
- `mode` (string): Connection mode - `0` (auto), `1` (reverse/single-end), `2` (forward/dual-end)
- `type` (string): Pool type - `0` (TCP), `1` (QUIC), `2` (WebSocket), `3` (HTTP2, server only)
- `min` (string): Minimum pool size (client dual-end mode)
- `max` (string): Maximum pool size (dual-end mode)
- `dial` (string): Outbound source IP (default: auto)
- `read` (string): Read timeout (e.g., `30s`, `5m`, `1h`)
- `rate` (string): Bandwidth limit in Mbps (0=unlimited)
- `slot` (string): Connection slots (0=unlimited)
- `proxy` (string): PROXY protocol v1 - `0` (disabled), `1` (enabled)
- `block` (string): Block protocols - `1` (SOCKS), `2` (HTTP), `3` (TLS), combine like `123`
- `notcp` (string): Disable TCP - `0` (enabled), `1` (disabled)
- `noudp` (string): Disable UDP - `0` (enabled), `1` (disabled)

**Example** (basic client tunnel):
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "create_instance",
    "arguments": {
      "alias": "SSH Tunnel",
      "role": "client",
      "tunnel_address": "server.example.com",
      "tunnel_port": "443",
      "target_address": "127.0.0.1",
      "target_port": "22"
    }
  }
}
```

**Example** (advanced configuration):
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "create_instance",
    "arguments": {
      "alias": "Production Web Server",
      "role": "server",
      "tunnel_port": "443",
      "target_address": "127.0.0.1",
      "target_port": "8080",
      "log": "info",
      "tls": "1",
      "mode": "0",
      "type": "0",
      "rate": "100",
      "slot": "5000"
    }
  }
}
```

#### 4. update_instance

Update instance metadata (alias, restart policy, peer, tags).

**Arguments**:
- `id` (string, required): Instance ID
- `alias` (string, optional): New alias
- `restart` (boolean, optional): Auto-restart policy
- `peer` (object, optional): Service and peer information
  - `sid` (string): Service ID
  - `type` (string): Service type
  - `alias` (string): Service alias
- `tags` (object, optional): Metadata tags

**Example**:
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "update_instance",
    "arguments": {
      "id": "abc123",
      "alias": "New Name",
      "restart": true,
      "peer": {
        "sid": "service-001",
        "type": "web-server",
        "alias": "Production Web"
      },
      "tags": {
        "env": "production",
        "team": "devops"
      }
    }
  }
}
```

#### 5. control_instance

Control instance state with actions (start, stop, restart, reset).

**Arguments**:
- `id` (string, required): Instance ID
- `action` (string, required): Control action - `start`, `stop`, `restart`, `reset`

**Example**:
```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "tools/call",
  "params": {
    "name": "control_instance",
    "arguments": {
      "id": "abc123",
      "action": "restart"
    }
  }
}
```

**Reset Action**: Resets traffic statistics (TCPRX, TCPTX, UDPRX, UDPTX) to zero while preserving the instance.

#### 6. set_instance_basic

Set instance basic configuration (type, tunnel/target addresses, log level).

**Arguments**:
- `id` (string, required): Instance ID
- `type` (string, optional): Instance type (`server` or `client`)
- `tunnel_address` (string, optional): Tunnel bind/server IP address
- `tunnel_port` (string, optional): Tunnel bind/server port
- `target_address` (string, optional): Single target IP address
- `target_port` (string, optional): Single target port
- `targets` (string, optional): Multiple targets in format `host1:port1,host2:port2`
- `log` (string, optional): Log level - `none`, `debug`, `info`, `warn`, `error`, `event`

**Example**:
```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "method": "tools/call",
  "params": {
    "name": "set_instance_basic",
    "arguments": {
      "id": "abc123",
      "tunnel_port": "10102",
      "target_address": "192.168.1.100",
      "target_port": "3389",
      "log": "debug"
    }
  }
}
```

**Multiple Targets Example**:
```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "method": "tools/call",
  "params": {
    "name": "set_instance_basic",
    "arguments": {
      "id": "abc123",
      "targets": "10.0.1.10:8080,10.0.1.11:8080,10.0.1.12:8080"
    }
  }
}
```

#### 7. set_instance_security

Set instance security and encryption settings (password, TLS mode, certificates).

**Arguments**:
- `id` (string, required): Instance ID
- `password` (string, optional): Connection password (empty string to remove)
- `tls` (string, optional): TLS mode - `0` (none), `1` (self-signed), `2` (custom cert)
- `crt` (string, optional): Certificate file path (required when `tls=2`)
- `key` (string, optional): Private key file path (required when `tls=2`)
- `sni` (string, optional): SNI hostname for client TLS verification

**Example**:
```json
{
  "jsonrpc": "2.0",
  "id": 7,
  "method": "tools/call",
  "params": {
    "name": "set_instance_security",
    "arguments": {
      "id": "abc123",
      "password": "my-secret-password",
      "tls": "2",
      "crt": "/etc/nodepass/cert.pem",
      "key": "/etc/nodepass/key.pem"
    }
  }
}
```

#### 8. set_instance_connection

Set instance connection pool configuration.

**Arguments**:
- `id` (string, required): Instance ID
- `mode` (string, optional): Connection mode - `0` (auto), `1` (single-end), `2` (dual-end)
- `type` (string, optional): Pool type - `0` (TCP), `1` (QUIC), `2` (WebSocket), `3` (HTTP/2)
- `min` (string, optional): Minimum pool size (client instances only)
- `max` (string, optional): Maximum pool size (server instances only)

**Example**:
```json
{
  "jsonrpc": "2.0",
  "id": 7,
  "method": "tools/call",
  "params": {
    "name": "set_instance_connection",
    "arguments": {
      "id": "abc123",
      "mode": "2",
      "type": "1",
      "min": "128"
    }
  }
}
```

#### 9. set_instance_network

Set instance network tuning parameters.

**Arguments**:
- `id` (string, required): Instance ID
- `dns` (string, optional): DNS cache TTL (e.g., `5m`, `1h`, `30s`)
- `dial` (string, optional): Outbound source IP address for connections
- `read` (string, optional): Data read timeout (e.g., `30s`, `5m`)
- `rate` (string, optional): Bandwidth rate limit in Mbps
- `slot` (string, optional): Maximum concurrent connection slots

**Example**:
```json
{
  "jsonrpc": "2.0",
  "id": 9,
  "method": "tools/call",
  "params": {
    "name": "set_instance_network",
    "arguments": {
      "id": "abc123",
      "dns": "10m",
      "rate": "100",
      "slot": "5000"
    }
  }
}
```

#### 10. set_instance_protocol

Set instance protocol control settings.

**Arguments**:
- `id` (string, required): Instance ID
- `notcp` (string, optional): Disable TCP - `0` (enabled), `1` (disabled)
- `noudp` (string, optional): Disable UDP - `0` (enabled), `1` (disabled)
- `proxy` (string, optional): PROXY protocol - `0` (disabled), `1` (enabled)

**Example**:
```json
{
  "jsonrpc": "2.0",
  "id": 10,
  "method": "tools/call",
  "params": {
    "name": "set_instance_protocol",
    "arguments": {
      "id": "abc123",
      "noudp": "1",
      "proxy": "1"
    }
  }
}
```

#### 11. set_instance_traffic

Set instance traffic control and load balancing.

**Arguments**:
- `id` (string, required): Instance ID
- `block` (string, optional): Block protocols - `1` (SOCKS), `2` (HTTP), `3` (TLS), combine like `123`
- `lbs` (string, optional): Load balancing strategy - `0` (round-robin), `1` (random), `2` (least-connections)

**Example**:
```json
{
  "jsonrpc": "2.0",
  "id": 11,
  "method": "tools/call",
  "params": {
    "name": "set_instance_traffic",
    "arguments": {
      "id": "abc123",
      "block": "23",
      "lbs": "2"
    }
  }
}
```

#### 12. set_instance_advanced

Set instance advanced or custom parameters not covered by specific tools.

**Arguments**:
- `id` (string, required): Instance ID
- `parameters` (object, required): Key-value pairs of custom URL query parameters

**Example**:
```json
{
  "jsonrpc": "2.0",
  "id": 12,
  "method": "tools/call",
  "params": {
    "name": "set_instance_advanced",
    "arguments": {
      "id": "abc123",
      "parameters": {
        "custom_param": "value",
        "timeout": "60s"
      }
    }
  }
}
```

#### 13. get_instance_config

Get instance configuration in structured format.

**Arguments**:
- `id` (string, required): Instance ID

**Example**:
```json
{
  "jsonrpc": "2.0",
  "id": 13,
  "method": "tools/call",
  "params": {
    "name": "get_instance_config",
    "arguments": {
      "id": "abc123"
    }
  }
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 13,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Instance abc123 configuration retrieved"
      }
    ],
    "config": {
      "type": "server",
      "tunnel_address": "0.0.0.0",
      "tunnel_port": "8080",
      "target_address": "localhost",
      "target_port": "3000",
      "parameters": {
        "log": "info",
        "tls": "1",
        "dns": "5m",
        "max": "1024",
        "mode": "0",
        "type": "0",
        "dial": "auto",
        "read": "1h",
        "rate": "100",
        "slot": "65536",
        "proxy": "0",
        "notcp": "0",
        "noudp": "0"
      }
    }
  }
}
```

#### 14. delete_instance

Delete an instance.

**Arguments**:
- `id` (string, required): Instance ID

**Example**:
```json
{
  "jsonrpc": "2.0",
  "id": 14,
  "method": "tools/call",
  "params": {
    "name": "delete_instance",
    "arguments": {
      "id": "abc123"
    }
  }
}
```

#### 15. export_instances

Export all instances configuration to nodepass.json file (excludes API Key instance). File is saved in the same directory as nodepass.gob. Exported fields: alias, url, restart, meta, tcprx, tcptx, udprx, udptx.

**Arguments**: None

**Example**:
```json
{
  "jsonrpc": "2.0",
  "id": 15,
  "method": "tools/call",
  "params": {
    "name": "export_instances",
    "arguments": {}
  }
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 15,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Exported 5 instances to /path/to/nodepass.json"
      }
    ]
  }
}
```

#### 16. import_instances

Import instances configuration from nodepass.json file. Automatically generates new unique IDs for each instance. Instances are started based on their restart field.

**Arguments**: None

**Example**:
```json
{
  "jsonrpc": "2.0",
  "id": 16,
  "method": "tools/call",
  "params": {
    "name": "import_instances",
    "arguments": {}
  }
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 16,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Imported 5 instances from /path/to/nodepass.json"
      }
    ]
  }
}
```

### Network Tools

#### 17. tcping_target

Test TCP connectivity to a target.

**Arguments**:
- `target` (string, required): Target address (host:port)

**Example**:
```json
{
  "jsonrpc": "2.0",
  "id": 17,
  "method": "tools/call",
  "params": {
    "name": "tcping_target",
    "arguments": {
      "target": "example.com:443"
    }
  }
}
```

### Master Management

#### 18. get_master_info

Get master node information (CPU, memory, network, uptime, etc.).

**Arguments**: None

**Example**:
```json
{
  "jsonrpc": "2.0",
  "id": 18,
  "method": "tools/call",
  "params": {
    "name": "get_master_info",
    "arguments": {}
  }
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 18,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Master node information retrieved"
      }
    ],
    "info": {
      "mid": "abc123",
      "alias": "Master Node",
      "os": "linux",
      "arch": "amd64",
      "noc": 8,
      "cpu": 15,
      "mem_total": 16777216,
      "mem_used": 8388608,
      "uptime": 3600,
      "ver": "1.0.0"
    }
  }
}
```

#### 19. update_master_info

Update master node alias.

**Arguments**:
- `alias` (string, required): New alias for master node

**Example**:
```json
{
  "jsonrpc": "2.0",
  "id": 19,
  "method": "tools/call",
  "params": {
    "name": "update_master_info",
    "arguments": {
      "alias": "Production Master"
    }
  }
}
```

## Tool Design Philosophy

### Fine-Grained Configuration Updates

NodePass MCP tools follow a **domain-driven design** approach, organizing instance configuration updates into logical categories. This design provides several benefits:

1. **Precision**: Update only what you need without affecting other settings
2. **Safety**: Reduce risk of unintended configuration changes
3. **Clarity**: Tool names clearly indicate what they modify
4. **Composability**: Combine multiple tool calls for complex configurations
5. **Validation**: Each tool validates only relevant parameters

### Configuration Categories

Instance configuration tools are organized by functional domain:

| Tool | Domain | Purpose |
|------|--------|---------|
| `update_instance` | Metadata | Alias, peer, tags, restart policy |
| `control_instance` | State Control | Start, stop, restart, reset |
| `set_instance_basic` | Addressing | Type, tunnel/target addresses, log level |
| `set_instance_security` | Encryption | Password, TLS mode, certificates, SNI |
| `set_instance_connection` | Connection Pool | Mode, type, pool size limits |
| `set_instance_network` | Network Tuning | DNS cache, source IP, timeouts, rate limits |
| `set_instance_protocol` | Protocol Control | TCP/UDP enable/disable, PROXY protocol |
| `set_instance_traffic` | Traffic Control | Protocol blocking, load balancing |
| `set_instance_advanced` | Custom Parameters | Any additional URL parameters |
| `get_instance_config` | Configuration Query | Retrieve current configuration in structured format |

### Best Practices

#### Single Concern Updates

Update one configuration domain at a time for clarity and safety:

```json
// Good: Set only network tuning
{
  "name": "set_instance_network",
  "arguments": {
    "id": "abc123",
    "rate": "100",
    "slot": "5000"
  }
}

// Good: Separate security configuration
{
  "name": "set_instance_security",
  "arguments": {
    "id": "abc123",
    "tls": "2",
    "crt": "/path/to/cert.pem",
    "key": "/path/to/key.pem"
  }
}
```

#### Sequential Tool Calls

For complex reconfigurations, make multiple sequential calls:

```json
// Step 1: Set addresses
{"name": "set_instance_basic", "arguments": {"id": "abc123", "tunnel_port": "10102"}}

// Step 2: Enable TLS
{"name": "set_instance_security", "arguments": {"id": "abc123", "tls": "1"}}

// Step 3: Tune network
{"name": "set_instance_network", "arguments": {"id": "abc123", "rate": "100"}}

// Step 4: Restart to apply
{"name": "control_instance", "arguments": {"id": "abc123", "action": "restart"}}
```

#### Parameter Validation

Each update tool performs domain-specific validation:

- **Basic**: Validates address formats and port ranges
- **Security**: Checks TLS mode compatibility and certificate paths
- **Connection**: Validates mode/type combinations
- **Network**: Verifies timeout formats and numeric limits
- **Protocol**: Ensures valid enable/disable flags
- **Traffic**: Validates block codes and LBS strategies

### Backward Compatibility

The new fine-grained tools are designed to:

- **Coexist**: Work alongside existing metadata updates (`update_instance`)
- **Restart automatically**: Changes trigger instance restart only when needed
- **Preserve config**: Unmodified parameters remain unchanged
- **Fail gracefully**: Invalid updates return clear error messages

## Complete Example

Here's a complete workflow using curl:

```bash
# Set variables
API_URL="https://localhost:8443/api/v2"
API_KEY="your-api-key-here"

# 1. Initialize
curl -X POST "$API_URL" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2025-11-25",
      "capabilities": {}
    }
  }'

# 2. List available tools
curl -X POST "$API_URL" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/list"
  }'

# 3. List all instances
curl -X POST "$API_URL" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -d '{
    "jsonrpc": "2.0",
    "id": 3,
    "method": "tools/call",
    "params": {
      "name": "list_instances",
      "arguments": {}
    }
  }'

# 4. Create new instance
curl -X POST "$API_URL" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -d '{
    "jsonrpc": "2.0",
    "id": 4,
    "method": "tools/call",
    "params": {
      "name": "create_instance",
      "arguments": {
        "alias": "SSH Tunnel",
        "role": "client",
        "tunnel_address": "server.example.com",
        "tunnel_port": "443",
        "target_address": "127.0.0.1",
        "target_port": "22",
        "log": "info",
        "tls": "1"
      }
    }
  }'

# 5. Get master information
curl -X POST "$API_URL" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $API_KEY" \
  -d '{
    "jsonrpc": "2.0",
    "id": 5,
    "method": "tools/call",
    "params": {
      "name": "get_master_info",
      "arguments": {}
    }
  }'
```

## Practical Use Cases

### Use Case 1: Migrate Instance to New Server

Scenario: Move a tunnel to a different server with enhanced security.

```bash
INSTANCE_ID="abc123"

# Step 1: Update basic configuration (new tunnel server)
curl -X POST "$API_URL" -H "X-API-Key: $API_KEY" -H "Content-Type: application/json" -d '{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "set_instance_basic",
    "arguments": {
      "id": "'$INSTANCE_ID'",
      "tunnel_address": "new-server.example.com",
      "tunnel_port": "443"
    }
  }
}'

# Step 2: Enable TLS with custom certificates
curl -X POST "$API_URL" -H "X-API-Key: $API_KEY" -H "Content-Type: application/json" -d '{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "set_instance_security",
    "arguments": {
      "id": "'$INSTANCE_ID'",
      "tls": "2",
      "crt": "/etc/nodepass/prod.crt",
      "key": "/etc/nodepass/prod.key",
      "sni": "new-server.example.com"
    }
  }
}'

# Step 3: Optimize network settings
curl -X POST "$API_URL" -H "X-API-Key: $API_KEY" -H "Content-Type: application/json" -d '{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "set_instance_network",
    "arguments": {
      "id": "'$INSTANCE_ID'",
      "dns": "1h",
      "rate": "500",
      "slot": "10000"
    }
  }
}'
```

### Use Case 2: Configure Load Balanced Backend

Scenario: Set up a client to connect to multiple backend servers with load balancing.

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "set_instance_basic",
    "arguments": {
      "id": "xyz789",
      "targets": "10.0.1.10:8080,10.0.1.11:8080,10.0.1.12:8080"
    }
  }
}

{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "set_instance_traffic",
    "arguments": {
      "id": "xyz789",
      "lbs": "2",
      "block": "23"
    }
  }
}
```

### Use Case 3: Apply Network Restrictions

Scenario: Lock down an instance to TCP-only with PROXY protocol.

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "set_instance_protocol",
    "arguments": {
      "id": "def456",
      "noudp": "1",
      "proxy": "1"
    }
  }
}
```

### Use Case 4: Performance Tuning for High-Traffic Instance

Scenario: Optimize connection pool and network settings for high-throughput scenario.

```json
// Step 1: Upgrade to QUIC with larger pool
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "set_instance_connection",
    "arguments": {
      "id": "high-traffic-01",
      "mode": "2",
      "type": "1",
      "min": "256"
    }
  }
}

// Step 2: Increase rate limit and slots
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "set_instance_network",
    "arguments": {
      "id": "high-traffic-01",
      "rate": "1000",
      "slot": "50000",
      "dns": "30m"
    }
  }
}
```

## Python Client Example

```python
import requests
import json

class NodePassMCP:
    def __init__(self, base_url, api_key):
        self.base_url = base_url
        self.api_key = api_key
        self.headers = {
            'Content-Type': 'application/json',
            'X-API-Key': api_key
        }
        self.request_id = 0
    
    def call(self, method, params=None):
        self.request_id += 1
        payload = {
            'jsonrpc': '2.0',
            'id': self.request_id,
            'method': method,
            'params': params or {}
        }
        response = requests.post(
            f"{self.base_url}/v2",
            headers=self.headers,
            json=payload,
            verify=False  # For self-signed certs
        )
        return response.json()
    
    def initialize(self):
        return self.call('initialize', {
            'protocolVersion': '2025-11-25',
            'capabilities': {}
        })
    
    def list_tools(self):
        return self.call('tools/list')
    
    def call_tool(self, name, arguments=None):
        return self.call('tools/call', {
            'name': name,
            'arguments': arguments or {}
        })
    
    def list_instances(self):
        return self.call_tool('list_instances')
    
    def create_instance(self, role, tunnel_port, target_port, **kwargs):
        """Create a new instance with structured configuration
        
        Required:
            role: 'server' or 'client'
            tunnel_port: Tunnel port number
            target_port: Target port number
        
        Optional (pass as kwargs):
            alias, tunnel_address, target_address, targets, password,
            log, tls, crt, key, sni, dns, lbs, mode, type, min, max,
            dial, read, rate, slot, proxy, block, notcp, noudp
        """
        args = {
            'role': role,
            'tunnel_port': tunnel_port,
            'target_port': target_port
        }
        args.update(kwargs)
        return self.call_tool('create_instance', args)
    
    def set_instance_basic(self, instance_id, **kwargs):
        """Set basic configuration (type, addresses, log level)"""
        args = {'id': instance_id}
        args.update(kwargs)
        return self.call_tool('set_instance_basic', args)
    
    def set_instance_security(self, instance_id, **kwargs):
        """Set security settings (password, TLS, certificates, SNI)"""
        args = {'id': instance_id}
        args.update(kwargs)
        return self.call_tool('set_instance_security', args)
    
    def set_instance_connection(self, instance_id, **kwargs):
        """Set connection pool settings"""
        args = {'id': instance_id}
        args.update(kwargs)
        return self.call_tool('set_instance_connection', args)
    
    def set_instance_network(self, instance_id, **kwargs):
        """Set network tuning parameters"""
        args = {'id': instance_id}
        args.update(kwargs)
        return self.call_tool('set_instance_network', args)
    
    def set_instance_protocol(self, instance_id, **kwargs):
        """Set protocol control settings"""
        args = {'id': instance_id}
        args.update(kwargs)
        return self.call_tool('set_instance_protocol', args)
    
    def set_instance_traffic(self, instance_id, **kwargs):
        """Set traffic control and load balancing"""
        args = {'id': instance_id}
        args.update(kwargs)
        return self.call_tool('set_instance_traffic', args)
    
    def get_master_info(self):
        return self.call_tool('get_master_info')

# Usage Example 1: Basic instance creation and configuration
client = NodePassMCP('https://localhost:8443/api', 'your-api-key')

# Initialize connection
client.initialize()

# Create a new instance
result = client.create_instance(
    role='client',
    tunnel_port='10101',
    target_port='22',
    alias='SSH Tunnel',
    tunnel_address='server.example.com',
    target_address='127.0.0.1',
    log='info',
    tls='1'
)
instance_id = result['result']['instance']['id']

# Configure the instance step by step
# 1. Enable TLS security
client.set_instance_security(
    instance_id,
    tls='1',
    sni='server.example.com'
)

# 2. Optimize network settings
client.set_instance_network(
    instance_id,
    dns='1h',
    rate='100',
    slot='5000'
)

# 3. Configure connection pool
client.set_instance_connection(
    instance_id,
    mode='2',
    type='1',
    min='128'
)

# Usage Example 2: Migrate instance to load-balanced backends
client.set_instance_basic(
    instance_id,
    targets='10.0.1.10:8080,10.0.1.11:8080,10.0.1.12:8080'
)

client.set_instance_traffic(
    instance_id,
    lbs='2'  # Least connections
)

# Usage Example 3: Restrict protocols
client.set_instance_protocol(
    instance_id,
    noudp='1',
    proxy='1'
)

# Get master system information
info = client.get_master_info()
print(f"Master uptime: {info['result']['info']['uptime']}s")
```

## Integration with AI Assistants

### Claude Desktop

Configure MCP server in `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "nodepass": {
      "command": "npx",
      "args": ["-y", "@nodepass/mcp-client"],
      "env": {
        "NODEPASS_URL": "https://localhost:8443/api",
        "NODEPASS_API_KEY": "your-api-key"
      }
    }
  }
}
```

### Custom Integration

Any MCP-compatible client can integrate:

1. **Initialize**: Establish connection with `initialize` method
2. **Discover**: Use `tools/list` to get available operations
3. **Execute**: Call tools via `tools/call` method

## Security Considerations

### TLS/HTTPS

Always use HTTPS in production:

```bash
nodepass "master://0.0.0.0:8443?log=info&tls=1"
```

### API Key Protection

- Store API key securely (environment variables, secrets manager)
- Never commit API keys to version control
- Rotate keys regularly
- Use HTTPS to prevent key interception

### Network Security

- Restrict Master API to trusted networks
- Use firewall rules to limit access
- Consider VPN or SSH tunneling for remote access

## Error Handling

### Common Error Codes

| Code | Message | Description |
|------|---------|-------------|
| -32700 | Parse error | Invalid JSON |
| -32600 | Invalid Request | Invalid JSON-RPC 2.0 format |
| -32601 | Method not found | Unknown MCP method |
| -32602 | Invalid params | Missing or invalid parameters |
| -32603 | Internal error | Server-side error |

### Example Error Response

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32602,
    "message": "Invalid params",
    "data": "instance not found"
  }
}
```

## Best Practices

### 1. Version Negotiation

Always specify protocol version in `initialize`:

```json
{
  "protocolVersion": "2025-11-25"
}
```

### 2. Error Handling

Check for `error` field in responses:

```python
response = client.call_tool('get_instance', {'id': 'abc123'})
if 'error' in response:
    print(f"Error: {response['error']['message']}")
else:
    print(f"Success: {response['result']}")
```

### 3. Idempotency

Some operations are idempotent:
- `get_instance`, `list_instances`: Always safe to retry
- `create_instance`: May create duplicates
- `delete_instance`: Safe if instance doesn't exist

### 4. Resource Cleanup

Delete instances when no longer needed:

```json
{
  "jsonrpc": "2.0",
  "method": "tools/call",
  "params": {
    "name": "delete_instance",
    "arguments": {"id": "abc123"}
  }
}
```

## Comparison with REST API

### When to Use MCP (v2)

- AI assistant integration
- Automated workflows with LLMs
- Tool-based interactions
- Programmatic access from AI agents

### When to Use REST (v1)

- Web frontend development
- Traditional API clients
- Real-time event monitoring (SSE)
- Human-readable API documentation (Swagger)

### Feature Parity

| Feature | REST API | MCP Protocol |
|---------|----------|--------------|
| List instances | ✅ GET /instances | ✅ list_instances |
| Get instance | ✅ GET /instances/{id} | ✅ get_instance |
| Create instance | ✅ POST /instances | ✅ create_instance |
| Update instance | ✅ PATCH /instances/{id} | ✅ update_instance |
| Replace URL | ✅ PUT /instances/{id} | ✅ set_instance_* |
| Delete instance | ✅ DELETE /instances/{id} | ✅ delete_instance |
| TCP ping | ✅ GET /tcping | ✅ tcping_target |
| Get master info | ✅ GET /info | ✅ get_master_info |
| Update master | ✅ POST /info | ✅ update_master_info |
| Real-time events | ✅ GET /events (SSE) | ❌ Not needed |
| OpenAPI docs | ✅ /openapi.json | ❌ Self-describing |

## Troubleshooting

### Connection Issues

**Problem**: Cannot connect to MCP endpoint

**Solution**:
1. Verify Master is running: `curl https://localhost:8443/api/v1/info`
2. Check TLS settings match (HTTP vs HTTPS)
3. Verify API key is correct
4. Check firewall rules

### Authentication Failures

**Problem**: `401 Unauthorized` or `403 Forbidden`

**Solution**:
1. Check API key in `X-API-Key` header
2. Verify key matches the one shown on Master startup
3. Ensure no extra whitespace in header value

### Version Mismatch

**Problem**: Protocol version not supported

**Solution**:
1. Check server version: `get_master_info` → `ver` field
2. Update client to use `"protocolVersion": "2025-11-25"`
3. Server auto-negotiates to compatible version

### Tool Not Found

**Problem**: `"error": {"code": -32602, "message": "unknown tool"}`

**Solution**:
1. List available tools: `tools/list`
2. Check tool name spelling
3. Verify server version supports the tool

## Reference

### Official Resources

- [MCP Specification](https://modelcontextprotocol.io/specification/2025-11-25)

### Related Documentation

- [REST API Reference](/docs/api.md)
- [Configuration Options](/docs/configuration.md)
- [Usage Examples](/docs/examples.md)
- [How NodePass Works](/docs/how-it-works.md)
- [Installation Guide](/docs/installation.md)
- [Troubleshooting Guide](/docs/troubleshooting.md)
- [Usage Instructions](/docs/usage.md)