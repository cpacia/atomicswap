package orderbook

import (
	"crypto/sha256"
	"github.com/cpacia/atomicswap/pb"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-peer"
	"github.com/multiformats/go-multihash"
	"time"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("orderbook")

type LimitOrder struct {
	*pb.LimitOrder
	signature []byte
	OrderID   string
}

func (lo *LimitOrder) ID() (*cid.Cid, error) {
	ser, err := proto.Marshal(lo)
	if err != nil {
		return nil, err
	}
	h := sha256.Sum256(ser)
	enc, err := multihash.Encode(h[:], multihash.SHA2_256)
	if err != nil {
		return nil, err
	}
	mh, err := multihash.Cast(enc)
	if err != nil {
		return nil, err
	}
	return cid.NewCidV1(cid.Raw, mh), nil
}

func (lo *LimitOrder) SignedLimitOrder() (*pb.SignedLimitOrder, error) {
	ser, err := proto.Marshal(lo.LimitOrder)
	if err != nil {
		return nil, err
	}
	signed := &pb.SignedLimitOrder{
		SerializedLimitOrder: ser,
		Signature:            lo.signature,
	}
	return signed, nil
}

type OrderBook struct {
	orders map[string]LimitOrder
}

func NewOrderBook() *OrderBook {
	return &OrderBook{make(map[string]LimitOrder)}
}

func (ob *OrderBook) OpenOrders() []LimitOrder {
	var orders []LimitOrder
	for _, o := range ob.orders {
		orders = append(orders, o)
	}
	return orders
}

// Maybe add a new order to our order book
func (ob *OrderBook) ProcessNewLimitOrder(serializedOrder []byte) {
	// Deserialized signed order
	signed := new(pb.SignedLimitOrder)
	err := proto.Unmarshal(serializedOrder, signed)
	if err != nil {
		log.Error(err)
		return
	}
	// Deserialize nested limit order
	limitpb := new(pb.LimitOrder)
	err = proto.Unmarshal(signed.SerializedLimitOrder, limitpb)
	if err != nil {
		log.Error(err)
		return
	}
	// Calculate the ID
	lo := LimitOrder{LimitOrder: limitpb, signature: signed.Signature}
	id, err := lo.ID()
	if err != nil {
		log.Error(err)
		return
	}
	// We already have this order, return
	if _, ok := ob.orders[id.String()]; ok {
		return
	}

	// Validate signature
	pid, err := peer.IDB58Decode(lo.PeerID)
	if err != nil {
		log.Error(err)
		return
	}
	pubKey, err := pid.ExtractPublicKey()
	if err != nil {
		log.Error(err)
		return
	}
	valid, err := pubKey.Verify(signed.SerializedLimitOrder, signed.Signature)
	if !valid || err != nil {
		log.Error("invalid signature on limit order")
		return
	}

	// Check expiration
	expirationDate, err := ptypes.Timestamp(lo.Expiry)
	if err != nil {
		log.Error(err)
		return
	}
	if expirationDate.Before(time.Now()) {
		log.Error("received expired order")
		return
	}

	// TODO: validate signed UTXO

	// If we made it this far lets add it to our orderbook
	log.Infof("Added order: %s to order book", id.String())
	ob.orders[id.String()] = lo
}

// Maybe remove an order from our orderbook
func (ob *OrderBook) ProcessCloseOrder(serializedOrder []byte) {
	// Deserialized signed order
	signed := new(pb.SignedRemoveOrder)
	err := proto.Unmarshal(serializedOrder, signed)
	if err != nil {
		log.Error(err)
		return
	}

	// If we don't have this order then we can just return
	id, err := cid.Decode(signed.OrderID)
	if err != nil {
		log.Error(err)
		return
	}
	lo, ok := ob.orders[id.String()]
	if !ok {
		return
	}

	// Validate signature
	pid, err := peer.IDB58Decode(lo.PeerID)
	if err != nil {
		log.Error(err)
		return
	}
	pubKey, err := pid.ExtractPublicKey()
	if err != nil {
		log.Error(err)
		return
	}
	valid, err := pubKey.Verify([]byte(signed.OrderID), signed.Signature)
	if !valid || err != nil {
		log.Error("invalid signature on close order")
		return
	}

	// If we made it this far we can remove the order from the orderbook
	log.Infof("Removed order: %s from order book", id.String())
	delete(ob.orders, id.String())
}
