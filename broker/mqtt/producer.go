package mqtt

import (
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"

	"code.finan.one/finan-one-be/fo-utils/l"
)

// ISender ...
type ISender interface {
	SendBytes(msg []byte) error
	SendString(msg string) error
}

type IProducer interface {
	GetSender(topic string, qos byte) ISender
}

// MQTTProducer ...
type mqttProducer struct {
	producer mqtt.Client
}

// NewMQTTProducer ...
func NewMQTTProducer(cfg *Config) (IProducer, error) {
	cfg.Prepare()
	clientID := fmt.Sprintf("mqtt_admin_%s", uuid.NewString())
	opts := mqtt.NewClientOptions()
	opts.AddBroker(cfg.Broker) // Add broker here regardless of UserName
	opts.SetClientID(clientID)
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

	p := &mqttProducer{
		producer: c,
	}

	return p, nil
}

// WithTopic ...
func (p *mqttProducer) GetSender(topic string, qos byte) ISender {
	return &sender{p, topic, qos}
}

type sender struct {
	*mqttProducer
	topic string
	qos   byte
}

// SendBytes produce message into mqtt
func (s sender) SendBytes(data []byte) error {
	token := s.producer.Publish(s.topic, s.qos, false, data)
	token.Wait()
	if token.Error() != nil {
		ll.Error("[mqtt.SendBytes] Send event to mqtt failed", l.Error(token.Error()))
		return token.Error()
	}
	// ll.Info("[mqtt.SendBytes] sent message", l.String("topic", s.topic), l.Int("qos", int(s.qos)), l.ByteString("data", data))
	return nil
}

// SendString produce string into mqtt
func (s sender) SendString(msg string) error {
	return s.SendBytes([]byte(msg))
}
