package mqtt

import (
	"testing"

	"code.finan.one/finan-one-be/fo-utils/l"
)

func TestNewProducer(t *testing.T) {
	topic := "foo/bar"
	qos := byte(0)
	broker := "tcp://localhost:1883"
	ll.Info("Test MQTT Producer", l.String("topic", topic))

	{
		producer, err := NewMQTTProducer(&Config{
			Broker:   broker,
			UserName: "root",
			Password: "pass_root",
		})

		if err != nil {
			t.Error(err)
			return
		}

		sender := producer.GetSender(topic, qos)

		msg := "Message test"
		err = sender.SendString(msg)
		if err != nil {
			t.Error("Error", l.Error(err))
		}
	}
}
