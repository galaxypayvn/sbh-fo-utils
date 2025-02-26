package redisqueue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gocraft/work"
	"github.com/gomodule/redigo/redis"

	"code.finan.one/finan-one-be/fo-utils/l"
)

type WContext struct{}

var ll = l.New()

// IConsumer ...
type IConsumer interface {
	GetReceiver(topic string) IReceiver
}

type redisQueueConsumer struct {
	consumer *work.WorkerPool
}

// GetReceiver ...
func (c *redisQueueConsumer) GetReceiver(topic string) IReceiver {
	return &receiver{c, topic}
}

func NewConsumer(cfg *Config) (IConsumer, error) {
	// Prepare configuration
	cfg.Prepare()

	// Create Redis pool
	redisPool := &redis.Pool{
		MaxActive:   _defaultMaxActive,
		MaxIdle:     _defaultMaxIdle,
		IdleTimeout: _defaultIdleTimeout,
		Wait:        true,
		Dial: func() (redis.Conn, error) {
			options := []redis.DialOption{
				redis.DialDatabase(cfg.Database),
			}
			if cfg.Password != "" {
				options = append(options, redis.DialPassword(cfg.Password))
			}

			return redis.DialURL(cfg.Address, options...)
		},
	}

	// Create worker pool
	wp := work.NewWorkerPool(WContext{}, _defaultConcurrency, cfg.QueuePrefix, redisPool)

	// Middleware để log dữ liệu trước và sau khi xử lý job
	if cfg.QueueLog != "" {
		// Create producer
		producer, _ := NewProducer(cfg)
		enqueuer := producer.GetSender(cfg.QueueLog)
		wp.Middleware(func(c *work.Job, next work.NextMiddlewareFunc) error {
			if c.Name == cfg.QueueLog || contains(cfg.QueueLogExclude, c.Name) {
				// If the job name matches the QueueLog or is in the QueueLogExclude list, there is no need to log again
				return next()
			}
			// Initialize log for job
			startTime := time.Now()
			jobLog := &JobExecution{
				JobID:      c.ID,
				JobName:    c.Name,
				Args:       c.Args,
				EnqueuedAt: c.EnqueuedAt,
				Unique:     c.Unique,
				Fails:      c.Fails,
				FailedAt:   c.FailedAt,
				StartTime:  startTime,
				LastError:  c.LastErr,
			}

			defer func() {
				if r := recover(); r != nil {
					// Bắt lỗi panic và gán vào biến mdlErr
					mdlErr := fmt.Errorf("panic: %v", r)

					// Cập nhật trạng thái job log cho panic
					jobLog.Status = "failed"
					jobLog.Error = mdlErr.Error()

					// Serialize log data to JSON
					jobLogBytes, _ := json.Marshal(jobLog)

					// Đẩy log dữ liệu vào queue khi xảy ra panic
					if sendErr := enqueuer.SendBytes(jobLogBytes); sendErr != nil {
						ll.Error("Unable to push log data into queue", l.Error(sendErr))
					}
				}
			}()

			// Execute the job and record errors if any
			mdlErr := next()

			// Update log after job is done
			jobLog.EndTime = time.Now()
			if mdlErr == nil {
				jobLog.Status = "completed"
			} else {
				jobLog.Status = "failed"
				jobLog.Error = mdlErr.Error()
			}

			// Serialize log data to JSON
			jobLogBytes, jsonErr := json.Marshal(jobLog)
			if jsonErr != nil {
				ll.Error("Unable to serialize job log", l.Error(jsonErr))
			} else {
				// Push log data into queue
				if sendErr := enqueuer.SendBytes(jobLogBytes); sendErr != nil {
					ll.Error("Unable to push log data into queue", l.Error(sendErr))
				}
			}

			return mdlErr
		})
	}

	// Push log data into queue
	return &redisQueueConsumer{consumer: wp}, nil
}

// IReceiver ...
type IReceiver interface {
	Handle(ctx context.Context, jobOptions JobOptions, handler EventHandler)
}

type receiver struct {
	*redisQueueConsumer
	topic string
}

// EventHandler ...
type EventHandler func(c *WContext, job *work.Job) error

// Handle ...
func (r *receiver) Handle(ctx context.Context, jobOptions JobOptions, handler EventHandler) {
	// Set default values if not configured
	if jobOptions.MaxConcurrency == 0 {
		jobOptions.MaxConcurrency = 5
	}
	if jobOptions.Priority == 0 {
		jobOptions.Priority = 1
	}
	if jobOptions.MaxFails == 0 {
		jobOptions.MaxFails = 1 // set 1 nghĩa là ko thực hiện lại khi lỗi,  set 2 nghĩa là cho phép thực hiện lại 1 lần nữa khi lỗi,
	}

	r.consumer.JobWithOptions(r.topic, work.JobOptions{
		Priority:       jobOptions.Priority,
		MaxConcurrency: jobOptions.MaxConcurrency,
		MaxFails:       jobOptions.MaxFails,
		SkipDead:       jobOptions.SkipDead,
		// Backoff:        ExponentialBackoff,
	}, handler)
	r.consumer.Start()
	defer func() {
		r.consumer.Stop()
	}()

	for {
		select {
		case <-ctx.Done():
			ll.Info("Context Done()", l.Error(ctx.Err()))
			return
		}
	}
}

// ExponentialBackoff: Tăng gấp đôi thời gian chờ sau mỗi lần thất bại
func ExponentialBackoff(job *work.Job) int64 {
	retryCount := job.Fails        // Số lần retry đã thất bại
	baseDelay := int64(2)          // Giây
	return baseDelay << retryCount // Nhân đôi thời gian chờ sau mỗi lần thất bại
}

func contains(slice []string, item string) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}
