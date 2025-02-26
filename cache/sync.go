package cache

import (
	"context"
	"time"

	memcommoncachestore "code.finan.one/finan-one-be/fo-utils/sdk/memory/common-cache"
	rediscommoncachestore "code.finan.one/finan-one-be/fo-utils/sdk/redis/common-cache"
	"github.com/redis/go-redis/v9"
)

type emptySync struct {
}

func (e emptySync) GetListKeys(ctx context.Context, limit int) ([]string, error) {
	return nil, nil
}

func (e emptySync) SetVersion(ctx context.Context, key ...string) error {
	return nil
}

var _ ICacheSync = (*emptySync)(nil)

type CacheSync struct {
	redisKey            string
	namespace           string
	memCacheRepository  ICacheRepository
	redisSyncRepository ISyncRepository
}

func (s CacheSync) GetListKeys(ctx context.Context, limit int) ([]string, error) {
	// Lấy danh sách key từ redisSyncRepository
	listKey, err := s.redisSyncRepository.Pop(ctx, limit)
	if err != nil {
		return nil, err
	}

	var listSyncCache []string
	// Lấy các phiên bản từ bộ nhớ
	listVersion, notFoundKeys, err := s.memCacheRepository.GetMap(ctx, listKey)
	if err != nil {
		return nil, err
	}

	// Thêm các key không tìm thấy vào listSyncCache
	listSyncCache = append(listSyncCache, notFoundKeys...)

	// Kiểm tra thời gian hết hạn (version) của các key
	nowUnix := time.Now().Unix()
	for key, item := range listVersion {
		if ver, ok := item.(int64); !ok || ver < nowUnix {
			listSyncCache = append(listSyncCache, key)
		}
	}
	return listSyncCache, nil
}

func (s CacheSync) SetVersion(ctx context.Context, keys ...string) error {
	now := time.Now().Add(3 * time.Second).Unix()

	// Dùng SetBulk để thiết lập phiên bản cho nhiều key
	entries := make(map[string]interface{}, len(keys))
	for _, key := range keys {
		entries[key] = now
	}
	return s.memCacheRepository.SetBulk(ctx, entries)
}

func NewSync(redisClient *redis.Client, prefixKey, namespace string, cacheConfig CacheConfig) (*CacheSync, error) {
	memTTL := cacheConfig.Providers[ProviderMemory].TTL
	redisTTL := cacheConfig.Providers[ProviderRedis].TTL

	memCacheRepo, err := memcommoncachestore.New(memTTL, prefixKey, namespace)
	if err != nil {
		return nil, err
	}

	redisCacheRepo := rediscommoncachestore.New(redisClient, redisTTL, prefixKey, namespace)

	return &CacheSync{
		namespace:           namespace,
		memCacheRepository:  memCacheRepo,
		redisSyncRepository: redisCacheRepo,
	}, nil
}

var _ ICacheSync = (*CacheSync)(nil)
