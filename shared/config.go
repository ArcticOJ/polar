package shared

import (
	"github.com/hashicorp/yamux"
	"time"
)

func MuxConfig() *yamux.Config {
	conf := yamux.DefaultConfig()
	conf.KeepAliveInterval = time.Second
	conf.MaxStreamWindowSize = 2 << 20
	return conf
}
