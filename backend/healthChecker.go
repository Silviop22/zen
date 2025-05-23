package backend

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"
	"zen/utils/logger"
)

type HealthCheckConfig struct {
	Interval           time.Duration
	Timeout            time.Duration
	HealthyThreshold   int
	UnhealthyThreshold int
}

type HealthChecker struct {
	config        *HealthCheckConfig
	pool          *Pool
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	mu            sync.RWMutex
	backendHealth map[string]*BackendHealth
}

type BackendHealth struct {
	consecutiveSuccesses int
	consecutiveFailures  int
	lastCheckTime        time.Time
	lastError            error
}

func NewHealthChecker(pool *Pool, config *HealthCheckConfig) *HealthChecker {
	if config == nil {
		config = &HealthCheckConfig{
			Interval:           30 * time.Second,
			Timeout:            5 * time.Second,
			HealthyThreshold:   2,
			UnhealthyThreshold: 3,
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &HealthChecker{
		config:        config,
		pool:          pool,
		ctx:           ctx,
		cancel:        cancel,
		backendHealth: make(map[string]*BackendHealth),
	}
}

func (hc *HealthChecker) Start() {
	logger.Info("Starting health checker with interval: {}", hc.config.Interval)

	backends := hc.pool.GetAliveBackends()
	hc.mu.Lock()
	for _, backend := range backends {
		hc.backendHealth[backend.Address] = &BackendHealth{
			consecutiveSuccesses: hc.config.HealthyThreshold,
		}
	}
	hc.mu.Unlock()

	hc.wg.Add(1)
	go hc.healthCheckLoop()
}

func (hc *HealthChecker) Stop() {
	logger.Info("Stopping health checker...")
	hc.cancel()
	hc.wg.Wait()
	logger.Info("Health checker stopped")
}

func (hc *HealthChecker) healthCheckLoop() {
	defer hc.wg.Done()

	ticker := time.NewTicker(hc.config.Interval)
	defer ticker.Stop()

	hc.checkAllBackends()

	for {
		select {
		case <-hc.ctx.Done():
			return
		case <-ticker.C:
			hc.checkAllBackends()
		}
	}
}

func (hc *HealthChecker) checkAllBackends() {
	allBackends := hc.pool.GetAllBackends()

	var wg sync.WaitGroup
	for _, backend := range allBackends {
		wg.Add(1)
		go func(b *Backend) {
			defer wg.Done()
			hc.checkBackend(b)
		}(backend)
	}

	wg.Wait()
	logger.Debug("Health check cycle completed for {} backends", len(allBackends))
}

func (hc *HealthChecker) checkBackend(backend *Backend) {
	startTime := time.Now()
	healthy := hc.isBackendHealthy(backend.Address)
	checkDuration := time.Since(startTime)

	hc.mu.Lock()
	defer hc.mu.Unlock()

	health, exists := hc.backendHealth[backend.Address]
	if !exists {
		health = &BackendHealth{}
		hc.backendHealth[backend.Address] = health
	}

	health.lastCheckTime = startTime

	if healthy {
		health.consecutiveSuccesses++
		health.consecutiveFailures = 0
		health.lastError = nil
		logger.Debug("Health check SUCCESS for {} (took {}ms)",
			backend.Address, checkDuration.Milliseconds())
	} else {
		health.consecutiveFailures++
		health.consecutiveSuccesses = 0
		logger.Debug("Health check FAILED for {} (took {}ms)",
			backend.Address, checkDuration.Milliseconds())
	}

	hc.evaluateBackendStatus(backend, health)
}

func (hc *HealthChecker) evaluateBackendStatus(backend *Backend, health *BackendHealth) {
	currentlyAlive := backend.Alive
	shouldBeAlive := currentlyAlive

	if !currentlyAlive && health.consecutiveSuccesses >= hc.config.HealthyThreshold {
		shouldBeAlive = true
		logger.Info("Backend {} is now HEALTHY ({}/%{} successful checks)",
			backend.Address, health.consecutiveSuccesses, hc.config.HealthyThreshold)
	} else if currentlyAlive && health.consecutiveFailures >= hc.config.UnhealthyThreshold {
		shouldBeAlive = false
		logger.Warn("Backend {} is now UNHEALTHY ({}/{} failed checks)",
			backend.Address, health.consecutiveFailures, hc.config.UnhealthyThreshold)
	}

	if shouldBeAlive != currentlyAlive {
		backend.Alive = shouldBeAlive
		hc.pool.updateBackendStatus(backend.Address, shouldBeAlive)

		if shouldBeAlive {
			logger.Info("Backend {} marked as ALIVE", backend.Address)
		} else {
			logger.Warn("Backend {} marked as DEAD", backend.Address)
		}
	}
}

func (hc *HealthChecker) isBackendHealthy(address string) bool {
	conn, err := net.DialTimeout("tcp", address, hc.config.Timeout)
	if err != nil {
		hc.storeLastError(address, err)
		return false
	}

	conn.Close()
	return true
}

func (hc *HealthChecker) storeLastError(address string, err error) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if health, exists := hc.backendHealth[address]; exists {
		health.lastError = err

		errStr := err.Error()
		switch {
		case strings.Contains(errStr, "connection refused"):
			logger.Debug("Backend {} connection refused (service down)", address)
		case strings.Contains(errStr, "timeout"):
			logger.Debug("Backend {} connection timeout (slow/overloaded)", address)
		case strings.Contains(errStr, "network unreachable"):
			logger.Debug("Backend {} network unreachable", address)
		default:
			logger.Debug("Backend {} connection error: {}", address, err)
		}
	}
}

func (hc *HealthChecker) GetHealthStatus() map[string]*BackendHealth {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	status := make(map[string]*BackendHealth)
	for addr, health := range hc.backendHealth {
		status[addr] = &BackendHealth{
			consecutiveSuccesses: health.consecutiveSuccesses,
			consecutiveFailures:  health.consecutiveFailures,
			lastCheckTime:        health.lastCheckTime,
			lastError:            health.lastError,
		}
	}
	return status
}
