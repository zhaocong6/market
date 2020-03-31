package market

import (
	"context"
	"github.com/zhaocong6/goUtils/goroutinepool"
	"log"
	"runtime/debug"
)

//manage结构体
//用于管理task任务, 和关闭task运行任务
//使用context通信
var Manage struct {
	tasks  map[Organize]*Worker
	Ctx    context.Context
	Cancel context.CancelFunc
	pool   *goroutinepool.Worker
}

func init() {
	Manage.Ctx, Manage.Cancel = context.WithCancel(context.Background())

	Manage.pool = goroutinepool.NewPool(goroutinepool.Options{
		Capacity:  20,
		JobBuffer: 500,
	})

	Manage.tasks = map[Organize]*Worker{}
	Manage.tasks[OkEx] = newOkEx(Manage.Ctx)
	Manage.tasks[HuoBi] = newHuoBi(Manage.Ctx)
}

//运行work
func Run() {
	for _, t := range Manage.tasks {

		go func(t *Worker) {

			defer func() {
				if err := recover(); err != nil {
					log.Println(err, string(debug.Stack()))
				}
			}()

			t.RunTask()
		}(t)
	}

	go func() {

		defer func() {
			if err := recover(); err != nil {
				log.Println(err, string(debug.Stack()))
			}
		}()

		subscribeHandle()
	}()
}

//关闭task
//使用context 通信
func Close() {
	Manage.Cancel()
}

func Find(organize string, symbol ...string) (m map[string]*Marketer) {
	switch organize {
	case "huobi":
		m = Manage.tasks[HuoBi].List.Find(symbol...).ToMap()
	case "okex":
		m = Manage.tasks[OkEx].List.Find(symbol...).ToMap()
	}
	return m
}

//订阅请求统一处理
func subscribeHandle() {
	for {
		select {
		case sub := <-readSubscribing:
			Manage.tasks[sub.Organize].subscribeHandle(sub)
		}
	}
}
