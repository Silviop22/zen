package backend

import (
	"errors"
	"net"
	"sync"
	"time"
	"zen/utils/logger"
)

var (
	ErrPoolClosed    = errors.New("connection pool is closed")
	ErrPoolExhausted = errors.New("connection pool exhausted")
)

type ConnectionPool struct {
	config      *ConnectionPoolConfig
	mu          sync.Mutex
	idleConns   []*PoolConn
	activeCount int
	closed      bool
}

type ConnectionPoolConfig struct {
	address        string
	maxIdle        int
	maxActive      int
	idleTimeout    time.Duration
	connectTimeout time.Duration
}

type PoolConn struct {
	conn       net.Conn
	lastUsedAt time.Time
}

func NewConnectionPool(address string, maxIdle, maxActive int, idleTimeout time.Duration) *ConnectionPool {
	config := newConfig(address, maxIdle, maxActive, idleTimeout)
	pool := &ConnectionPool{
		config:    config,
		idleConns: make([]*PoolConn, 0, maxIdle),
	}

	go pool.periodicCleanup()

	return pool
}

func newConfig(address string, maxIdle, maxActive int, idleTimeout time.Duration) *ConnectionPoolConfig {
	return &ConnectionPoolConfig{
		address:        address,
		maxIdle:        maxIdle,
		maxActive:      maxActive,
		idleTimeout:    idleTimeout,
		connectTimeout: 5 * time.Second,
	}
}

func (cp *ConnectionPool) Get() (net.Conn, error) {
	logger.Debug("Attempting to get a connection from the pool.")
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.closed {
		return nil, ErrPoolClosed
	}

	for len(cp.idleConns) > 0 {
		n := len(cp.idleConns) - 1
		poolConn := cp.idleConns[n]
		cp.idleConns = cp.idleConns[:n]

		logger.Debug("Reusing idle connection to %s", poolConn.conn.RemoteAddr())
		return &PooledConnection{conn: poolConn.conn, pool: cp}, nil
	}

	if cp.activeCount >= cp.config.maxActive {
		logger.Warn("Max active connections reached: %d. Pool exhausted.", cp.config.maxActive)
		return nil, ErrPoolExhausted
	}

	address := cp.config.address
	conn, err := net.DialTimeout("tcp", address, cp.config.connectTimeout)
	if err != nil {
		logger.Error("Failed to establish connection with backend server: %s - %v", address, err)
		return nil, err
	}

	cp.activeCount++
	logger.Info("New connection established with backend server: %s", address)
	return &PooledConnection{conn: conn, pool: cp}, nil
}

func (cp *ConnectionPool) put(conn net.Conn) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.closed {
		conn.Close()
		return
	}

	if len(cp.idleConns) >= cp.config.maxIdle {
		conn.Close()
		cp.activeCount--
		return
	}

	cp.idleConns = append(cp.idleConns, &PoolConn{
		conn:       conn,
		lastUsedAt: time.Now(),
	})
}

func (cp *ConnectionPool) Close() {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	cp.closed = true

	for _, idleConn := range cp.idleConns {
		idleConn.conn.Close()
	}

	cp.idleConns = nil
}

func (cp *ConnectionPool) periodicCleanup() {
	ticker := time.NewTicker(cp.config.idleTimeout / 2)
	defer ticker.Stop()

	for range ticker.C {
		cp.cleanup()
	}
}

func (cp *ConnectionPool) cleanup() {
	logger.Debug("Running periodic cleanup of idle connections.")
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.closed {
		return
	}

	now := time.Now()
	remainingIdleConnections := make([]*PoolConn, 0, len(cp.idleConns))

	for _, idleConn := range cp.idleConns {
		if now.Sub(idleConn.lastUsedAt) > cp.config.idleTimeout {
			logger.Debug("Closing idle connection: %s", idleConn.conn.RemoteAddr())
			idleConn.conn.Close()
			cp.activeCount--
		} else {
			remainingIdleConnections = append(remainingIdleConnections, idleConn)
		}
	}

	cp.idleConns = remainingIdleConnections
}
