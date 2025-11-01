package rabbitmq

import (
	"crypto/tls"
	"errors"
	"fmt"
	"math"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/streadway/amqp"

	"code.finan.one/finan-one-be/fo-utils/l"
)

// IProducer
type IProducer interface {
	Start()
	GetSender(exchName, routingKey string) ISender
}

// ISender ...
type ISender interface {
	SendBytes(data []byte) (err error)
	SendString(msg string) error
}

type messageInfo struct {
	msg        []byte
	routingKey string
	xchName    string
	mandatory  bool
	immediate  bool
	retries    int64
}

// rabbitmqProducer ...
type rabbitmqProducer struct {
	conn          *amqp.Connection
	done          chan error
	config        *Config
	status        bool
	retries       int
	messages      chan *messageInfo
	closed        int32
	inputCounter  int64
	outputCounter int64
	errorCounter  int64
	ticker        *time.Ticker

	// max concurrent thread publish msg. default = 1
	maxThread int
	ll        l.Logger
}

// NewRabbitmqProducer ...
func NewRabbitmqProducer(cfg *Config, ll l.Logger) IProducer {

	cfg.Prepare()

	pObj := &rabbitmqProducer{
		config:    cfg,
		done:      make(chan error),
		status:    false,
		retries:   cfg.Retries,
		messages:  make(chan *messageInfo, cfg.InternalQueueSize),
		ticker:    time.NewTicker(2 * time.Minute),
		ll:        ll,
		maxThread: cfg.MaxThread,
	}

	if err := pObj.connectQueue(); err != nil {
		panic(err)
	}

	return pObj
}

// GetSender ...
func (p *rabbitmqProducer) GetSender(exchName, routingKey string) ISender {
	return &sender{p, exchName, routingKey}
}

// Connect ...
func (p *rabbitmqProducer) Connect() error {
	p.maxThread = 1
	return p.connectQueue()
}

// Start ...
func (p *rabbitmqProducer) Start() {
	stopTicker := make(chan struct{})
	var countClose int64 = 0
	go func() {
		for {
			select {
			case <-p.ticker.C:
				p.ll.S.Debugf(`{"type": "producer", "counter": {"input": %v, "output_success": %v, "error": %v}}`,
					p.InputCount(), p.OutputCount(), p.ErrorCount())
			case <-stopTicker:
				n := atomic.AddInt64(&countClose, 1)
				if n >= int64(p.maxThread) {
					return
				}
			}
		}
	}()

	p.ll.S.Infof("Running producer with %v goroutines", p.maxThread)
	m := &sync.Mutex{}
	for i := 0; i < p.maxThread; i++ {
		go func(id int) {
			m.Lock()
			c, err := p.conn.Channel()
			for err != nil {
				time.Sleep(10 * time.Millisecond)
				c, err = p.conn.Channel()
			}
			m.Unlock()
			p.ll.S.Infof("Got channel for %v", id)
			defer func() { _ = c.Close() }()
			for msg := range p.messages {
				if msg == nil {
					p.ll.Info(fmt.Sprintf("Got nil on %v. Breaking...", id))
					stopTicker <- struct{}{}
					return
				}
				err := p.publish(c, msg)
				if err != nil {
					// maybe channel is dead, get new one
					_ = c.Close()
					m.Lock()
					c, err = p.conn.Channel()
					for err != nil {
						time.Sleep(100 * time.Millisecond)
						c, err = p.conn.Channel()
					}
					m.Unlock()
					//msg.retries++
					go func() {
						_ = p.publishWithTimeout(msg)
					}()
				}
			}
		}(i)
	}

	go func() {
		var err error
		for {
			// Go into reconnect loop when
			// c.done is passed non nil values
			if err = <-p.done; err != nil {
				if strings.Contains(err.Error(), "channel closed") && !p.IsClosed() { // reconnect case
					p.status = false
					err = p.reconnect()
					retry := 0
					var base = 100
					step := 10
					exp := 2
					for err != nil {
						time.Sleep(time.Duration(base+int(math.Pow(float64(step), float64(exp)))) * time.Millisecond)
						// Very likely chance of failing
						// should not cause worker to terminate
						retry++
						if retry > p.retries {
							panic(fmt.Errorf("cannot retry connection after %v times", p.retries))
						}
						err = p.reconnect()
					}
				} else { // stop case
					p.conn.Close()
					return
				}
			}
		}
	}()
}

// Close ...
func (p *rabbitmqProducer) Close() {
	atomic.StoreInt32(&(p.closed), 1)
	time.Sleep(1 * time.Second)
	close(p.messages)
	p.done <- errors.New("stop rabbitmq producer")
	_ = p.conn.Close()
	p.ticker.Stop()
}

// IsClosed ...
func (p *rabbitmqProducer) IsClosed() bool {
	return atomic.LoadInt32(&(p.closed)) == 1
}

// OutputCount ...
func (p *rabbitmqProducer) OutputCount() int64 {
	return atomic.LoadInt64(&(p.outputCounter))
}

// InputCount ...
func (p *rabbitmqProducer) InputCount() int64 {
	return atomic.LoadInt64(&(p.inputCounter))
}

// ErrorCount ...
func (p *rabbitmqProducer) ErrorCount() int64 {
	return atomic.LoadInt64(&(p.errorCounter))
}

type sender struct {
	*rabbitmqProducer

	exchName   string
	routingKey string
}

// SendBytes ...
func (s *sender) SendBytes(data []byte) (err error) {
	if data == nil {
		return
	}
	if !s.IsClosed() {
		err = s.publishWithTimeout(&messageInfo{msg: data, xchName: s.exchName, routingKey: s.routingKey})
	} else {
		err = ErrSendToClosedProducer
	}
	atomic.AddInt64(&(s.inputCounter), 1)
	return
}

// SendString produce string into rabbitmq
func (s sender) SendString(msg string) error {
	return s.SendBytes([]byte(msg))
}

func (p *rabbitmqProducer) connectQueue() error {
	var err error

	if p.config == nil {
		err = errors.New("missing rabbitmq configuration")
		return err
	}

	// Tạo TLS config nếu có cấu hình TLS
	tlsConfig, err := p.config.GetTLSConfig()
	if err != nil {
		return fmt.Errorf("failed to get TLS config: %s", err)
	}

	dialFunc := func(network, addr string) (net.Conn, error) {
		timeout := time.Duration(p.config.NetworkTimeoutInSec) * time.Second
		if p.config.NetworkTimeoutInSec == 0 {
			timeout = _defaultNetworkTimeoutInSec * time.Second
		}

		if tlsConfig != nil {
			// Sử dụng TLS connection
			return tls.DialWithDialer(&net.Dialer{
				Timeout: timeout,
			}, network, addr, tlsConfig)
		}
		// Kết nối non-TLS thông thường
		return net.DialTimeout(network, addr, timeout)
	}

	p.conn, err = amqp.DialConfig(p.config.URI, amqp.Config{
		Dial: dialFunc,
	})

	if err != nil {
		return fmt.Errorf("dial error: %s", err)
	}

	go func() {
		// Waits here for the channel to be closed
		closed := <-p.conn.NotifyClose(make(chan *amqp.Error))

		if closed != nil {
			// Let Handle know it's not time to reconnect
			// ensure goroutine go to end in every case
			select {
			case p.done <- errors.New("channel closed"):
			case <-time.After(10 * time.Second):
				return
			}
		}
	}()
	p.status = true

	return nil
}

func (p *rabbitmqProducer) publish(c *amqp.Channel, mi *messageInfo) error {
	msg := amqp.Publishing{
		ContentType: "application/json",
		Body:        mi.msg,
		Timestamp:   time.Now(),
	}
	err := c.Publish(mi.xchName, mi.routingKey, mi.mandatory, mi.immediate, msg)
	if err != nil {
		atomic.AddInt64(&(p.errorCounter), 1)
		err = fmt.Errorf("cannot publish base-event to exchange, %v - %v - %v - %v - %v. %v",
			mi.xchName, mi.routingKey, mi.mandatory, mi.immediate, string(mi.msg), err)
		return err
	} else if mi.retries > 0 {
		atomic.AddInt64(&p.errorCounter, -mi.retries)
	}
	atomic.AddInt64(&(p.outputCounter), 1)

	return nil
}

func (p *rabbitmqProducer) publishWithTimeout(mess *messageInfo) error {
	select {
	case p.messages <- mess:
	case <-time.After(time.Duration(RabbitmqProducerTimeout) * time.Millisecond):
		return errors.New("publish base-event to rabbbit timeout")
	}

	return nil
}

func (p *rabbitmqProducer) reconnect() error {
	p.status = false
	time.Sleep(20 * time.Second)
	if err := p.connectQueue(); err != nil {
		return err
	}

	return nil
}
