package main

import (
	"flag"
	"net"
	"os"
	"zen/backend"
	"zen/balancer"
	"zen/config"
	"zen/handler"
	"zen/utils/logger"
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
		logger.Fatal("Failed to parse configuration file:", err)
		os.Exit(1)
	}

	logger.Info("Starting server...")
	ln, err := net.Listen("tcp", ":"+cfg.Server.Port)
	if err != nil {
		logger.Fatal("Failed to start server on port {}:", cfg.Server.Port, err)
		cleanUp()
		os.Exit(1)
	}

	backendPool := getBackendPool(&cfg)
	backendPool.GetAliveBackends()
	loadBalancer := balancer.NewRoundRobin(backendPool)
	proxy := handler.NewConnectionHandler(loadBalancer)

	for {
		conn, err := ln.Accept()
		if err != nil {
			logger.Error("Failed to accept connection:", err)
			continue
		}

		go proxy.HandleConnection(conn)
	}
}

func cleanUp() {
	logger.Info("Shutting down server...")
	//TODO: Perform cleanup
	logger.Info("Server shut down successfully.")
}

func getBackendPool(cfg *config.Config) *backend.Pool {
	logger.Info("Getting list of upstream servers...")
	backendPool := backend.NewBackendPool(cfg.Upstream)
	if backendPool == nil {
		logger.Fatal("Failed to create backend pool")
		cleanUp()
		os.Exit(1)
	}

	logger.Info("Backend pool created with {} backends", len(backendPool.GetAliveBackends()))
	return backendPool
}
