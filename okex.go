package market

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gorilla/websocket"
	"log"
	"strings"
	"time"
)

const okexUrl = "wss://real.OKEx.com:8443/ws/v3"

//ws连接超时时间
//超过这个时间 服务器没有ping或者pong 将断开重连
const okexPingCheck int64 = 5
const okexWsPingTimeout int64 = 10

//记录okex服务器最后pong时间
type okexHandler struct {
	pongLastTime int64
}

//创建一个okex
//该流程中没有创建ws连接
func newOkEx(ctx context.Context) *Worker {
	return &Worker{
		ctx:   ctx,
		wsUrl: okexUrl,
		handler: &okexHandler{
			pongLastTime: time.Now().Unix(),
		},
		Organize:         OkEx,
		Status:           runIng,
		Subscribes:       make(map[string][]byte),
		Subscribing:      make(map[string][]byte),
		LastRunTimestamp: time.Duration(time.Now().UnixNano() / 1e6),
		WsConn:           nil,
		List:             newList(),
	}
}

//对订阅数据进行格式化
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

//ping pong检测
//超过规定时间, okex服务器没有返回pong 就断开了连接
//满足pong后 向okex服务器发出ping请求
func (h *okexHandler) pingPongHandle(w *Worker) {
	for {
		select {
		case <-time.NewTimer(time.Second * time.Duration(okexPingCheck)).C:
			if (time.Now().Unix() - h.pongLastTime) > okexWsPingTimeout {
				log.Printf("%s pingpong断线", OkEx)
				w.closeRedialSub()
			} else {
				w.writeMessage(websocket.TextMessage, []byte("ping"))
			}
		}
	}
}

//对okex返回数据进行格式化
//目前只处理二进制数据, okex返回其他数据不处理
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

//okex josn结构体
type okexProvider struct {
	Table string `json:"table"` //订阅类型和深度
	Data  []struct {
		Asks         Depth     `json:"asks"`          //卖方深度
		Bids         Depth     `json:"bids"`          //买方深度
		InstrumentId string    `json:"instrument_id"` //合约或者币对
		Timestamp    time.Time `json:"timestamp"`     //数据时间戳(毫秒)
	} `json:"data"`
}

//解析json数据
//判断是否是深度数据
func (h *okexHandler) marketerMsg(msg []byte) (*Marketer, error) {
	okexData := &okexProvider{}
	err := json.Unmarshal(msg, okexData)
	if err != nil {
		return nil, err
	}
	if okexData.Table == "" {
		return nil, errors.New("序列化市场深度错误")
	}

	//okex重连以后, 不会主动pong
	h.pongLastTime = time.Now().Unix()
	return h.newMarketer(okexData)
}

//将深度数据转换成统一的行情数据
func (h *okexHandler) newMarketer(p *okexProvider) (*Marketer, error) {
	timestamp := time.Duration(p.Data[0].Timestamp.UnixNano() / 1e6)

	return &Marketer{
		Organize:  OkEx,
		Symbol:    p.Data[0].InstrumentId,
		BuyFirst:  p.Data[0].Bids[0][0],
		SellFirst: p.Data[0].Asks[0][0],
		BuyDepth:  p.Data[0].Bids,
		SellDepth: p.Data[0].Asks,
		Timestamp: timestamp,
		Temporize: time.Duration(time.Now().UnixNano()/1e6) - timestamp,
	}, nil
}

//验证是否是pong数据
func (h *okexHandler) pongMsg(msg []byte) {
	if string(msg) == "pong" {
		h.pongLastTime = time.Now().Unix()
	}
}

//订阅消息结构体
type okexSubscriber struct {
	Event   string `json:"event"`
	Channel string `json:"channel"`
}

//验证是否是订阅成功消息
//订阅成功后处理数据
func (h *okexHandler) subscribed(msg []byte, w *Worker) {
	subscribe := &okexSubscriber{}
	json.Unmarshal(msg, subscribe)
	if subscribe.Event == "subscribe" {
		w.subscribed(strings.Split(subscribe.Channel, ":")[1])
	}
}
