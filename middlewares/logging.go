package middlewares

import (
	"github.com/rs/zerolog"
	"storj.io/drpc"
	"storj.io/drpc/drpcmetadata"
	"strings"
	"time"
)

type (
	stream struct {
		drpc.Stream
		logger *zerolog.Logger
	}
	loggingMiddleware struct {
		next   drpc.Handler
		logger *zerolog.Logger
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
	s.logger.Debug().Interface("data", msg).Err(err).Msg("message sent")
	return
}

func (s stream) MsgRecv(msg drpc.Message, enc drpc.Encoding) (err error) {
	err = s.Stream.MsgRecv(msg, enc)
	s.logger.Debug().Interface("data", msg).Err(err).Msg("message received")
	return
}

func (mw loggingMiddleware) HandleRPC(_stream drpc.Stream, rpc string) error {
	fields := getFields(_stream, rpc)
	start := time.Now().UTC()
	s := stream{
		Stream: _stream,
		logger: mw.logger,
	}
	err := mw.next.HandleRPC(s, rpc)
	mw.logger.Debug().
		Stringer("duration", time.Now().UTC().Sub(start)).
		Stringer("start", start).
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
	if len(segments) >= 2 {
		fields["rpc.service"] = segments[0]
		fields["rpc.method"] = segments[1]
	}
	return fields
}
