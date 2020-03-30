package market

import (
	"fmt"
	"testing"
	"time"
)

func TestMarketer_MarshalJson(t *testing.T) {
	m := NewTestMarketer()
	fmt.Println(m.MarshalJson())
}

func TestLister_Del(t *testing.T) {
	l := newList()
	l.Add("btc", NewTestMarketer())
	l.Add("usdt", NewTestMarketer())
	l.Del("usdt")
	fmt.Println(l)
}

func TestLister_Find(t *testing.T) {
	l := newList()
	l.Add("btc", NewTestMarketer())
	l.Add("usdt", NewTestMarketer())
	newL := l.Find("a")
	fmt.Println(newL)
}

func TestLister_MarshalJson(t *testing.T) {
	l := newList()
	l.Add("btc", NewTestMarketer())
	l.Add("usdt", NewTestMarketer())
	fmt.Println(l.MarshalJson())
}

func NewTestMarketer() *Marketer {
	var d = make(Depth, 1)
	s := [2]string{"20321", "213"}
	d[0] = s

	return &Marketer{
		BuyFirst:  "123213",
		SellFirst: "213123",
		BuyDepth:  d,
		SellDepth: d,
		Timestamp: time.Duration(time.Now().UnixNano() / 1e6),
	}
}

func Test_WriteSubscribing(t *testing.T) {
	s := &Subscriber{
		Symbol:     "ETH-USDT",
		MarketType: SpotMarket,
		Organize:   OkEx,
	}
	WriteSubscribing <- s

	select {
	case sub := <-readSubscribing:
		Manage.Tasks[sub.Organize].subscribeHandle(sub)
	}
}

func Test_WriteRingBuffer(t *testing.T) {
	m := NewTestMarketer()

	go func() {
		writeMarketPool.writeRingBuffer(m)
	}()

	fmt.Println(<-ReadMarketPool)
}
