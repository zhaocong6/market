package market

import (
	"context"
	"fmt"
)

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

func Close() {
	Manage.Cancel()
}

func subscribeHandle() {
	for {
		select {
		case sub := <-readSubscribing:
			Manage.Tasks[sub.Organize].subscribeHandle(sub)
		}
	}
}
