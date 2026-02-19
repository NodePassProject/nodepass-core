# Usage Instructions

NodePass creates tunnels with an unencrypted TCP control channel and configurable TLS encryption options for data exchange. This guide covers the three operating modes and explains how to use each effectively.

## Command Line Syntax

NodePass supports two command-line syntax formats that provide the same functionality. Choose the format that best fits your workflow and preference.

### URL-Based Syntax

The URL-based syntax provides a compact and expressive configuration format:

```bash
nodepass "<mode>://<tunnel_addr>/<target_addr>?param1=value1&param2=value2"
```

**Advantages:**
- Compact and easy to use in scripts and configuration files
- Single string contains all parameters
- Easy to pass as environment variables in containerized deployments
- Ideal for automation and quick testing

**Example:**
```bash
nodepass "server://0.0.0.0:10101/0.0.0.0:8080?log=info&tls=1&max=2048"
```

### Flag-Based Syntax

The flag-based syntax uses traditional command-line flags:

```bash
nodepass <mode> --tunnel-addr <addr> --tunnel-port <port> --target-addr <addr> --target-port <port> [--option value ...]
```

**Advantages:**
- More readable when typing commands interactively
- Explicit parameter names make configuration self-documenting
- Easier to understand for users familiar with traditional Unix tools
- Supports shell completion for parameter discovery

**Example:**
```bash
nodepass server \
  --tunnel-addr 0.0.0.0 \
  --tunnel-port 10101 \
  --target-addr 0.0.0.0 \
  --target-port 8080 \
  --log info \
  --tls 1 \
  --max 2048
```

### Parameter Overview

Where:
- `<mode>`: Operating mode - `server`, `client`, or `master`
- `<tunnel_addr>`: Tunnel endpoint address for control channel communications
- `<target_addr>`: Destination address for business data with bidirectional flow support (or API prefix in master mode)

### Common Parameters

All parameters can be specified using either URL query parameters or command-line flags:

| Parameter | Flag | Description | Default | Valid Values |
|-----------|------|-------------|---------|--------------|
| `log` | `--log` | Log verbosity level | `info` | `none`, `debug`, `info`, `warn`, `error`, `event` |
| `dns` | `--dns` | DNS cache TTL duration | `5m` | Time units: `1h`, `30m`, `15s`, etc. |
| `sni` | `--sni` | SNI hostname for TLS (client only) | auto | Hostname string |
| `lbs` | `--lbs` | Load balancing strategy | `0` | `0`=random, `1`=roundrobin, `2`=leastconn |
| `min` | `--min` | Minimum pool capacity (client) | `64` | Positive integer |
| `max` | `--max` | Maximum pool capacity (server) | `1024` | Positive integer |
| `mode` | `--mode` | Run mode control | `0` | `0`=auto, `1`=force-mode-1, `2`=force-mode-2 |
| `type` | `--type` | Connection pool type (server) | `0` | `0`=TCP, `1`=QUIC, `2`=WebSocket, `3`=HTTP/2 |
| `tls` | `--tls` | TLS encryption mode | `0` | `0`=none, `1`=self-signed, `2`=custom |
| `crt` | `--crt` | Certificate file path | N/A | File path (when `tls=2`) |
| `key` | `--key` | Private key file path | N/A | File path (when `tls=2`) |
| `dial` | `--dial` | Source IP for outbound | `auto` | IP address or `auto` |
| `read` | `--read` | Data read timeout | `0` | Time units: `30s`, `5m`, `1h`, etc. |
| `rate` | `--rate` | Bandwidth rate limit (Mbps) | `0` | `0`=unlimited or positive integer |
| `slot` | `--slot` | Max concurrent connections | `65536` | `0`=unlimited or positive integer |
| `proxy` | `--proxy` | PROXY protocol support | `0` | `0`=disabled, `1`=enabled |
| `block` | `--block` | Protocol blocking | `0` | Digits: `1`=SOCKS, `2`=HTTP, `3`=TLS |
| `notcp` | `--notcp` | Disable TCP | `0` | `0`=enabled, `1`=disabled |
| `noudp` | `--noudp` | Disable UDP | `0` | `0`=enabled, `1`=disabled |

**Note:** For detailed flag-based syntax and complete parameter reference, see the [CLI Reference](/docs/cli.md).

## Operating Modes

NodePass offers three complementary operating modes to suit various deployment scenarios.

### Server Mode

Server mode establishes tunnel control channels and supports bidirectional data flow forwarding.

```bash
nodepass "server://<tunnel_addr>/<target_addr>?log=<level>&tls=<mode>&crt=<cert_file>&key=<key_file>&dns=<duration>&type=<pool_type>&max=<max_pool>&mode=<run_mode>&dial=<source_ip>&read=<timeout>&rate=<mbps>&slot=<limit>&proxy=<mode>&notcp=<0|1>&noudp=<0|1>"
```

#### Parameters

- `tunnel_addr`: Address for the TCP tunnel endpoint (control channel) that clients will connect to (e.g., 10.1.0.1:10101)
- `target_addr`: The destination address for business data with bidirectional flow support (e.g., 10.1.0.1:8080)
- `log`: Log level (debug, info, warn, error, event)
- `dns`: DNS cache TTL duration (default: 5m, supports time units like `1h`, `30m`, `15s`, etc.)
- `type`: Connection pool type (0, 1, 2, 3)
  - `0`: Use TCP-based connection pool (default)
  - `1`: Use QUIC-based UDP connection pool with stream multiplexing(requires TLS, minimum `tls=1`)
  - `2`: Use WebSocket/WSS-based connection pool
  - `3`: Use HTTP/2-based connection pool with multiplexed streams (requires TLS, minimum `tls=1`)
  - Configuration is automatically delivered to client during handshake
- `tls`: TLS encryption mode for the target data channel (0, 1, 2)
  - `0`: No TLS encryption (plain TCP/UDP)
  - `1`: Self-signed certificate (automatically generated)
  - `2`: Custom certificate (requires `crt` and `key` parameters)
- `crt`: Path to certificate file (required when `tls=2`)
- `key`: Path to private key file (required when `tls=2`)
- `max`: Maximum connection pool capacity (default: 1024)
- `mode`: Run mode control for data flow direction
  - `0`: Automatic detection (default) - attempts local binding first, falls back if unavailable
  - `1`: Force reverse mode - server binds to target address locally and receives traffic
  - `2`: Force forward mode - server connects to remote target address
- `dial`: Source IP address for outbound connections to target (default: `auto` for system-selected IP)
- `read`: Data read timeout duration (default: 0, supports time units like 30s, 5m, 1h, etc.)
- `rate`: Bandwidth rate limit (default: 0 means no limit)
- `slot`: Maximum concurrent connection limit (default: 65536, 0 means unlimited)
- `proxy`: PROXY protocol support (default: `0`, `1` enables PROXY protocol v1 header before data transfer)
- `notcp`: TCP support control (default: `0` enabled, `1` disabled)
- `noudp`: UDP support control (default: `0` enabled, `1` disabled)

#### How Server Mode Works

Server mode supports automatic mode detection or forced mode selection through the `mode` parameter:

**Mode 0: Automatic Detection** (default)
- Attempts to bind to `target_addr` locally first
- If successful, operates in reverse mode (server receives traffic)  
- If binding fails, operates in forward mode (server sends traffic)

**Mode 1: Reverse Mode** (server receives traffic)
1. Listens for TCP tunnel connections (control channel) on `tunnel_addr`
2. Binds to and listens for incoming TCP and UDP traffic on `target_addr` 
3. When a connection arrives at `target_addr`, it signals the connected client through the control channel
4. Creates a data channel for each connection with the specified TLS encryption level

**Mode 2: Forward Mode** (server sends traffic)
1. Listens for TCP tunnel connections (control channel) on `tunnel_addr`
2. Waits for clients to listen locally and receive connections through the tunnel
3. Establishes connections to remote `target_addr` and forwards data

#### Examples

**URL-Based Syntax:**

```bash
# Automatic mode detection with no TLS encryption
nodepass "server://10.1.0.1:10101/10.1.0.1:8080?log=debug&tls=0"

# Force reverse mode with self-signed certificate
nodepass "server://10.1.0.1:10101/10.1.0.1:8080?log=debug&tls=1&mode=1"

# Force forward mode with custom certificate
nodepass "server://10.1.0.1:10101/192.168.1.100:8080?log=debug&tls=2&mode=2&crt=/path/to/cert.pem&key=/path/to/key.pem"

# Multiple targets with load balancing
nodepass "server://10.1.0.1:10101/10.1.0.1:8080,10.1.0.2:8080,10.1.0.3:8080?log=debug&lbs=1&tls=1"

# QUIC pool with automatic TLS
nodepass "server://10.1.0.1:10101/192.168.1.100:8080?log=debug&type=1&mode=2"

# WebSocket pool with custom certificate
nodepass "server://10.1.0.1:10101/192.168.1.100:8080?log=debug&type=2&tls=2&mode=2&crt=/path/to/cert.pem&key=/path/to/key.pem"

# HTTP/2 pool with automatic TLS
nodepass "server://10.1.0.1:10101/192.168.1.100:8080?log=debug&type=3&mode=2&tls=1"

# Block SOCKS protocols
nodepass "server://10.1.0.1:10101/10.1.0.1:8080?block=1&log=info"
```

**Flag-Based Syntax:**

```bash
# Automatic mode detection with no TLS encryption
nodepass server \
  --tunnel-addr 10.1.0.1 \
  --tunnel-port 10101 \
  --target-addr 10.1.0.1 \
  --target-port 8080 \
  --log debug \
  --tls 0

# Force reverse mode with self-signed certificate
nodepass server \
  --tunnel-addr 10.1.0.1 \
  --tunnel-port 10101 \
  --target-addr 10.1.0.1 \
  --target-port 8080 \
  --log debug \
  --tls 1 \
  --mode 1

# Force forward mode with custom certificate
nodepass server \
  --tunnel-addr 10.1.0.1 \
  --tunnel-port 10101 \
  --target-addr 192.168.1.100 \
  --target-port 8080 \
  --log debug \
  --tls 2 \
  --mode 2 \
  --crt /path/to/cert.pem \
  --key /path/to/key.pem

# Multiple targets with load balancing
nodepass server \
  --tunnel-addr 10.1.0.1 \
  --tunnel-port 10101 \
  --targets "10.1.0.1:8080,10.1.0.2:8080,10.1.0.3:8080" \
  --log debug \
  --lbs 1 \
  --tls 1

# QUIC pool with automatic TLS
nodepass server \
  --tunnel-addr 10.1.0.1 \
  --tunnel-port 10101 \
  --target-addr 192.168.1.100 \
  --target-port 8080 \
  --log debug \
  --type 1 \
  --mode 2

# Block SOCKS protocols
nodepass server \
  --tunnel-addr 10.1.0.1 \
  --tunnel-port 10101 \
  --target-addr 10.1.0.1 \
  --target-port 8080 \
  --block 1 \
  --log info
```

### Client Mode

Client mode connects to a NodePass server and supports bidirectional data flow forwarding.

```bash
nodepass "client://<tunnel_addr>/<target_addr>?log=<level>&dns=<duration>&min=<min_pool>&mode=<run_mode>&dial=<source_ip>&read=<timeout>&rate=<mbps>&slot=<limit>&proxy=<mode>&notcp=<0|1>&noudp=<0|1>"
```

#### Parameters

- `tunnel_addr`: Address of the NodePass server's tunnel endpoint to connect to (e.g., 10.1.0.1:10101)
- `target_addr`: The destination address for business data with bidirectional flow support (e.g., 127.0.0.1:8080)
- `log`: Log level (debug, info, warn, error, event)
- `dns`: DNS cache TTL duration (default: 5m, supports time units like `1h`, `30m`, `15s`, etc.)
- `min`: Minimum connection pool capacity (default: 64)
- `mode`: Run mode control for client behavior
  - `0`: Automatic detection (default) - attempts local binding first, falls back to handshake mode
  - `1`: Force single-end forwarding mode - local proxy with connection pooling
  - `2`: Force dual-end handshake mode - requires server coordination
- `dial`: Source IP address for outbound connections to target (default: `auto` for system-selected IP)
- `read`: Data read timeout duration (default: 0, supports time units like 30s, 5m, 1h, etc.)
- `rate`: Bandwidth rate limit (default: 0 means no limit)
- `slot`: Maximum concurrent connection limit (default: 65536, 0 means unlimited)
- `proxy`: PROXY protocol support (default: `0`, `1` enables PROXY protocol v1 header before data transfer)
- `notcp`: TCP support control (default: `0` enabled, `1` disabled)
- `noudp`: UDP support control (default: `0` enabled, `1` disabled)

**Note**: Connection pool type configuration is automatically received from the server during handshake. Clients do not need to specify the `type` parameter.

#### How Client Mode Works

Client mode supports automatic mode detection or forced mode selection through the `mode` parameter:

**Mode 0: Automatic Detection** (default)
- Attempts to bind to `tunnel_addr` locally first
- If successful, operates in single-end forwarding mode
- If binding fails, operates in dual-end handshake mode

**Mode 1: Single-End Forwarding Mode**
1. Listens for TCP and UDP connections on the local tunnel address
2. Uses connection pooling technology to pre-establish TCP connections to target address, eliminating connection latency
3. Directly forwards received traffic to the target address with high performance
4. No handshake with server required, enables point-to-point direct forwarding
5. Suitable for local proxy and simple forwarding scenarios

**Mode 2: Dual-End Handshake Mode**
- **Client Receives Traffic** (when server sends traffic)
  1. Connects to the server's TCP tunnel endpoint (control channel)
  2. Listens locally and waits for connections through the tunnel
  3. Establishes connections to local `target_addr` and forwards data

- **Client Sends Traffic** (when server receives traffic)
  1. Connects to the server's TCP tunnel endpoint (control channel)
  2. Listens for signals from the server through this control channel
  3. When a signal is received, establishes a data connection with the TLS security level specified by the server
  4. Creates a connection to `target_addr` and forwards traffic

#### Examples

**URL-Based Syntax:**

```bash
# Automatic mode detection - Local proxy listening on port 1080, forwarding to target server
nodepass "client://127.0.0.1:1080/target.example.com:8080?log=debug"

# Force single-end forwarding mode - High performance local proxy
nodepass "client://127.0.0.1:1080/target.example.com:8080?mode=1&log=debug"

# Force dual-end handshake mode - Connect to NodePass server
nodepass "client://server.example.com:10101/127.0.0.1:8080?mode=2"

# Connect with debug logging and custom connection pool capacity
nodepass "client://server.example.com:10101/192.168.1.100:8080?log=debug&min=128"

# Multiple targets with load balancing
nodepass "client://127.0.0.1:1080/10.1.0.1:8080,10.1.0.2:8080?lbs=2&mode=1"

# Custom SNI hostname for TLS
nodepass "client://server.example.com:10101/127.0.0.1:8080?mode=2&sni=backend.internal.example.com"

# Resource-constrained configuration with forced mode
nodepass "client://server.example.com:10101/127.0.0.1:8080?mode=2&min=16&log=info"

# Client for real-time applications
nodepass "client://server.example.com:10101/127.0.0.1:7777?mode=2&min=64&read=30s"

# Block SOCKS and HTTP protocols
nodepass "client://127.0.0.1:1080/target.example.com:8080?block=12&mode=1"

# Custom DNS cache TTL for dynamic environments
nodepass "client://server.example.com:10101/database.local:3306?dns=30s&mode=2"
```

**Flag-Based Syntax:**

```bash
# Automatic mode detection - Local proxy
nodepass client \
  --tunnel-addr 127.0.0.1 \
  --tunnel-port 1080 \
  --target-addr target.example.com \
  --target-port 8080 \
  --log debug

# Force single-end forwarding mode
nodepass client \
  --tunnel-addr 127.0.0.1 \
  --tunnel-port 1080 \
  --target-addr target.example.com \
  --target-port 8080 \
  --mode 1 \
  --log debug

# Force dual-end handshake mode
nodepass client \
  --tunnel-addr server.example.com \
  --tunnel-port 10101 \
  --target-addr 127.0.0.1 \
  --target-port 8080 \
  --mode 2

# Connect with custom connection pool capacity
nodepass client \
  --tunnel-addr server.example.com \
  --tunnel-port 10101 \
  --target-addr 192.168.1.100 \
  --target-port 8080 \
  --log debug \
  --min 128

# Multiple targets with load balancing
nodepass client \
  --tunnel-addr 127.0.0.1 \
  --tunnel-port 1080 \
  --targets "10.1.0.1:8080,10.1.0.2:8080" \
  --lbs 2 \
  --mode 1

# Custom SNI hostname for TLS
nodepass client \
  --tunnel-addr server.example.com \
  --tunnel-port 10101 \
  --target-addr 127.0.0.1 \
  --target-port 8080 \
  --mode 2 \
  --sni backend.internal.example.com

# Resource-constrained configuration
nodepass client \
  --tunnel-addr server.example.com \
  --tunnel-port 10101 \
  --target-addr 127.0.0.1 \
  --target-port 8080 \
  --mode 2 \
  --min 16 \
  --log info

# Block SOCKS and HTTP protocols
nodepass client \
  --tunnel-addr 127.0.0.1 \
  --tunnel-port 1080 \
  --target-addr target.example.com \
  --target-port 8080 \
  --block 12 \
  --mode 1

# Custom DNS cache TTL
nodepass client \
  --tunnel-addr server.example.com \
  --tunnel-port 10101 \
  --target-addr database.local \
  --target-port 3306 \
  --dns 30s \
  --mode 2
```

### Master Mode (API)

Master mode runs a RESTful API server for centralized management of NodePass instances.

```bash
nodepass "master://<api_addr>[<prefix>]?log=<level>&tls=<mode>&crt=<cert_file>&key=<key_file>"
```

#### Parameters

- `api_addr`: Address where the API service will listen (e.g., 0.0.0.0:9090)
- `prefix`: Optional API prefix path (e.g., /management). Default is `/api`
- `log`: Log level (debug, info, warn, error, event)
- `tls`: TLS encryption mode for the API service (0, 1, 2)
  - `0`: No TLS encryption (HTTP)
  - `1`: Self-signed certificate (HTTPS with auto-generated cert)
  - `2`: Custom certificate (HTTPS with provided cert)
- `crt`: Path to certificate file (required when `tls=2`)
- `key`: Path to private key file (required when `tls=2`)

#### How Master Mode Works

In master mode, NodePass:
1. Runs a RESTful API server that allows dynamic management of NodePass instances
2. Provides endpoints for creating, starting, stopping, and monitoring client and server instances
3. Includes Swagger UI for easy API exploration at `{prefix}/v1/docs`
4. Automatically inherits TLS and logging settings for instances created through the API

#### API Endpoints

All endpoints are relative to the configured prefix (default: `/api`):

**Protected Endpoints (Require API Key):**
- `GET {prefix}/v1/instances` - List all instances
- `POST {prefix}/v1/instances` - Create a new instance with JSON body: `{"url": "server://0.0.0.0:10101/0.0.0.0:8080"}`
- `GET {prefix}/v1/instances/{id}` - Get instance details
- `PATCH {prefix}/v1/instances/{id}` - Update instance with JSON body: `{"action": "start|stop|restart"}`
- `DELETE {prefix}/v1/instances/{id}` - Delete instance
- `GET {prefix}/v1/events` - Server-Sent Events stream (SSE)
- `GET {prefix}/v1/info` - Get system information

**Public Endpoints (No API Key Required):**
- `GET {prefix}/v1/openapi.json` - OpenAPI specification
- `GET {prefix}/v1/docs` - Swagger UI documentation

#### Examples

**URL-Based Syntax:**

```bash
# Start master with HTTP using default API prefix (/api)
nodepass "master://0.0.0.0:9090?log=info"

# Start master with custom API prefix (/management)
nodepass "master://0.0.0.0:9090/management?log=info"

# Start master with HTTPS (self-signed certificate)
nodepass "master://0.0.0.0:9090/admin?log=info&tls=1"

# Start master with HTTPS (custom certificate)
nodepass "master://0.0.0.0:9090?log=info&tls=2&crt=/path/to/cert.pem&key=/path/to/key.pem"
```

**Flag-Based Syntax:**

```bash
# Start master with HTTP using default API prefix
nodepass master \
  --tunnel-addr 0.0.0.0 \
  --tunnel-port 9090 \
  --log info

# Start master with HTTPS (self-signed certificate)
nodepass master \
  --tunnel-addr 0.0.0.0 \
  --tunnel-port 9090 \
  --log info \
  --tls 1

# Start master with HTTPS (custom certificate)
nodepass master \
  --tunnel-addr 0.0.0.0 \
  --tunnel-port 9090 \
  --log info \
  --tls 2 \
  --crt /path/to/cert.pem \
  --key /path/to/key.pem
```

**Note:** For custom API prefix with flag-based syntax, include the prefix in the path portion of the configuration. See the [CLI Reference](/docs/cli.md) for details.

## Managing NodePass Instances

### Creating and Managing via API

NodePass master mode provides RESTful API for instance management, and all API requests require authentication using an API Key.

#### API Key Retrieval

When starting master mode, the system automatically generates an API Key and displays it in the logs:

```bash
# Start master mode
nodepass "master://0.0.0.0:9090?log=info"

# The log output will show:
# INFO: API Key created: abc123def456...
```

#### API Request Examples

All protected API endpoints require the `X-API-Key` header:

```bash
# Get API Key (assume: abc123def456789)

# Create instance via API (using default prefix)
curl -X POST http://localhost:9090/api/v1/instances \
  -H "X-API-Key: abc123def456789" \
  -d '{"url":"server://0.0.0.0:10101/0.0.0.0:8080?tls=1"}'

# Using custom prefix
curl -X POST http://localhost:9090/admin/v1/instances \
  -H "X-API-Key: abc123def456789" \
  -d '{"url":"server://0.0.0.0:10101/0.0.0.0:8080?tls=1"}'

# List all running instances
curl http://localhost:9090/api/v1/instances \
  -H "X-API-Key: abc123def456789"

# Control an instance (replace {id} with actual instance ID)
curl -X PATCH http://localhost:9090/api/v1/instances/{id} \
  -H "X-API-Key: abc123def456789" \
  -d '{"action":"restart"}'
```

#### Public Endpoints

The following endpoints do not require API Key authentication:
- `GET {prefix}/v1/openapi.json` - OpenAPI specification
- `GET {prefix}/v1/docs` - Swagger UI documentation

## Bidirectional Data Flow Explanation

NodePass supports flexible bidirectional data flow configuration:

### Client Single-End Forwarding Mode
- **Client**: Listens on local tunnel address, uses connection pooling technology to directly forward to target address
- **Connection Pool Optimization**: Pre-establishes TCP connections, eliminates connection latency, provides high-performance forwarding
- **No Server Required**: Operates independently without server handshake
- **Use Case**: Local proxy, simple port forwarding, testing environments, high-performance forwarding

### Server Receives Mode
- **Server**: Listens for incoming connections on target_addr, forwards through tunnel to client
- **Client**: Connects to local target_addr to provide services
- **Use Case**: Expose internal services to external access

### Server Sends Mode
- **Server**: Connects to remote target_addr to fetch data, sends through tunnel to client
- **Client**: Listens locally to receive connections from server
- **Use Case**: Access remote services through tunnel proxy

The system automatically selects the appropriate operation mode based on tunnel and target addresses:
- If the client's tunnel address is a local address, enables single-end forwarding mode
- If target address is a local address, uses Server Receives Mode
- If target address is a remote address, uses Server Sends Mode

## Tunnel Key

NodePass uses tunnel keys to authenticate connections between clients and servers. The key can be specified in two ways:

### Key Derivation Rules

1. **Explicit Key**: Specify the username part in the URL as the key
   ```bash
   # Use "mypassword" as the tunnel key
   nodepass server://mypassword@10.1.0.1:10101/10.1.0.1:8080
   nodepass client://mypassword@10.1.0.1:10101/127.0.0.1:8080
   ```

2. **Port-Derived Key**: If no username is specified, the system uses the hexadecimal value of the port number as the key
   ```bash
   # Port 10101's hexadecimal value "2775" will be used as the tunnel key
   nodepass server://10.1.0.1:10101/10.1.0.1:8080
   nodepass client://10.1.0.1:10101/127.0.0.1:8080
   ```

### Handshake Process

The handshake process between client and server is as follows:

1. **Client Connection**: Client connects to the server's tunnel address
2. **Key Authentication**: Client sends encrypted tunnel key
3. **Server Verification**: Server decrypts and verifies if the key matches
4. **Configuration Sync**: Upon successful verification, server sends tunnel configuration including:
   - Data flow direction
   - Maximum connection pool capacity
   - TLS security mode
5. **Connection Established**: Handshake complete, data transmission begins

This design ensures that only clients with the correct key can establish tunnel connections, while allowing the server to centrally manage connection pool capacity.

## Choosing Between Syntax Formats

Both URL-based and flag-based syntaxes provide identical functionality. Choose based on your use case:

### Use URL-Based Syntax When:

- **Automation & Scripting**: Compact single-string format is easier to manage in scripts
- **Configuration Files**: Store complete configurations in config files or environment variables
- **Container Deployments**: Pass configuration via Docker/Kubernetes environment variables
- **Quick Testing**: Minimal typing for rapid prototyping and testing
- **Documentation**: Copy-paste friendly for sharing configurations

**Example:**
```bash
export NODEPASS_SERVER="server://0.0.0.0:10101/0.0.0.0:8080?tls=1&log=info"
nodepass "$NODEPASS_SERVER"
```

### Use Flag-Based Syntax When:

- **Manual Operation**: More readable when typing commands interactively
- **Learning**: Explicit parameter names make configurations self-documenting
- **Incremental Building**: Build commands step-by-step with shell completion
- **Variable Interpolation**: Parameter values come from shell variables
- **Team Environments**: Clearer for teams less familiar with URL encoding

**Example:**
```bash
TUNNEL_PORT=10101
TARGET_PORT=8080
nodepass server \
  --tunnel-addr 0.0.0.0 \
  --tunnel-port $TUNNEL_PORT \
  --target-addr 0.0.0.0 \
  --target-port $TARGET_PORT \
  --tls 1 \
  --log info
```

### Mixing Syntaxes

You cannot mix URL query parameters with command-line flags in the same invocation. Choose one syntax format and use it consistently for each command.

**Valid:**
```bash
# URL-based only
nodepass "server://0.0.0.0:10101/0.0.0.0:8080?tls=1"

# Flag-based only
nodepass server --tunnel-addr 0.0.0.0 --tunnel-port 10101 --target-addr 0.0.0.0 --target-port 8080 --tls 1
```

**Invalid:**
```bash
# Do NOT mix syntaxes
nodepass "server://0.0.0.0:10101/0.0.0.0:8080" --tls 1  # Wrong!
```

## Next Steps

- Learn about [configuration options](/docs/configuration.md) to fine-tune NodePass
- Explore [examples](/docs/examples.md) of common deployment scenarios
- Understand [how NodePass works](/docs/how-it-works.md) under the hood
- Check the [troubleshooting guide](/docs/troubleshooting.md) if you encounter issues