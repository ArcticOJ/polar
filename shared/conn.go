package shared

import (
	"encoding/json"
	"net"
)

type (
	EncodedConn struct {
		conn     net.Conn
		_encoder *json.Encoder
		_decoder *json.Decoder
	}
)

func NewEncodedConn(conn net.Conn) (c *EncodedConn) {
	c = &EncodedConn{
		conn:     conn,
		_decoder: json.NewDecoder(conn),
		_encoder: json.NewEncoder(conn),
	}
	c._decoder.UseNumber()
	return
}

func (c *EncodedConn) Write(obj interface{}) (e error) {
	return c._encoder.Encode(obj)
}

func (c *EncodedConn) More() bool {
	return c._decoder.More()
}

func (c *EncodedConn) Read(obj interface{}) error {
	return c._decoder.Decode(&obj)
}

func (c *EncodedConn) Close() error {
	return c.conn.Close()
}

func (c *EncodedConn) Conn() net.Conn {
	return c.conn
}
