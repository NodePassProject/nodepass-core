# CLI Reference

NodePass provides two syntax formats for configuration: URL-based syntax and traditional flag-based command-line syntax. Both formats offer the same functionality, allowing you to choose the style that best fits your workflow.

## Syntax Overview

### URL-Based Syntax

The URL-based syntax provides a compact and expressive way to configure NodePass:

```bash
nodepass "<mode>://<tunnel_addr>/<target_addr>?param1=value1&param2=value2"
```

This is the recommended syntax for most use cases, especially for scripting and automation.

### Flag-Based Syntax

The traditional command-line flag syntax provides explicit parameter names:

```bash
nodepass <mode> --tunnel-addr <addr> --tunnel-port <port> --target-addr <addr> --target-port <port> [options]
```

This syntax is ideal when you prefer explicit parameter names or are more familiar with traditional Unix-style command-line tools.

## Global Commands

These commands work regardless of syntax format:

```bash
nodepass help              # Display help information
nodepass --help            # Display help information
nodepass -h                # Display help information

nodepass version           # Show version and platform information
nodepass --version         # Show version and platform information
nodepass -v                # Show version and platform information
```

## Server Mode

### URL-Based Syntax

```bash
nodepass "server://[password@]<tunnel_addr>/<target_addr>?[options]"
```

### Flag-Based Syntax

```bash
nodepass server [flags]
```

### Available Flags

#### Connection Parameters

- `--password <string>`
  - Connection password for tunnel authentication
  - Alternative to specifying password in URL (password@host)
  - Example: `--password mySecretKey123`

- `--tunnel-addr <ip>`
  - IP address for the tunnel endpoint (control channel)
  - Default: listens on all interfaces if not specified
  - Example: `--tunnel-addr 0.0.0.0` or `--tunnel-addr 10.1.0.1`

- `--tunnel-port <port>`
  - Port number for the tunnel endpoint
  - Must be specified (no default)
  - Example: `--tunnel-port 10101`

- `--target-addr <ip>`
  - IP address or hostname of the target service
  - For reverse mode: local binding address
  - For forward mode: remote target address
  - Example: `--target-addr 127.0.0.1` or `--target-addr backend.example.com`

- `--target-port <port>`
  - Port number of the target service
  - Must be specified (no default)
  - Example: `--target-port 8080`

- `--targets <target_list>`
  - Multiple target addresses for load balancing or failover
  - Comma-separated list of addr:port pairs
  - Example: `--targets 10.1.0.1:8080,10.1.0.2:8080,10.1.0.3:8080`

#### TLS Configuration

- `--tls <mode>`
  - TLS encryption mode for data channels: `0`, `1`, or `2`
  - `0`: No TLS encryption (plain TCP/UDP)
  - `1`: Self-signed certificate (automatically generated)
  - `2`: Custom certificate (requires --crt and --key)
  - Default: `0`
  - Example: `--tls 1`

- `--crt <path>`
  - Path to TLS certificate file (PEM format)
  - Required when `--tls 2`
  - Example: `--crt /etc/nodepass/cert.pem`

- `--key <path>`
  - Path to TLS private key file (PEM format)
  - Required when `--tls 2`
  - Example: `--key /etc/nodepass/key.pem`

#### Connection Pool Configuration

- `--type <mode>`
  - Connection pool type: `0`, `1`, `2`, or `3`
  - `0`: TCP-based connection pool (default)
  - `1`: QUIC-based UDP connection pool with multiplexing (requires TLS)
  - `2`: WebSocket/WSS-based connection pool
  - `3`: HTTP/2-based connection pool with multiplexed streams (requires TLS)
  - Default: `0`
  - Example: `--type 1`

- `--min <number>`
  - Minimum connection pool capacity
  - Number of persistent connections to maintain
  - Only applies in local proxy mode (mode 1)
  - Default: `64`
  - Example: `--min 128`

- `--max <number>`
  - Maximum connection pool capacity
  - Controls the upper limit of pre-established connections
  - Delivered to client during handshake
  - Default: `1024`
  - Example: `--max 2048`

#### Load Balancing

- `--lbs <strategy>`
  - Load balancing strategy for multiple targets
  - `random`: Random selection among targets
  - `roundrobin`: Round-robin distribution
  - `leastconn`: Least connections first
  - Default: `random`
  - Example: `--lbs roundrobin`

#### Operational Mode

- `--mode <mode>`
  - Run mode control for data flow direction: `0`, `1`, or `2`
  - `0`: Automatic detection (attempts local binding first)
  - `1`: Force reverse mode (server receives traffic)
  - `2`: Force forward mode (server connects to target)
  - Default: `0`
  - Example: `--mode 2`

#### Network Configuration

- `--dial <ip>`
  - Source IP address for outbound connections to target
  - Useful for multi-homed systems or policy routing
  - Supports both IPv4 and IPv6
  - Default: `auto` (system-selected)
  - Example: `--dial 192.168.1.100`

- `--read <duration>`
  - Data read timeout duration
  - Format: time units like `30s`, `5m`, `1h`
  - Default: `0` (no timeout)
  - Example: `--read 60s`

#### Traffic Control

- `--rate <mbps>`
  - Bandwidth rate limit in Mbps
  - Applied per connection
  - Default: `0` (unlimited)
  - Example: `--rate 100`

- `--slot <limit>`
  - Maximum concurrent connection limit
  - Controls how many simultaneous connections are allowed
  - Default: `65536`
  - Set to `0` for unlimited
  - Example: `--slot 10000`

#### Protocol Support

- `--proxy <mode>`
  - PROXY protocol v1 header support
  - `0`: Disabled (default)
  - `1`: Enabled - sends client IP information to backend
  - Example: `--proxy 1`

- `--block <protocols>`
  - Block specific protocols
  - Comma-separated list of protocols to disable
  - Valid values: `tcp`, `udp`, `http`, `https`, `socks4`, `socks5`
  - Example: `--block http,https`

- `--notcp <0|1>`
  - TCP protocol support control
  - `0`: Enabled (default)
  - `1`: Disabled - only UDP allowed
  - Example: `--notcp 0`

- `--noudp <0|1>`
  - UDP protocol support control
  - `0`: Enabled (default)
  - `1`: Disabled - only TCP allowed
  - Example: `--noudp 0`

#### Logging and DNS

- `--log <level>`
  - Log verbosity level
  - Valid values: `none`, `debug`, `info`, `warn`, `error`, `event`
  - Default: `info`
  - Example: `--log debug`

- `--dns <duration>`
  - DNS cache TTL duration
  - Format: time units like `1h`, `30m`, `15s`
  - Default: `5m`
  - Set to `0` to disable caching
  - Example: `--dns 10m`

### Server Examples

#### URL-Based Examples

```bash
# Basic server with automatic TLS
nodepass "server://0.0.0.0:10101/127.0.0.1:8080?tls=1&log=debug"

# Server with custom certificate and forced forward mode
nodepass "server://0.0.0.0:10101/backend.example.com:8080?tls=2&crt=/etc/ssl/cert.pem&key=/etc/ssl/key.pem&mode=2"

# QUIC-based server with multiple targets
nodepass "server://0.0.0.0:10101/10.1.0.1:8080,10.1.0.2:8080?type=1&lbs=roundrobin"

# HTTP/2 server with rate limiting
nodepass "server://0.0.0.0:10101/127.0.0.1:8080?type=3&tls=1&rate=100&slot=1000"
```

#### Flag-Based Examples

```bash
# Basic server with automatic TLS
nodepass server \
  --tunnel-addr 0.0.0.0 \
  --tunnel-port 10101 \
  --target-addr 127.0.0.1 \
  --target-port 8080 \
  --tls 1 \
  --log debug

# Server with custom certificate and forced forward mode
nodepass server \
  --tunnel-addr 0.0.0.0 \
  --tunnel-port 10101 \
  --target-addr backend.example.com \
  --target-port 8080 \
  --tls 2 \
  --crt /etc/ssl/cert.pem \
  --key /etc/ssl/key.pem \
  --mode 2

# QUIC-based server with multiple targets
nodepass server \
  --tunnel-addr 0.0.0.0 \
  --tunnel-port 10101 \
  --targets "10.1.0.1:8080,10.1.0.2:8080" \
  --type 1 \
  --lbs roundrobin

# HTTP/2 server with rate limiting
nodepass server \
  --tunnel-addr 0.0.0.0 \
  --tunnel-port 10101 \
  --target-addr 127.0.0.1 \
  --target-port 8080 \
  --type 3 \
  --tls 1 \
  --rate 100 \
  --slot 1000
```

## Client Mode

### URL-Based Syntax

```bash
nodepass "client://[password@]<tunnel_addr>/<target_addr>?[options]"
```

### Flag-Based Syntax

```bash
nodepass client [flags]
```

### Available Flags

#### Connection Parameters

- `--password <string>`
  - Connection password for tunnel authentication
  - Must match the server's password
  - Example: `--password mySecretKey123`

- `--tunnel-addr <ip_or_hostname>`
  - Address of the NodePass server's tunnel endpoint
  - Can be IP address or hostname
  - For local proxy mode: local binding address
  - For handshake mode: remote server address
  - Example: `--tunnel-addr 127.0.0.1` or `--tunnel-addr server.example.com`

- `--tunnel-port <port>`
  - Port number for the tunnel endpoint
  - Must be specified (no default)
  - Example: `--tunnel-port 10101`

- `--target-addr <ip_or_hostname>`
  - Target service address
  - For local proxy mode: remote target address
  - For handshake mode: local service address
  - Example: `--target-addr 127.0.0.1` or `--target-addr database.local`

- `--target-port <port>`
  - Port number of the target service
  - Must be specified (no default)
  - Example: `--target-port 8080`

- `--targets <target_list>`
  - Multiple target addresses for load balancing or failover
  - Comma-separated list of addr:port pairs
  - Example: `--targets 127.0.0.1:8080,127.0.0.1:8081`

#### DNS and TLS Configuration

- `--dns <duration>`
  - DNS cache TTL duration
  - Format: time units like `1h`, `30m`, `15s`
  - Default: `5m`
  - Set to `0` to disable caching
  - Example: `--dns 10m`

- `--sni <hostname>`
  - SNI (Server Name Indication) hostname for TLS
  - Overrides the hostname used in TLS handshake
  - Useful for accessing servers behind SNI-based routing
  - Example: `--sni backend.example.com`

#### Connection Pool Configuration

- `--min <number>`
  - Minimum connection pool capacity
  - Number of persistent connections to maintain
  - Only applies in local proxy mode (mode 1)
  - Default: `64`
  - Example: `--min 128`

#### Load Balancing

- `--lbs <strategy>`
  - Load balancing strategy for multiple targets
  - `random`: Random selection among targets
  - `roundrobin`: Round-robin distribution
  - `leastconn`: Least connections first
  - Default: `random`
  - Example: `--lbs leastconn`

#### Operational Mode

- `--mode <mode>`
  - Run mode control for client behavior: `0`, `1`, or `2`
  - `0`: Automatic detection (attempts local binding first)
  - `1`: Force single-end forwarding mode (local proxy with pooling)
  - `2`: Force dual-end handshake mode (requires server coordination)
  - Default: `0`
  - Example: `--mode 1`

#### Network Configuration

- `--dial <ip>`
  - Source IP address for outbound connections
  - Useful for multi-homed systems or policy routing
  - Supports both IPv4 and IPv6
  - Default: `auto` (system-selected)
  - Example: `--dial 192.168.1.50`

- `--read <duration>`
  - Data read timeout duration
  - Format: time units like `30s`, `5m`, `1h`
  - Default: `0` (no timeout)
  - Example: `--read 45s`

#### Traffic Control

- `--rate <mbps>`
  - Bandwidth rate limit in Mbps
  - Applied per connection
  - Default: `0` (unlimited)
  - Example: `--rate 50`

- `--slot <limit>`
  - Maximum concurrent connection limit
  - Controls how many simultaneous connections are allowed
  - Default: `65536`
  - Set to `0` for unlimited
  - Example: `--slot 5000`

#### Protocol Support

- `--proxy <mode>`
  - PROXY protocol v1 header support
  - `0`: Disabled (default)
  - `1`: Enabled - sends client IP information to backend
  - Example: `--proxy 1`

- `--block <protocols>`
  - Block specific protocols
  - Comma-separated list of protocols to disable
  - Valid values: `tcp`, `udp`, `http`, `https`, `socks4`, `socks5`
  - Example: `--block socks4,socks5`

- `--notcp <0|1>`
  - TCP protocol support control
  - `0`: Enabled (default)
  - `1`: Disabled - only UDP allowed
  - Example: `--notcp 0`

- `--noudp <0|1>`
  - UDP protocol support control
  - `0`: Enabled (default)
  - `1`: Disabled - only TCP allowed
  - Example: `--noudp 0`

#### Logging

- `--log <level>`
  - Log verbosity level
  - Valid values: `none`, `debug`, `info`, `warn`, `error`, `event`
  - Default: `info`
  - Example: `--log debug`

### Client Examples

#### URL-Based Examples

```bash
# Local proxy with high-performance connection pooling
nodepass "client://127.0.0.1:1080/target.example.com:8080?mode=1&min=256"

# Connect to remote server with custom DNS cache
nodepass "client://server.example.com:10101/127.0.0.1:8080?mode=2&dns=30s"

# Multiple targets with load balancing
nodepass "client://127.0.0.1:1080/10.1.0.1:8080,10.1.0.2:8080?lbs=leastconn&log=debug"

# Real-time application with timeout
nodepass "client://server.example.com:10101/127.0.0.1:7777?mode=2&read=30s&rate=100"
```

#### Flag-Based Examples

```bash
# Local proxy with high-performance connection pooling
nodepass client \
  --tunnel-addr 127.0.0.1 \
  --tunnel-port 1080 \
  --target-addr target.example.com \
  --target-port 8080 \
  --mode 1 \
  --min 256

# Connect to remote server with custom DNS cache
nodepass client \
  --tunnel-addr server.example.com \
  --tunnel-port 10101 \
  --target-addr 127.0.0.1 \
  --target-port 8080 \
  --mode 2 \
  --dns 30s

# Multiple targets with load balancing
nodepass client \
  --tunnel-addr 127.0.0.1 \
  --tunnel-port 1080 \
  --targets "10.1.0.1:8080,10.1.0.2:8080" \
  --lbs leastconn \
  --log debug

# Real-time application with timeout
nodepass client \
  --tunnel-addr server.example.com \
  --tunnel-port 10101 \
  --target-addr 127.0.0.1 \
  --target-port 7777 \
  --mode 2 \
  --read 30s \
  --rate 100
```

## Master Mode

### URL-Based Syntax

```bash
nodepass "master://<api_addr>[/<prefix>]?[options]"
```

### Flag-Based Syntax

```bash
nodepass master [flags]
```

### Available Flags

#### Connection Parameters

- `--tunnel-addr <ip>`
  - IP address for the API service to listen on
  - Default: listens on all interfaces if not specified
  - Example: `--tunnel-addr 0.0.0.0` or `--tunnel-addr 127.0.0.1`

- `--tunnel-port <port>`
  - Port number for the API service
  - Must be specified (no default)
  - Example: `--tunnel-port 9090`

#### TLS Configuration

- `--tls <mode>`
  - TLS encryption mode for the API service: `0`, `1`, or `2`
  - `0`: No TLS (HTTP)
  - `1`: Self-signed certificate (HTTPS with auto-generated cert)
  - `2`: Custom certificate (HTTPS with provided cert)
  - Default: `0`
  - Example: `--tls 1`

- `--crt <path>`
  - Path to TLS certificate file (PEM format)
  - Required when `--tls 2`
  - Example: `--crt /etc/nodepass/cert.pem`

- `--key <path>`
  - Path to TLS private key file (PEM format)
  - Required when `--tls 2`
  - Example: `--key /etc/nodepass/key.pem`

#### Logging

- `--log <level>`
  - Log verbosity level
  - Valid values: `none`, `debug`, `info`, `warn`, `error`, `event`
  - Default: `info`
  - Example: `--log info`

### Master Examples

#### URL-Based Examples

```bash
# Basic HTTP API server
nodepass "master://0.0.0.0:9090?log=info"

# API server with custom prefix
nodepass "master://0.0.0.0:9090/admin?log=info"

# HTTPS API server with self-signed certificate
nodepass "master://0.0.0.0:9090?tls=1&log=debug"

# HTTPS API server with custom certificate
nodepass "master://0.0.0.0:9090/management?tls=2&crt=/etc/ssl/cert.pem&key=/etc/ssl/key.pem"
```

#### Flag-Based Examples

```bash
# Basic HTTP API server
nodepass master \
  --tunnel-addr 0.0.0.0 \
  --tunnel-port 9090 \
  --log info

# HTTPS API server with self-signed certificate
nodepass master \
  --tunnel-addr 0.0.0.0 \
  --tunnel-port 9090 \
  --tls 1 \
  --log debug

# HTTPS API server with custom certificate
nodepass master \
  --tunnel-addr 0.0.0.0 \
  --tunnel-port 9090 \
  --tls 2 \
  --crt /etc/ssl/cert.pem \
  --key /etc/ssl/key.pem
```

## Parameter Mapping

When using flag-based syntax, the parameters are automatically converted to URL format internally. Here's how the mapping works:

| Flag Parameter | URL Query Parameter | Description |
|---------------|-------------------|-------------|
| `--password` | Username part of URL | `password@host` or `password` parameter |
| `--tunnel-addr` | Host part of URL | IP or hostname in `host:port` |
| `--tunnel-port` | Host part of URL | Port in `host:port` |
| `--target-addr` | Path part of URL | IP or hostname in `/addr:port` |
| `--target-port` | Path part of URL | Port in `/addr:port` |
| `--targets` | Path part of URL | `/target1:port1,target2:port2` |
| `--log` | `?log=` | Log level query parameter |
| `--dns` | `?dns=` | DNS cache TTL query parameter |
| `--sni` | `?sni=` | SNI hostname query parameter |
| `--lbs` | `?lbs=` | Load balancing strategy parameter |
| `--min` | `?min=` | Minimum pool size query parameter |
| `--max` | `?max=` | Maximum pool size query parameter |
| `--mode` | `?mode=` | Run mode query parameter |
| `--type` | `?type=` | Pool type query parameter |
| `--tls` | `?tls=` | TLS mode query parameter |
| `--crt` | `?crt=` | Certificate file query parameter |
| `--key` | `?key=` | Key file query parameter |
| `--dial` | `?dial=` | Source IP query parameter |
| `--read` | `?read=` | Read timeout query parameter |
| `--rate` | `?rate=` | Bandwidth rate query parameter |
| `--slot` | `?slot=` | Connection slot query parameter |
| `--proxy` | `?proxy=` | PROXY protocol query parameter |
| `--block` | `?block=` | Block protocols query parameter |
| `--notcp` | `?notcp=` | TCP disable query parameter |
| `--noudp` | `?noudp=` | UDP disable query parameter |

## Best Practices

### When to Use URL-Based Syntax

- **Automation and Scripting**: URL format is compact and easy to pass as a single argument
- **Configuration Files**: Storing complete configurations in config files or environment variables
- **Container Deployments**: Passing configuration via Docker/Kubernetes environment variables
- **Quick Testing**: Rapid prototyping and testing with minimal typing

### When to Use Flag-Based Syntax

- **Manual Operation**: More readable when typing commands interactively
- **Incremental Configuration**: Building commands step-by-step with shell completion
- **Documentation**: Easier to understand and document for new users
- **Shell Scripts**: When parameter values come from shell variables

### Combining Both Syntaxes

You can actually mix both syntaxes if the URL is provided as the first positional argument:

```bash
# This is valid: URL as first argument
nodepass "server://0.0.0.0:10101/127.0.0.1:8080"

# This is valid: command with flags
nodepass server --tunnel-addr 0.0.0.0 --tunnel-port 10101 --target-addr 127.0.0.1 --target-port 8080
```

However, avoid mixing URL query parameters with command-line flags in the same invocation, as this may lead to unexpected behavior.

## Next Steps

- Learn about [usage patterns and operating modes](/docs/usage.md)
- Explore [configuration options in detail](/docs/configuration.md)
- See [practical examples](/docs/examples.md) for common scenarios
- Check the [troubleshooting guide](/docs/troubleshooting.md) if you encounter issues