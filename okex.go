package market

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gorilla/websocket"
	"strings"
	"time"
)

const okexUrl = "wss://real.OKEx.com:8443/ws/v3"
const okexWsTimeout = time.Second * 3

type okexHandler struct {
	pongLastTime time.Duration
}

func newOkEx(ctx context.Context) *Worker {
	return &Worker{
		ctx:   ctx,
		wsUrl: okexUrl,
		handler: &okexHandler{
			pongLastTime: time.Duration(time.Now().UnixNano() / 1e6),
		},
		Organize:         OkEx,
		Status:           runIng,
		Subscribes:       make(map[string][]byte),
		Subscribing:      make(map[string][]byte),
		LastRunTimestamp: time.Duration(time.Now().UnixNano() / 1e6),
		WsConn:           nil,
		List:             Lister{},
	}
}

func (h *okexHandler) formatSubscribeHandle(s *Subscriber) (b []byte) {
	switch s.MarketType {
	case SpotMarket:
		b = []byte(`{"op": "subscribe", "args": ["spot/depth5:` + s.Symbol + `"]}`)
	case FuturesMarket:
	case OptionMarket:
	case WapMarket:
	}

	return
}

func (h *okexHandler) resubscribeHandle(w *Worker) {
	for {
		select {
		case <-time.NewTimer(time.Second * 5).C:
			for _, sub := range w.Subscribing {
				w.Subscribe(sub)
			}
		}
	}
}

func (h *okexHandler) pingPongHandle(w *Worker) {
	for {
		select {
		case <-time.NewTimer(okexWsTimeout).C:
			if (time.Duration(time.Now().UnixNano()/1e6) - h.pongLastTime) > okexWsTimeout {
				w.WsConn.Close()
			} else {
				w.WsConn.WriteMessage(websocket.TextMessage, []byte("ping"))
			}
		}
	}
}

func (h *okexHandler) formatMsgHandle(msgType int, msg []byte, w *Worker) (*Marketer, error) {
	switch msgType {
	case websocket.BinaryMessage:
		msg, err := decode(msg)
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

type okexProvider struct {
	Table string `json:"table"`
	Data  []struct {
		Asks         Depth     `json:"asks"`
		Bids         Depth     `json:"bids"`
		InstrumentId string    `json:"instrument_id"`
		Timestamp    time.Time `json:"timestamp"`
	} `json:"data"`
}

func (h *okexHandler) marketerMsg(msg []byte) (*Marketer, error) {
	okexData := &okexProvider{}
	err := json.Unmarshal(msg, okexData)
	if err != nil {
		return nil, err
	}
	if okexData.Table == "" {
		return nil, errors.New("序列化市场深度错误")
	}

	h.pongLastTime = time.Duration(okexData.Data[0].Timestamp.UnixNano() / 1e6)
	return h.newMarketer(okexData)
}

func (h *okexHandler) newMarketer(p *okexProvider) (*Marketer, error) {
	return &Marketer{
		Symbol:    p.Data[0].InstrumentId,
		BuyFirst:  p.Data[0].Bids[0][0],
		SellFirst: p.Data[0].Asks[0][0],
		BuyDepth:  p.Data[0].Bids,
		SellDepth: p.Data[0].Asks,
		Timestamp: time.Duration(p.Data[0].Timestamp.UnixNano() / 1e6),
	}, nil
}

func (h *okexHandler) pongMsg(msg []byte) {
	if string(msg) == "pong" {
		h.pongLastTime = time.Duration(time.Now().UnixNano() / 1e6)
	}
}

type okexSubscriber struct {
	Event   string `json:"event"`
	Channel string `json:"channel"`
}

func (h *okexHandler) subscribed(msg []byte, w *Worker) {
	subscribe := &okexSubscriber{}
	json.Unmarshal(msg, subscribe)
	if subscribe.Event == "subscribe" {
		w.subscribed(strings.Split(subscribe.Channel, ":")[1])
	}
}
