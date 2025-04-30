package balancer

import (
	"zen/backend"
)

type LoadBalancer interface {
	Next() (*backend.Backend, error)
}
