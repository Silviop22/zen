package model

import "zen/backend"

type Backend struct {
	Address        string
	ConnectionPool *backend.ConnectionPool
	Alive          bool
}
