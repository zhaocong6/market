package market

import (
	"encoding/json"
	"strconv"
	"sync"
	"time"
)

//深度数据格式
type Depth [][2]string

//格式化浮点深度数据
//最大格式化float64
func (d Depth) formatFloat(params [][2]float64) Depth {
	val := make(Depth, len(params))

	for k, v := range params {
		val[k][0] = strconv.FormatFloat(v[0], 'g', -1, 64)
		val[k][1] = strconv.FormatFloat(v[1], 'g', -1, 64)
	}

	return val
}

//基础行情结构
type Marketer struct {
	Organize  Organize      `json:"organize"`   //交易所
	Symbol    string        `json:"symbol"`     //订阅币对
	BuyFirst  string        `json:"buy_first"`  //买一价格
	SellFirst string        `json:"sell_first"` //卖一价格
	BuyDepth  Depth         `json:"buy_depth"`  //市场买深度
	SellDepth Depth         `json:"sell_depth"` //市场卖深度
	Timestamp time.Duration `json:"timestamp"`  //数据更新时间(毫秒)
}

//序列化为json
func (m *Marketer) MarshalJson() string {
	j, _ := json.Marshal(m)
	return string(j)
}

//基础的lister类型
//主要为了实现主动查询
type Lister struct {
	data map[string]*Marketer
	lock sync.RWMutex
}

func newList() *Lister {
	return &Lister{
		data: make(map[string]*Marketer),
	}
}

//序列化为json
func (l *Lister) MarshalJson() string {
	l.lock.RLock()
	defer l.lock.RUnlock()

	j, _ := json.Marshal(l.data)
	return string(j)
}

//追加一条数据
//如果数据已经存在, 则更新数据
func (l *Lister) Add(k string, m *Marketer) {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.data[k] = m
}

//删除一条数据
func (l *Lister) Del(k string) {
	l.lock.Lock()
	defer l.lock.Unlock()

	delete(l.data, k)
}

//查找一个或者多个key
//返回一个新的lister结构体
func (l *Lister) Find(s ...string) *Lister {
	l.lock.RLock()
	defer l.lock.RUnlock()

	newL := newList()
	for _, k := range s {
		if v, ok := l.data[k]; ok {
			newL.data[k] = v
		}
	}

	return newL
}

func (l *Lister) ToMap() map[string]*Marketer {
	return l.data
}

//lister gc机制
//exs单位是秒. 表示数据过期的时间
//数据过期后删除
func (l *Lister) gc(exs time.Duration) {
	t := time.Duration(time.Now().UnixNano() / 1e6)
	l.lock.Lock()
	defer l.lock.Unlock()
	for k, v := range l.data {
		if (t - v.Timestamp) > exs {
			delete(l.data, k)
		}
	}
}

//交易类型
type marketType int

//币币交易/现货交易类型
const SpotMarket marketType = 1

//期货交易/交割交易类型
const FuturesMarket marketType = 2

//永续交易类型
const WapMarket marketType = 3

//期权交易类型
const OptionMarket marketType = 4

//平台常量类型
type Organize string

//火币平台常量
const HuoBi Organize = "huobi"

//okex平台常量
const OkEx Organize = "okex"

//外部订阅时的结构体
type Subscriber struct {
	Symbol     string
	Organize   Organize
	MarketType marketType
}

//只允许写入Subscriber channel
//暴露给外部使用
var WriteSubscribing chan<- *Subscriber

//只允许读取Subscriber channel
//不允许外部使用
var readSubscribing <-chan *Subscriber

func init() {
	var subscribing = make(chan *Subscriber, 2)
	WriteSubscribing = subscribing
	readSubscribing = subscribing
}

//只允许读取market channel
type readMarketer <-chan *Marketer

//只允许写入market channel
type writeMarketer struct {
	buffer chan<- *Marketer
	lock   sync.Mutex
}

var readWriteMarketer = make(chan *Marketer, 1000)

//读取暴露给外部使用
var ReadMarketPool readMarketer = readWriteMarketer

//写入数据只能内部使用
var writeMarketPool struct {
	writeMarketer
}

func init() {
	writeMarketPool.buffer = readWriteMarketer
}

//使用channel对market实现环形数据结构
//超过channel缓存时, 删除过期的值
//主动停止timer, 防止可能的内存泄露
func (w writeMarketer) writeRingBuffer(m *Marketer) {
	w.lock.Lock()
	defer func() {
		w.lock.Unlock()
	}()

	if len(w.buffer) == cap(w.buffer) {
		select {
		case <-ReadMarketPool:
		default:
		}
	}
	w.buffer <- m
}
