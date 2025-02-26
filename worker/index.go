package worker

import (
	"code.finan.one/finan-one-be/fo-utils/worker/consumer"
	"code.finan.one/finan-one-be/fo-utils/worker/scheduler"
	workerstatus "code.finan.one/finan-one-be/fo-utils/worker/status"
)

// HandlerWOption ...
type HandlerWOption struct {
	MsgHandler
	Replica int64
}

// MsgHandler ...
type MsgHandler interface {
	Handle(msg []byte) workerstatus.ProcessStatus
}
type IWorker interface {
	Start()
}

// Worker ...
type worker struct {
	scheduler *scheduler.Scheduler
	consumer  *consumer.Consumer
}

func (w *worker) Start() {
	if w.scheduler != nil {
		go w.scheduler.Start()
	}
	if w.consumer != nil {
		go w.consumer.Start()
	}
}
