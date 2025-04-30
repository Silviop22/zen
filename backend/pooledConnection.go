package backend

import (
	"net"
	"sync"
	"time"
)

type PooledConnection struct {
	conn net.Conn
	pool *ConnectionPool
	once sync.Once
}

func (pc *PooledConnection) Read(b []byte) (int, error)         { return pc.conn.Read(b) }
func (pc *PooledConnection) Write(b []byte) (int, error)        { return pc.conn.Write(b) }
func (pc *PooledConnection) LocalAddr() net.Addr                { return pc.conn.LocalAddr() }
func (pc *PooledConnection) RemoteAddr() net.Addr               { return pc.conn.RemoteAddr() }
func (pc *PooledConnection) SetDeadline(t time.Time) error      { return pc.conn.SetDeadline(t) }
func (pc *PooledConnection) SetReadDeadline(t time.Time) error  { return pc.conn.SetReadDeadline(t) }
func (pc *PooledConnection) SetWriteDeadline(t time.Time) error { return pc.conn.SetWriteDeadline(t) }
func (pc *PooledConnection) Close() error {
	pc.once.Do(func() {
		pc.pool.put(pc.conn)
	})
	return nil
}
