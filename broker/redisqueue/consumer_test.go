package redisqueue

import (
	"context"
	"testing"

	"github.com/gocraft/work"

	"code.finan.one/finan-one-be/fo-utils/l"
)

func TestNewConsumer(t *testing.T) {
	topic := "test_work"
	ll.Info("Consumer", l.String("topic", topic))
	// create consumer
	consumer, err := NewConsumer(&Config{
		Address:  "redis://localhost:6379",
		Password: "redis",
		Database: 5,
	})
	if err != nil {
		t.Error(err)
	}

	receiver := consumer.GetReceiver(topic)

	ctx := context.Background()
	receiver.Handle(ctx, JobOptions{MaxConcurrency: 1}, handler)
}

var handler EventHandler = func(c *WContext, job *work.Job) error {
	ll.Info("Result", l.Any("args", job.Args))
	return nil
}
