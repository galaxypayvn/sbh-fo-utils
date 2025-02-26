package redisqueue

import (
	"testing"

	"code.finan.one/finan-one-be/fo-utils/l"
)

func TestNewProducer(t *testing.T) {
	topic := "test_work"
	ll.Info("Test Producer", l.String("topic", topic))

	{
		producer, err := NewProducer(&Config{
			Address:  "redis://localhost:6379",
			Password: "redis",
			Database: 5,
		})

		if err != nil {
			t.Error(err)
			return
		}

		sender := producer.GetSender(topic)

		msg := `{"address": "example.ping@example.com", "subject": "hello world", "customer_id": 4}`
		err = sender.SendString(msg)
		if err != nil {
			t.Error("Error", l.Error(err))
		}
	}
}
