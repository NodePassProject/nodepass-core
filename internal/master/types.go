package master

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"regexp"
	"sync"
	"time"

	"github.com/NodePassProject/nodepass/internal/common"
)

type Master struct {
	common.Common
	MID           string
	Alias         string
	Prefix        string
	Version       string
	Hostname      string
	LogLevel      string
	CrtPath       string
	KeyPath       string
	Instances     sync.Map
	Server        *http.Server
	MTLSConfig    *tls.Config
	MasterURL     *url.URL
	StatePath     string
	StateMu       sync.Mutex
	Subscribers   sync.Map
	NotifyChannel chan *InstanceEvent
	TCPingSem     chan struct{}
	StartTime     time.Time
	PeriodicDone  chan struct{}
}

type Instance struct {
	ID             string `json:"id"`
	Alias          string `json:"alias"`
	Type           string `json:"type"`
	Status         string `json:"status"`
	URL            string `json:"url"`
	Config         string `json:"config"`
	Restart        bool   `json:"restart"`
	Meta           Meta   `json:"meta"`
	Mode           int32  `json:"mode"`
	Ping           int32  `json:"ping"`
	Pool           int32  `json:"pool"`
	TCPS           int32  `json:"tcps"`
	UDPS           int32  `json:"udps"`
	TCPRX          uint64 `json:"tcprx"`
	TCPTX          uint64 `json:"tcptx"`
	UDPRX          uint64 `json:"udprx"`
	UDPTX          uint64 `json:"udptx"`
	tcpRXBase      uint64
	tcpTXBase      uint64
	udpRXBase      uint64
	udpTXBase      uint64
	tcpRXReset     uint64
	tcpTXReset     uint64
	udpRXReset     uint64
	udpTXReset     uint64
	cmd            *exec.Cmd
	stopped        chan struct{}
	deleted        bool
	cancelFunc     context.CancelFunc
	lastCheckPoint time.Time
}

type Meta struct {
	Peer Peer              `json:"peer"`
	Tags map[string]string `json:"tags"`
}

type Peer struct {
	SID   string `json:"sid"`
	Type  string `json:"type"`
	Alias string `json:"alias"`
}

type InstanceLogWriter struct {
	InstanceID string
	Instance   *Instance
	Target     io.Writer
	Master     *Master
	CheckPoint *regexp.Regexp
}

type InstanceEvent struct {
	Type     string    `json:"type"`
	Time     time.Time `json:"time"`
	Instance *Instance `json:"instance"`
	Logs     string    `json:"logs"`
}

type SystemInfo struct {
	CPU       int    `json:"cpu"`
	MemTotal  uint64 `json:"mem_total"`
	MemUsed   uint64 `json:"mem_used"`
	SwapTotal uint64 `json:"swap_total"`
	SwapUsed  uint64 `json:"swap_used"`
	NetRX     uint64 `json:"netrx"`
	NetTX     uint64 `json:"nettx"`
	DiskR     uint64 `json:"diskr"`
	DiskW     uint64 `json:"diskw"`
	SysUp     uint64 `json:"sysup"`
}

type TCPingResult struct {
	Target    string  `json:"target"`
	Connected bool    `json:"connected"`
	Latency   int64   `json:"latency"`
	Error     *string `json:"error"`
}

type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type MCPResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Result  any    `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    any    `json:"data,omitempty"`
	} `json:"error,omitempty"`
}

type MCPToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}
