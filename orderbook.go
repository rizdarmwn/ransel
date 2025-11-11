package main

import (
	"fmt"
	"math/big"
	"sort"
	"time"
)

type Match struct {
	Ask        *Order
	Bid        *Order
	SizeFilled *big.Int
	Price      *big.Int
}

type Order struct {
	Size      *big.Int
	Bid       bool
	Limit     *Limit
	Timestamp int64
}
type Orders []*Order

func (o Orders) Len() int           { return len(o) }
func (o Orders) Swap(i, j int)      { o[i], o[j] = o[j], o[i] }
func (o Orders) Less(i, j int) bool { return o[i].Timestamp < o[j].Timestamp }

func NewOrder(bid bool, size *big.Int) *Order {
	return &Order{
		Size:      size,
		Bid:       bid,
		Timestamp: time.Now().UnixNano(),
	}
}

func (o *Order) String() string {
	return fmt.Sprintf("[size: %.2f]", o.Size)
}

type Limit struct {
	Price       *big.Int
	Orders      Orders
	TotalVolume *big.Int
}

type Limits []*Limit

func NewLimit(price *big.Int) *Limit {
	return &Limit{
		Price:       price,
		Orders:      []*Order{},
		TotalVolume: new(big.Int),
	}
}

func (l *Limit) AddOrder(o *Order) {
	o.Limit = l
	l.Orders = append(l.Orders, o)
	l.TotalVolume.Add(l.TotalVolume, o.Size)
}

func (l *Limit) DeleteOrder(o *Order) {
	for i := 0; i < len(l.Orders); i++ {
		if l.Orders[i] == o {
			l.Orders[i] = l.Orders[len(l.Orders)-i]
			l.Orders = l.Orders[:len(l.Orders)-1]
		}
	}

	o.Limit = nil
	l.TotalVolume.Sub(l.TotalVolume, o.Size)

	sort.Sort(l.Orders)
}

type ByBestAsk struct {
	Limits
}

func (a ByBestAsk) Len() int           { return len(a.Limits) }
func (a ByBestAsk) Swap(i, j int)      { a.Limits[i], a.Limits[j] = a.Limits[j], a.Limits[i] }
func (a ByBestAsk) Less(i, j int) bool { return a.Limits[i].Price.Cmp(a.Limits[j].Price) == -1 }

type ByBestBid struct {
	Limits
}

func (b ByBestBid) Len() int           { return len(b.Limits) }
func (b ByBestBid) Swap(i, j int)      { b.Limits[i], b.Limits[j] = b.Limits[j], b.Limits[i] }
func (b ByBestBid) Less(i, j int) bool { return b.Limits[i].Price.Cmp(b.Limits[j].Price) == 1 }

type Orderbook struct {
	Asks []*Limit
	Bids []*Limit

	AskLimits map[*big.Int]*Limit
	BidLimits map[*big.Int]*Limit
}

func NewOrderbook() *Orderbook {
	return &Orderbook{
		Asks:      []*Limit{},
		Bids:      []*Limit{},
		AskLimits: make(map[*big.Int]*Limit),
		BidLimits: make(map[*big.Int]*Limit),
	}
}

func (ob *Orderbook) PlaceOrder(price *big.Int, o *Order) []Match {
	// TODO: Matching logic
	if o.Size.Cmp(big.NewInt(0)) == 1 {
		ob.add(price, o)
	}

	return []Match{}
}

func (ob *Orderbook) add(price *big.Int, o *Order) {
	var limit *Limit

	if o.Bid {
		limit = ob.BidLimits[price]
	} else {
		limit = ob.AskLimits[price]
	}

	if limit == nil {
		limit = NewLimit(price)
		limit.AddOrder(o)
		if o.Bid {
			ob.Bids = append(ob.Bids, limit)
			ob.BidLimits[price] = limit
		} else {
			ob.Asks = append(ob.Asks, limit)
			ob.AskLimits[price] = limit
		}
	}
}
