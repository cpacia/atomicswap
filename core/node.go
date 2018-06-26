package core

import (
	"context"
	"crypto/sha256"
	"github.com/cpacia/atomicswap/pb"
	r "github.com/cpacia/atomicswap/repo"
	ob "github.com/cpacia/atomicswap/orderbook"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/ipfs/go-cid"
	fs "github.com/libp2p/go-floodsub"
	"github.com/libp2p/go-libp2p-host"
	"github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p-peer"
	pstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/multiformats/go-multihash"
	"github.com/op/go-logging"
	"io"
	"sync"
	"time"
	"github.com/cpacia/atomicswap/net/service"
)

var (
	log   = logging.MustGetLogger("cmd")
	Topic *cid.Cid
)

const (
	ReSubscribeInterval     = time.Hour
	ReconnectInterval       = time.Minute
	MinConnectedSubscribers = 2
)

func init() {
	h := sha256.Sum256([]byte("floodsub:OrderBook"))
	enc, err := multihash.Encode(h[:], multihash.SHA2_256)
	if err != nil {
		panic(err)
	}
	mh, err := multihash.Cast(enc)
	if err != nil {
		panic(err)
	}
	Topic = cid.NewCidV1(cid.Raw, mh)
}

type addPeer struct {
	peerID peer.ID
}

type removePeer struct {
	peerID peer.ID
}

type newOrder struct {
	serializedMessage []byte
}

type closeOrder struct {
	serializedMessage []byte
}

// This struct contains the relevant components of our node that we'll need
// to run our atomic swap protocol.
type AtomicSwapNode struct {
	repo          *r.Repo
	peerHost      host.Host
	routing       *dht.IpfsDHT
	floodsub      *fs.PubSub
	msgChan       chan interface{}
	connectedSubs map[peer.ID]bool
	orderBook     *ob.OrderBook
	wireService   *service.WireService
}

func NewAtomicSwapNode(repo *r.Repo, peerHost host.Host, routing *dht.IpfsDHT, floodsub *fs.PubSub) *AtomicSwapNode {
	return &AtomicSwapNode{
		repo:          repo,
		peerHost:      peerHost,
		routing:       routing,
		floodsub:      floodsub,
		msgChan:       make(chan interface{}),
		connectedSubs: make(map[peer.ID]bool),
		orderBook:     ob.NewOrderBook(),
	}
}

func (n *AtomicSwapNode) MsgChan() chan interface{} {
	return n.msgChan
}

func (n *AtomicSwapNode) SetWireService(ws *service.WireService) {
	n.wireService = ws
}

// Here we are going to set self as a subscriber in the dht and query the dht for other
// subscribers and open connections to a few of them.
func (n *AtomicSwapNode) StartOnlineServices() {
	go n.subscribeTopic()
	go n.connectToSubscribers()
	go n.messageHandler()
}

// This is the main loop which handles adding and removing of peers and adding and removing
// orders from the order book. We'll route all everything that should be processed sequentially
// through here.
func (n *AtomicSwapNode) messageHandler() {
	for {
		select {
		case m := <-n.msgChan:
			switch msg := m.(type) {
			case addPeer:
				n.connectedSubs[msg.peerID] = true
			case removePeer:
				if _, ok := n.connectedSubs[msg.peerID]; ok {
					log.Infof("Lost subscriber peer %s", msg.peerID.Pretty())
					delete(n.connectedSubs, msg.peerID)
				}
			case newOrder:
				n.orderBook.ProcessNewLimitOrder(msg.serializedMessage)
			case closeOrder:
				n.orderBook.ProcessCloseOrder(msg.serializedMessage)
			}
		}
	}
}

func (n *AtomicSwapNode) PublishLimitOrder(quantity, price uint64, buyBTC bool) error {
	ts, err := ptypes.TimestampProto(time.Now().Add(time.Hour * 24 * 30))
	if err != nil {
		return err
	}
	lopb := &pb.LimitOrder{
		PeerID:   n.peerHost.ID().Pretty(),
		Expiry:   ts,
		Quantity: quantity,
		Price:    price,
		BuyBTC:   buyBTC,
	}

	// TODO: add signed UTXO to limit order

	ser, err := proto.Marshal(lopb)
	if err != nil {
		return err
	}
	privKey := n.repo.PrivKey()
	signature, err := privKey.Sign(ser)
	if err != nil {
		return err
	}
	signed := &pb.SignedLimitOrder{
		SerializedLimitOrder: ser,
		Signature:            signature,
	}
	serializedWithSig, err := proto.Marshal(signed)
	if err != nil {
		return err
	}
	n.msgChan <- newOrder{serializedMessage: serializedWithSig}
	any, err := ptypes.MarshalAny(signed)
	if err != nil {
		return err
	}
	m := &pb.Message{
		MessageType: pb.Message_LimitOrder,
		Payload:     any,
	}
	serializedMessage, err := proto.Marshal(m)
	if err != nil {
		return err
	}
	return n.floodsub.Publish("OrderBook", serializedMessage)
}

func (n *AtomicSwapNode) subscribeTopic() {
	go n.setSelfAsSubscriber()

	sub, err := n.floodsub.Subscribe("OrderBook")
	if err != nil {
		log.Error(err)
		return
	}
	for {
		msg, err := sub.Next(context.Background())
		if err == io.EOF || err == context.Canceled {
			return
		} else if err != nil {
			log.Error(err)
			return
		}
		mpb := new(pb.Message)
		err = proto.Unmarshal(msg.Data, mpb)
		if err != nil {
			log.Error(err)
			return
		}
		switch mpb.MessageType {
		case pb.Message_LimitOrder:
			n.msgChan <- newOrder{serializedMessage: mpb.Payload.Value}
		case pb.Message_OrderClose:
			n.msgChan <- closeOrder{serializedMessage: mpb.Payload.Value}
		}

	}
}

// This will append us (our peerID and IP addrs) at the "topic" key in the dht.
// This makes us a subscriber and lets others know we are subscribed to this topic.
func (n *AtomicSwapNode) setSelfAsSubscriber() {
	subscribe := func() {
		err := n.routing.Provide(context.Background(), Topic, true)
		if err != nil {
			log.Error(err)
		}
	}
	subscribe() // Run once to start

	// We'll do this repeatedly to make sure we stay subscribed and our subscription does not expire
	ticker := time.NewTicker(ReSubscribeInterval)
	for {
		select {
		case <-ticker.C:
			subscribe()
		}
	}
}

// This will query the dht for other subscribers and connect to a few of them
func (n *AtomicSwapNode) connectToSubscribers() {
	n.connectionRound()
	ticker := time.NewTicker(ReconnectInterval)
	for {
		select {
		case <-ticker.C:
			for peer := range n.connectedSubs {
				conns:= n.peerHost.Network().ConnsToPeer(peer)
				if len(conns) == 0 {
					n.msgChan <- removePeer{peer}
				}
			}
			if len(n.connectedSubs) < MinConnectedSubscribers {
				n.connectionRound()
			}
		}
	}
}

func (n *AtomicSwapNode) connectionRound() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	provs := n.routing.FindProvidersAsync(ctx, Topic, 10)
	wg := &sync.WaitGroup{}
	for p := range provs {
		wg.Add(1)
		go func(pi pstore.PeerInfo) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(ctx, time.Second*10)
			defer cancel()
			err := n.peerHost.Connect(ctx, pi)
			if err != nil {
				log.Debug("pubsub discover: ", err)
				return
			}
			log.Debug("connected to pubsub peer:", pi.ID)
			n.msgChan <- addPeer{pi.ID}
			if n.wireService != nil {
				m := &pb.Message{
					MessageType: pb.Message_GetOrderBook,
				}
				n.wireService.SendMessage(pi.ID, m)
			}
		}(p)
	}
	wg.Wait()
}

func (n *AtomicSwapNode) OrderBook() *ob.OrderBook {
	return n.orderBook
}