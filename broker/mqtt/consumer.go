package mqtt

import (
	"context"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"code.finan.one/finan-one-be/fo-utils/l"
)

var ll = l.New()

// IConsumer ...
type IConsumer interface {
	GetReceiver(topic string, qos byte) IReceiver
}

type mqttConsumer struct {
	consumer mqtt.Client
}

// GetReceiver ...
func (c *mqttConsumer) GetReceiver(topic string, qos byte) IReceiver {
	return &receiver{c, topic, qos}
}
func NewMQTTConsumer(cfg *Config) (IConsumer, error) {
	cfg.Prepare()
	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.Broker) // Add broker here regardless of UserName
	opts.SetConnectTimeout(time.Duration(cfg.ConnectTimeout) * time.Millisecond)
	opts.SetConnectRetryInterval(time.Duration(cfg.ReconnectPeriod) * time.Millisecond)
	opts.AutoReconnect = true
	if len(cfg.UserName) > 0 {
		opts.SetUsername(cfg.UserName)
		opts.SetPassword(cfg.Password)
	}
	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		return nil, token.Error()
	}

	cs := &mqttConsumer{
		consumer: c,
	}
	return cs, nil
}

// IReceiver ...
type IReceiver interface {
	Handle(ctx context.Context, handler EventHandler)
}

type receiver struct {
	*mqttConsumer

	topic string
	qos   byte
}

// EventHandler ...
type EventHandler func(context.Context, []byte) error

// Handle ...
func (r *receiver) Handle(ctx context.Context, handler EventHandler) {
	token := r.consumer.Subscribe(r.topic, r.qos, func(client mqtt.Client, msg mqtt.Message) {
		handler(ctx, msg.Payload())
	})
	token.Wait()
	if token.Error() != nil {
		ll.Error("Consumer failed to subscribe topic", l.String("topic", r.topic), l.Error(token.Error()))
		return
	}

	defer func() {
		_ = r.consumer.Unsubscribe(r.topic)
	}()

	for {
		select {
		case <-ctx.Done():
			ll.Info("Context Done()", l.Error(ctx.Err()))
			return
		}
	}
}
