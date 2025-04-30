package backend

import (
	"sync/atomic"
)

type Pool struct {
	aliveBackends atomic.Value
}

func NewBackendPool(backends []string) *Pool {
	bps := make([]*Backend, 0)
	for _, addr := range backends {
		connPool := NewConnectionPool(addr, 10, 100, 30)
		bps = append(bps, &Backend{
			Address:        addr,
			ConnectionPool: connPool,
			Alive:          true})
	}
	value := atomic.Value{}
	value.Store(bps)

	return &Pool{
		aliveBackends: value,
	}
}

func (backendPool *Pool) GetAliveBackends() []*Backend {
	return backendPool.aliveBackends.Load().([]*Backend)
}
