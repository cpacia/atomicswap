package net

import (
	"context"
	"fmt"
	"github.com/cpacia/atomicswap/repo"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-host"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("net")

func NewPeerHost(port int, repo *repo.Repo) (host.Host, error) {
	privKey := repo.PrivKey()

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port)),
		libp2p.Identity(privKey),
	}

	// This function will initialize a new libp2p host with our options plus a bunch of default options
	// The default options includes default transports, muxers, security, and peer store.
	host, err := libp2p.New(context.Background(), opts...)
	if err != nil {
		return nil, err
	}
	return host, nil
}
