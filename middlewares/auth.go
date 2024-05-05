package middlewares

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"github.com/ArcticOJ/polar/v0/shared"
	"storj.io/drpc"
	"storj.io/drpc/drpcmetadata"
)

type authMiddleware struct {
	hash string
	next drpc.Handler
}

// TODO: refactor this cringe and trivial authentication mechanism as it literally does nothing lol.

func AuthMiddleware(secret string) Middleware {
	hash := md5.Sum([]byte(secret))
	return func(next drpc.Handler) drpc.Handler {
		return authMiddleware{
			hash: hex.EncodeToString(hash[:]),
			next: next,
		}
	}
}

func (a authMiddleware) HandleRPC(stream drpc.Stream, rpc string) error {
	data, ok := drpcmetadata.Get(stream.Context())
	if !ok {
		return errors.New("invalid metadata")
	}
	hash, ok := data[shared.SecretHashMetadataKey]
	if !ok {
		return errors.New("secret hash not found")
	}
	if hash != a.hash {
		return errors.New("invalid secret hash")
	}
	return a.next.HandleRPC(stream, rpc)
}
