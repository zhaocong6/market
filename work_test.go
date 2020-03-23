package market

import (
	"fmt"
	"testing"
	"time"
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
		time.Sleep(time.Second)
		fmt.Println(Manage.Tasks[OkEx].List.Find("ETH-USDT"))
		fmt.Println(Manage.Tasks[HuoBi].List.Find("ethusdt"))
	}
}
