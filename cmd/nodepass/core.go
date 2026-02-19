package main

import (
	"fmt"
	"net/url"
	"os"
	"runtime"

	"github.com/NodePassProject/logs"
	"github.com/NodePassProject/nodepass/internal/client"
	"github.com/NodePassProject/nodepass/internal/master"
	"github.com/NodePassProject/nodepass/internal/server"
)

func start(args []string) error {
	parsedURL, err := newCommandLine(args).parse()
	if err != nil {
		return fmt.Errorf("start: parse command failed: %w", err)
	}

	logger := initLogger(parsedURL.Query().Get("log"))

	core, err := createCore(parsedURL, logger)
	if err != nil {
		return fmt.Errorf("start: create core failed: %w", err)
	}

	core.Run()
	return nil
}

func initLogger(level string) *logs.Logger {
	logger := logs.NewLogger(logs.Info, true)
	switch level {
	case "none":
		logger.SetLogLevel(logs.None)
	case "debug":
		logger.SetLogLevel(logs.Debug)
		logger.Debug("Init log level: DEBUG")
	case "warn":
		logger.SetLogLevel(logs.Warn)
		logger.Warn("Init log level: WARN")
	case "error":
		logger.SetLogLevel(logs.Error)
		logger.Error("Init log level: ERROR")
	case "event":
		logger.SetLogLevel(logs.Event)
		logger.Event("Init log level: EVENT")
	default:
	}
	return logger
}

func createCore(parsedURL *url.URL, logger *logs.Logger) (interface{ Run() }, error) {
	switch parsedURL.Scheme {
	case "server":
		tlsCode, tlsConfig := getTLSProtocol(parsedURL, logger)
		return server.NewServer(parsedURL, tlsCode, tlsConfig, logger)
	case "client":
		return client.NewClient(parsedURL, logger)
	case "master":
		tlsCode, tlsConfig := getTLSProtocol(parsedURL, logger)
		return master.NewMaster(parsedURL, tlsCode, tlsConfig, logger, version)
	default:
		return nil, fmt.Errorf("createCore: unknown core: %v", parsedURL)
	}
}

func exit(err error) {
	errMsg := "none"
	if err != nil {
		errMsg = err.Error()
	}
	fmt.Fprintf(os.Stderr,
		"nodepass-%s %s/%s pid=%d error=%s\n\nrun 'nodepass --help' for usage\n",
		version, runtime.GOOS, runtime.GOARCH, os.Getpid(), errMsg)

	os.Exit(1)
}
