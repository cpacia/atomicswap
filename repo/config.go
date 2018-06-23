package repo

import (
	iaddr "github.com/ipfs/go-ipfs-addr"
	pstore "github.com/libp2p/go-libp2p-peerstore"
)

var defaultBoostrapPeers = []string{
	"/ip4/127.0.0.1/tcp/9000/ipfs/12D3KooWGqzvbKZJVhRRCv7mJ4KvKQ9TESLemSgDopzoL1YtdhsE",
	"/ip4/127.0.0.1/tcp/9001/ipfs/12D3KooWNWb7URKbZwM6ZZGzjWYHGS97ofd5A8LoBw78jpmzzzN2",
	"/ip4/127.0.0.1/tcp/9002/ipfs/12D3KooWFkzX4DRP9iMDEd4TrQfuNcrMXu8KzXk3NqQM9SgaLSKg",
}

func ParseBootstrapPeer(addr string) (iaddr.IPFSAddr, error) {
	ia, err := iaddr.ParseString(addr)
	if err != nil {
		return nil, err
	}
	return ia, err
}

func ParseBootstrapPeers(addrs []string) ([]pstore.PeerInfo, error) {
	var peers []pstore.PeerInfo
	for _, addr := range addrs {
		ia, err := ParseBootstrapPeer(addr)
		if err != nil {
			return nil, err
		}
		pi, err := pstore.InfoFromP2pAddr(ia.Multiaddr())
		peers = append(peers, *pi)
	}
	return peers, nil
}
