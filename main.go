package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"sync"

	"github.com/caarlos0/env/v11"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rizdarmwn/ransel/orderbook"
)

type Config struct {
	PrivateKey string `env:"PRIVATE_KEY" envDefault:"ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"` // Default private key is for Anvil
	ChainURL   string `env:"CHAIN_URL" envDefault:"http://localhost:8545"`
	Port       string `env:"PORT" envDefault:"3000"`
}

type (
	OrderType string
	Market    string

	PlaceOrderRequest struct {
		UserID uint64
		Type   OrderType
		Bid    bool
		Size   *big.Int
		Price  *big.Int
		Market Market
	}

	Order struct {
		ID        uint64
		Price     *big.Int
		Size      *big.Int
		Bid       bool
		Timestamp int64
	}

	OrderbookData struct {
		TotalBidVolume *big.Int
		TotalAskVolume *big.Int
		Asks           []*Order
		Bids           []*Order
	}

	Match struct {
		ID         uint64
		Bid        bool
		SizeFilled *big.Int
		Price      *big.Int
	}
)

const (
	MARKET_ORDER OrderType = "MARKET"
	LIMIT_ORDER  OrderType = "LIMIT"
	MARKET_ETH   Market    = "ETH"
)

var (
	mu sync.Mutex
)

func main() {
	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		log.Fatal(err)
	}

	client, err := ethclient.Dial(cfg.ChainURL)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	ex, err := NewExchange(cfg.PrivateKey, client)
	if err != nil {
		log.Fatal(err)
	}

	privKey, err := crypto.HexToECDSA("92db14e403b83dfe3df233f83dfa3a0d7096f21ca9b0d6d6b8d88b2b4ec1564e")
	if err != nil {
		log.Fatal(err)
	}

	user := &User{
		ID:         67,
		privateKey: privKey,
	}

	ex.users[user.ID] = user

	address := "0x976EA74026E726554dB657fA54763abd0C3a0aa9"
	balance, _ := client.BalanceAt(context.Background(), common.HexToAddress(address), nil)
	fmt.Println(balance)

	mux.HandleFunc("/order", ex.handlePlaceOrder)
	mux.HandleFunc("/book/{market}", ex.handleGetBook)
	mux.HandleFunc("/order/{id}", ex.handleCancelOrder)

	fmt.Println("Running server on port ", cfg.Port)

	log.Fatal(http.ListenAndServe(":"+cfg.Port, mux))
}

type User struct {
	ID         uint64
	privateKey *ecdsa.PrivateKey
}

func NewUser(privateKey string) (*User, error) {
	privKey, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		return nil, err
	}

	return &User{
		privateKey: privKey,
	}, nil
}

type Exchange struct {
	users      map[uint64]*User
	privateKey *ecdsa.PrivateKey
	orderbooks map[Market]*orderbook.Orderbook
	Client     *ethclient.Client
}

func NewExchange(privateKey string, client *ethclient.Client) (*Exchange, error) {
	orderbooks := make(map[Market]*orderbook.Orderbook)
	orderbooks[MARKET_ETH] = orderbook.NewOrderbook()

	pk, err := crypto.HexToECDSA(privateKey)
	if err != nil {
		return nil, err
	}

	return &Exchange{
		Client:     client,
		users:      make(map[uint64]*User),
		privateKey: pk,
		orderbooks: orderbooks,
	}, nil
}

func (ex *Exchange) handlePlaceLimitOrder(market Market, price *big.Int, order *orderbook.Order) error {
	ob := ex.orderbooks[market]
	ob.PlaceLimitOrder(price, order)

	user, ok := ex.users[order.UserID]
	if !ok {
		return errors.New("user not found")
	}

	exPublicKey := ex.privateKey.Public()
	exPublicKeyECDSA, ok := exPublicKey.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("invalid public key type")
	}

	exchangeAddress := crypto.PubkeyToAddress(*exPublicKeyECDSA)
	return transferETH(ex.Client, user.privateKey, exchangeAddress, order.Size)
}

func (ex *Exchange) handlePlaceMarketOrder(market Market, order *orderbook.Order) []orderbook.Match {
	ob := ex.orderbooks[market]

	matches := ob.PlaceMarketOrder(order)

	return matches
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
	order := orderbook.NewOrder(placeOrderData.Bid, placeOrderData.Size, placeOrderData.UserID)

	w.Header().Set("Content-Type", "application/json")
	switch placeOrderData.Type {
	case LIMIT_ORDER:
		w.WriteHeader(http.StatusCreated)
		if err := ex.handlePlaceLimitOrder(market, placeOrderData.Price, order); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"msg": "Limit order placed successfully"})
	case MARKET_ORDER:
		w.WriteHeader(http.StatusCreated)

		matches := ex.handlePlaceMarketOrder(market, order)
		// TODO: Implement handle matches
		matchedOrder := make([]*Match, len(matches))
		for i := range matches {
			if order.Bid {
				matchedOrder[i] = &Match{
					ID:         matches[i].Ask.ID,
					Bid:        matches[i].Ask.Bid,
					SizeFilled: matches[i].SizeFilled,
					Price:      matches[i].Price,
				}
			} else {
				matchedOrder[i] = &Match{
					ID:         matches[i].Bid.ID,
					Bid:        matches[i].Bid.Bid,
					SizeFilled: matches[i].SizeFilled,
					Price:      matches[i].Price,
				}
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
