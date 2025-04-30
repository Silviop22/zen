package balancer

import (
	"zen/backend/model"
)

type LoadBalancer interface {
	Next() (*model.Backend, error)
}
