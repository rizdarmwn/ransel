package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strconv"
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
	mux.HandleFunc("/book/{market}", ex.handleGetBook)
	mux.HandleFunc("/order/{id}", ex.handleCancelOrder)
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
	ID        uint64
	Price     *big.Int
	Size      *big.Int
	Bid       bool
	Timestamp int64
}

type OrderbookData struct {
	TotalBidVolume *big.Int
	TotalAskVolume *big.Int
	Asks           []*Order
	Bids           []*Order
}

type Match struct {
	SizeFilled *big.Int
	Price      *big.Int
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
	defer mu.Unlock()
	market := Market(placeOrderData.Market)
	ob := ex.orderbooks[market]
	order := orderbook.NewOrder(placeOrderData.Bid, placeOrderData.Size)

	w.Header().Set("Content-Type", "application/json")
	switch placeOrderData.Type {
	case LIMIT_ORDER:
		w.WriteHeader(http.StatusCreated)
		ob.PlaceLimitOrder(placeOrderData.Price, order)
		json.NewEncoder(w).Encode(map[string]any{"msg": "Limit order placed successfully"})
	case MARKET_ORDER:
		w.WriteHeader(http.StatusCreated)
		matches := ob.PlaceMarketOrder(order)
		matchedOrder := make([]*Match, len(matches))
		for i := range matches {
			matchedOrder[i] = &Match{
				SizeFilled: matches[i].SizeFilled,
				Price:      matches[i].Price,
			}
		}
		json.NewEncoder(w).Encode(map[string]any{"msg": matchedOrder})
	default:
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{"error": "Invalid order type"})
		return
	}
}

func (ex *Exchange) handleCancelOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	if idStr == "" {
		http.Error(w, "Invalid order ID", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid order ID", http.StatusBadRequest)
		return
	}

	ob := ex.orderbooks[MARKET_ETH]
	o := ob.Orders[id]
	if o == nil {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	mu.Lock()
	defer mu.Unlock()
	ob.CancelOrder(o)
	json.NewEncoder(w).Encode(map[string]any{"msg": "Order canceled successfully"})
}

func (ex *Exchange) handleGetBook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	market := Market(r.PathValue("market"))
	ob, ok := ex.orderbooks[market]
	if !ok {
		http.Error(w, "Market not found", http.StatusNotFound)
		return
	}

	orderbookData := OrderbookData{
		TotalBidVolume: ob.BidTotalVolume(),
		TotalAskVolume: ob.AskTotalVolume(),
		Asks:           []*Order{},
		Bids:           []*Order{},
	}

	for _, limit := range ob.Asks() {
		for _, order := range limit.Orders {
			o := Order{
				ID:        order.ID,
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
				ID:        order.ID,
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
