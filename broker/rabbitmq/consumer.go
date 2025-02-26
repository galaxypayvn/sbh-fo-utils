package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/streadway/amqp"

	"code.finan.one/finan-one-be/fo-utils/l"
)

var ll = l.New()

// IConsumer ...
type IConsumer interface {
	GetReceiver(cfg *QueueConfig) (IReceiver, error)
}

// rabbitmqConsumer ...
type rabbitmqConsumer struct {
	cfg  *Config
	conn *amqp.Connection
}

// GetReceiver ...
func (c *rabbitmqConsumer) GetReceiver(cfg *QueueConfig) (IReceiver, error) {
	r := &receiver{
		rabbitmqConsumer: c,
		queueConfig:      cfg,
		done:             make(chan error),
	}

	var loop = 0
	var err error
	for {
		loop++
		err = r.Connect()
		if err != nil {
			if loop > c.cfg.Retries {
				ll.Fatal("failed to connect to rabbitmq", l.Error(err))
				break
			}
			time.Sleep(30 * time.Second)
			continue
		}
		break
	}

	err = r.SetupQueue()
	if err != nil {
		return nil, err
	}

	return r, nil
}

// NewRabbitmqConsumer ...
func NewRabbitmqConsumer(cfg *Config, ll l.Logger) (IConsumer, error) {
	cfg.Prepare()
	ll.S.Infof("Connecting to rabbitmq on %s", cfg.URI)
	conn, err := amqp.DialConfig(cfg.URI, amqp.Config{
		Vhost: cfg.VHost,
		Dial: func(network, addr string) (net.Conn, error) {
			return net.DialTimeout(network, addr, time.Duration(cfg.NetworkTimeoutInSec)*time.Second)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("dial: %s", err)
	}

	return &rabbitmqConsumer{
		cfg,
		conn,
	}, nil
}

// IReceiver ...
type IReceiver interface {
	Handle(ctx context.Context, handler EventHandler)
}

// EventHandler ...
type EventHandler func(context.Context, <-chan amqp.Delivery) error

type receiver struct {
	*rabbitmqConsumer

	queueConfig *QueueConfig
	channel     *amqp.Channel
	done        chan error
}

func (r *receiver) Connect() error {
	var err error
	go func() {
		// Waits here for the channel to be closed
		ll.S.Debugf("Closing: %s", <-r.conn.NotifyClose(make(chan *amqp.Error)))
		// Let Handle know it's not time to reconnect
		r.done <- errors.New("channel Closed")
	}()
	ll.Info("got Connection, getting Channel")
	r.channel, err = r.conn.Channel()
	if err != nil {
		return fmt.Errorf("channel: %s", err)
	}
	ll.S.Infof("got Channel")
	for _, binding := range r.queueConfig.Bindings {
		ll.Info("declaring Exchange", l.Any("exchange", binding.Exchange))
		if err = r.channel.ExchangeDeclare(
			binding.Exchange.Name,
			binding.Exchange.Type,
			binding.Exchange.Durable,
			binding.Exchange.AutoDelete,
			binding.Exchange.Internal,
			binding.Exchange.NoWait,
			binding.Exchange.Args,
		); err != nil {
			return fmt.Errorf("exchange Declare: %s", err)
		}
	}

	return nil
}

func (r *receiver) SetupQueue() error {
	queue, err := r.channel.QueueDeclare(
		r.queueConfig.Name,
		r.queueConfig.Durable,
		r.queueConfig.AutoDelete,
		r.queueConfig.Exclusive,
		r.queueConfig.NoWait,
		r.queueConfig.Args,
	)
	if err != nil {
		return fmt.Errorf("queue Declare: %s", err)
	}

	for _, binding := range r.queueConfig.Bindings {
		ll.S.Infof("declared Queue (%q %d messages, %d consumers), binding to Exchange %q (key %q)",
			queue.Name, queue.Messages, queue.Consumers, binding.Exchange.Name, binding.RoutingKey)
		if err = r.channel.QueueBind(
			r.queueConfig.Name,
			binding.RoutingKey,
			binding.Exchange.Name,
			binding.NoWait,
			binding.Args,
		); err != nil {
			return fmt.Errorf("queue Bind: %s", err)
		}
	}

	return nil
}

func (r *receiver) reconnect() error {
	ll.Info("Consumer - reconnect")
	time.Sleep(30 * time.Second)

	if err := r.Connect(); err != nil {
		return err
	}

	if err := r.SetupQueue(); err != nil {
		return err
	}

	return nil
}

// Close ...
func (r *receiver) Close() {
	r.done <- errors.New("stop Consumer")
	_ = r.channel.Close()
	_ = r.conn.Close()
}

func (r *receiver) Handle(ctx context.Context, handler EventHandler) {
	deliveries, err := r.channel.Consume(
		r.queueConfig.Name,
		r.cfg.ConsumeConfig.Consumer,
		r.cfg.ConsumeConfig.AutoAck,
		r.cfg.ConsumeConfig.Exclusive,
		r.cfg.ConsumeConfig.NoLocal,
		r.cfg.ConsumeConfig.NoWait,
		r.cfg.ConsumeConfig.Args,
	)

	if err != nil {
		ll.Error("Channel consume failed", l.Error(err))
		return
	}

	for {
		for i := 0; i < r.cfg.MaxThread; i++ {
			go handler(ctx, deliveries)
		}

		// Go into reconnect loop when
		// c.done is passed non nil values
		if e := <-r.done; e != nil {
			if strings.Contains(e.Error(), "Channel Closed") { // retry
				err = r.reconnect()
				retries := 0
				for err != nil {
					// Very likely chance of failing
					// should not cause worker to terminate
					retries++
					if retries > r.cfg.Retries {
						ll.Fatal("Cannot reconnect to rabbitmq")
					}
					err = r.reconnect()
				}
			} else { // stop
				return
			}
		}
	}
}
