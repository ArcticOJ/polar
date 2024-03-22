// from bryk.io/net/drpc/middleware/server/logging.go

package middlewares

import (
	"github.com/ArcticOJ/blizzard/v0/logger"
	"storj.io/drpc"
	"storj.io/drpc/drpcmetadata"
	"strings"
	"time"
)

type (
	stream struct {
		drpc.Stream
	}
	loggingMiddleware struct {
		next drpc.Handler
	}
)

func Logging() Middleware {
	return func(next drpc.Handler) drpc.Handler {
		return loggingMiddleware{
			next: next,
		}
	}
}

func (s stream) MsgSend(msg drpc.Message, enc drpc.Encoding) (err error) {
	err = s.Stream.MsgSend(msg, enc)
	logger.Polar.Debug().Interface("data", msg).Err(err).Msg("message sent")
	return
}

func (s stream) MsgRecv(msg drpc.Message, enc drpc.Encoding) (err error) {
	err = s.Stream.MsgRecv(msg, enc)
	logger.Polar.Debug().Interface("data", msg).Err(err).Msg("message received")
	return
}

func (md loggingMiddleware) HandleRPC(_stream drpc.Stream, rpc string) error {
	fields := getFields(_stream, rpc)
	start := time.Now().UTC()
	s := stream{
		Stream: _stream,
	}
	err := md.next.HandleRPC(s, rpc)
	logger.Polar.Debug().
		Dur("duration", time.Now().UTC().Sub(start)).
		Time("start", start).
		Err(err).
		Fields(fields).
		Msg("rpc handled")
	return nil
}

func getFields(stream drpc.Stream, rpc string) (fields map[string]string) {
	if m, ok := drpcmetadata.Get(stream.Context()); ok {
		fields = m
	} else {
		fields = make(map[string]string)
	}
	segments := strings.Split(rpc, "/")
	if len(segments) == 3 {
		fields["rpc.system"] = segments[0]
		fields["rpc.service"] = segments[1]
		fields["rpc.method"] = segments[2]
	}
	return fields
}
