package cmd

import (
	"context"
	"errors"
	"github.com/cpacia/atomicswap/net"
	"github.com/cpacia/atomicswap/repo"
	"github.com/libp2p/go-floodsub"
	"github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-kad-dht/opts"
	"github.com/libp2p/go-libp2p-protocol"
	"github.com/libp2p/go-libp2p-record"
	"github.com/op/go-logging"
	"os"
	"sync"
)

var stdoutLogFormat = logging.MustStringFormatter(
	`%{color:reset}%{color}%{time:15:04:05.000} [%{shortfunc}] [%{level}] %{message}`,
)

var log = logging.MustGetLogger("cmd")

type Start struct {
	DataDir string `short:"d" long:"datadir" description:"specify the data directory to be used"`
	Port    int    `short:"p" long:"port" description:"the port to use" default:"0"`
}

// The start command will start up our atomic swap node, connect to the p2p network, and download the order book, and initialize the API
func (x *Start) Execute(args []string) error {
	// First create our repo which is where we'll store or app related data
	// This will also create and save our node's identity private key if it does not yet exist
	r, err := repo.NewRepo(x.DataDir)
	if err != nil {
		return err
	}

	// Force the user to select a port. We could just use the default but given that this is primarily
	// a demo we'll likely be running multiple nodes on localhost.
	if x.Port == 0 {
		return errors.New("You must specify a port when starting up. Use the -p flag.")
	}

	// Set up logging
	backendStdout := logging.NewLogBackend(os.Stdout, "", 0)
	backendStdoutFormatter := logging.NewBackendFormatter(backendStdout, stdoutLogFormat)
	logging.SetBackend(backendStdoutFormatter)

	// Build our host. This is the core of libp2p. We're going to initialize it with with the default
	// transports, muxers, security, and peerstore.
	peerHost, err := net.NewPeerHost(x.Port, r)
	if err != nil {
		return err
	}

	// Create our pubsub implementation. Floodsub is a flooding pubsub system that we'll use for our orderbook.
	_, err = floodsub.NewFloodsubWithProtocols(context.Background(), peerHost, []protocol.ID{net.FloodSubID})
	if err != nil {
		return err
	}

	// Next we have to set up DHT routing. Let's start by configuring a pubkey validator
	validator := record.NamespacedValidator{
		"pk": record.PublicKeyValidator{},
	}

	// Create the dht instance. It needs the host and a datastore instance
	routing, err := dht.New(
		context.Background(), peerHost,
		dhtopts.Datastore(r.Datastore()),
		dhtopts.Validator(validator),
	)

	// Finally let's bootstrap everything and get us up and running
	err = net.Bootstrap(routing, peerHost, net.BootstrapConfigWithPeers(r.BootstrapPeers()))

	if err != nil {
		return err
	}

	// Now we're listening let's just hang out here.
	log.Infof("Listening on %s, peerID: %s\n", peerHost.Addrs()[0], peerHost.ID().Pretty())
	var wg sync.WaitGroup
	wg.Add(1)
	wg.Wait()

	return nil
}
