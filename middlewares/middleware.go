package middlewares

import "storj.io/drpc"

// Huge thanks to bryk.io for such cool RPC extensions :D

type Middleware = func(next drpc.Handler) drpc.Handler
