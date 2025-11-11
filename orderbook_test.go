package main

import (
	"fmt"
	"math/big"
	"testing"
)

func TestLimit(t *testing.T) {
	l := NewLimit(big.NewInt(10_000))
	buyOrderA := NewOrder(true, big.NewInt(5))
	buyOrderB := NewOrder(true, big.NewInt(8))
	buyOrderC := NewOrder(true, big.NewInt(10))

	l.AddOrder(buyOrderA)
	l.AddOrder(buyOrderB)
	l.AddOrder(buyOrderC)

	l.DeleteOrder(buyOrderB)

	fmt.Println(l)
}

func TestOrderbook(t *testing.T) {
	ob := NewOrderbook()

	buyOrder := NewOrder(true, big.NewInt(10))

	ob.PlaceOrder(big.NewInt(18_000), buyOrder)

	fmt.Println(ob.Bids)
}
