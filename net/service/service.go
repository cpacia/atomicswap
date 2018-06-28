package service

import (
	"context"
	ob "github.com/cpacia/atomicswap/orderbook"
	"github.com/cpacia/atomicswap/pb"
	ggio "github.com/gogo/protobuf/io"
	"github.com/golang/protobuf/ptypes"
	ctxio "github.com/jbenet/go-context/io"
	"github.com/libp2p/go-libp2p-host"
	inet "github.com/libp2p/go-libp2p-net"
	"github.com/libp2p/go-libp2p-peer"
	"github.com/libp2p/go-libp2p-protocol"
	"github.com/op/go-logging"
	"io"
)

const SwapProtocol protocol.ID = "/SwapProtocol/1.0.0"

var log = logging.MustGetLogger("service")

type WireService struct {
	msgChan   chan interface{}
	orderBook *ob.OrderBook
	peerHost  host.Host
}

func NewWireService(msgChan chan interface{}, orderBook *ob.OrderBook, peerHost host.Host) *WireService {
	ws := &WireService{
		msgChan:   msgChan,
		orderBook: orderBook,
		peerHost:  peerHost,
	}
	ws.peerHost.SetStreamHandler(SwapProtocol, ws.handleNewStream)
	return ws
}

func (ws *WireService) SendMessage(peer peer.ID, pmes *pb.Message) error {
	s, err := ws.peerHost.NewStream(context.Background(), peer, SwapProtocol)
	if err != nil {
		return err
	}
	writer := ggio.NewDelimitedWriter(s)
	return writer.WriteMsg(pmes)
}

func (ws *WireService) SendRequest(peer peer.ID, pmes *pb.Message) (*pb.Message, error) {
	s, err := ws.peerHost.NewStream(context.Background(), peer, SwapProtocol)
	if err != nil {
		return nil, err
	}
	writer := ggio.NewDelimitedWriter(s)
	err = writer.WriteMsg(pmes)
	if err != nil {
		return nil, err
	}
	cr := ctxio.NewReader(context.Background(), s)
	r := ggio.NewDelimitedReader(cr, inet.MessageSizeMax)
	rmes := new(pb.Message)
	if err := r.ReadMsg(rmes); err != nil {
		s.Reset()
		if err == io.EOF {
			log.Debugf("Disconnected from peer %s", peer.Pretty())
		}
		return nil, err
	}
	return rmes, nil
}

func (ws *WireService) handleNewStream(s inet.Stream) {
	go ws.handleNewMessage(s)
}

// We're just going to handle the stream and close it out here
// Not very efficient but this is only a demo. In a production app
// you'd likely want to reuse open streams.
func (ws *WireService) handleNewMessage(s inet.Stream) {
	defer s.Close()
	cr := ctxio.NewReader(context.Background(), s)
	r := ggio.NewDelimitedReader(cr, inet.MessageSizeMax)
	mPeer := s.Conn().RemotePeer()
	pmes := new(pb.Message)
	if err := r.ReadMsg(pmes); err != nil {
		s.Reset()
		if err == io.EOF {
			log.Debugf("Disconnected from peer %s", mPeer.Pretty())
		}
		return
	}
	handler := ws.handlerForMsgType(pmes.MessageType)
	rmes, err := handler(mPeer, pmes)
	if err != nil {
		log.Error(err)
		return
	}
	if rmes != nil {
		writer := ggio.NewDelimitedWriter(s)
		err = writer.WriteMsg(rmes)
		if err != nil {
			log.Error(err)
		}
	}
}

func (ws *WireService) handlerForMsgType(t pb.Message_MessageType) func(peer.ID, *pb.Message) (*pb.Message, error) {
	switch t {
	case pb.Message_LimitOrder:
		return ws.handleLimitOrder
	case pb.Message_GetOrderBook:
		return ws.handleGetOrderBook
	default:
		return nil
	}
}

func (ws *WireService) handleLimitOrder(p peer.ID, msg *pb.Message) (*pb.Message, error) {
	ws.orderBook.ProcessNewLimitOrder(msg.Payload.Value, false)
	return nil, nil
}

func (ws *WireService) handleGetOrderBook(p peer.ID, msg *pb.Message) (*pb.Message, error) {
	for _, o := range ws.orderBook.OpenOrders() {
		so, err := o.SignedLimitOrder()
		if err != nil {
			continue
		}
		payload, err := ptypes.MarshalAny(so)
		if err != nil {
			continue
		}
		m := &pb.Message{
			MessageType: pb.Message_LimitOrder,
			Payload:     payload,
		}
		ws.SendMessage(p, m)
	}
	return nil, nil
}
