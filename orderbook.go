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

func (o *Order) IsFilled() bool {
	return o.Size.Cmp(big.NewInt(0)) == 0
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
			l.Orders[i] = l.Orders[len(l.Orders)-1]
			l.Orders = l.Orders[:len(l.Orders)-1]
		}
	}

	o.Limit = nil
	l.TotalVolume.Sub(l.TotalVolume, o.Size)

	sort.Sort(l.Orders)
}

func (l *Limit) Fill(o *Order) []Match {
	var (
		matches        []Match
		ordersToDelete []*Order
	)
	for _, order := range l.Orders {
		match := l.fillOrder(order, o)
		matches = append(matches, match)

		l.TotalVolume.Sub(l.TotalVolume, match.SizeFilled)

		if order.IsFilled() {
			ordersToDelete = append(ordersToDelete, order)
		}

		if o.IsFilled() {
			break
		}
	}

	for _, order := range ordersToDelete {
		l.DeleteOrder(order)
	}

	return matches
}

func (l *Limit) fillOrder(a, b *Order) Match {
	var (
		bid        *Order
		ask        *Order
		sizeFilled *big.Int
	)

	if a.Bid {
		bid = a
		ask = b
	} else {
		bid = b
		ask = a
	}

	if a.Size.Cmp(b.Size) == 1 || a.Size.Cmp(b.Size) == 0 {
		a.Size.Sub(a.Size, b.Size)
		sizeFilled = b.Size
		b.Size = big.NewInt(0)
	} else {
		b.Size.Sub(b.Size, a.Size)
		sizeFilled = a.Size
		a.Size = big.NewInt(0)
	}

	return Match{
		Bid:        bid,
		Ask:        ask,
		SizeFilled: sizeFilled,
		Price:      l.Price,
	}
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
	asks []*Limit
	bids []*Limit

	AskLimits map[*big.Int]*Limit
	BidLimits map[*big.Int]*Limit
}

func NewOrderbook() *Orderbook {
	return &Orderbook{
		asks:      []*Limit{},
		bids:      []*Limit{},
		AskLimits: make(map[*big.Int]*Limit),
		BidLimits: make(map[*big.Int]*Limit),
	}
}

func (ob *Orderbook) PlaceMarketOrder(o *Order) []Match {
	matches := []Match{}

	if o.Bid {
		if o.Size.Cmp(ob.AskTotalVolume()) == 1 {
			panic(fmt.Errorf("not enough volume [size: %.2f] for market order [size: %.2f]", ob.AskTotalVolume(), o.Size))
		}
		for _, limit := range ob.Asks() {
			limitMatches := limit.Fill(o)
			matches = append(matches, limitMatches...)

			if len(limit.Orders) == 0 {
				ob.clearLimit(true, limit)
			}
		}
	} else {
		if o.Size.Cmp(ob.BidTotalVolume()) == 1 {
			panic(fmt.Errorf("not enough volume [size: %.2f] for market order [size: %.2f]", ob.BidTotalVolume(), o.Size))
		}
		for _, limit := range ob.Bids() {
			limitMatches := limit.Fill(o)
			matches = append(matches, limitMatches...)

			if len(limit.Orders) == 0 {
				ob.clearLimit(true, limit)
			}
		}
	}

	return matches
}

func (ob *Orderbook) PlaceLimitOrder(price *big.Int, o *Order) {
	var limit *Limit

	if o.Bid {
		for existingPrice, existingLimit := range ob.BidLimits {
			if existingPrice.Cmp(price) == 0 {
				limit = existingLimit
				break
			}
		}
	} else {
		for existingPrice, existingLimit := range ob.AskLimits {
			if existingPrice.Cmp(price) == 0 {
				limit = existingLimit
				break
			}
		}
	}

	if limit == nil {
		limit = NewLimit(price)
		if o.Bid {
			ob.bids = append(ob.bids, limit)
			ob.BidLimits[price] = limit
		} else {
			ob.asks = append(ob.asks, limit)
			ob.AskLimits[price] = limit
		}
	}
	limit.AddOrder(o)
}

func (ob *Orderbook) clearLimit(bid bool, l *Limit) {
	if bid {
		for price := range ob.BidLimits {
			if price.Cmp(l.Price) == 0 {
				delete(ob.BidLimits, price)
				break
			}
		}
		for i := 0; i < len(ob.bids); i++ {
			if ob.bids[i] == l {
				ob.bids[i] = ob.bids[len(ob.bids)-1]
				ob.bids = ob.bids[:len(ob.bids)-1]
			}
		}
	} else {
		for price := range ob.AskLimits {
			if price.Cmp(l.Price) == 0 {
				delete(ob.AskLimits, price)
				break
			}
		}
		for i := 0; i < len(ob.asks); i++ {
			if ob.asks[i] == l {
				ob.asks[i] = ob.asks[len(ob.asks)-1]
				ob.asks = ob.asks[:len(ob.asks)-1]
			}
		}
	}
}

func (ob *Orderbook) CancelOrder(o *Order) {
	limit := o.Limit
	limit.DeleteOrder(o)
}

func (ob *Orderbook) BidTotalVolume() *big.Int {
	totalVolume := big.NewInt(0)

	for i := 0; i < len(ob.bids); i++ {
		totalVolume.Add(totalVolume, ob.bids[i].TotalVolume)
	}

	return totalVolume
}

func (ob *Orderbook) AskTotalVolume() *big.Int {
	totalVolume := big.NewInt(0)

	for i := 0; i < len(ob.asks); i++ {
		totalVolume.Add(totalVolume, ob.asks[i].TotalVolume)
	}

	return totalVolume
}

func (ob *Orderbook) Asks() []*Limit {
	sort.Sort(ByBestAsk{ob.asks})
	return ob.asks
}

func (ob *Orderbook) Bids() []*Limit {
	sort.Sort(ByBestBid{ob.bids})
	return ob.bids
}
