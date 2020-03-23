package market

import (
	"fmt"
	"testing"
)

func Test_Run(t *testing.T) {
	Run()

	s := &Subscriber{
		Symbol:     "ETH-USDT",
		MarketType: SpotMarket,
		Organize:   OkEx,
	}
	WriteSubscribing <- s

	h := &Subscriber{
		Symbol:     "ethusdt",
		MarketType: SpotMarket,
		Organize:   HuoBi,
	}

	WriteSubscribing <- h

	for {
		select {
		case sub := <-ReadMarketPool:
			fmt.Println(sub)
		}
	}
}
