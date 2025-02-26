package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"sync"
	"time"

	"code.finan.one/finan-one-be/fo-utils/broker/mqtt"
	"code.finan.one/finan-one-be/fo-utils/broker/redisqueue"

	"github.com/gocraft/work"
	"github.com/streadway/amqp"

	"code.finan.one/finan-one-be/fo-utils/broker/eventbus"
	"code.finan.one/finan-one-be/fo-utils/broker/rabbitmq"
	"code.finan.one/finan-one-be/fo-utils/l"
	workerstatus "code.finan.one/finan-one-be/fo-utils/worker/status"
)

var stackSize int = 4 << 10 // 4 KB

// Consumer ...
type Consumer struct {
	ctx         context.Context
	lock        sync.Mutex
	ll          l.Logger
	ConsumerJob []*ConsumerGroup
}

// NewConsumer ...
func NewConsumer(ctx context.Context, ll l.Logger, consumerGroup []*ConsumerGroup) *Consumer {
	return &Consumer{
		ctx:         ctx,
		lock:        sync.Mutex{},
		ll:          ll,
		ConsumerJob: consumerGroup,
	}
}

// Close ...
func (s *Consumer) Close() {
	s.ll.Info("[Consumer] Consumer closed")
}

// Start ...
func (s *Consumer) Start() {
	if len(s.ConsumerJob) == 0 {
		return
	}

	s.ll.Info("[Consumer] Start Consumer")
	var (
		wait                sync.WaitGroup
		redisqueueMode      redisqueue.IConsumer
		redisqueueModeOnce  sync.Once
		redisqueueModeMutex sync.Mutex
	)
	// Start consumers for each job configuration
	for _, item := range s.ConsumerJob {
		for _, consumerJobCfg := range item.handlerConfigs {
			wait.Add(1)
			switch item.mode {
			case ModeEventbus:
				go func(c eventbus.IConsumer, cfg *ConsumerHandlerConfig) {
					defer wait.Done()
					s.startConsumerEventBus(c, cfg, &wait)
				}(eventbus.NewEventBusConsumer(), consumerJobCfg)

			case ModeRabbitmq:
				go func(cfg *rabbitmq.Config, consumerCfg *ConsumerHandlerConfig) {
					defer wait.Done()
					c, err := rabbitmq.NewRabbitmqConsumer(cfg, s.ll)
					if err != nil {
						s.ll.Error("[Consumer] failed to create rabbitmq consumer", l.Error(err))
						return
					}
					s.startConsumerRabbitmq(c, consumerCfg, &wait)
				}(item.cfg.(*rabbitmq.Config), consumerJobCfg)

			case ModeMQTT:
				go func(cfg *mqtt.Config, consumerCfg *ConsumerHandlerConfig) {
					defer wait.Done()
					c, err := mqtt.NewMQTTConsumer(cfg)
					if err != nil {
						s.ll.Error("[Consumer] failed to create mqtt consumer", l.Error(err))
						return
					}
					s.startConsumerMQTT(c, consumerCfg, &wait)
				}(item.cfg.(*mqtt.Config), consumerJobCfg)

			case ModeRedisQueue:
				go func(consumerCfg *ConsumerHandlerConfig) {
					defer wait.Done()
					groupConfig := item.cfg.(*redisqueue.Config)
					redisqueueMode, _ = redisqueue.NewConsumer(groupConfig)
					s.startConsumerRedisQueue(redisqueueMode, consumerCfg, &wait)
				}(consumerJobCfg)

			case ModeRedisQueueTmp:
				go func(consumerCfg *ConsumerHandlerConfig) {
					defer wait.Done()
					redisqueueModeMutex.Lock()         // Lock to prevent concurrent access
					defer redisqueueModeMutex.Unlock() // Ensure unlock after use
					// TODO: Sử dụng sync.Once để đảm bảo chỉ tạo một lần, tránh tạo nhiều worker pool lúc băn log trong middleware, sẽ ko có woker nào xử lý do khác pool
					redisqueueModeOnce.Do(func() {
						groupConfig := item.cfg.(*redisqueue.Config)
						redisqueueMode, _ = redisqueue.NewConsumer(groupConfig)
					})
					s.startConsumerRedisQueue(redisqueueMode, consumerCfg, &wait)
				}(consumerJobCfg)
			default:
				s.ll.Warn("[Consumer] unknown mode", l.Any("mode", item.mode))
				wait.Done()
			}
		}
	}

	wait.Wait()
	s.ll.Info("[Consumer] Exit consumers")
}

func (s *Consumer) startConsumerEventBus(c eventbus.IConsumer, consumerJobCfg *ConsumerHandlerConfig, wait *sync.WaitGroup) {
	defer wait.Done()
	r := c.GetReceiver(consumerJobCfg.Topic)
	r.Handle(s.ctx, func(ctx context.Context, msg []byte) error {
		defer func() {
			if r := recover(); r != nil {
				stack := make([]byte, stackSize)
				length := runtime.Stack(stack, true)
				s.ll.Error(
					"[EventBus] have a panic when process event_bus base-event",
					l.String("eventbus-err", string(stack[:length])),
					l.String("eventbus-key", consumerJobCfg.GetKey()),
					l.String("eventbus-data", string(msg)),
				)
			}
		}()

		// Call handler and get result
		_ = consumerJobCfg.Handler.Handle(msg)
		// TODO: handle response code
		return nil
	})
}

func (s *Consumer) startConsumerRabbitmq(c rabbitmq.IConsumer, consumerJobCfg *ConsumerHandlerConfig, wait *sync.WaitGroup) {
	defer wait.Done()
	r, err := c.GetReceiver(&consumerJobCfg.QueueConfig)
	if err != nil {
		s.ll.Error("[ConsumerRabbitmq] failed to get consumer", l.Error(err))
		return
	}
	r.Handle(s.ctx, func(ctx context.Context, deliveries <-chan amqp.Delivery) error {
		for {
			select {
			case delivery := <-deliveries:
				func(delivery amqp.Delivery) {
					defer func() {
						if r := recover(); r != nil {
							stack := make([]byte, stackSize)
							length := runtime.Stack(stack, true)
							s.ll.Error(
								"have a panic when process rabbitmq base-event",
								l.String("rabbitmq-err", string(stack[:length])),
								l.String("rabbitmq-queue", consumerJobCfg.QueueConfig.Name),
								l.String("rabbitmq-data", string(delivery.Body)),
							)
						}
					}()
					if len(delivery.Body) == 0 {
						return
					}

					s.ll.Info("[ConsumerRabbitmq] Received base-event from Rabbitmq", l.String("queue_name", consumerJobCfg.QueueConfig.Name), l.String("payload", string(delivery.Body)))
					event := &Event{}
					err = json.Unmarshal(delivery.Body, event)
					if err != nil {
						s.ll.Error("[ConsumerRabbitmq] failed to parse msg", l.Error(err), l.String("Received base-event", string(delivery.Body)))
						return
					}

					// Call handler and get result
					resp := consumerJobCfg.Handler.Handle(delivery.Body)
					switch resp.Code {
					case workerstatus.Success:
						if !consumerJobCfg.ConsumeConfig.AutoAck {
							err = delivery.Ack(false)
							if err != nil {
								s.ll.Error("[ConsumerRabbitmq] failed to ack delivery for retry", l.Error(err))
							}
						}
					case workerstatus.Retry:
						if !consumerJobCfg.ConsumeConfig.AutoAck {
							err = delivery.Ack(false)
							if err != nil {
								s.ll.Error("[ConsumerRabbitmq] failed to ack delivery for retry", l.Error(err))
							}
						}

						// TODO: need implement to retry
						/*err = delivery.Nack(false, true)
						if err != nil {
							s.ll.Error("[ConsumerRabbitmq] failed to nack delivery for retry", l.Error(err))
						}*/

					case workerstatus.Drop:
						if !consumerJobCfg.ConsumeConfig.AutoAck {
							err = delivery.Ack(false)
							if err != nil {
								s.ll.Error("[ConsumerRabbitmq] failed to ack delivery for retry", l.Error(err))
							}
						}
					default:
						if !consumerJobCfg.ConsumeConfig.AutoAck {
							err = delivery.Ack(false)
							if err != nil {
								s.ll.Error("[ConsumerRabbitmq] failed to ack delivery for retry", l.Error(err))
							}
						}
					}
				}(delivery)
			case <-s.ctx.Done():
				s.Close()
				s.ll.S.Infof("[ConsumerRabbitmq] Exiting ... %v", consumerJobCfg.QueueConfig.Name)
				return nil
			default:
				// When there are no new messages or stop signals, the loop will pause for a short period of time (100ms) to reduce CPU load.
				time.Sleep(100 * time.Millisecond)
			}
		}
	})
}

func (s *Consumer) startConsumerMQTT(c mqtt.IConsumer, consumerJobCfg *ConsumerHandlerConfig, wait *sync.WaitGroup) {
	defer wait.Done()
	r := c.GetReceiver(consumerJobCfg.Topic, 2)
	r.Handle(s.ctx, func(ctx context.Context, msg []byte) error {
		defer func() {
			if r := recover(); r != nil {
				stack := make([]byte, stackSize)
				length := runtime.Stack(stack, true)
				s.ll.Error(
					"[ConsumerMQTT] have a panic when process mqtt message",
					l.String("mqtt-err", string(stack[:length])),
					l.String("mqtt-key", consumerJobCfg.GetKey()),
					l.String("mqtt-data", string(msg)),
				)
			}
		}()
		s.ll.Info("[ConsumerMQTT] Received message from MQTT", l.String("topic", consumerJobCfg.Topic))

		// Call handler and get result
		_ = consumerJobCfg.Handler.Handle(msg)
		// TODO: handle response code
		return nil
	})
}

func (s *Consumer) startConsumerRedisQueue(c redisqueue.IConsumer, consumerJobCfg *ConsumerHandlerConfig, wait *sync.WaitGroup) {
	defer wait.Done()
	r := c.GetReceiver(consumerJobCfg.Topic)
	r.Handle(s.ctx, consumerJobCfg.JobOptions, func(c *redisqueue.WContext, job *work.Job) error {
		defer func() {
			if r := recover(); r != nil {
				stack := make([]byte, stackSize)
				length := runtime.Stack(stack, true)
				s.ll.Error(
					"[ConsumerRedisQueue] have a panic when process redis message",
					l.String("redisqueue-err", string(stack[:length])),
					l.String("redisqueue-key", consumerJobCfg.GetKey()),
					l.Any("redisqueue-args", job.Args),
				)
			}
		}()
		data, err := json.Marshal(job.Args)
		if err != nil {
			return err
		}

		// Call handler and get result
		resp := consumerJobCfg.Handler.Handle(data)

		switch resp.Code {
		case workerstatus.Retry:
			if len(resp.Message) == 0 {
				return errors.New("job failed, retry requested, err: no additional information")
			}
			return errors.New("job failed, retry requested, err: " + string(resp.Message))
		case workerstatus.Drop:
			// If you need to drop the job, don't retry and consider it done
			if len(resp.Message) == 0 {
				return errors.New("job failed, end requested, err: no additional information")
			}
			return errors.New("job failed, end requested, err: " + string(resp.Message))
		case workerstatus.Success:
			// If successful, complete the job
			return nil
		default:
			if len(resp.Message) == 0 {
				return errors.New("unexpected status, retry requested, err: no additional information")
			}
			return errors.New("unexpected status, retry requested, err: " + string(resp.Message))
		}
	})
}
