package consumer

import (
	"encoding/json"
	"fmt"

	"code.finan.one/finan-one-be/fo-utils/broker/redisqueue"

	"code.finan.one/finan-one-be/fo-utils/broker/rabbitmq"
	wokerstatus "code.finan.one/finan-one-be/fo-utils/worker/status"
)

const (
	ModeEventbus      = "event_bus"
	ModeRabbitmq      = "rabbitmq"
	ModeMQTT          = "mqtt"
	ModeRedisQueue    = "redis_queue"
	ModeRedisQueueTmp = "redis_queue_tmp"
)

// Event ...
type Event struct {
	ExchangeName   string           `json:"exchangeName"`
	RoutingKey     string           `json:"routingKey"`
	IssueAt        int64            `json:"issueAt"`
	Issuer         string           `json:"issuer"`
	MessageVersion string           `json:"messageVersion"`
	RawMessage     *json.RawMessage `json:"base-event"`
}

// IConsumerTask ...
type IConsumerTask interface {
	Handle(msg []byte) wokerstatus.ProcessStatus
}

type IConsumerGroupConfig interface{}

// ConsumerHandlerConfig ...
type ConsumerHandlerConfig struct {
	// config for rabbitmq
	QueueConfig   rabbitmq.QueueConfig   `json:"queue_config" mapstructure:"queue_config"`
	ConsumeConfig rabbitmq.ConsumeConfig `json:"consume_config" mapstructure:"consume_config"`

	// config for redis
	JobOptions redisqueue.JobOptions `json:"job_options" mapstructure:"job_options"`

	// config
	// topic for event_bus, mqtt, redis queue
	GroupID string `json:"group_id" mapstructure:"group_id"`
	Topic   string `json:"topic" mapstructure:"topic"`

	// config general
	Handler IConsumerTask `json:"handler" mapstructure:"handler"`

	mode string // pass from ConsumerGroup
}

func (o ConsumerHandlerConfig) GetKey() string {
	switch o.mode {
	case ModeEventbus:
		return o.Topic
	case ModeRabbitmq:
		return o.QueueConfig.Name
	case ModeMQTT:
		return o.Topic
	case ModeRedisQueue:
		return o.Topic
	default:
		panic(fmt.Errorf("unknown mode: %v", o.mode))
	}
}
