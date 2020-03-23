package market

import (
	"context"
	"fmt"
)

//manage结构体
//用于管理task任务, 和关闭task运行任务
//使用context通信
var Manage struct {
	Tasks  map[Organize]*Worker
	Ctx    context.Context
	Cancel context.CancelFunc
}

func init() {
	Manage.Ctx, Manage.Cancel = context.WithCancel(context.Background())
	Manage.Tasks = map[Organize]*Worker{}
	Manage.Tasks[OkEx] = newOkEx(Manage.Ctx)
	Manage.Tasks[HuoBi] = newHuoBi(Manage.Ctx)
}

//运行work
func Run() {
	for _, t := range Manage.Tasks {

		go func(t *Worker) {

			defer func() {
				if err := recover(); err != nil {
					fmt.Println(err)
				}
			}()

			t.RunTask()
		}(t)
	}

	go func() {

		defer func() {
			if err := recover(); err != nil {
				fmt.Println(err)
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

//订阅请求统一处理
func subscribeHandle() {
	for {
		select {
		case sub := <-readSubscribing:
			Manage.Tasks[sub.Organize].subscribeHandle(sub)
		}
	}
}
