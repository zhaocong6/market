package market

import (
	"context"
	"github.com/gorilla/websocket"
	"net/http"
	"net/url"
	"time"
)

const runIng = 1
const workerListGcTime = 2

type (
	Handler interface {
		formatSubscribeHandle(*Subscriber) []byte
		pingPongHandle(*Worker)
		resubscribeHandle(*Worker)
		formatMsgHandle(int, []byte, *Worker) (*Marketer, error)
		subscribed(msg []byte, worker *Worker)
	}

	Task interface {
		RunTask()
	}

	Worker struct {
		ctx              context.Context
		wsUrl            string
		Organize         Organize
		Status           int
		LastRunTimestamp time.Duration
		WsConn           *websocket.Conn
		Subscribing      map[string][]byte
		Subscribes       map[string][]byte
		List             Lister
		handler          Handler
	}
)

func (w *Worker) RunTask() {
	var err error

	w.WsConn, err = dial(w.wsUrl)
	if err != nil {
		panic(err)
	}
	defer w.WsConn.Close()

	w.listenHandle()
}

func dial(u string) (*websocket.Conn, error) {
RETRY:
	uProxy, _ := url.Parse("http://127.0.0.1:12333")

	websocket.DefaultDialer = &websocket.Dialer{
		Proxy:            http.ProxyURL(uProxy),
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := websocket.DefaultDialer.Dial(u, nil)
	if err != nil {
		time.Sleep(time.Second * 3)
		goto RETRY
	}

	return conn, nil
}

func (w *Worker) Subscribe(msg []byte) error {
	if w.WsConn != nil {
		err := w.WsConn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			return err
		}
	}

	return nil
}

func (w *Worker) closeRedialSub() error {
	var err error
	w.WsConn.Close()
	w.WsConn, err = dial(w.wsUrl)
	for _, msg := range w.Subscribes {
		w.Subscribe(msg)
	}
	return err
}

func (w *Worker) subscribeHandle(s *Subscriber) {
	w.Subscribing[s.Symbol] = w.handler.formatSubscribeHandle(s)
	w.Subscribe(w.Subscribing[s.Symbol])
}

func (w *Worker) subscribed(symbol string) {
	if sub, ok := w.Subscribing[symbol]; ok {
		w.Subscribes[symbol] = sub
		delete(w.Subscribing, symbol)
	}
}

func (w *Worker) listenHandle() {
	go w.handler.pingPongHandle(w)
	go w.handler.resubscribeHandle(w)
	go w.workerListGc()
	for {
		select {
		case <-w.ctx.Done():
			return
		default:
			msgType, msg, err := w.WsConn.ReadMessage()
			if err != nil {
				err = w.closeRedialSub()
				if err != nil {
					return
				}

				msgType, msg, err = w.WsConn.ReadMessage()
				if err != nil {
					return
				}
			}

			data, err := w.handler.formatMsgHandle(msgType, msg, w)
			if data != nil {
				w.List.Add(data.Symbol, data)
				writeMarketPool.writeRingBuffer(data)
			}
		}
	}
}

func (w *Worker) workerListGc() {
	for {
		select {
		case <-time.NewTimer(workerListGcTime * time.Second).C:
			w.List.gc(workerListGcTime)
		}
	}
}
