package main

import (
	"flag"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
	"zen/backend"
	"zen/balancer"
	"zen/config"
	"zen/handler"
	"zen/utils/logger"
)

var (
	backendPool   *backend.Pool
	healthChecker *backend.HealthChecker
)

func init() {
	level := logger.LevelInfo
	if os.Getenv("DEBUG") == "1" {
		level = logger.LevelDebug
	}

	logger.SetLevel(level)
}

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "Path to the configuration file")
	flag.Parse()

	if configPath == "" {
		configPath = "config.yaml"
	}

	var cfg config.Config
	err := config.ParseConfig(&cfg, configPath)
	if err != nil {
		logger.Fatal("Failed to parse configuration file: %s", err)
		os.Exit(1)
	}

	logger.Info("Starting load balancer server...")
	ln, err := net.Listen("tcp", ":"+cfg.Server.Port)
	if err != nil {
		logger.Fatal("Failed to start server on port %s: %s", cfg.Server.Port, err)
		cleanUp()
		os.Exit(1)
	}

	backendPool = getBackendPool(&cfg)

	if cfg.HealthCheck.Enabled {
		healthCheckConfig := &backend.HealthCheckConfig{
			Interval:           cfg.HealthCheck.Interval,
			Timeout:            cfg.HealthCheck.Timeout,
			HealthyThreshold:   cfg.HealthCheck.HealthyThreshold,
			UnhealthyThreshold: cfg.HealthCheck.UnhealthyThreshold,
		}
		healthChecker = backend.NewHealthChecker(backendPool, healthCheckConfig)
		healthChecker.Start()
		logger.Info("Health checker started")
	} else {
		logger.Info("Health checking disabled")
	}

	loadBalancer := balancer.NewRoundRobin(backendPool)
	proxy := handler.NewConnectionHandler(loadBalancer)

	go handleShutdown()

	logger.Info("Load balancer ready on port %s", cfg.Server.Port)

	for {
		conn, err := ln.Accept()
		if err != nil {
			logger.Error("Failed to accept connection: %s", err)
			continue
		}

		go proxy.HandleConnection(conn)
	}
}

func handleShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigChan
	logger.Info("Received signal: %s. Shutting down...", sig)

	cleanUp()
	os.Exit(0)
}

func cleanUp() {
	logger.Info("Shutting down server...")

	if healthChecker != nil {
		healthChecker.Stop()
	}

	if backendPool != nil {
		backendPool.Close()
	}

	time.Sleep(1 * time.Second)

	logger.Info("Server shut down successfully.")
}

func getBackendPool(cfg *config.Config) *backend.Pool {
	logger.Info("Initializing backend pool with %d upstream servers", len(cfg.Upstream))

	if len(cfg.Upstream) == 0 {
		logger.Fatal("No upstream servers configured")
		cleanUp()
		os.Exit(1)
	}

	backendPool := backend.NewBackendPool(cfg.Upstream)
	if backendPool == nil {
		logger.Fatal("Failed to create backend pool")
		cleanUp()
		os.Exit(1)
	}

	total, alive := backendPool.GetBackendCount()
	logger.Info("Backend pool initialized: %d/%d backends alive", alive, total)
	return backendPool
}
