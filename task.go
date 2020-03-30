package market

import (
	"context"
	"github.com/gorilla/websocket"
	"github.com/zhaocong6/goUtils/chanlock"
	"log"
	"net/http"
	"sync"
	"time"
)

//task运行中状态
const runIng = 1

//worker list gc时间
const workerListGcTime = 2

type (

	//各个交易所handle接口
	Handler interface {
		formatSubscribeHandle(*Subscriber) []byte                //格式化订阅消息, 转化成统一的sub
		pingPongHandle(*Worker)                                  //ping pong机制
		formatMsgHandle(int, []byte, *Worker) (*Marketer, error) //处理ws返回数据
		subscribed(msg []byte, worker *Worker)                   //处理订阅成功后的业务
	}

	//worker基础
	Worker struct {
		ctx              context.Context   //context
		wsUrl            string            //ws地址
		Organize         Organize          //交易所
		Status           int               //状态
		LastRunTimestamp time.Duration     //最后运行时间
		WsConn           *websocket.Conn   //ws连接
		Subscribing      map[string][]byte //订阅中数据
		Subscribes       map[string][]byte //订阅成功数据
		subLock          sync.Mutex
		List             *Lister           //订阅成功返回后的行情数据list
		handler          Handler           //handel接口
		redialLock       chanlock.ChanLock //重连并发锁
		lastRedialTime   int64             //最后一次重连时间
	}

	coJob struct {
		w       *Worker
		msgType int
		msg     []byte
	}
)

//运行task
//ws连接
//数据监听
func (w *Worker) RunTask() {
	log.Printf("%s 服务启动", w.Organize)
	w.WsConn, _ = dial(w.wsUrl)
	defer func() {
		w.WsConn.Close()
		log.Printf("%s 服务关闭", w.Organize)
	}()

	w.listenHandle()
}

var DefaultDialer = &websocket.Dialer{
	Proxy:            http.ProxyFromEnvironment,
	HandshakeTimeout: 10 * time.Second,
}

//ws连接
//失败后3秒重新连接
//直到连接成功
func dial(u string) (*websocket.Conn, error) {
	log.Printf("%s 连接中.", u)

RETRY:
	conn, _, err := DefaultDialer.Dial(u, nil)
	if err != nil {
		time.Sleep(time.Second * 3)
		log.Println(err)
		goto RETRY
	}

	log.Printf("%s 连接成功.", u)
	return conn, nil
}

//发送订阅
func (w *Worker) Subscribe(msg []byte) error {
	if w.WsConn != nil {
		err := w.WsConn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			return err
		}
	}

	return nil
}

//关闭连接
//重新创建一个连接
//发送订阅
func (w *Worker) closeRedialSub() error {
	if w.redialLock.TryLock(time.Millisecond) == false {
		return nil
	}

	defer func() {
		if w.lastRedialTime != 0 {
			w.lastRedialTime = time.Now().Unix()
		}

		w.redialLock.Unlock()
	}()

	if (time.Now().Unix() - w.lastRedialTime) < 10 {
		return nil
	}

	log.Printf("%s 断线重连", w.Organize)

	var err error
	w.WsConn.Close()
	w.lastRedialTime = time.Now().Unix()
	w.WsConn, err = dial(w.wsUrl)

	w.subLock.Lock()
	defer w.subLock.Unlock()

	for k, v := range w.Subscribes {
		w.Subscribing[k] = v
		delete(w.Subscribes, k)
		w.Subscribe(w.Subscribing[k])
	}
	return err
}

//处理订阅数据格式
//订阅
func (w *Worker) subscribeHandle(s *Subscriber) {
	w.subLock.Lock()
	defer w.subLock.Unlock()

	w.Subscribing[s.Symbol] = w.handler.formatSubscribeHandle(s)
	w.Subscribe(w.Subscribing[s.Symbol])
}

//处理订阅成功
func (w *Worker) subscribed(symbol string) {
	w.subLock.Lock()
	defer w.subLock.Unlock()

	if sub, ok := w.Subscribing[symbol]; ok {
		w.Subscribes[symbol] = sub
		delete(w.Subscribing, symbol)
	}
}

//重新订阅Subscribing中的数据
func (w *Worker) resubscribeHandle() {
	for {
		select {
		case <-time.NewTimer(time.Second * 5).C:
			for _, sub := range w.Subscribing {
				w.Subscribe(sub)
			}
		}
	}
}

//监听
//创建ping pong事件处理协程
//创建重新订阅事件协程
//创建list gc协作程
func (w *Worker) listenHandle() {
	go w.handler.pingPongHandle(w)
	go w.resubscribeHandle()
	go w.workerListGc()
	for {
		select {
		//等待关闭事件
		case <-w.ctx.Done():
			return
		default:
			//等待ws数据
			msgType, msg, err := w.WsConn.ReadMessage()
			if err != nil {
				err = w.closeRedialSub()
				if err != nil {
					return
				}
				time.Sleep(time.Second)
				continue
			}

			Manage.pool.Put(&coJob{
				w:       w,
				msgType: msgType,
				msg:     msg,
			})
		}
	}
}

//设置一个gc定时器
func (w *Worker) workerListGc() {
	for {
		select {
		case <-time.NewTimer(workerListGcTime * time.Second).C:
			w.List.gc(workerListGcTime)
		}
	}
}

func (c coJob) Handle() error {
	data, err := c.w.handler.formatMsgHandle(c.msgType, c.msg, c.w)
	if err != nil {
		return err
	}

	//拷贝两份指针
	//list用于被动查询
	//pool用于主动通信
	if data != nil {
		c.w.List.Add(data.Symbol, data)
		writeMarketPool.writeRingBuffer(data)
	}
	return nil
}
