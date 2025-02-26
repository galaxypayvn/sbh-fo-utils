package consumer

import (
	"code.finan.one/finan-one-be/fo-utils/broker/mqtt"
	"code.finan.one/finan-one-be/fo-utils/broker/rabbitmq"
	redisqueue "code.finan.one/finan-one-be/fo-utils/broker/redisqueue"
	"code.finan.one/finan-one-be/fo-utils/l"
)

// ConsumerGroup ...
type ConsumerGroup struct {
	ll             l.Logger
	cfg            IConsumerGroupConfig
	mode           string // event_bus, rabbitmq, mqtt, redis_queue
	handlerConfigs map[string]*ConsumerHandlerConfig
}

// NewEventBusConsumerGroup ...
func NewEventBusConsumerGroup(ll l.Logger, chConfigs []*ConsumerHandlerConfig) *ConsumerGroup {
	g := &ConsumerGroup{
		ll:             ll,
		mode:           ModeEventbus,
		handlerConfigs: make(map[string]*ConsumerHandlerConfig),
	}
	for _, val := range chConfigs {
		val.mode = ModeEventbus
		g.handlerConfigs[val.GetKey()] = val
	}
	return g
}

// NewRabbitmqConsumerGroup ...
func NewRabbitmqConsumerGroup(ll l.Logger, cfg *rabbitmq.Config, chConfigs []*ConsumerHandlerConfig) *ConsumerGroup {
	g := &ConsumerGroup{
		cfg:            cfg,
		ll:             ll,
		mode:           ModeRabbitmq,
		handlerConfigs: make(map[string]*ConsumerHandlerConfig),
	}
	for _, val := range chConfigs {
		val.mode = ModeRabbitmq
		g.handlerConfigs[val.GetKey()] = val
	}

	return g
}

// NewMQTTConsumerGroup ...
func NewMQTTConsumerGroup(ll l.Logger, cfg *mqtt.Config, chConfigs []*ConsumerHandlerConfig) *ConsumerGroup {
	g := &ConsumerGroup{
		cfg:            cfg,
		ll:             ll,
		mode:           ModeMQTT,
		handlerConfigs: make(map[string]*ConsumerHandlerConfig),
	}
	for _, val := range chConfigs {
		val.mode = ModeMQTT
		g.handlerConfigs[val.GetKey()] = val
	}
	return g
}

// NewRedisQueueConsumerGroup ...
func NewRedisQueueConsumerGroup(ll l.Logger, cfg *redisqueue.Config, chConfigs []*ConsumerHandlerConfig) *ConsumerGroup {
	// Initialize ConsumerGroup with basic properties
	g := &ConsumerGroup{
		cfg:            cfg,
		ll:             ll,
		mode:           ModeRedisQueue,
		handlerConfigs: make(map[string]*ConsumerHandlerConfig, len(chConfigs)),
	}

	// Configure each handler
	for _, val := range chConfigs {
		val.mode = ModeRedisQueue
		g.handlerConfigs[val.GetKey()] = val
	}

	// Check and confirm QueueLog configuration
	if cfg.QueueLog != "" && g.handlerConfigs[cfg.QueueLog] == nil {
		ll.Fatal("you must declare topic queue log for queue log: " + cfg.QueueLog)
	}

	return g
}
func (c ConsumerGroup) CheckHandleForQueue(mapConsumerHandler map[string]map[string]IConsumerTask) {
	for key, handlerCfg := range c.handlerConfigs { // list config handle but not have handler yet. need to mapping here
		handler, found := mapConsumerHandler[c.mode][key]
		if !found {
			c.ll.Fatal("not found handler for queue",
				l.String("key", key))
		}
		handlerCfg.Handler = handler
	}
}
