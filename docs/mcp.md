# MCP Integration

## Overview

NodePass Master supports the [Model Context Protocol (MCP)](https://modelcontextprotocol.io) version **2025-11-25**, enabling AI assistants to directly manage NodePass instances through a standardized JSON-RPC 2.0 interface. MCP provides a more structured and extensible way for AI agents to interact with NodePass, complementing the existing REST API.

| Feature | REST API (v1) | MCP Protocol (v2) |
|---------|---------------|-------------------|
| **Path** | `/api/v1/*` | `/api/v2` |
| **Protocol** | HTTP Methods | JSON-RPC 2.0 |
| **Schema** | OpenAPI 3.0 | MCP Tools/Resources |
| **Events** | SSE (Server-Sent Events) | Stateless (no events) |
| **Client** | Browsers, curl, scripts | AI assistants (Claude, GPT, etc.) |
| **Authentication** | API Key header | API Key header |
| **Discovery** | `/openapi.json` | `initialize` + `tools/list` |

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
      "tools": {},
      "resources": {}
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

### 4. List Resources

**Method**: `resources/list`

Browse available instance resources.

**Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "resources/list"
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "result": {
    "resources": [
      {
        "uri": "nodepass://instance/abc123",
        "name": "My Instance",
        "description": "client instance: scheme://host:port",
        "mimeType": "application/json"
      }
    ]
  }
}
```

### 5. Read Resource

**Method**: `resources/read`

Get detailed instance information.

**Request**:
```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "resources/read",
  "params": {
    "uri": "nodepass://instance/abc123"
  }
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "result": {
    "contents": [
      {
        "uri": "nodepass://instance/abc123",
        "mimeType": "application/json",
        "text": "{\"id\":\"abc123\",\"alias\":\"My Instance\",...}"
      }
    ]
  }
}
```

## Available Tools

NodePass provides 9 MCP tools covering all instance and master management operations:

### Instance Management

#### 1. list_instances

List all NodePass instances.

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

Create a new NodePass instance.

**Arguments**:
- `url` (string, required): Instance URL (scheme://host:port/host:port)
- `alias` (string, optional): Instance alias

**Example**:
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "create_instance",
    "arguments": {
      "url": "client://server.example.com:443/127.0.0.1:22",
      "alias": "SSH Tunnel"
    }
  }
}
```

#### 4. update_instance

Update instance configuration.

**Arguments**:
- `id` (string, required): Instance ID
- `alias` (string, optional): New alias
- `action` (string, optional): Control action (start, stop, restart, reset)
- `restart` (boolean, optional): Auto-restart policy
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
      "action": "restart",
      "restart": true,
      "tags": {
        "env": "production",
        "team": "devops"
      }
    }
  }
}
```

#### 5. replace_instance_url

Replace instance URL (reconnects the instance).

**Arguments**:
- `id` (string, required): Instance ID
- `url` (string, required): New instance URL

**Example**:
```json
{
  "jsonrpc": "2.0",
  "id": 5,
  "method": "tools/call",
  "params": {
    "name": "replace_instance_url",
    "arguments": {
      "id": "abc123",
      "url": "client://new-server.example.com:443/127.0.0.1:22"
    }
  }
}
```

#### 6. delete_instance

Delete an instance.

**Arguments**:
- `id` (string, required): Instance ID

**Example**:
```json
{
  "jsonrpc": "2.0",
  "id": 6,
  "method": "tools/call",
  "params": {
    "name": "delete_instance",
    "arguments": {
      "id": "abc123"
    }
  }
}
```

### Network Tools

#### 7. tcping

Test TCP connectivity to a target.

**Arguments**:
- `target` (string, required): Target address (host:port)

**Example**:
```json
{
  "jsonrpc": "2.0",
  "id": 7,
  "method": "tools/call",
  "params": {
    "name": "tcping",
    "arguments": {
      "target": "example.com:443"
    }
  }
}
```

### Master Management

#### 8. get_master_info

Get master node information (CPU, memory, network, uptime, etc.).

**Arguments**: None

**Example**:
```json
{
  "jsonrpc": "2.0",
  "id": 8,
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
  "id": 8,
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
      "cpu": 15,
      "mem_total": 16777216,
      "mem_used": 8388608,
      "uptime": 3600,
      "ver": "1.0.0"
    }
  }
}
```

#### 9. update_master_info

Update master node alias.

**Arguments**:
- `alias` (string, required): New alias for master node

**Example**:
```json
{
  "jsonrpc": "2.0",
  "id": 9,
  "method": "tools/call",
  "params": {
    "name": "update_master_info",
    "arguments": {
      "alias": "Production Master"
    }
  }
}
```

## Resources

MCP resources provide a browsable view of all instances.

### Resource URI Format

```
nodepass://instance/{instance_id}
```

Example: `nodepass://instance/abc123`

### Browse Instances

Use `resources/list` to get all instance URIs, then `resources/read` to fetch details.

**Workflow**:
```
1. resources/list  → Get all instance URIs
2. resources/read  → Read specific instance JSON
```

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
        "url": "client://server.example.com:443/127.0.0.1:22",
        "alias": "SSH Tunnel"
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
    
    def create_instance(self, url, alias=None):
        args = {'url': url}
        if alias:
            args['alias'] = alias
        return self.call_tool('create_instance', args)
    
    def get_master_info(self):
        return self.call_tool('get_master_info')

# Usage
client = NodePassMCP('https://localhost:8443/api', 'your-api-key')

# Initialize
print(client.initialize())

# List instances
print(client.list_instances())

# Create instance
print(client.create_instance(
    'client://server.example.com:443/127.0.0.1:22',
    'SSH Tunnel'
))

# Get master info
print(client.get_master_info())
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
4. **Browse**: Use `resources/list` and `resources/read` for instance exploration

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
| Replace URL | ✅ PUT /instances/{id} | ✅ replace_instance_url |
| Delete instance | ✅ DELETE /instances/{id} | ✅ delete_instance |
| TCP ping | ✅ GET /tcping | ✅ tcping |
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