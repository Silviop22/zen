package balancer

import (
	"errors"
	"sync/atomic"
	"zen/backend"
)

type RoundRobin struct {
	backendPool *backend.Pool
	counter     atomic.Uint64
}

func NewRoundRobin(backendPool *backend.Pool) *RoundRobin {
	return &RoundRobin{
		backendPool: backendPool,
	}
}

func (rr *RoundRobin) Next() (*backend.Backend, error) {
	aliveBackends := rr.backendPool.GetAliveBackends()
	if aliveBackends == nil || len(aliveBackends) == 0 {
		return nil, errors.New("no available backends")
	}

	next := rr.counter.Add(1)

	selectedIndex := int(next % uint64(len(aliveBackends)))

	return aliveBackends[selectedIndex], nil
}
