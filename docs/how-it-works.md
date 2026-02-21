# How NodePass Works

This page describes the internal architecture and data flow of NodePass as implemented in the source code, with architecture graphs illustrating how the components interact.

---

## Table of Contents

- [Core Architecture](#core-architecture)
- [Startup and Handshake](#startup-and-handshake)
- [Operating Modes](#operating-modes)
  - [Single-End Forwarding (Mode 1)](#single-end-forwarding-mode-1)
  - [Dual-End Pooled (Mode 2)](#dual-end-pooled-mode-2)
  - [TLS Termination](#tls-termination)
- [Connection Pool](#connection-pool)
  - [Pool Types](#pool-types)
  - [Pool Lifecycle](#pool-lifecycle)
  - [Auto-Scaling](#auto-scaling)
- [Signal Protocol](#signal-protocol)
- [TLS Modes](#tls-modes)
- [UDP Handling](#udp-handling)
- [Health and Load Management](#health-and-load-management)
- [Master Mode](#master-mode)

---

## Core Architecture

NodePass splits all traffic into two independent channels. The control channel carries only coordination signals; the data channel carries application bytes. This separation means the data path can be fully encrypted and optimised independently of the control path.

```
  ┌─────────────────────────────────────────────────────────────────────┐
  │                        NodePass Architecture                        │
  │                                                                     │
  │   ┌──────────┐   Control Channel (TCP + XOR/Base64 JSON signals)    │
  │   │          │ ─────────────────────────────────────────────────►   │
  │   │  Server  │                                                      │
  │   │          │ ◄─────────────────────────────────────────────────   │
  │   └────┬─────┘   Data Channel (Pool — TCP / QUIC / WS / H2 + TLS)   │
  │        │                                         ▲                  │
  │        │ listens / dials                         │ dials / listens  │
  │        ▼                                         │                  │
  │   ┌──────────┐                            ┌──────────┐              │
  │   │  Target  │                            │  Target  │              │
  │   │ (remote) │                            │ (local)  │              │
  │   └──────────┘                            └──────────┘              │
  │    Server Side                             Client Side              │
  └─────────────────────────────────────────────────────────────────────┘
```

Three top-level modes share this foundation:

```
  nodepass <url>
       │
       ├── server://   Accepts clients. Manages pool. Signals when traffic arrives.
       │
       ├── client://   Connects to server or listens locally.
       │               Dials targets on signal.
       │
       └── master://   REST API. Spawns and supervises server/client instances.
```

### Channel Responsibilities

```
  ┌──────────────────────────────────────────────────────────────────┐
  │  Control Channel                  Data Channel                   │
  │  ─────────────                    ────────────                   │
  │  • Authentication (HMAC token)    • Application bytes (TCP/UDP)  │
  │  • Config delivery (handshake)    • Configurable TLS (0 / 1 / 2) │
  │  • Pool connection IDs            • One connection per signal    │
  │  • Health ping / pong             • Managed by pool library      │
  │  • Pool flush requests            • Separate from control path   │
  │  • TLS fingerprint exchange                                      │
  │                                                                  │
  │  Encoding: JSON → XOR(key) → Base64 → \n                         │
  │  Transport: plain TCP (lightweight by design — no app data here) │
  └──────────────────────────────────────────────────────────────────┘
```

---

## Startup and Handshake

Before any pool connections are made, a temporary HTTPS server negotiates the tunnel configuration. The server is torn down immediately after the single exchange completes.

```
  Client                                          Server
    │                                               │
    │   1. Server binds TunnelListener (TCP)        │
    │      Spins up ephemeral http.Server + TLS     │
    │                                               │
    │──── GET / ─────────────────────────────────►  │
    │     Authorization: Bearer <HMAC token>        │
    │                                               │
    │                    2. Verify HMAC token       │
    │                    3. Resolve DataFlow        │
    │                    4. Prepare config payload  │
    │                                               │
    │◄─── 200 OK ─────────────────────────────────  │
    │     { flow, max, tls, type }                  │
    │                                               │
    │   5. Client stores config                     │
    │   6. Server closes ephemeral http.Server      │
    │   7. Server re-binds TunnelListener (fresh)   │
    │                                               │
    │──── Pool connections established ───────────► │
    │     (TCP / QUIC / WebSocket / HTTP2)          │
    │                                               │
    │   (tls=1 only)                                │
    │◄─── verify signal: server fingerprint ──────  │
    │──── verify signal: client fingerprint ──────► │
    │   Fingerprints compared — mismatch = abort    │
    │                                               │
    │──── Control channel active ─────────────────► │
```

**HMAC token derivation:**

```
  TunnelKey = URL password  (or hex(FNV32a(port)) if no password given)

  AuthToken = hex( HMAC-SHA256( key=TunnelKey, data="" ) )
```

The same key is used for XOR-encoding all subsequent control signals, providing a consistent identity throughout the session.

---

## Operating Modes

### Mode Detection

```
  Client.Start()
       │
       ├── RunMode == "1" ──► InitTunnelListener → SingleStart
       │
       ├── RunMode == "2" ──► CommonStart (connects to server)
       │
       └── RunMode == "0" (auto)
               │
               ├── InitTunnelListener OK? ──► RunMode = "1" → SingleStart
               │
               └── Fails (port taken / remote addr) ──► RunMode = "2" → CommonStart
```

---

### Single-End Forwarding (Mode 1)

The client operates entirely independently — no server, no control channel, no pool negotiation.

```
  ┌─────────────────────────────────────────────────────────────────┐
  │                  Client Single-End Forwarding                   │
  │                                                                 │
  │   External Caller                     Target Service            │
  │        │                                    ▲                   │
  │        │  TCP connect                       │ TCP connect       │
  │        ▼                                    │                   │
  │   ┌─────────────────────────────────────────┤                   │
  │   │  TunnelListener (:port)                 │                   │
  │   │       │                                 │                   │
  │   │       │  Accept()                       │                   │
  │   │       ▼                                 │                   │
  │   │  [goroutine per connection]             │                   │
  │   │       │                                 │                   │
  │   │       │  DialWithRotation(target)       │                   │
  │   │       ├────────────────────────────────►│                   │
  │   │       │                                 │                   │
  │   │       │  conn.DataExchange(src, dst)    │                   │
  │   │       │◄───────────────────────────────►│                   │
  │   └─────────────────────────────────────────┘                   │
  │                                                                 │
  │   UDP path mirrors this with ReadFromUDP / WriteToUDP +         │
  │   a TargetUDPSession map keyed by client address                │
  └─────────────────────────────────────────────────────────────────┘
```

**Multiple targets with load balancing:**

```
  DialWithRotation
       │
       ├── lbs=0  Round-robin across TargetTCPAddrs[]
       │
       ├── lbs=1  Probe all targets with TCP ping, route to lowest latency
       │
       └── lbs=2  Primary-backup: prefer index 0, fall back on failure
                  Reset to 0 after FallbackInterval
```

---

### Dual-End Pooled (Mode 2)

The server negotiates DataFlow direction during handshake. Two sub-modes result:

#### DataFlow `-` — Server Receives Traffic

```
  ┌──────────────────────────────────────────────────────────────────────┐
  │  DataFlow "-"  (server binds to target address, receives traffic)    │
  │                                                                      │
  │  External        Server                      Client         Local    │
  │  Caller          (:target)                                  Target   │
  │     │                │                          │              │     │
  │     │──TCP connect──►│                          │              │     │
  │     │                │                          │              │     │
  │     │         [TunnelTCPLoop]                   │              │     │
  │     │                │                          │              │     │
  │     │                │──signal{tcp,id,remote}──►│              │     │
  │     │                │                          │              │     │
  │     │                │              [TunnelTCPOnce]            │     │
  │     │                │                          │              │     │
  │     │                │◄──pool conn (id)─────────│              │     │
  │     │                │                          │─TCP connect─►│     │
  │     │                │                          │              │     │
  │     │◄═══════════════╪══════════════════════════╪═════════════►│     │
  │     │     DataExchange (remoteConn ⟷ targetConn)│              │     │
  │     │                │                          │              │     │
  └──────────────────────────────────────────────────────────────────────┘
```

#### DataFlow `+` — Server Sends Traffic

```
  ┌──────────────────────────────────────────────────────────────────────┐
  │  DataFlow "+"  (client binds locally, server dials remote target)    │
  │                                                                      │
  │  Local           Client                       Server        Remote   │
  │  Caller          (:target)                                  Target   │
  │     │                │                          │              │     │
  │     │──TCP connect──►│                          │              │     │
  │     │                │                          │              │     │
  │     │         [TunnelTCPLoop]                   │              │     │
  │     │                │──signal{tcp,id,remote}──►│              │     │
  │     │                │                          │              │     │
  │     │                │              [TunnelTCPOnce]            │     │
  │     │                │◄──pool conn (id)─────────│              │     │
  │     │                │                          │─TCP connect─►│     │
  │     │                │                          │              │     │
  │     │◄═══════════════╪══════════════════════════╪═════════════►│     │
  │     │     DataExchange (remoteConn ⟷ targetConn)│              │     │
  │     │                │                          │              │     │
  └──────────────────────────────────────────────────────────────────────┘
```

**Data flow direction determination:**

```
  Server.Start()
       │
       ├── InitTargetListener OK?
       │       │
       │       ├── Yes ──► DataFlow = "-"  (server receives)
       │       │           RunMode  = "1"
       │       │
       │       └── No  ──► DataFlow = "+"  (server sends)
       │                   RunMode  = "2"
       │
       └── DataFlow delivered to client during handshake
           Client.DataFlow set from server config:
               "+" → client InitTargetListener (listens locally)
               "-" → client dials target on each signal
```

---

### TLS Termination

In single-end forwarding mode, setting `tls=1` or `tls=2` wraps the `TunnelListener` itself with TLS. Callers connect with TLS; the client terminates it and forwards plain TCP to the target.

```
  ┌─────────────────────────────────────────────────────────────────┐
  │                      TLS Termination Flow                       │
  │                                                                 │
  │  HTTPS Client          NodePass Client         HTTP Backend     │
  │       │                      │                      │           │
  │       │   TLS ClientHello    │                      │           │
  │       │─────────────────────►│                      │           │
  │       │                      │                      │           │
  │       │        tls.NewListener wraps raw TCPListener│           │
  │       │        Accept() returns *tls.Conn           │           │
  │       │        TLS handshake completes              │           │
  │       │                      │                      │           │
  │       │   TLS established    │                      │           │
  │       │◄─────────────────────│                      │           │
  │       │                      │                      │           │
  │       │   HTTP GET /         │                      │           │
  │       │─────────────────────►│                      │           │
  │       │   (encrypted)        │ plain TCP connect    │           │
  │       │                      │─────────────────────►│           │
  │       │                      │                      │           │
  │       │                      │   HTTP GET /         │           │
  │       │                      │─────────────────────►│           │
  │       │                      │   (plain)            │           │
  │       │                      │◄─────────────────────│           │
  │       │◄─────────────────────│                      │           │
  │       │   (encrypted)        │                      │           │
  └─────────────────────────────────────────────────────────────────┘

  Applies to any TCP protocol:
  HTTPS → HTTP  │  WSS → WS  │  TLS DB → plain DB  │  any TLS → plain
```

How it is wired in `client.go`:

```
  InitTunnelListener()          raw *net.TCPListener
       │
       └── TLSConfig != nil?
               │
               Yes ──► TunnelListener = tls.NewListener(rawListener, TLSConfig)
               No  ──► TunnelListener = rawListener  (unchanged)

  SingleTCPLoop → Accept() → *tls.Conn or *net.TCPConn
                           → DialWithRotation(target) → plain conn
                           → conn.DataExchange(tlsConn, plainConn)
```

---

## Connection Pool

The pool pre-warms connections between server and client before traffic arrives, moving handshake cost off the critical path.

```
  ┌──────────────────────────────────────────────────────────────────┐
  │                    Pool Architecture                             │
  │                                                                  │
  │   Server Pool Manager                Client Pool Manager         │
  │   ───────────────────                ───────────────────         │
  │                                                                  │
  │   accept() loop                      dial() loop                 │
  │        │                                  │                      │
  │        ▼                                  ▼                      │
  │   ┌──────────┐  ◄── net.Conn ──►  ┌──────────────┐               │
  │   │  connMap │    (pooled conn)   │   connMap    │               │
  │   │ id→conn  │                    │  id→conn     │               │
  │   └──────────┘                    └──────────────┘               │
  │        │                                  │                      │
  │   ┌──────────┐                    ┌──────────────┐               │
  │   │  idChan  │  ── id string ──►  │   idChan     │               │
  │   └──────────┘                    └──────────────┘               │
  │                                                                  │
  │   IncomingGet(id, timeout)         OutgoingGet(id, timeout)      │
  │   reads next id from idChan        looks up id in connMap        │
  │   returns matching conn            returns conn, removes entry   │
  │                                                                  │
  │   One connection, one use. Pool manager continuously refills.    │
  └──────────────────────────────────────────────────────────────────┘
```

### Pool Types

NodePass supports four pool transports. The server selects the type; it is delivered to the client during handshake — the client does not need to specify it.

#### Type 0 — TCP Pool (default)

```
  Server                              Client
    │                                   │
    │   Listen on TunnelTCPAddr         │
    │◄──────────────────────────────────│  dial TunnelTCPAddr
    │                                   │
    │   per connection:                 │
    │   • optional TLS wrap (tls code)  │
    │   • generate 8-char hex ID        │
    │   • store in connMap              │
    │   • push ID to idChan             │
    │                                   │
    │   Multiple independent TCP connections fill the pool
    │   Each carries one data exchange then closes
```

#### Type 1 — QUIC Pool

```
  Server                              Client
    │                                   │
    │   Listen on TunnelUDPAddr (QUIC)  │
    │◄──────────────────────────────────│  single QUIC connection
    │                                   │
    │   Single QUIC connection carries all pool streams:
    │                                   │
    │   ┌──────────────────────────┐    │
    │   │  QUIC Connection         │    │
    │   │  ┌──────┐ ┌──────┐       │    │
    │   │  │stream│ │stream│ ...   │    │
    │   │  │  #1  │ │  #2  │       │    │
    │   │  └──────┘ └──────┘       │    │
    │   └──────────────────────────┘    │
    │                                   │
    │   Mandatory TLS 1.3               │
    │   0-RTT reconnection supported    │
    │   No head-of-line blocking        │
    │   One stream = one pool slot      │
```

#### Type 2 — WebSocket Pool

```
  Server                              Client
    │                                   │
    │   HTTP listener (ws:// or wss://) │
    │◄──────────────────────────────────│  HTTP Upgrade request
    │                                   │
    │   GET / HTTP/1.1                  │
    │   Upgrade: websocket              │
    │   Sec-WebSocket-Key: ...          │
    │                                   │
    │──► 101 Switching Protocols        │
    │                                   │
    │   Full-duplex WebSocket frame     │
    │   traverses HTTP proxies and CDNs │
    │   Uses standard HTTPS port        │
    │   Requires TLS (tls=1 minimum)    │
```

#### Type 3 — HTTP/2 Pool

```
  Server                              Client
    │                                   │
    │   TLS listener (h2 ALPN)          │
    │◄──────────────────────────────────│  TLS + ALPN "h2"
    │                                   │
    │   Single TLS connection carries all streams:
    │                                   │
    │   ┌──────────────────────────┐    │
    │   │  HTTP/2 Connection       │    │
    │   │  HPACK header compress.  │    │
    │   │  ┌──────┐ ┌──────┐       │    │
    │   │  │stream│ │stream│ ...   │    │
    │   │  │  #1  │ │  #2  │       │    │
    │   │  └──────┘ └──────┘       │    │
    │   └──────────────────────────┘    │
    │                                   │
    │   Binary framing protocol         │
    │   Per-stream flow control         │
    │   Requires TLS (tls=1 minimum)    │
```

### Pool Lifecycle

```
  Pool Manager goroutine (runs continuously)
       │
       ▼
  ┌─────────────────────────────────────────────────────────┐
  │  Loop:                                                  │
  │                                                         │
  │  1. Check capacity: active < target?                    │
  │          │                                              │
  │          ├── Yes: create new connection/stream          │
  │          │        assign random 8-char hex ID           │
  │          │        store in connMap[id] = conn           │
  │          │        push id to idChan (buffered)          │
  │          │                                              │
  │          └── No:  sleep(interval), then loop            │
  │                                                         │
  │  2. Adjust interval (see Auto-Scaling below)            │
  │                                                         │
  │  3. Adjust capacity target (see Auto-Scaling below)     │
  └─────────────────────────────────────────────────────────┘

  Connection acquisition (data path):
       │
       ├── IncomingGet: read next ID from idChan, return connMap[id]
       │
       └── OutgoingGet: look up specific ID in connMap, remove + return
```

### Auto-Scaling

```
  ┌────────────────────────────────────────────────────────────────┐
  │  Capacity Adjustment                                           │
  │                                                                │
  │  success rate high  ──► expand capacity (up to MaxPool)        │
  │  success rate low   ──► contract capacity (down to MinPool)    │
  │                                                                │
  │  Bounds:                                                       │
  │    MinPool — set by client (guarantees warm connection floor)  │
  │    MaxPool — delivered by server during handshake              │
  │                                                                │
  ├────────────────────────────────────────────────────────────────┤
  │  Interval Adjustment                                           │
  │                                                                │
  │  idle count low  ──► shorten interval (faster refill)          │
  │  idle count high ──► lengthen interval (slow down, save CPU)   │
  │                                                                │
  │  Bounds:                                                       │
  │    MinPoolInterval (default 100ms) — floor on creation rate    │
  │    MaxPoolInterval (default 1s)    — ceiling on creation rate  │
  └────────────────────────────────────────────────────────────────┘
```

---

## Signal Protocol

All control signals travel over a persistent TCP connection (the control channel) as newline-delimited records.

### Encoding Pipeline

```
  Send side:
  ┌──────────┐    ┌─────────────────┐    ┌────────────┐    ┌──────┐
  │  Signal  │───►│ json.Marshal()  │───►│ XOR(key)   │───►│ B64  │──► \n
  │  struct  │    │                 │    │            │    │      │
  └──────────┘    └─────────────────┘    └────────────┘    └──────┘

  Receive side:
  \n ──► strip \n ──► Base64 decode ──► XOR(key) ──► json.Unmarshal ──► Signal struct

  XOR key = TunnelKey string, cycled with modulo indexing
  Both sides share TunnelKey — derived from URL password or port hash
```

### Signal Types

```
  ┌───────────┬──────────────┬──────────────────────────────────────────┐
  │  Signal   │  Direction   │  Purpose                                 │
  ├───────────┼──────────────┼──────────────────────────────────────────┤
  │  tcp      │  S → C       │  New TCP connection. Carries pool conn   │
  │           │  C → S       │  ID and original remote address.         │
  ├───────────┼──────────────┼──────────────────────────────────────────┤
  │  udp      │  S → C       │  New UDP datagram. Carries pool conn ID  │
  │           │  C → S       │  and originating client address.         │
  ├───────────┼──────────────┼──────────────────────────────────────────┤
  │  verify   │  bidirect    │  Exchange TLS certificate SHA-256        │
  │           │              │  fingerprint. tls=1 only. Mismatch       │
  │           │              │  cancels the session.                    │
  ├───────────┼──────────────┼──────────────────────────────────────────┤
  │  ping     │  either      │  Health check request.                   │
  ├───────────┼──────────────┼──────────────────────────────────────────┤
  │  pong     │  either      │  Health check response. Receiver logs    │
  │           │              │  latency, pool size, slot counts,        │
  │           │              │  and byte counters.                      │
  ├───────────┼──────────────┼──────────────────────────────────────────┤
  │  flush    │  either      │  Instruct peer to flush and reset the    │
  │           │              │  connection pool. Triggered when pool    │
  │           │              │  error count > active/2.                 │
  └───────────┴──────────────┴──────────────────────────────────────────┘
```

### Signal Flow (TCP connection example)

```
  DataFlow "-": Server receives traffic, signals client

  External        Server                 Client           Local
  Caller                                                  Target
    │               │                      │                │
    │──connect─────►│                      │                │
    │               │                      │                │
    │         accept()                     │                │
    │               │                      │                │
    │         IncomingGet(pool)            │                │
    │         id = "a3f9c12b"              │                │
    │               │                      │                │
    │               │──signal──────────────►                │
    │               │  { action:"tcp",     │                │
    │               │    id:"a3f9c12b",    │                │
    │               │    remote:"1.2.3.4" }│                │
    │               │                      │                │
    │               │               OutgoingGet("a3f9c12b") │
    │               │               conn from pool          │
    │               │                      │                │
    │               │                      │──connect──────►│
    │               │                      │                │
    │◄══════════════╪══════════════════════╪═══════════════►│
    │            DataExchange (full-duplex, until EOF)      │
```

### Signal Dispatch Loop

```
  CommonQueue goroutine                 CommonOnce goroutine
  ─────────────────────                ──────────────────────
  BufReader.ReadBytes('\n')             for signal := range SignalChan:
       │                                    │
       │ Decode (B64 + XOR)                 ├── "tcp"    → go TunnelTCPOnce
       │ json.Unmarshal → Signal            ├── "udp"    → go TunnelUDPOnce
       │                                    ├── "verify" → go OutgoingVerify
       ▼                                    ├── "flush"  → pool.Flush()
  SignalChan ──────────────────────────►    ├── "ping"   → write pong signal
  (buffered, SemaphoreLimit capacity)       └── "pong"   → log checkpoint stats
```

---

## TLS Modes

TLS is configured with the `tls=` parameter and applies to the data channel pool connections. In client single mode it additionally controls listener TLS termination.

```
  ┌────────────────────────────────────────────────────────────────────┐
  │  tls=0   No TLS                                                    │
  │                                                                    │
  │   Pool conn ──► raw TCP ──► target                                 │
  │   Fast. No encryption overhead.                                    │
  └────────────────────────────────────────────────────────────────────┘

  ┌────────────────────────────────────────────────────────────────────┐
  │  tls=1   RAM Certificate (self-signed, ECDSA P-256, TLS 1.3)       │
  │                                                                    │
  │   Startup:                                                         │
  │     NewTLSConfig()                                                 │
  │       ecdsa.GenerateKey(P-256)                                     │
  │       x509.CreateCertificate(1-year validity)                      │
  │       tls.X509KeyPair → tls.Config                                 │
  │                                                                    │
  │   After handshake:                                                 │
  │     Server regenerates cert (fresh key material per session)       │
  │     Server → verify signal → client fingerprint                    │
  │     Client → verify signal → server fingerprint                    │
  │     SHA-256(cert.Raw) compared on both sides                       │
  │     Mismatch → context.Cancel()                                    │
  │                                                                    │
  │   Pool conn ──► tls.Server/Client(rawConn, cfg) ──► target         │
  └────────────────────────────────────────────────────────────────────┘

  ┌────────────────────────────────────────────────────────────────────┐
  │  tls=2   File Certificate (PEM, hot reload)                        │
  │                                                                    │
  │   Startup:                                                         │
  │     tls.LoadX509KeyPair(crtFile, keyFile)                          │
  │                                                                    │
  │   GetCertificate callback (called per TLS handshake):              │
  │     if time.Since(lastReload) >= ReloadInterval (default 1h):      │
  │       reload cert from disk                                        │
  │       update cachedCert in-place                                   │
  │       zero downtime — existing connections unaffected              │
  │                                                                    │
  │   Pool conn ──► tls.Server/Client(rawConn, cfg) ──► target         │
  └────────────────────────────────────────────────────────────────────┘
```

### TLS Fingerprint Verification (tls=1)

```
  Server side                           Client side
       │                                     │
       │  cert = TLSConfig.Certificates[0]   │
       │  fp = SHA-256(cert.Certificate[0])  │
       │  signal{verify, id, fp: serverFP}   │
       │─────────────────────────────────────►
       │                                     │
       │          state = conn.ConnectionState()
       │          fp = SHA-256(PeerCertificates[0].Raw)
       │◄─────────────────────────────────────
       │  signal{verify, id, fp: clientFP}   │
       │                                     │
       │  Compare serverFP == clientFP       │
       │       │                             │
       │       ├── Match ──► VerifyChan ◄─── TunnelLoop unblocks
       │       │                             │
       │       └── Mismatch ──► Cancel()     │
```

---

## UDP Handling

UDP is stateless at the transport layer. NodePass imposes sessions on top using a `sync.Map` keyed by client address.

```
  ┌──────────────────────────────────────────────────────────────────┐
  │                     UDP Session Lifecycle                        │
  │                                                                  │
  │  UDP Client           NodePass           UDP Target              │
  │      │                    │                  │                   │
  │      │──datagram─────────►│                  │                   │
  │      │                    │                  │                   │
  │      │          sessionKey = clientAddr.String()                 │
  │      │                    │                  │                   │
  │      │          TargetUDPSession.Load(key)?  │                   │
  │      │                    │                  │                   │
  │      │          ┌── Found ──► reuse conn     │                   │
  │      │          │                            │                   │
  │      │          └── Not found:               │                   │
  │      │               acquire pool conn       │                   │
  │      │               Store(key, conn)        │                   │
  │      │               spawn read-back goroutine                   │
  │      │                    │                  │                   │
  │      │          writeUDPFrame(conn, data)    │                   │
  │      │                    │──4-byte len──────►                   │
  │      │                    │──payload─────────►                   │
  │      │                    │                  │                   │
  │      │          read-back goroutine:         │                   │
  │      │                    │◄──4-byte len──────                   │
  │      │                    │◄──payload─────────                   │
  │      │◄──datagram─────────│                  │                   │
  │      │                    │                  │                   │
  │      │         Session expires after UDPReadTimeout (default 30s)│
  │      │         Delete(key) + conn.Close()    │                   │
  └──────────────────────────────────────────────────────────────────┘
```

**UDP framing format:**

```
  ┌───────────────────────────────┐
  │  4 bytes  │  N bytes          │
  │  length   │  payload          │
  │  (uint32) │                   │
  └───────────────────────────────┘

  writeUDPFrame: binary.Write(conn, BigEndian, uint32(len)) + conn.Write(data)
  readUDPFrame:  binary.Read(conn, BigEndian, &length) + io.ReadFull(conn, buf[:length])
```

---

## Health and Load Management

### Health Check Loop

```
  HealthCheck goroutine (ReportInterval ticker, default 5s)
       │
       ├── pool.ErrorCount() > pool.Active() / 2 ?
       │       │
       │       Yes ──► send flush signal to peer
       │               pool.Flush() locally
       │               pool.ResetError()
       │
       ├── lbs=1 (latency-based) && len(targets) > 1 ?
       │       │
       │       Yes ──► ProbeBestTarget()
       │               goroutine per target: TCP dial + measure RTT
       │               store best index in TargetIdx atomically
       │
       └── CheckPoint = now
           send ping signal to peer
           wait for pong → log:
               MODE | PING latency | POOL active | TCPS | UDPS | TCPRX | TCPTX | UDPRX | UDPTX
```

### Slot and Rate Limiting

```
  Per connection:
  ┌──────────────────────────────────────────────────────────────┐
  │  TryAcquireSlot(isUDP)                                       │
  │      │                                                       │
  │      │  currentTotal = atomic.Load(TCPSlot + UDPSlot)        │
  │      │                                                       │
  │      ├── currentTotal >= SlotLimit? ──► reject connection    │
  │      │                                                       │
  │      └── atomic.Add(TCPSlot or UDPSlot, +1)                  │
  │          defer ReleaseSlot → atomic.Add(-1) on close         │
  └──────────────────────────────────────────────────────────────┘

  Per byte:
  ┌──────────────────────────────────────────────────────────────┐
  │  conn.StatConn wraps net.Conn                                │
  │      • RX/TX counters updated on every Read/Write            │
  │      • RateLimiter (token bucket) applied per connection     │
  │        rate=100 → 100 * 125000 = 12.5 MB/s token budget      │
  └──────────────────────────────────────────────────────────────┘
```

### Protocol Blocking

```
  DetectBlockProtocol(conn)
       │
       │  bufio.Reader.Peek(8)  — non-destructive first-byte inspection
       │
       ├── BlockSOCKS: b[0]==0x04 + b[1]==0x01/0x02  ──► reject "SOCKS4"
       │               b[0]==0x05 + b[1] in [1..3]   ──► reject "SOCKS5"
       │
       ├── BlockHTTP:  b[0..n] is uppercase ASCII + space ──► reject "HTTP"
       │
       └── BlockTLS:   b[0]==0x16 (TLS record type)  ──► reject "TLS"

       Allowed: return ReaderConn{Conn, reader} — peeked bytes still readable
       Blocked: log warning, close connection
```

### DNS Cache

```
  ResolveAddr(network, address)
       │
       ├── host is empty or raw IP? ──► skip cache, resolve directly
       │
       └── Resolve(network, address)
               │
               ├── DNSCacheEntries.Load(address)
               │       │
               │       ├── found + not expired ──► return cached addr
               │       └── found + expired     ──► delete, fall through
               │
               ├── net.ResolveTCPAddr + net.ResolveUDPAddr
               │
               └── store DnsCacheEntry{TCPAddr, UDPAddr, ExpiredAt=now+DNSCacheTTL}
                   DNSCacheTTL default = 5m, configurable via dns= param
```

---

## Master Mode

Master mode runs a REST API server that manages multiple NodePass instances. Each instance is a server or client running as an independent goroutine.

```
  ┌───────────────────────────────────────────────────────────────────┐
  │                       Master Mode Architecture                    │
  │                                                                   │
  │   API Consumer (curl / dashboard / AI agent)                      │
  │         │                                                         │
  │         │  HTTP or HTTPS                                          │
  │         ▼                                                         │
  │   ┌───────────────────────────────────────────────┐               │
  │   │  REST API Server  (optional TLS — same modes) │               │
  │   │                                               │               │
  │   │  POST   {prefix}/v1/instances                 │               │
  │   │  GET    {prefix}/v1/instances                 │               │
  │   │  GET    {prefix}/v1/instances/{id}            │               │
  │   │  PATCH  {prefix}/v1/instances/{id}            │               │
  │   │  DELETE {prefix}/v1/instances/{id}            │               │
  │   │  GET    {prefix}/v1/events  (SSE stream)      │               │
  │   │  GET    {prefix}/v1/info                      │               │
  │   │  GET    {prefix}/v1/openapi.json              │               │
  │   │  GET    {prefix}/v1/docs   (Swagger UI)       │               │
  │   └───────────────────┬───────────────────────────┘               │
  │                       │                                           │
  │            Instance Registry (in-memory map)                      │
  │                       │                                           │
  │          ┌────────────┼────────────┐                              │
  │          ▼            ▼            ▼                              │
  │   ┌────────────┐ ┌──────────┐ ┌──────────┐                        │
  │   │ server://  │ │client:// │ │client:// │  ...                   │
  │   │ instance A │ │instance B│ │instance C│                        │
  │   │ (goroutine)│ │(goroutine│ │(goroutine│                        │
  │   └────────────┘ └──────────┘ └──────────┘                        │
  │                                                                   │
  │   MCP 2.0 JSON-RPC endpoint available for AI assistant access     │
  └───────────────────────────────────────────────────────────────────┘
```

**Instance lifecycle via API:**

```
  POST /instances  { "url": "server://:10101/:8080?tls=1" }
       │
       ├── Parse URL
       ├── createCore(parsedURL, logger) → server.NewServer / client.NewClient
       ├── assign UID
       ├── store in registry
       └── go instance.Run()

  PATCH /instances/{id}  { "action": "restart" }
       │
       ├── "start"   → go instance.Run()
       ├── "stop"    → instance.Stop() with ShutdownTimeout
       └── "restart" → Stop() then go Run()

  GET /events  (SSE)
       │
       └── streams instance status changes in real-time
           event: { id, status, pool, ping, tcps, udps, tcprx, ... }
```

---

## Full Connection Walkthrough

End-to-end trace for a single TCP connection through a pooled tunnel (`DataFlow "-"`, `tls=1`, `type=0`):

```
  ① External caller connects to server's target port

  ② Server TunnelTCPLoop:
       targetConn = TargetListener.Accept()
       wrap in StatConn (RX/TX counters + rate limiter)
       DetectBlockProtocol → allowed
       TryAcquireSlot(false) → TCPSlot++
       id, remoteConn = TunnelPool.IncomingGet(timeout)

  ③ Server writes signal to control channel:
       json{ action:"tcp", remote:"1.2.3.4:56789", id:"a3f9c12b" }
       → XOR(TunnelKey) → Base64 → \n → ControlConn.Write

  ④ Client CommonQueue reads signal:
       BufReader.ReadBytes('\n') → Decode → json.Unmarshal
       push Signal to SignalChan

  ⑤ Client CommonOnce dispatches:
       case "tcp": go TunnelTCPOnce(signal)

  ⑥ Client TunnelTCPOnce:
       remoteConn = TunnelPool.OutgoingGet("a3f9c12b", timeout)
       TryAcquireSlot(false) → TCPSlot++
       targetConn = DialWithRotation("tcp", timeout)
       SendProxyV1Header (if proxy=1)

  ⑦ Both sides:
       conn.DataExchange(remoteConn, targetConn, readTimeout, buf1, buf2)
       bidirectional io.Copy loop until EOF or context cancel
       defer: remoteConn.Close(), targetConn.Close(), ReleaseSlot

  ⑧ Pool manager refills the consumed slot asynchronously

  Parallel path — control channel health check (every 5s):
       ping signal ──► peer writes pong ──► logger records:
       CHECK_POINT | MODE=1 | PING=3ms | POOL=62 | TCPS=1 | UDPS=0 | ...
```

**Key invariants throughout:**
- `remoteConn` and `targetConn` are always closed in `defer` — no leaks on early return
- `TCPSlot` is always released in `defer` paired with `TryAcquireSlot`
- Pool connection is removed from the map on `OutgoingGet` — one connection, one use
- `WriteChan` serialises all control writes through a single goroutine — no concurrent writes to `ControlConn`

---

## Buffer and Memory Management

NodePass uses `sync.Pool` for buffer reuse, keeping allocations off the GC hot path under high concurrency.

```
  ┌──────────────────────────────────────────────────────────────────┐
  │  Buffer Pools (one per Common instance)                          │
  │                                                                  │
  │  TCPBufferPool  sync.Pool{ New: make([]byte, TCPDataBufSize) }   │
  │  UDPBufferPool  sync.Pool{ New: make([]byte, UDPDataBufSize) }   │
  │                                                                  │
  │  Defaults (overridable via env):                                 │
  │    NP_TCP_DATA_BUF_SIZE = 16384  (16 KB per TCP exchange)        │
  │    NP_UDP_DATA_BUF_SIZE = 16384  (16 KB per UDP datagram)        │
  │                                                                  │
  │  Lifecycle:                                                      │
  │    GetTCPBuffer()  → pool.Get() → slice[:TCPDataBufSize]         │
  │    PutTCPBuffer()  → pool.Put() — only if cap >= TCPDataBufSize  │
  │                                                                  │
  │  DataExchange uses two buffers (one per direction):              │
  │    buf1 := GetTCPBuffer()   // A → B copy buffer                 │
  │    buf2 := GetTCPBuffer()   // B → A copy buffer                 │
  │    defer PutTCPBuffer(buf1)                                      │
  │    defer PutTCPBuffer(buf2)                                      │
  └──────────────────────────────────────────────────────────────────┘
```

### Environment Tuning Reference

Key runtime constants that can be overridden via environment variables before startup:

```
  ┌───────────────────────────┬──────────────┬────────────────────────────┐
  │  Variable                 │  Default     │  Effect                    │
  ├───────────────────────────┼──────────────┼────────────────────────────┤
  │  NP_TCP_DATA_BUF_SIZE     │  16384       │  TCP copy buffer bytes     │
  │  NP_UDP_DATA_BUF_SIZE     │  16384       │  UDP datagram buffer bytes │
  │  NP_SEMAPHORE_LIMIT       │  65536       │  SignalChan + WriteChan cap│
  │  NP_HANDSHAKE_TIMEOUT     │  5s          │  Handshake deadline        │
  │  NP_TCP_DIAL_TIMEOUT      │  5s          │  Target TCP connect limit  │
  │  NP_UDP_DIAL_TIMEOUT      │  5s          │  Target UDP connect limit  │
  │  NP_UDP_READ_TIMEOUT      │  30s         │  UDP session idle expiry   │
  │  NP_POOL_GET_TIMEOUT      │  5s          │  Pool acquisition deadline │
  │  NP_MIN_POOL_INTERVAL     │  100ms       │  Fastest pool refill rate  │
  │  NP_MAX_POOL_INTERVAL     │  1s          │  Slowest pool refill rate  │
  │  NP_REPORT_INTERVAL       │  5s          │  Health check / ping cycle │
  │  NP_FALLBACK_INTERVAL     │  5m          │  lbs=2 primary reset timer │
  │  NP_SERVICE_COOLDOWN      │  3s          │  Restart backoff on error  │
  │  NP_SHUTDOWN_TIMEOUT      │  5s          │  Graceful stop deadline    │
  │  NP_RELOAD_INTERVAL       │  1h          │  tls=2 cert reload period  │
  └───────────────────────────┴──────────────┴────────────────────────────┘
```

---

## Shutdown and Cleanup

Graceful shutdown follows a structured teardown order to avoid data loss and resource leaks:

```
  SIGINT / SIGTERM received
       │
       ├── context.Cancel() — propagates to all goroutines via Ctx
       │
       ├── TunnelPool.Close()    — drains and closes all pool connections
       │
       ├── TargetUDPSession.Range → close each UDP session conn
       │
       ├── TargetUDPConn.Close() — stop accepting UDP datagrams
       │
       ├── TunnelUDPConn.Close() — stop UDP tunnel socket
       │
       ├── ControlConn.Close()   — disconnect control channel
       │
       ├── TargetListener.Close()— stop accepting new target connections
       │
       ├── TunnelListener.Close()— stop accepting new tunnel connections
       │
       ├── Drain(SignalChan)     — discard queued signals
       ├── Drain(WriteChan)      — discard queued control writes
       └── Drain(VerifyChan)     — discard pending verify handshakes

  ShutdownTimeout (default 5s) wraps the above sequence.
  If exceeded, the process exits regardless of in-flight connections.
```

---

## Next Steps

- [Usage Instructions](/docs/usage.md) — command syntax and parameter reference
- [Examples](/docs/examples.md) — practical deployment scenarios
- [Configuration](/docs/configuration.md) — tuning options
- [Troubleshooting](/docs/troubleshooting.md) — common issues