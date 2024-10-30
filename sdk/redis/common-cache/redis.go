package rediscommoncachestore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"github.com/sony/gobreaker"
	"strings"
	"sync"
	"time"
)

// repositoryImpl ...
type repositoryImpl struct {
	client     *redis.Client
	defaultTTL int
	prefixKey  string
	namespace  string
}

func (r *repositoryImpl) resolveKey(key string) string {
	return r.prefixKey + ":caches:" + r.namespace + ":" + key
}

// Get ...
var mu sync.Mutex
var breaker = gobreaker.NewCircuitBreaker(gobreaker.Settings{
	Name:    "RedisGet",
	Timeout: 30 * time.Second,
})

func (r *repositoryImpl) Get(ctx context.Context, key string) (interface{}, error) {
	var err error
	retries := 3

	for i := 0; i < retries; i++ {
		// Execute Redis Get inside the circuit breaker
		result, err := breaker.Execute(func() (interface{}, error) {
			mu.Lock() // Protect the map or client if needed
			defer mu.Unlock()
			return r.client.Get(ctx, r.resolveKey(key)).Result()
		})

		if err == nil {
			return result, nil
		}

		// Check for specific Redis "key not found" error
		if errors.Is(err, redis.Nil) {
			return nil, errors.New("key not found: " + key)
		}

		// If circuit is open or there's another error, apply exponential backoff
		time.Sleep(time.Duration(i*i) * 100 * time.Millisecond) // Exponential backoff
	}

	return nil, fmt.Errorf("failed to get key %s after %d retries: %w", key, retries, err)
}

// GetList returns the list of data in cache, list key not found and error
func (r *repositoryImpl) GetList(ctx context.Context, keys []string) ([]interface{}, []string, error) {
	// Tạo danh sách các khóa đã được resolve
	resolvedKeys := make([]string, len(keys))
	for i, key := range keys {
		resolvedKeys[i] = r.resolveKey(key)
	}

	// Fetch all keys using MGET
	rs, err := r.client.MGet(ctx, resolvedKeys...).Result()
	if err != nil {
		return nil, nil, err
	}

	var notFoundKeys []string
	var listOut []interface{}

	for i, result := range rs {
		if result == nil {
			notFoundKeys = append(notFoundKeys, keys[i])
		} else {
			listOut = append(listOut, result)
		}
	}

	return listOut, notFoundKeys, nil
}

// GetMap returns the list of data in cache, list key not found and error
func (r *repositoryImpl) GetMap(ctx context.Context, keys []string) (map[string]interface{}, []string, error) {
	// Tạo danh sách các khóa đã được resolve
	resolvedKeys := make([]string, len(keys))
	for i, key := range keys {
		resolvedKeys[i] = r.resolveKey(key)
	}

	// Fetch all keys using MGET
	rs, err := r.client.MGet(ctx, resolvedKeys...).Result()
	if err != nil {
		return nil, nil, err
	}

	notFoundKeys := []string{}
	listOut := make(map[string]interface{})

	for i, result := range rs {
		if result == nil {
			notFoundKeys = append(notFoundKeys, keys[i])
		} else {
			listOut[keys[i]] = result
		}
	}

	return listOut, notFoundKeys, nil
}

// Set ...
func (r *repositoryImpl) Set(ctx context.Context, key string, entry interface{}) error {
	bb, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, r.resolveKey(key), string(bb), time.Duration(r.defaultTTL)*time.Second).Err()
}

// SetString ...
func (r *repositoryImpl) SetString(ctx context.Context, key string, entry string) error {
	return r.client.Set(ctx, r.resolveKey(key), entry, time.Duration(r.defaultTTL)*time.Second).Err()
}

// SetBulk ...
func (r *repositoryImpl) SetBulk(ctx context.Context, entries map[string]interface{}) error {
	pipe := r.client.Pipeline()
	for key, entry := range entries {
		bb, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		pipe.Set(ctx, r.resolveKey(key), string(bb), time.Duration(r.defaultTTL)*time.Second)
	}
	_, err := pipe.Exec(ctx)
	return err
}

// Delete ...
func (r *repositoryImpl) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, r.resolveKey(key)).Err()
}

// Reset ...
func (r *repositoryImpl) Reset(ctx context.Context) error {
	keysPattern := r.resolveKey("*")
	keys, err := r.client.Keys(ctx, keysPattern).Result()
	if err != nil {
		return err
	}

	pipe := r.client.Pipeline()
	for _, key := range keys {
		pipe.Del(ctx, key)
	}
	_, err = pipe.Exec(ctx)
	return err
}

// Push ...
func (r *repositoryImpl) Push(ctx context.Context, data ...string) error {
	if len(data) == 0 {
		return nil
	}
	elems := make([]interface{}, len(data))
	for i, item := range data {
		elems[i] = item
	}
	return r.client.RPush(ctx, r.resolveKey("list"), elems...).Err()
}

// Pop ...
func (r *repositoryImpl) Pop(ctx context.Context, count int) ([]string, error) {
	key := r.resolveKey("list")
	rs, err := r.client.LPopCount(ctx, key, count).Result() // Redis 6.2+
	if err != nil && err != redis.Nil {
		return nil, err
	}
	return rs, nil
}

// New creates a new instance of repositoryImpl, contains whole common functions
// for a service
func New(client *redis.Client, defaultTTL int, prefixKey, namespace string) *repositoryImpl {
	return &repositoryImpl{
		prefixKey:  strings.ToLower(prefixKey),
		namespace:  strings.ToLower(namespace),
		client:     client,
		defaultTTL: defaultTTL,
	}
}
