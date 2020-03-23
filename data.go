package market

import (
	"encoding/json"
	"strconv"
	"time"
)

type Depth [][2]string

func (d Depth) formatFloat(params [][2]float64) Depth {
	val := make(Depth, len(params))

	for k, v := range params {
		val[k][0] = strconv.FormatFloat(v[0], 'g', -1, 64)
		val[k][1] = strconv.FormatFloat(v[1], 'g', -1, 64)
	}

	return val
}

type Marketer struct {
	Organize  Organize      `json:"organize"`
	Symbol    string        `json:"symbol"`
	BuyFirst  string        `json:"buy_first"`
	SellFirst string        `json:"sell_first"`
	BuyDepth  Depth         `json:"buy_depth"`
	SellDepth Depth         `json:"sell_depth"`
	Timestamp time.Duration `json:"timestamp"`
}

func (m *Marketer) MarshalJson() string {
	j, _ := json.Marshal(m)
	return string(j)
}

type Lister map[string]*Marketer

func (l Lister) MarshalJson() string {
	j, _ := json.Marshal(l)
	return string(j)
}

func (l Lister) Add(k string, m *Marketer) {
	l[k] = m
}

func (l Lister) Del(k string) {
	delete(l, k)
}

func (l Lister) Find(s ...string) Lister {
	newL := make(Lister)

	for _, k := range s {
		if v, ok := l[k]; ok {
			newL[k] = v
		}
	}

	return newL
}

type marketType int

const SpotMarket marketType = 1
const FuturesMarket marketType = 2
const WapMarket marketType = 3
const OptionMarket marketType = 4

type Organize string

const HuoBi Organize = "huobi"
const OkEx Organize = "okex"

type Subscriber struct {
	Symbol     string
	Organize   Organize
	MarketType marketType
}

var WriteSubscribing chan<- *Subscriber
var readSubscribing <-chan *Subscriber

func init() {
	var subscribing = make(chan *Subscriber, 2)
	WriteSubscribing = subscribing
	readSubscribing = subscribing
}

type readMarketer <-chan *Marketer
type writeMarketer chan<- *Marketer

var ReadMarketPool readMarketer
var writeMarketPool writeMarketer

func init() {
	m := make(chan *Marketer, 1000)
	ReadMarketPool = m
	writeMarketPool = m
}

func (w writeMarketer) writeRingBuffer(m *Marketer) {
	t := time.NewTimer(time.Millisecond)
	defer t.Stop()

	go func() {
		select {
		case <-t.C:
			<-ReadMarketPool
		}
	}()

	w <- m
}
