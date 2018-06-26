package api

import (
	"encoding/json"
	"fmt"
	"github.com/cpacia/atomicswap/core"
	ob "github.com/cpacia/atomicswap/orderbook"
	"github.com/gorilla/mux"
	"net/http"
	"strconv"
)

type APIServer struct {
	node   *core.AtomicSwapNode
	router *mux.Router
}

func NewAPIServer(node *core.AtomicSwapNode) *APIServer {
	s := &APIServer{
		node:   node,
		router: mux.NewRouter(),
	}
	s.router.HandleFunc("/limitorder", s.handleLimitOrder).Methods("POST")
	s.router.HandleFunc("/orderbook", s.handleOrderBook).Methods("GET")
	return s
}

func (a *APIServer) Serve(port int) {
	http.ListenAndServe(":"+strconv.Itoa(port), a.router)
}

func (a *APIServer) handleLimitOrder(w http.ResponseWriter, r *http.Request) {
	type order struct {
		Quantity uint64 `json:"quantity"`
		Price    uint64 `json:"price"`
		BuyBTC   bool   `json:"buyBTC"`
	}
	var o order
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&o)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	err = a.node.PublishLimitOrder(o.Quantity, o.Price, o.BuyBTC)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (a *APIServer) handleOrderBook(w http.ResponseWriter, r *http.Request) {
	var orders []ob.LimitOrder
	for _, o := range a.node.OrderBook().OpenOrders() {
		id, err := o.ID()
		if err != nil {
			continue
		}
		o.OrderID = id.String()
		orders = append(orders, o)
	}
	ser, err := json.MarshalIndent(orders, "", "    ")
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	fmt.Fprint(w, string(ser))
}
