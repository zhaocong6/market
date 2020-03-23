package market

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gorilla/websocket"
	"strings"
	"time"
)

var huoBiUrl = "wss://api.huobi.pro/ws"

const huobiWsTimeout = time.Second * 30
const huobiWsPingTimeout = time.Second * 3

type huoBiHandler struct {
	pingLastTime time.Duration
}

func newHuoBi(ctx context.Context) *Worker {
	return &Worker{
		ctx:              ctx,
		wsUrl:            huoBiUrl,
		handler:          &huoBiHandler{},
		Organize:         HuoBi,
		Status:           runIng,
		Subscribes:       make(map[string][]byte),
		Subscribing:      make(map[string][]byte),
		LastRunTimestamp: time.Duration(time.Now().UnixNano() / 1e6),
		WsConn:           nil,
		List:             Lister{},
	}
}

func (h *huoBiHandler) formatSubscribeHandle(s *Subscriber) (b []byte) {
	switch s.MarketType {
	case SpotMarket:
		b = []byte(`{"id":"id1","sub":"market.` + s.Symbol + `.depth.step1"}`)
	case FuturesMarket:
	case OptionMarket:
	case WapMarket:
	}

	return
}

type huobiSubscriber struct {
	Status string `json:"status"`
	Subbed string `json:"subbed"`
}

func (h *huoBiHandler) subscribed(msg []byte, w *Worker) {
	subscribe := &huobiSubscriber{}
	json.Unmarshal(msg, subscribe)
	if subscribe.Status == "ok" {
		w.subscribed(strings.Split(subscribe.Subbed, ".")[1])
	}
}

func (h *huoBiHandler) resubscribeHandle(w *Worker) {
	for {
		select {
		case <-time.NewTimer(time.Second * 10).C:
			for _, sub := range w.Subscribing {
				w.Subscribe(sub)
			}
		}
	}
}

func (h *huoBiHandler) pingPongHandle(w *Worker) {
	for {
		select {
		case <-time.NewTimer(huobiWsPingTimeout).C:
			now := time.Duration(time.Now().UnixNano() / 1e6)

			if (now - h.pingLastTime) > huobiWsTimeout {
				w.WsConn.Close()
			} else if (now - h.pingLastTime) > huobiWsPingTimeout {
				pong, _ := json.Marshal(struct {
					pong time.Duration
				}{
					pong: now,
				})
				w.WsConn.WriteMessage(websocket.TextMessage, pong)
			}
		}
	}
}

func (h *huobiProvider) setSymbol() {
	h.Symbol = strings.Split(h.Ch, ".")[1]
}

func (h *huoBiHandler) formatMsgHandle(msgType int, msg []byte, w *Worker) (*Marketer, error) {
	switch msgType {
	case websocket.BinaryMessage:
		msg, err := gzipDecode(msg)
		if err != nil {
			return nil, err
		}

		market, err := h.marketerMsg(msg)

		if err == nil {
			return market, err
		}

		h.pongMsg(msg)
		h.subscribed(msg, w)
		return nil, nil
	default:
		return nil, nil
	}
}

type huobiProvider struct {
	Ch     string `json:"ch"`
	Symbol string
	Tick   struct {
		Bids      [][2]float64 `json:"bids"`
		Asks      [][2]float64 `json:"asks"`
		bidsDepth Depth
		asksDepth Depth
	} `json:"tick"`
	Timestamp time.Duration `json:"ts"`
}

func (h *huoBiHandler) marketerMsg(msg []byte) (*Marketer, error) {
	huobiData := &huobiProvider{}
	err := json.Unmarshal(msg, huobiData)
	if err != nil {
		return nil, err
	}
	if len(huobiData.Tick.Bids) == 0 || len(huobiData.Tick.Asks) == 0 {
		return nil, errors.New("序列化市场深度错误")
	}

	huobiData.Tick.bidsDepth = make(Depth, len(huobiData.Tick.Bids))
	huobiData.Tick.asksDepth = make(Depth, len(huobiData.Tick.Asks))
	huobiData.Tick.bidsDepth = huobiData.Tick.bidsDepth.formatFloat(huobiData.Tick.Bids)
	huobiData.Tick.asksDepth = huobiData.Tick.asksDepth.formatFloat(huobiData.Tick.Asks)
	huobiData.setSymbol()

	return h.newMarketer(huobiData)
}

func (h *huoBiHandler) newMarketer(p *huobiProvider) (*Marketer, error) {
	return &Marketer{
		Symbol:    p.Symbol,
		BuyFirst:  p.Tick.bidsDepth[0][0],
		SellFirst: p.Tick.asksDepth[0][0],
		BuyDepth:  p.Tick.bidsDepth,
		SellDepth: p.Tick.asksDepth,
		Timestamp: p.Timestamp,
	}, nil
}

type huobiPing struct {
	Ping int64 `json:"ping"`
}

func (h *huoBiHandler) pongMsg(msg []byte) {
	huobiPing := &huobiPing{}
	json.Unmarshal(msg, huobiPing)
	if huobiPing.Ping != 0 {
		h.pingLastTime = time.Duration(time.Now().UnixNano() / 1e6)
	}
}
