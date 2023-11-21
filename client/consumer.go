package polar

import (
	"bufio"
	"github.com/ArcticOJ/polar/v0/types"
	"github.com/vmihailenco/msgpack"
	"io"
	"net"
)

type Consumer struct {
	runtime     string
	conn        net.Conn
	enc         *msgpack.Encoder
	MessageChan chan types.Submission
}

func (p *Polar) NewConsumer() (c *Consumer, e error) {
	c = &Consumer{
		MessageChan: make(chan types.Submission, 1),
	}
	c.conn, e = p.consumerSession.Open()
	if e != nil {
		return
	}
	c.enc = msgpack.NewEncoder(c.conn)
	return
}

func (c *Consumer) Consume() error {
	defer c.Close()
	s := bufio.NewScanner(c.conn)
	for {
		if e := c.send(types.CommandConsume, nil); e != nil {
			return e
		}
		if !s.Scan() {
			return io.EOF
		}
		var sub types.Submission
		buf := s.Bytes()
		if msgpack.Unmarshal(buf, &sub) != nil {
			continue
		}
		c.MessageChan <- sub
	}
}

func (c *Consumer) Reject(id uint32) error {
	return c.send(types.CommandReject, id)
}

func (c *Consumer) send(command types.Command, args interface{}) (e error) {
	if e = c.enc.Encode(types.Request{Command: command, Args: args}); e != nil {
		return
	}
	// terminate message with newline
	_, e = c.conn.Write([]byte("\n"))
	return
}

func (c *Consumer) Close() {
	close(c.MessageChan)
	c.conn.Close()
}
