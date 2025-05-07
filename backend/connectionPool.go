package backend

import (
	"errors"
	"io"
	"net"
	"sync"
	"time"
	"zen/utils/logger"
)

var (
	ErrPoolClosed      = errors.New("connection pool is closed")
	ErrPoolExhausted   = errors.New("connection pool exhausted")
	ErrConnectionError = errors.New("connection error")
	ErrContextCanceled = errors.New("operation canceled by context")
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
		return nil, net.ErrClosed
	}

	if n := len(cp.idleConns); n > 0 {
		conn := cp.idleConns[n-1].conn
		cp.idleConns = cp.idleConns[:n-1]
		return &PooledConnection{conn: conn, pool: cp}, nil
	}

	if cp.activeCount >= cp.config.maxActive {
		logger.Warn("Max active connections reached: {}. Waiting for an idle connection.", cp.config.maxActive)
		return nil, errors.New("connection pool exhausted")
	}

	address := cp.config.address
	conn, err := net.Dial("tcp", address)
	if err != nil {
		logger.Error("Failed to establish connection with backend server: {}", address, err)
		return nil, err
	}

	cp.activeCount++
	logger.Info("New connection established with backend server: {}", address)
	return &PooledConnection{conn: conn, pool: cp}, nil
}

func (cp *ConnectionPool) put(conn net.Conn) {
	cp.mu.Lock()
	defer cp.mu.Unlock()

	if cp.closed {
		conn.Close()
		return
	}

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		err := cp.validateConnection(tcpConn, conn)
		if err != nil {
			logger.Error("Failed to validate connection: {}", err)
		}
	}

	if len(cp.idleConns) >= cp.config.maxIdle {
		conn.Close()
		return
	}

	cp.idleConns = append(cp.idleConns, &PoolConn{
		conn:       conn,
		lastUsedAt: time.Now(),
	})
	cp.activeCount--

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

func (cp *ConnectionPool) validateConnection(tcpConn *net.TCPConn, conn net.Conn) error {
	if err := tcpConn.SetReadDeadline(time.Now().Add(time.Millisecond)); err != nil {
		conn.Close()
		return nil
	}

	one := make([]byte, 1)
	if _, err := tcpConn.Read(one); err != io.EOF {
		// Either there's data (not what we want) or an error
		conn.Close()
		cp.activeCount--
		return nil
	}

	return tcpConn.SetReadDeadline(time.Time{})
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
			logger.Debug("Closing idle connection: {}", idleConn.conn.RemoteAddr())
			idleConn.conn.Close()
		} else {
			remainingIdleConnections = append(remainingIdleConnections, idleConn)
		}
	}

	cp.idleConns = remainingIdleConnections
}
