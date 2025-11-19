package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/rizdarmwn/ransel/orderbook"
)

var (
	mu sync.Mutex
)

func main() {
	mux := http.NewServeMux()
	ex := NewExchange()
	mux.HandleFunc("/order", ex.handlePlaceOrder)
	mux.HandleFunc("/book", ex.handleGetBook)
	fmt.Println("Running server")
	log.Fatal(http.ListenAndServe(":3000", mux))
}

type OrderType string

const (
	MARKET_ORDER OrderType = "MARKET"
	LIMIT_ORDER  OrderType = "LIMIT"
)

type Market string

const (
	MARKET_ETH Market = "ETH"
)

type Exchange struct {
	orderbooks map[Market]*orderbook.Orderbook
}

func NewExchange() *Exchange {
	orderbooks := make(map[Market]*orderbook.Orderbook)
	orderbooks[MARKET_ETH] = orderbook.NewOrderbook()

	return &Exchange{
		orderbooks: orderbooks,
	}
}

type PlaceOrderRequest struct {
	Type   OrderType
	Bid    bool
	Size   *big.Int
	Price  *big.Int
	Market Market
}

type Order struct {
	Price     *big.Int
	Size      *big.Int
	Bid       bool
	Timestamp int64
}

type OrderbookData struct {
	Asks []*Order
	Bids []*Order
}

func (ex *Exchange) handlePlaceOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var placeOrderData PlaceOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&placeOrderData); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	mu.Lock()
	market := Market(placeOrderData.Market)
	ob := ex.orderbooks[market]
	order := orderbook.NewOrder(placeOrderData.Bid, placeOrderData.Size)

	ob.PlaceLimitOrder(placeOrderData.Price, order)
	mu.Unlock()
	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"msg": "Order placed successfully"})
}

func (ex *Exchange) handleGetBook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	market := Market(r.URL.Query().Get("market"))
	ob, ok := ex.orderbooks[market]
	if !ok {
		http.Error(w, "Market not found", http.StatusNotFound)
		return
	}

	orderbookData := OrderbookData{
		Asks: []*Order{},
		Bids: []*Order{},
	}

	for _, limit := range ob.Asks() {
		for _, order := range limit.Orders {
			o := Order{
				Price:     limit.Price,
				Size:      order.Size,
				Bid:       order.Bid,
				Timestamp: order.Timestamp,
			}
			orderbookData.Asks = append(orderbookData.Asks, &o)
		}
	}

	for _, limit := range ob.Bids() {
		for _, order := range limit.Orders {
			o := Order{
				Price:     limit.Price,
				Size:      order.Size,
				Bid:       order.Bid,
				Timestamp: order.Timestamp,
			}
			orderbookData.Bids = append(orderbookData.Bids, &o)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orderbookData)
}

func parseID(p string) int {
	parts := strings.Split(path.Base(p), "/")
	if len(parts) < 1 {
		return -1
	}

	id, err := strconv.Atoi(parts[0])
	if err != nil {
		return -1
	}
	return id
}
