package shared

import (
	"bufio"
	"github.com/vmihailenco/msgpack"
	"net"
)

type (
	EncodedConn struct {
		conn     net.Conn
		_writer  *msgpack.Encoder
		_scanner *bufio.Scanner
	}
)

func NewEncodedConn(conn net.Conn) *EncodedConn {
	return &EncodedConn{
		conn:     conn,
		_scanner: bufio.NewScanner(conn),
		_writer:  msgpack.NewEncoder(conn),
	}
}

func (c *EncodedConn) Write(obj interface{}) (e error) {
	if e = c._writer.Encode(obj); e != nil {
		return e
	}
	// terminate payload with newline
	_, e = c.conn.Write([]byte("\n"))
	return
}

func (c *EncodedConn) Scan() bool {
	return c._scanner.Scan()
}

func (c *EncodedConn) Read(obj interface{}) error {
	return msgpack.Unmarshal(c._scanner.Bytes(), &obj)
}

func (c *EncodedConn) Close() error {
	return c.conn.Close()
}

func (c *EncodedConn) Conn() net.Conn {
	return c.conn
}
