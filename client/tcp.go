package polar

import (
	"context"
	"fmt"
	"net"
)

func newConn(ctx context.Context, host string, port uint16) (c net.Conn, e error) {
	d := net.Dialer{}
	c, e = d.DialContext(ctx, "tcp", net.JoinHostPort(host, fmt.Sprint(port)))
	return
}
