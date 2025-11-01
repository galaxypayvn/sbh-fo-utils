package rabbitmq

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"
	"strings"
)

var (
	RabbitmqProducerTimeout = 10000
	ErrSendToClosedProducer = errors.New("send to closed producer...exiting")
)

const (
	_defaultMaxThread           = 1
	_defaultQueueSize           = 1000
	_defaultNetworkTimeoutInSec = 25
	_defaultRetries             = 10
)

// Exchange - config exchange for queue consumer
type Exchange struct {
	Name       string                 `json:"name" mapstructure:"name"` //require
	Type       string                 `json:"type" mapstructure:"type"` // direct|fanout|topic|x-custom
	Durable    bool                   `json:"durable" mapstructure:"durable"`
	NoWait     bool                   `json:"no_wait" mapstructure:"no_wait"`
	AutoDelete bool                   `json:"auto_delete" mapstructure:"auto_delete"`
	Internal   bool                   `json:"internal" mapstructure:"internal"`
	Args       map[string]interface{} `json:"args" mapstructure:"args"`
}

// QueueConfig - config queue for consumer
type QueueConfig struct {
	Type       string                 `json:"type" mapstructure:"type"`
	Name       string                 `json:"name" mapstructure:"name"` //require
	Bindings   []Binding              `json:"bindings" mapstructure:"bindings"`
	Durable    bool                   `json:"durable" mapstructure:"durable"`
	AutoDelete bool                   `json:"auto_delete" mapstructure:"auto_delete"`
	Exclusive  bool                   `json:"exclusive" mapstructure:"exclusive"`
	NoWait     bool                   `json:"no_wait" mapstructure:"no_wait"`
	Args       map[string]interface{} `json:"args" mapstructure:"args"`
}

// ConsumeConfig - config consume for consumer
type ConsumeConfig struct {
	Consumer  string                 `json:"consumer" mapstructure:"consumer"`   // Tên của hàng đợi từ đó consumer sẽ nhận tin nhắn.
	AutoAck   bool                   `json:"auto_ack" mapstructure:"auto_ack"`   //  Nếu true, tin nhắn sẽ được tự động xác nhận ngay khi được nhận. Nếu false, tin nhắn phải được xác nhận thủ công.
	Exclusive bool                   `json:"exclusive" mapstructure:"exclusive"` // true: Hàng đợi sẽ chỉ được sử dụng bởi consumer hiện tại. Nếu có một consumer khác cố gắng truy cập vào cùng hàng đợi này, họ sẽ không thể đăng ký hoặc sẽ nhận được lỗi. false: Hàng đợi có thể được chia sẻ bởi nhiều consumer khác nhau.
	NoLocal   bool                   `json:"no_local" mapstructure:"no_local"`   // true: Consumer sẽ không nhận các tin nhắn được sản xuất từ cùng một kết nối. Điều này hữu ích khi bạn không muốn một ứng dụng nhận lại các tin nhắn mà nó vừa gửi đi.
	NoWait    bool                   `json:"no_wait" mapstructure:"no_wait"`     // Tham số này xác định liệu RabbitMQ có nên chờ phản hồi (acknowledgment) từ server trước khi trả về hay không
	Args      map[string]interface{} `json:"args" mapstructure:"args"`
}

// Binding - config binding for queue consumer
type Binding struct {
	RoutingKey string                 `json:"routing_key" mapstructure:"routing_key"` //require
	Exchange   Exchange               `json:"exchange" mapstructure:"exchange"`       //require
	NoWait     bool                   `json:"no_wait" mapstructure:"no_wait"`
	Args       map[string]interface{} `json:"args" mapstructure:"args"`
}

// Config ...
type Config struct {
	// amqp://user:pass@host:port/vhost?heartbeat=10&connection_timeout=10000&channel_max=100
	// hoặc amqps://user:pass@host:port/vhost cho TLS connection
	URI   string `json:"uri" mapstructure:"uri"`
	VHost string `json:"vhost" mapstructure:"vhost"`
	// QueueConfig       QueueConfig   `json:"queue_config" mapstructure:"queue_config"`
	ConsumeConfig       ConsumeConfig `json:"consume_config" mapstructure:"consume_config"`
	Retries             int           `json:"retries" mapstructure:"retries"`                         // the number of retries connect to rabbitmq
	InternalQueueSize   int           `json:"internal_queue_size" mapstructure:"internal_queue_size"` // xác định kích thước của hàng đợi này tin nhắn
	MaxThread           int           `json:"max_thread" mapstructure:"max_thread"`
	NetworkTimeoutInSec int           `json:"network_timeout_in_sec" mapstructure:"network_timeout_in_sec"`
	// TLS configuration
	CACertFile         string `json:"ca_cert_file" mapstructure:"ca_cert_file"`                 // đường dẫn đến file CA certificate (self-signed)
	SkipCAVerification bool   `json:"skip_ca_verification" mapstructure:"skip_ca_verification"` // bỏ qua xác thực CA (true = không check CA signed)
}

func (c *Config) Prepare() {
	if c == nil {
		return
	}

	if c.InternalQueueSize == 0 {
		c.InternalQueueSize = _defaultQueueSize
	}

	if c.MaxThread == 0 {
		c.MaxThread = _defaultMaxThread
	}

	if c.Retries == 0 {
		c.Retries = _defaultRetries
	}

	if c.NetworkTimeoutInSec == 0 {
		c.NetworkTimeoutInSec = _defaultNetworkTimeoutInSec
	}
}

// GetTLSConfig tạo TLS config từ CA certificate file (self-signed)
// Tự động phát hiện TLS nếu URI bắt đầu với amqps://
// Nếu SkipCAVerification = true, sẽ bỏ qua xác thực CA
// Nếu URI là amqps:// nhưng không có CA file, tự động skip verification để tránh lỗi với self-signed CA
func (c *Config) GetTLSConfig() (*tls.Config, error) {
	// Kiểm tra nếu URI sử dụng amqps:// (TLS)
	useTLS := strings.HasPrefix(strings.ToLower(c.URI), "amqps://")

	// Chỉ sử dụng TLS nếu URI là amqps:// hoặc có cấu hình TLS explicit
	if !useTLS && c.CACertFile == "" && !c.SkipCAVerification {
		return nil, nil // không sử dụng TLS
	}

	// Nếu URI là amqps:// nhưng không có CA file và không explicit set SkipCAVerification,
	// tự động skip verification để tránh lỗi với self-signed CA
	skipVerify := c.SkipCAVerification
	if useTLS && c.CACertFile == "" && !c.SkipCAVerification {
		// Tự động skip nếu dùng amqps:// mà không có CA file (giả định là self-signed)
		skipVerify = true
	}

	cfg := &tls.Config{
		InsecureSkipVerify: skipVerify,
	}

	// Nếu có file CA certificate, load nó vào
	if c.CACertFile != "" {
		caCert, err := os.ReadFile(c.CACertFile)
		if err != nil {
			return nil, err
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, errors.New("failed to parse CA certificate")
		}

		cfg.RootCAs = caCertPool
	}

	return cfg, nil
}
