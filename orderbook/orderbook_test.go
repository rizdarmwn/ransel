package orderbook

import (
	"fmt"
	"math/big"
	"reflect"
	"testing"
)

func assertEq(t *testing.T, a, b any) {
	if !reflect.DeepEqual(a, b) {
		t.Errorf("%+v != %+v", a, b)
	}
}

func TestLimit(t *testing.T) {
	l := NewLimit(big.NewInt(10_000))
	buyOrderA := NewOrder(true, big.NewInt(5), 0)
	buyOrderB := NewOrder(true, big.NewInt(8), 0)
	buyOrderC := NewOrder(true, big.NewInt(10), 0)

	l.AddOrder(buyOrderA)
	l.AddOrder(buyOrderB)
	l.AddOrder(buyOrderC)

	l.DeleteOrder(buyOrderB)

	fmt.Println(l)
}

func TestPlaceLimitOrder(t *testing.T) {
	ob := NewOrderbook()

	sellOrderA := NewOrder(false, big.NewInt(10), 0)
	sellOrderB := NewOrder(false, big.NewInt(20), 0)
	ob.PlaceLimitOrder(big.NewInt(10_000), sellOrderA)
	ob.PlaceLimitOrder(big.NewInt(9_000), sellOrderB)

	assertEq(t, len(ob.Orders), 2)
	assertEq(t, len(ob.asks), 2)
}

func TestPlaceMarketOrder(t *testing.T) {
	ob := NewOrderbook()

	sellOrder := NewOrder(false, big.NewInt(20), 0)
	ob.PlaceLimitOrder(big.NewInt(10_000), sellOrder)

	buyOrder := NewOrder(true, big.NewInt(10), 0)
	matches := ob.PlaceMarketOrder(buyOrder)

	assertEq(t, len(matches), 1)
	assertEq(t, len(ob.asks), 1)
	assertEq(t, ob.AskTotalVolume(), big.NewInt(10))
	assertEq(t, matches[0].Ask, sellOrder)
	assertEq(t, matches[0].Bid, buyOrder)
	assertEq(t, matches[0].SizeFilled, big.NewInt(10))
	assertEq(t, matches[0].Price, big.NewInt(10_000))
	assertEq(t, buyOrder.IsFilled(), true)

	fmt.Printf("%+v", matches)
}

func TestPlaceMarketOrderMultiFill(t *testing.T) {
	ob := NewOrderbook()

	buyOrderA := NewOrder(true, big.NewInt(5), 0)
	buyOrderB := NewOrder(true, big.NewInt(8), 0)
	buyOrderC := NewOrder(true, big.NewInt(10), 0)
	buyOrderD := NewOrder(true, big.NewInt(1), 0)

	ob.PlaceLimitOrder(big.NewInt(5_000), buyOrderC)
	ob.PlaceLimitOrder(big.NewInt(5_000), buyOrderD)
	ob.PlaceLimitOrder(big.NewInt(9_000), buyOrderB)
	ob.PlaceLimitOrder(big.NewInt(10_000), buyOrderA)

	assertEq(t, ob.BidTotalVolume(), big.NewInt(24))

	sellOrder := NewOrder(false, big.NewInt(20), 0)
	matches := ob.PlaceMarketOrder(sellOrder)

	assertEq(t, ob.BidTotalVolume(), big.NewInt(4))
	assertEq(t, len(ob.bids), 1)
	assertEq(t, len(matches), 3)
}

func TestCancelOrder(t *testing.T) {
	ob := NewOrderbook()

	buyOrder := NewOrder(true, big.NewInt(10), 0)
	ob.PlaceLimitOrder(big.NewInt(10_000), buyOrder)

	assertEq(t, ob.BidTotalVolume(), big.NewInt(10))

	ob.CancelOrder(buyOrder)
	_, ok := ob.Orders[buyOrder.ID]

	assertEq(t, ob.BidTotalVolume(), big.NewInt(0))
	assertEq(t, ok, false)
}
