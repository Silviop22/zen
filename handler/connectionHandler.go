package handler

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
	"zen/backend"
	"zen/balancer"
	"zen/utils/logger"
)

type ConnectionHandler struct {
	balancer         balancer.LoadBalancer
	maxRetries       int
	retryDelay       time.Duration
	connectTimeout   time.Duration
	requestTimeout   time.Duration
	handshakeTimeout time.Duration
	proxyIdleTimeout time.Duration
}

func NewConnectionHandler(balancer balancer.LoadBalancer) *ConnectionHandler {
	return &ConnectionHandler{
		balancer:         balancer,
		maxRetries:       3,
		retryDelay:       10 * time.Millisecond,
		connectTimeout:   2 * time.Second,
		requestTimeout:   10 * time.Second,
		handshakeTimeout: 5 * time.Second,
		proxyIdleTimeout: 300 * time.Second,
	}
}

func (ch *ConnectionHandler) HandleConnection(clientConnection net.Conn) {
	address := clientConnection.RemoteAddr().String()
	logger.Info("New connection from %s", address)

	ctx, cancel := context.WithTimeout(context.Background(), ch.requestTimeout)
	defer cancel()

	// This prevents clients from holding connections without sending data
	clientConnection.SetReadDeadline(time.Now().Add(ch.handshakeTimeout))

	backendConnection, selectedBackend, err := ch.getBackendConnectionWithRetry(ctx)
	if err != nil {
		logger.Error("Failed to establish connection to any backend for %s: %s", address, err)
		ch.sendErrorResponse(clientConnection, "Service temporarily unavailable")
		clientConnection.Close()
		return
	}

	logger.Info("Successfully connected to backend %s for client %s", selectedBackend.Address, address)

	ch.setProxyTimeouts(clientConnection, backendConnection)

	var waitGroup sync.WaitGroup
	waitGroup.Add(2)

	var clientToBackendErr, backendToClientErr error

	go copyData(backendConnection, clientConnection, &waitGroup, &backendToClientErr)
	go copyData(clientConnection, backendConnection, &waitGroup, &clientToBackendErr)

	waitGroup.Wait()

	if clientToBackendErr != nil && clientToBackendErr != io.EOF {
		logger.Debug("Error copying client to backend for %s: %s", address, clientToBackendErr)
	}
	if backendToClientErr != nil && backendToClientErr != io.EOF {
		logger.Debug("Error copying backend to client for %s: %s", address, backendToClientErr)
	}

	logger.Debug("Closing connection from %s", address)
	backendConnection.Close()
	clientConnection.Close()
}

func (ch *ConnectionHandler) getBackendConnectionWithRetry(ctx context.Context) (net.Conn, *backend.Backend, error) {
	var lastErr error
	triedBackends := make(map[string]bool)

	for attempt := 1; attempt <= ch.maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return nil, nil, fmt.Errorf("request timeout after %d attempts", attempt-1)
		default:
		}

		backendServer, err := ch.balancer.Next()
		if err != nil {
			lastErr = err
			logger.Debug("Attempt %d: No available backends: %s", attempt, err)
			if attempt < ch.maxRetries {
				ch.sleepWithContext(ctx, ch.retryDelay)
			}
			continue
		}

		if triedBackends[backendServer.Address] {
			logger.Debug("Attempt %d: Skipping already tried backend %s", attempt, backendServer.Address)

			availableCount := ch.balancer.GetAvailableCount()
			if len(triedBackends) >= availableCount {
				logger.Debug("All %d available backends have been tried", availableCount)
				break
			}

			if attempt < ch.maxRetries {
				ch.sleepWithContext(ctx, ch.retryDelay)
			}
			continue
		}

		triedBackends[backendServer.Address] = true

		logger.Debug("Attempt %d: Trying backend %s", attempt, backendServer.Address)

		conn, err := ch.getConnectionWithContext(ctx, backendServer)
		if err != nil {
			lastErr = err
			logger.Debug("Attempt %d: Failed to connect to backend %s: %s", attempt, backendServer.Address, err)

			if attempt < ch.maxRetries {
				ch.sleepWithContext(ctx, ch.retryDelay)
			}
			continue
		}

		logger.Debug("Attempt %d: Successfully connected to backend %s", attempt, backendServer.Address)
		return conn, backendServer, nil
	}

	return nil, nil, fmt.Errorf("all backends failed after %d attempts: %w", ch.maxRetries, lastErr)
}

func (ch *ConnectionHandler) getConnectionWithContext(ctx context.Context, backend *backend.Backend) (net.Conn, error) {
	type connResult struct {
		conn net.Conn
		err  error
	}

	resultChan := make(chan connResult, 1)

	go func() {
		conn, err := backend.ConnectionPool.Get()
		select {
		case resultChan <- connResult{conn: conn, err: err}:
		case <-ctx.Done():
			if conn != nil {
				conn.Close()
			}
		}
	}()

	timeout := ch.connectTimeout
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining < timeout {
			timeout = remaining
		}
	}

	select {
	case result := <-resultChan:
		return result.conn, result.err
	case <-time.After(timeout):
		return nil, fmt.Errorf("backend connection timeout (%v)", timeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (ch *ConnectionHandler) sleepWithContext(ctx context.Context, duration time.Duration) {
	select {
	case <-time.After(duration):
	case <-ctx.Done():
	}
}

func copyData(source net.Conn, target net.Conn, waitGroup *sync.WaitGroup, connectionError *error) {
	defer waitGroup.Done()

	buffer := make([]byte, 32*1024)

	for {
		source.SetReadDeadline(time.Now().Add(300 * time.Second))

		n, err := source.Read(buffer)
		if err != nil {
			*connectionError = err
			break
		}

		if n > 0 {
			target.SetWriteDeadline(time.Now().Add(30 * time.Second))

			_, writeErr := target.Write(buffer[:n])
			if writeErr != nil {
				*connectionError = writeErr
				break
			}
		}
	}

	if tcpConnection, ok := target.(*net.TCPConn); ok {
		tcpConnection.CloseWrite()
	}
}

func (ch *ConnectionHandler) getAvailableBackendCount() int {
	return ch.balancer.GetAvailableCount()
}

func (ch *ConnectionHandler) sendErrorResponse(conn net.Conn, message string) {
	errorMsg := fmt.Sprintf("HTTP/1.1 503 Service Unavailable\r\n"+
		"Content-Type: text/plain\r\n"+
		"Content-Length: %d\r\n"+
		"Connection: close\r\n\r\n"+
		"%s", len(message), message)

	conn.Write([]byte(errorMsg))
}

func (ch *ConnectionHandler) setProxyTimeouts(clientConn, backendConn net.Conn) {
	clientConn.SetDeadline(time.Time{})
	backendConn.SetDeadline(time.Time{})

	idleDeadline := time.Now().Add(ch.proxyIdleTimeout)

	clientConn.SetReadDeadline(idleDeadline)
	backendConn.SetReadDeadline(idleDeadline)
}
