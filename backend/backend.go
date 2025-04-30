package backend

type Backend struct {
	Address        string
	ConnectionPool *ConnectionPool
	Alive          bool
}
