package eventbus

import (
	"context"

	"github.com/asaskevich/EventBus"

	"code.finan.one/finan-one-be/fo-utils/broker/errors"
	"code.finan.one/finan-one-be/fo-utils/l"
)

var ll = l.New()

// IConsumer ...
type IConsumer interface {
	GetReceiver(topic string) IReceiver
}

// EventBusConsumer ...
type eventBusConsumer struct {
	consumer EventBus.Bus
}

// GetReceiver ...
func (c *eventBusConsumer) GetReceiver(topic string) IReceiver {
	return &receiver{c, topic}
}

func NewEventBusConsumer() IConsumer {
	cs := &eventBusConsumer{
		consumer: EventBus.New(),
	}
	return cs
}

// IReceiver ...
type IReceiver interface {
	Handle(ctx context.Context, handler EventHandler)
}

type receiver struct {
	*eventBusConsumer

	topic string
}

// EventHandler ...
type EventHandler func(context.Context, []byte) error

// Handle ...
func (r *receiver) Handle(ctx context.Context, handler EventHandler) {
	err := r.consumer.Subscribe(r.topic, handler)
	if err != nil {
		processError(err)
	}
}

func processError(err error) {
	errCode := errors.ErrCode(err)

	switch errCode {
	case errors.CodeOK:
		return

	case errors.CodeNoTube:
		ll.Info("No tube specified, skipped pushing job", l.Error(err))

	case errors.CodeServiceUnavailable:
		ll.Fatal("Connection problem", l.Error(err), l.String("code", errCode.String()))

	default:
		ll.Error("Unable to handle base-event", l.Error(err), l.String("code", errCode.String()))
	}
}
