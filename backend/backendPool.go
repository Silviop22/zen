package backend

import (
	"sync"
	"sync/atomic"
	"zen/utils/logger"
)

type Pool struct {
	allBackends   []*Backend   // All backends (both alive and dead)
	aliveBackends atomic.Value // Only alive backends
	mu            sync.RWMutex // Protects allBackends slice
}

func NewBackendPool(backends []string) *Pool {
	allBps := make([]*Backend, 0, len(backends))
	aliveBps := make([]*Backend, 0, len(backends))

	for _, addr := range backends {
		connPool := NewConnectionPool(addr, 10, 100, 30)
		backend := &Backend{
			Address:        addr,
			ConnectionPool: connPool,
			Alive:          true,
		}
		allBps = append(allBps, backend)
		aliveBps = append(aliveBps, backend)
	}

	aliveValue := atomic.Value{}
	aliveValue.Store(aliveBps)

	pool := &Pool{
		allBackends:   allBps,
		aliveBackends: aliveValue,
	}

	logger.Info("Backend pool created with {} backends", len(allBps))
	return pool
}

func (pool *Pool) GetAliveBackends() []*Backend {
	return pool.aliveBackends.Load().([]*Backend)
}

func (pool *Pool) GetAllBackends() []*Backend {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	backends := make([]*Backend, len(pool.allBackends))
	copy(backends, pool.allBackends)
	return backends
}

func (pool *Pool) updateBackendStatus(address string, alive bool) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	for _, backend := range pool.allBackends {
		if backend.Address == address {
			backend.Alive = alive
			break
		}
	}

	aliveBackends := make([]*Backend, 0, len(pool.allBackends))
	for _, backend := range pool.allBackends {
		if backend.Alive {
			aliveBackends = append(aliveBackends, backend)
		}
	}

	pool.aliveBackends.Store(aliveBackends)

	logger.Info("Backend pool updated: {}/{} backends alive",
		len(aliveBackends), len(pool.allBackends))
}

func (pool *Pool) GetBackendCount() (total int, alive int) {
	pool.mu.RLock()
	total = len(pool.allBackends)
	pool.mu.RUnlock()

	aliveBackends := pool.GetAliveBackends()
	alive = len(aliveBackends)

	return total, alive
}

func (pool *Pool) Close() {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	for _, backend := range pool.allBackends {
		backend.ConnectionPool.Close()
	}

	logger.Info("Backend pool closed")
}
