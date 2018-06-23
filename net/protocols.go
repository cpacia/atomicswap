package net

import "github.com/libp2p/go-libp2p-protocol"

// These protocol IDs are used to route messages. We could use the defaults but then other apps which use the defaults
// can connect to us even if we can't handle their messages. So we'll use unique protocol strings to make sure we are
// segregated from other apps.
const (
	SwapProtocolID = protocol.ID("/atomicswap/1.0.0")
	FloodSubID     = protocol.ID("/atomicfloodsub/1.0.0")
)
