package common

import (
	"bufio"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/NodePassProject/conn"
	"github.com/NodePassProject/logs"
)

const (
	ContextCheckInterval = 50 * time.Millisecond
	DefaultDNSTTL        = 5 * time.Minute
	DefaultMinPool       = 64
	DefaultMaxPool       = 1024
	DefaultServerName    = "none"
	DefaultLBStrategy    = "0"
	DefaultRunMode       = "0"
	DefaultPoolType      = "0"
	DefaultDialerIP      = "auto"
	DefaultReadTimeout   = 0 * time.Second
	DefaultRateLimit     = 0
	DefaultSlotLimit     = 65536
	DefaultProxyProtocol = "0"
	DefaultBlockProtocol = "0"
	DefaultTCPStrategy   = "0"
	DefaultUDPStrategy   = "0"
)

var (
	SemaphoreLimit   = getEnvAsInt("NP_SEMAPHORE_LIMIT", 65536)
	TcpDataBufSize   = getEnvAsInt("NP_TCP_DATA_BUF_SIZE", 16384)
	UdpDataBufSize   = getEnvAsInt("NP_UDP_DATA_BUF_SIZE", 16384)
	HandshakeTimeout = getEnvAsDuration("NP_HANDSHAKE_TIMEOUT", 5*time.Second)
	TcpDialTimeout   = getEnvAsDuration("NP_TCP_DIAL_TIMEOUT", 5*time.Second)
	UdpDialTimeout   = getEnvAsDuration("NP_UDP_DIAL_TIMEOUT", 5*time.Second)
	UdpReadTimeout   = getEnvAsDuration("NP_UDP_READ_TIMEOUT", 30*time.Second)
	PoolGetTimeout   = getEnvAsDuration("NP_POOL_GET_TIMEOUT", 5*time.Second)
	MinPoolInterval  = getEnvAsDuration("NP_MIN_POOL_INTERVAL", 100*time.Millisecond)
	MaxPoolInterval  = getEnvAsDuration("NP_MAX_POOL_INTERVAL", 1*time.Second)
	ReportInterval   = getEnvAsDuration("NP_REPORT_INTERVAL", 5*time.Second)
	FallbackInterval = getEnvAsDuration("NP_FALLBACK_INTERVAL", 5*time.Minute)
	ServiceCooldown  = getEnvAsDuration("NP_SERVICE_COOLDOWN", 3*time.Second)
	ShutdownTimeout  = getEnvAsDuration("NP_SHUTDOWN_TIMEOUT", 5*time.Second)
	ReloadInterval   = getEnvAsDuration("NP_RELOAD_INTERVAL", 1*time.Hour)
)

type Common struct {
	TargetIdx        uint64
	LastFallback     uint64
	TcpRX            uint64
	TcpTX            uint64
	UdpRX            uint64
	UdpTX            uint64
	ParsedURL        *url.URL
	Logger           *logs.Logger
	DnsCacheTTL      time.Duration
	DnsCacheEntries  sync.Map
	TlsCode          string
	TlsConfig        *tls.Config
	CoreType         string
	RunMode          string
	PoolType         string
	DataFlow         string
	ServerName       string
	ServerPort       string
	ClientIP         string
	DialerIP         string
	DialerFallback   uint32
	TunnelKey        string
	TunnelAddr       string
	TunnelTCPAddr    *net.TCPAddr
	TunnelUDPAddr    *net.UDPAddr
	TargetAddrs      []string
	TargetTCPAddrs   []*net.TCPAddr
	TargetUDPAddrs   []*net.UDPAddr
	BestLatency      int32
	LbStrategy       string
	TargetListener   *net.TCPListener
	TunnelListener   net.Listener
	ControlConn      net.Conn
	TunnelUDPConn    *conn.StatConn
	TargetUDPConn    *conn.StatConn
	TargetUDPSession sync.Map
	TunnelPool       TransportPool
	MinPoolCapacity  int
	MaxPoolCapacity  int
	ProxyProtocol    string
	BlockProtocol    string
	BlockSOCKS       bool
	BlockHTTP        bool
	BlockTLS         bool
	DisableTCP       string
	DisableUDP       string
	RateLimit        int
	RateLimiter      *conn.RateLimiter
	ReadTimeout      time.Duration
	BufReader        *bufio.Reader
	TcpBufferPool    *sync.Pool
	UdpBufferPool    *sync.Pool
	SignalChan       chan Signal
	WriteChan        chan []byte
	VerifyChan       chan struct{}
	HandshakeStart   time.Time
	CheckPoint       time.Time
	SlotLimit        int32
	TcpSlot          int32
	UdpSlot          int32
	Ctx              context.Context
	Cancel           context.CancelFunc
}

type dnsCacheEntry struct {
	tcpAddr   *net.TCPAddr
	udpAddr   *net.UDPAddr
	expiredAt time.Time
}

type ReaderConn struct {
	net.Conn
	Reader io.Reader
}

func (rc *ReaderConn) Read(b []byte) (int, error) {
	return rc.Reader.Read(b)
}

type TransportPool interface {
	IncomingGet(timeout time.Duration) (string, net.Conn, error)
	OutgoingGet(id string, timeout time.Duration) (net.Conn, error)
	Flush()
	Close()
	Ready() bool
	Active() int
	Capacity() int
	Interval() time.Duration
	AddError()
	ErrorCount() int
	ResetError()
}

type Signal struct {
	ActionType  string `json:"action"`
	RemoteAddr  string `json:"remote,omitempty"`
	PoolConnID  string `json:"id,omitempty"`
	Fingerprint string `json:"fp,omitempty"`
}
