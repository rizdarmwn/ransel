package main

import (
	"fmt"
	"math/big"
	"time"
)

type Order struct {
	Size      *big.Int
	Bid       bool
	Limit     *Limit
	Timestamp int64
}

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
	Orders      []*Order
	TotalVolume *big.Int
}

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
}

type Orderbook struct {
	Asks []*Limit
	Bids []*Limit

	AskLimits map[*big.Int]*Limit
	BidLimits map[*big.Int]*Limit
}
