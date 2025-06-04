package common_cache

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/dgraph-io/ristretto/v2"
)

// repositoryImpl ...
type repositoryImpl struct {
	client     *ristretto.Cache[string, interface{}]
	defaultTTL int
	prefixKey  string
	namespace  string
}

func (r *repositoryImpl) resolveKey(key string) string {
	return r.prefixKey + ":" + r.namespace + ":" + key
}

// Get ...
func (r *repositoryImpl) Get(ctx context.Context, key string) (interface{}, error) {
	rs, found := r.client.Get(r.resolveKey(key))
	if !found {
		return nil, errors.New("key not found: " + r.resolveKey(key))
	}
	return rs, nil
}

// GetList returns the list of data in cache, list key not found and error
func (r *repositoryImpl) GetList(ctx context.Context, keys []string) ([]interface{}, []string, error) {
	var (
		notFoundKeys []string
		listOut      []interface{}
		mu           sync.Mutex
		wg           sync.WaitGroup
	)

	// Batch keys to reduce the number of goroutines
	const batchSize = 10
	batchKeys := func(keys []string, batchSize int) [][]string {
		var batches [][]string
		for batchSize < len(keys) {
			keys, batches = keys[batchSize:], append(batches, keys[0:batchSize:batchSize])
		}
		batches = append(batches, keys)
		return batches
	}

	for _, batch := range batchKeys(keys, batchSize) {
		for _, key := range batch {
			wg.Add(1)
			go func(k string) {
				defer wg.Done()
				rs, found := r.client.Get(r.resolveKey(k))
				mu.Lock()
				defer mu.Unlock()
				if !found {
					notFoundKeys = append(notFoundKeys, k)
					return
				}
				listOut = append(listOut, rs)
			}(key)
		}
	}
	wg.Wait()
	return listOut, notFoundKeys, nil
}

// GetMap returns the list of data in cache, list key not found and error
func (r *repositoryImpl) GetMap(ctx context.Context, keys []string) (map[string]interface{}, []string, error) {
	var (
		notFoundKeys []string
		listOut      = make(map[string]interface{}, len(keys))
		mu           sync.Mutex
		wg           sync.WaitGroup
	)

	// Batch keys to reduce the number of goroutines
	const batchSize = 10
	batchKeys := func(keys []string, batchSize int) [][]string {
		var batches [][]string
		for batchSize < len(keys) {
			keys, batches = keys[batchSize:], append(batches, keys[0:batchSize:batchSize])
		}
		batches = append(batches, keys)
		return batches
	}

	// Chia các keys thành các batch
	for _, batch := range batchKeys(keys, batchSize) {
		for _, key := range batch {
			wg.Add(1)
			go func(k string) {
				defer wg.Done()
				rs, found := r.client.Get(r.resolveKey(k))
				mu.Lock()
				defer mu.Unlock()
				if !found {
					notFoundKeys = append(notFoundKeys, k)
					return
				}
				listOut[k] = rs
			}(key)
		}
	}
	wg.Wait()
	return listOut, notFoundKeys, nil
}

// GetMetrics returns cache metrics as string
func (r *repositoryImpl) GetMetrics() string {
	return r.client.Metrics.String()
}

// Set ...
func (r *repositoryImpl) Set(ctx context.Context, key string, entry interface{}) error {
	r.client.SetWithTTL(r.resolveKey(key), entry, 1, time.Duration(r.defaultTTL)*time.Second)
	return nil
}

// SetString ...
func (r *repositoryImpl) SetString(ctx context.Context, key string, entry string) error {
	r.client.SetWithTTL(r.resolveKey(key), entry, 1, time.Duration(r.defaultTTL)*time.Second)
	return nil
}

// SetBulk ...
func (r *repositoryImpl) SetBulk(ctx context.Context, entries map[string]interface{}) error {
	var (
		wg sync.WaitGroup
		mu sync.Mutex
		failedKeys []string
	)
	for key, entry := range entries {
		wg.Add(1)
		go func(k string, e interface{}) {
			defer wg.Done()
			ok := r.client.SetWithTTL(r.resolveKey(k), e, 1, time.Duration(r.defaultTTL)*time.Second)
			if !ok {
				mu.Lock()
				failedKeys = append(failedKeys, k)
				mu.Unlock()
			}
		}(key, entry)
	}
	wg.Wait()
	// Ensure all sets are visible
	r.client.Wait()
	if len(failedKeys) > 0 {
		return errors.New("failed to set keys: " + strings.Join(failedKeys, ", "))
	}
	return nil
}

// Delete ...
func (r *repositoryImpl) Delete(ctx context.Context, key string) error {
	r.client.Del(r.resolveKey(key))
	return nil
}

// Reset ...
func (r *repositoryImpl) Reset(ctx context.Context) error {
	r.client.Clear()
	return nil
}

// New creates a new instance of repositoryImpl, contains whole common functions
// for a service
func New(defaultTTL int, prefixKey, namespace string) (*repositoryImpl, error) {
	cache, err := ristretto.NewCache(
		&ristretto.Config[string, interface{}]{
			NumCounters:        1000000,
			MaxCost:            100000,
			BufferItems:        64,
			IgnoreInternalCost: true,
			Metrics:            false,
		},
	)
	if err != nil {
		return nil, err
	}

	return &repositoryImpl{
		prefixKey:  strings.ToLower(prefixKey),
		namespace:  strings.ToLower(namespace),
		defaultTTL: defaultTTL,
		client:     cache,
	}, nil
}
