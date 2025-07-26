package backend

import "sync/atomic"

type Backend struct {
	Address        string
	ConnectionPool *ConnectionPool
	alive          atomic.Bool
}

func (b *Backend) IsAlive() bool {
	return b.alive.Load()
}

func (b *Backend) SetAlive(alive bool) {
	b.alive.Store(alive)
}

func (b *Backend) CompareAndSetAlive(oldValue, newValue bool) bool {
	return b.alive.CompareAndSwap(oldValue, newValue)
}

func NewBackend(address string) *Backend {
	connPool := NewConnectionPool(address, 10, 100, 30)
	backend := &Backend{
		Address:        address,
		ConnectionPool: connPool,
	}
	backend.alive.Store(true) // Start as alive
	return backend
}
