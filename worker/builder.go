package worker

import (
	"context"

	"code.finan.one/finan-one-be/fo-utils/broker/mqtt"
	"code.finan.one/finan-one-be/fo-utils/broker/rabbitmq"
	redisqueue "code.finan.one/finan-one-be/fo-utils/broker/redisqueue"
	"code.finan.one/finan-one-be/fo-utils/l"
	"code.finan.one/finan-one-be/fo-utils/worker/consumer"
	"code.finan.one/finan-one-be/fo-utils/worker/scheduler"
)

// Builder ...
type Builder interface {
	Build() IWorker
	BuildSchedule() Builder
	WithScheduleConfig(cfg *scheduler.ScheduleHandlerConfig) Builder
	BuildConsumer() Builder
	WithEventBusConsumerConfig(chConfigs []*consumer.ConsumerHandlerConfig) Builder
	WithEventBusConsumerHandler(key string, h consumer.IConsumerTask) Builder
	WithRabbitmqConsumerConfig(cfg *rabbitmq.Config, chConfigs []*consumer.ConsumerHandlerConfig) Builder
	WithRabbitmqConsumerHandler(key string, h consumer.IConsumerTask) Builder
	WithMQTTConsumerConfig(cfg *mqtt.Config, chConfigs []*consumer.ConsumerHandlerConfig) Builder
	WithMQTTConsumerHandler(key string, h consumer.IConsumerTask) Builder
	WithRedisQueueConsumerConfig(cfg *redisqueue.Config, chConfigs []*consumer.ConsumerHandlerConfig) Builder
	WithRedisQueueConsumerHandler(key string, h consumer.IConsumerTask) Builder
}

type builder struct {
	ll  l.Logger
	ctx context.Context

	// store prepare config to init schedule
	scheduler      *scheduler.Scheduler
	schedulerGroup []*scheduler.SchedulerGroup
	// mapScheduleHandler map[string]scheduler.ISchedulerTask

	// store prepare config to init consumer
	consumer           *consumer.Consumer
	consumerGroup      []*consumer.ConsumerGroup
	mapConsumerHandler map[string]map[string]consumer.IConsumerTask
}

// New ...
func New(ctx context.Context, ll l.Logger) Builder {
	return &builder{
		ctx: ctx,
		ll:  ll,
		// mapScheduleHandler: make(map[string]scheduler.ISchedulerTask),
		mapConsumerHandler: make(map[string]map[string]consumer.IConsumerTask),
	}
}

func (b *builder) Build() IWorker {
	w := &worker{
		scheduler: b.scheduler,
		consumer:  b.consumer,
	}
	return w
}

func (b *builder) WithEventBusConsumerConfig(chConfigs []*consumer.ConsumerHandlerConfig) Builder {
	g := consumer.NewEventBusConsumerGroup(b.ll, chConfigs)
	// init consumerGroup with group and topic but not have handler
	b.consumerGroup = append(b.consumerGroup, g)
	return b
}

func (b *builder) WithEventBusConsumerHandler(key string, h consumer.IConsumerTask) Builder {
	if b.mapConsumerHandler[consumer.ModeEventbus] == nil {
		b.mapConsumerHandler[consumer.ModeEventbus] = make(map[string]consumer.IConsumerTask)
	}
	b.mapConsumerHandler[consumer.ModeEventbus][key] = h
	return b
}

// WithScheduleConfig ...
func (b *builder) WithScheduleConfig(cfg *scheduler.ScheduleHandlerConfig) Builder {
	g := scheduler.NewSchedulerGroup(cfg)
	// init consumerGroup with group and topic but not have handler
	b.schedulerGroup = append(b.schedulerGroup, g)
	return b
}

// // WithScheduleHandler ...
// func (b *builder) WithScheduleHandler(key string, h scheduler.ISchedulerTask) Builder {
// 	b.mapScheduleHandler[key] = h
// 	return b
// }

// BuildSchedule ...
func (b *builder) BuildSchedule() Builder {
	if len(b.schedulerGroup) == 0 {
		return b
	}
	b.scheduler = scheduler.NewScheduler(b.ctx, b.schedulerGroup, b.ll)
	return b
}

func (b *builder) WithRabbitmqConsumerConfig(cfg *rabbitmq.Config, chConfigs []*consumer.ConsumerHandlerConfig) Builder {
	g := consumer.NewRabbitmqConsumerGroup(b.ll, cfg, chConfigs)
	// init consumerGroup with group and topic but not have handler
	b.consumerGroup = append(b.consumerGroup, g)
	return b
}

func (b *builder) WithRabbitmqConsumerHandler(key string, h consumer.IConsumerTask) Builder {
	if b.mapConsumerHandler[consumer.ModeRabbitmq] == nil {
		b.mapConsumerHandler[consumer.ModeRabbitmq] = make(map[string]consumer.IConsumerTask)
	}
	b.mapConsumerHandler[consumer.ModeRabbitmq][key] = h
	return b
}

func (b *builder) WithMQTTConsumerConfig(cfg *mqtt.Config, chConfigs []*consumer.ConsumerHandlerConfig) Builder {
	g := consumer.NewMQTTConsumerGroup(b.ll, cfg, chConfigs)
	// init consumerGroup with group and topic but not have handler
	b.consumerGroup = append(b.consumerGroup, g)
	return b
}

func (b *builder) WithMQTTConsumerHandler(key string, h consumer.IConsumerTask) Builder {
	if b.mapConsumerHandler[consumer.ModeMQTT] == nil {
		b.mapConsumerHandler[consumer.ModeMQTT] = make(map[string]consumer.IConsumerTask)
	}
	b.mapConsumerHandler[consumer.ModeMQTT][key] = h
	return b
}

func (b *builder) WithRedisQueueConsumerConfig(cfg *redisqueue.Config, chConfigs []*consumer.ConsumerHandlerConfig) Builder {
	g := consumer.NewRedisQueueConsumerGroup(b.ll, cfg, chConfigs)
	// init consumerGroup with group and topic but not have handler
	b.consumerGroup = append(b.consumerGroup, g)
	return b
}

func (b *builder) WithRedisQueueConsumerHandler(key string, h consumer.IConsumerTask) Builder {
	if b.mapConsumerHandler[consumer.ModeRedisQueue] == nil {
		b.mapConsumerHandler[consumer.ModeRedisQueue] = make(map[string]consumer.IConsumerTask)
	}
	b.mapConsumerHandler[consumer.ModeRedisQueue][key] = h
	return b
}

// BuildConsumer ... map handler to config
func (b *builder) BuildConsumer() Builder {
	if len(b.consumerGroup) == 0 {
		return b
	}

	if b.mapConsumerHandler != nil {
		for _, c := range b.consumerGroup { // list config group
			c.CheckHandleForQueue(b.mapConsumerHandler)
		}
	}
	b.consumer = consumer.NewConsumer(b.ctx, b.ll, b.consumerGroup)

	return b
}
