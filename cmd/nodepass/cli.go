package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"runtime"
	"strings"
)

type commandLine struct {
	args       []string
	password   *string
	tunnelAddr *string
	tunnelPort *string
	targetAddr *string
	targetPort *string
	targets    *string
	log        *string
	dns        *string
	sni        *string
	lbs        *string
	min        *string
	max        *string
	mode       *string
	pool       *string
	tls        *string
	crt        *string
	key        *string
	dial       *string
	read       *string
	rate       *string
	slot       *string
	proxy      *string
	block      *string
	notcp      *string
	noudp      *string
}

func newCommandLine(args []string) *commandLine {
	return &commandLine{args: args}
}

func (c *commandLine) parse() (*url.URL, error) {
	if len(c.args) == 2 && strings.Contains(c.args[1], "://") {
		return url.Parse(c.args[1])
	}

	if len(c.args) < 2 {
		return nil, fmt.Errorf("usage: nodepass <command> [options] or nodepass <url>")
	}

	command := c.args[1]
	cmdArgs := c.args[2:]

	switch command {
	case "server":
		return c.parseServerCommand(cmdArgs)
	case "client":
		return c.parseClientCommand(cmdArgs)
	case "master":
		return c.parseMasterCommand(cmdArgs)
	case "help", "-h", "--help":
		c.printHelp()
		os.Exit(0)
	case "version", "-v", "--version":
		fmt.Printf("nodepass-%s %s/%s\n", version, runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	default:
		fullArg := strings.Join(c.args[1:], " ")
		if strings.Contains(fullArg, "://") {
			return url.Parse(fullArg)
		}
		return nil, fmt.Errorf("unknown command: %s", command)
	}

	return nil, nil
}

func (c *commandLine) addServerFlags(fs *flag.FlagSet) {
	c.password = fs.String("password", "", "Connection password")
	c.tunnelAddr = fs.String("tunnel-addr", "", "Tunnel address")
	c.tunnelPort = fs.String("tunnel-port", "", "Tunnel port")
	c.targetAddr = fs.String("target-addr", "", "Target address")
	c.targetPort = fs.String("target-port", "", "Target port")
	c.targets = fs.String("targets", "", "Multiple targets")
	c.log = fs.String("log", "", "Log level")
	c.tls = fs.String("tls", "", "TLS mode")
	c.crt = fs.String("crt", "", "Certificate file path")
	c.key = fs.String("key", "", "Key file path")
	c.lbs = fs.String("lbs", "", "Load balancing strategy")
	c.max = fs.String("max", "", "Maximum pool size")
	c.mode = fs.String("mode", "", "Run mode")
	c.pool = fs.String("type", "", "Pool type")
	c.dial = fs.String("dial", "", "Outbound source IP")
	c.read = fs.String("read", "", "Read timeout")
	c.rate = fs.String("rate", "", "Bandwidth limit in Mbps")
	c.slot = fs.String("slot", "", "Connection slot limit")
	c.proxy = fs.String("proxy", "", "PROXY protocol v1")
	c.block = fs.String("block", "", "Block protocols")
	c.notcp = fs.String("notcp", "", "Disable TCP")
	c.noudp = fs.String("noudp", "", "Disable UDP")
}

func (c *commandLine) addClientFlags(fs *flag.FlagSet) {
	c.password = fs.String("password", "", "Connection password")
	c.tunnelAddr = fs.String("tunnel-addr", "", "Server tunnel address")
	c.tunnelPort = fs.String("tunnel-port", "", "Server tunnel port")
	c.targetAddr = fs.String("target-addr", "", "Target address")
	c.targetPort = fs.String("target-port", "", "Target port")
	c.targets = fs.String("targets", "", "Multiple targets")
	c.log = fs.String("log", "", "Log level")
	c.dns = fs.String("dns", "", "DNS cache TTL")
	c.sni = fs.String("sni", "", "SNI hostname")
	c.tls = fs.String("tls", "", "TLS mode")
	c.crt = fs.String("crt", "", "Certificate file path")
	c.key = fs.String("key", "", "Key file path")
	c.lbs = fs.String("lbs", "", "Load balancing strategy")
	c.min = fs.String("min", "", "Minimum pool size")
	c.mode = fs.String("mode", "", "Connection mode")
	c.dial = fs.String("dial", "", "Outbound source IP")
	c.read = fs.String("read", "", "Read timeout")
	c.rate = fs.String("rate", "", "Bandwidth limit in Mbps")
	c.slot = fs.String("slot", "", "Connection slot limit")
	c.proxy = fs.String("proxy", "", "PROXY protocol v1")
	c.block = fs.String("block", "", "Block protocols")
	c.notcp = fs.String("notcp", "", "Disable TCP")
	c.noudp = fs.String("noudp", "", "Disable UDP")
}

func (c *commandLine) addMasterFlags(fs *flag.FlagSet) {
	c.tunnelAddr = fs.String("tunnel-addr", "", "Master API listening address")
	c.tunnelPort = fs.String("tunnel-port", "", "Master API listening port")
	c.log = fs.String("log", "", "Log level")
	c.tls = fs.String("tls", "", "TLS mode")
	c.crt = fs.String("crt", "", "Certificate file path")
	c.key = fs.String("key", "", "Key file path")
}

func (c *commandLine) buildQuery() url.Values {
	query := url.Values{}

	if c.log != nil && *c.log != "" {
		query.Set("log", *c.log)
	}
	if c.dns != nil && *c.dns != "" {
		query.Set("dns", *c.dns)
	}
	if c.sni != nil && *c.sni != "" {
		query.Set("sni", *c.sni)
	}
	if c.lbs != nil && *c.lbs != "" {
		query.Set("lbs", *c.lbs)
	}
	if c.min != nil && *c.min != "" {
		query.Set("min", *c.min)
	}
	if c.max != nil && *c.max != "" {
		query.Set("max", *c.max)
	}
	if c.mode != nil && *c.mode != "" {
		query.Set("mode", *c.mode)
	}
	if c.pool != nil && *c.pool != "" {
		query.Set("type", *c.pool)
	}
	if c.tls != nil && *c.tls != "" {
		query.Set("tls", *c.tls)
	}
	if c.crt != nil && c.key != nil && c.tls != nil && *c.tls == "2" {
		if *c.crt != "" {
			query.Set("crt", *c.crt)
		}
		if *c.key != "" {
			query.Set("key", *c.key)
		}
	}
	if c.dial != nil && *c.dial != "" {
		query.Set("dial", *c.dial)
	}
	if c.read != nil && *c.read != "" {
		query.Set("read", *c.read)
	}
	if c.rate != nil && *c.rate != "" {
		query.Set("rate", *c.rate)
	}
	if c.slot != nil && *c.slot != "" {
		query.Set("slot", *c.slot)
	}
	if c.proxy != nil && *c.proxy != "" {
		query.Set("proxy", *c.proxy)
	}
	if c.block != nil && *c.block != "" {
		query.Set("block", *c.block)
	}
	if c.notcp != nil && *c.notcp != "" {
		query.Set("notcp", *c.notcp)
	}
	if c.noudp != nil && *c.noudp != "" {
		query.Set("noudp", *c.noudp)
	}

	return query
}

func (c *commandLine) buildHost() string {
	addr := ""
	if c.tunnelAddr != nil && *c.tunnelAddr != "" {
		addr = *c.tunnelAddr
	}
	if c.tunnelPort != nil && *c.tunnelPort != "" {
		if addr != "" {
			return fmt.Sprintf("%s:%s", addr, *c.tunnelPort)
		}
		return ":" + *c.tunnelPort
	}
	return addr
}

func (c *commandLine) buildPath() string {
	if c.targets != nil && *c.targets != "" {
		return "/" + *c.targets
	}
	addr := ""
	if c.targetAddr != nil && *c.targetAddr != "" {
		addr = *c.targetAddr
	}
	if c.targetPort != nil && *c.targetPort != "" {
		if addr != "" {
			return fmt.Sprintf("/%s:%s", addr, *c.targetPort)
		}
		return "/:" + *c.targetPort
	}
	if addr != "" {
		return "/" + addr
	}
	return "/"
}

func (c *commandLine) buildUserInfo() *url.Userinfo {
	if c.password != nil && *c.password != "" {
		return url.User(*c.password)
	}
	return nil
}

func (c *commandLine) parseServerCommand(args []string) (*url.URL, error) {
	fs := flag.NewFlagSet("server", flag.ExitOnError)
	c.addServerFlags(fs)

	fs.Usage = func() {
		fmt.Fprintf(os.Stdout, "Usage: nodepass server [options]\n\nOptions:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	return &url.URL{
		Scheme:   "server",
		User:     c.buildUserInfo(),
		Host:     c.buildHost(),
		Path:     c.buildPath(),
		RawQuery: c.buildQuery().Encode(),
	}, nil
}

func (c *commandLine) parseClientCommand(args []string) (*url.URL, error) {
	fs := flag.NewFlagSet("client", flag.ExitOnError)
	c.addClientFlags(fs)

	fs.Usage = func() {
		fmt.Fprintf(os.Stdout, "Usage: nodepass client [options]\n\nOptions:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	return &url.URL{
		Scheme:   "client",
		User:     c.buildUserInfo(),
		Host:     c.buildHost(),
		Path:     c.buildPath(),
		RawQuery: c.buildQuery().Encode(),
	}, nil
}

func (c *commandLine) parseMasterCommand(args []string) (*url.URL, error) {
	fs := flag.NewFlagSet("master", flag.ExitOnError)
	c.addMasterFlags(fs)

	fs.Usage = func() {
		fmt.Fprintf(os.Stdout, "Usage: nodepass master [options]\n\nOptions:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	return &url.URL{
		Scheme:   "master",
		Host:     c.buildHost(),
		RawQuery: c.buildQuery().Encode(),
	}, nil
}

func (c *commandLine) printHelp() {
	fmt.Fprintf(os.Stdout, `NodePass - Universal TCP/UDP Tunneling Solution

Usage:
  nodepass <command> [options]
  nodepass <url>

Commands:
  server     Start a NodePass server
  client     Start a NodePass client
  master     Start a NodePass master
  help       Show this help message
  version    Show version information

For more information, visit: https://github.com/NodePassProject
`)
}
