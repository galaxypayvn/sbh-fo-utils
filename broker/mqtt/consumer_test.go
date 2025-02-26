package mqtt

import (
	"context"
	"fmt"
	"testing"

	"code.finan.one/finan-one-be/fo-utils/l"
)

func TestNewConsumer(t *testing.T) {
	topic := "foo/bar"
	qos := byte(0)

	ll.Info("Consumer", l.String("topic", topic))
	// create consumer
	broker := "tcp://localhost:1883"
	consumer, err := NewMQTTConsumer(&Config{
		Broker:   broker,
		UserName: "client",
		Password: "pass_client",
	})
	if err != nil {
		t.Error(err)
	}

	receiver := consumer.GetReceiver(topic, qos)

	ctx, cancel := context.WithCancel(context.Background())
	receiver.Handle(ctx, func(ctx context.Context, msg []byte) error {
		defer cancel()
		fmt.Printf("MSG: %s\n", string(msg))
		return nil
	})
}
