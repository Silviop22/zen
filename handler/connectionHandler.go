package handler

import (
	"io"
	"net"
	"sync"
	"zen/balancer"
	"zen/utils/logger"
)

type ConnectionHandler struct {
	balancer balancer.LoadBalancer
}

func NewConnectionHandler(balancer balancer.LoadBalancer) *ConnectionHandler {
	return &ConnectionHandler{
		balancer: balancer,
	}
}

func (ch *ConnectionHandler) HandleConnection(clientConnection net.Conn) {
	address := clientConnection.RemoteAddr().String()
	logger.Info("New connection from {}", address)

	backend, err := ch.balancer.Next()
	if err != nil {
		logger.Error("Failed to retrieve next available backend: {}", err)
		clientConnection.Write([]byte("ERROR: Could not select backend server\n"))
		clientConnection.Close()
		return
	}

	backendConnection, err := backend.ConnectionPool.Get()
	if err != nil {
		logger.Error("Failed to get backend connection: {}", err)
		clientConnection.Write([]byte("ERROR: Could not connect to backend server\n"))
		clientConnection.Close()
		return
	}

	var waitGroup sync.WaitGroup
	waitGroup.Add(2)

	var clientToBackendErr, backendToClientErr error

	go copyData(backendConnection, clientConnection, &waitGroup, &backendToClientErr)
	go copyData(clientConnection, backendConnection, &waitGroup, &clientToBackendErr)

	waitGroup.Wait()

	//TODO: Mark backends as !alive
	if clientToBackendErr != nil && clientToBackendErr != io.EOF {
		logger.Error("Error copying client to backend for {}: {}", address, clientToBackendErr)
	}
	if backendToClientErr != nil && backendToClientErr != io.EOF {
		logger.Error("Error copying backend to client for {}: {}", address, backendToClientErr)
	}

	logger.Info("Closing connection from {}", address)
	backendConnection.Close()
	clientConnection.Close()
}

func copyData(source net.Conn, target net.Conn, waitGroup *sync.WaitGroup, connectionError *error) {
	defer waitGroup.Done()
	_, *connectionError = io.Copy(source, target)

	if tcpConnection, ok := source.(*net.TCPConn); ok {
		tcpConnection.CloseWrite()
	}
	logger.Debug("Data copy completed from {} to {}", source.RemoteAddr(), target.RemoteAddr())
}
