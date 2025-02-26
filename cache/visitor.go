package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	memcommoncachestore "code.finan.one/finan-one-be/fo-utils/sdk/memory/common-cache"
	rediscommoncachestore "code.finan.one/finan-one-be/fo-utils/sdk/redis/common-cache"
	"github.com/redis/go-redis/v9"
)

type CacheVisitor struct {
	out                  interface{}
	prefixKey            string
	namespace            string
	memCacheRepository   ICacheRepository
	redisCacheRepository ICacheRepository
	redisSyncRepository  ISyncRepository
}

var _ ICacheVisitor = (*CacheVisitor)(nil)

func NewVisitor(redisClient *redis.Client, prefixKey, namespace string, cacheConfig CacheConfig) (*CacheVisitor, error) {
	memTTL := cacheConfig.Providers[ProviderMemory].TTL
	redisTTL := cacheConfig.Providers[ProviderRedis].TTL

	obj := &CacheVisitor{
		prefixKey: prefixKey,
		namespace: namespace,
	}

	// Initialize Memory Cache if enabled
	if cacheConfig.Providers[ProviderMemory].Enabled {
		obj.memCacheRepository, _ = memcommoncachestore.New(memTTL, prefixKey, namespace)
	} else {
		obj.memCacheRepository = &cacheImplEmpty{}
	}

	// Initialize Redis Cache if enabled
	if cacheConfig.Providers[ProviderRedis].Enabled {
		tmpRedisImpl := rediscommoncachestore.New(redisClient, redisTTL, prefixKey, namespace)
		obj.redisCacheRepository = tmpRedisImpl
		obj.redisSyncRepository = tmpRedisImpl
	} else {
		obj.redisCacheRepository = &cacheImplEmpty{}
		obj.redisSyncRepository = &cacheImplEmpty{}
	}

	return obj, nil
}

func (v *CacheVisitor) Model(out interface{}) ICacheVisitor {
	v.out = out
	return v
}

func (v *CacheVisitor) Get(ctx context.Context, key string) (interface{}, error) {
	// Lấy dữ liệu từ Memory Cache
	rs, err := v.memCacheRepository.Get(ctx, key)
	if err == nil {
		// Xử lý giá trị trả về từ Memory Cache
		return v.ensureCorrectType(rs)
	}

	// Lấy dữ liệu từ Redis Cache nếu không có trong Memory Cache
	rs, err = v.redisCacheRepository.Get(ctx, key)

	if err != nil {
		return nil, err
	}

	// Xử lý giá trị trả về từ Redis Cache
	newP, err := v.ensureCorrectType(rs)
	if err != nil {
		return nil, err
	}

	// Set lại dữ liệu vào Memory Cache sau khi lấy từ Redis
	err = v.memCacheRepository.Set(ctx, key, newP)
	if err != nil {
		fmt.Println("memCacheRepository.Set err", err.Error())
	}

	return newP, nil
}

func (v *CacheVisitor) GetList(ctx context.Context, keys []string) ([]interface{}, []string, error) {
	rsMem, notfoundKeys, _ := v.memCacheRepository.GetList(ctx, keys)
	// Xử lý từng phần tử trả về từ Memory Cache
	for i, item := range rsMem {
		rsMem[i], _ = v.ensureCorrectType(item)
	}

	if len(notfoundKeys) == 0 {
		return rsMem, nil, nil
	}

	// Lấy dữ liệu từ Redis Cache cho các khóa không tìm thấy trong Memory Cache
	rsRedis, notfoundKeys, err := v.redisCacheRepository.GetList(ctx, notfoundKeys)

	if err != nil {
		return nil, nil, err
	}

	// Xử lý từng phần tử trả về từ Redis Cache
	cacheToMem := make(map[string]interface{})
	for i, item := range rsRedis {
		typedItem, _ := v.ensureCorrectType(item)
		rsRedis[i] = typedItem
		cacheToMem[keys[i]] = typedItem
	}

	// Sử dụng SetBulk để set các dữ liệu từ Redis vào Memory Cache cùng một lúc
	err = v.memCacheRepository.SetBulk(ctx, cacheToMem)
	if err != nil {
		fmt.Println("memCacheRepository.SetBulk err", err.Error())
	}

	// Gộp kết quả từ Memory Cache và Redis Cache
	rs := append(rsMem, rsRedis...)
	return rs, notfoundKeys, nil
}

func (v *CacheVisitor) GetMap(ctx context.Context, keys []string) (map[string]interface{}, []string, error) {
	rsMem, notfoundKeys, _ := v.memCacheRepository.GetMap(ctx, keys)

	// Xử lý từng phần tử trả về từ Memory Cache
	for key, item := range rsMem {
		rsMem[key], _ = v.ensureCorrectType(item)
	}

	if len(notfoundKeys) == 0 {
		return rsMem, nil, nil
	}

	// Lấy dữ liệu từ Redis Cache cho các khóa không tìm thấy trong Memory Cache
	rsRedis, notfoundKeys, err := v.redisCacheRepository.GetMap(ctx, notfoundKeys)

	if err != nil {
		return nil, nil, err
	}

	// Chuẩn bị map để lưu lại vào Memory Cache
	cacheToMem := make(map[string]interface{})

	// Xử lý từng phần tử trả về từ Redis Cache
	for key, item := range rsRedis {
		typedItem, _ := v.ensureCorrectType(item)
		rsRedis[key] = typedItem
		cacheToMem[key] = typedItem
	}

	// Sử dụng SetBulk để set các dữ liệu từ Redis vào Memory Cache cùng một lúc
	err = v.memCacheRepository.SetBulk(ctx, cacheToMem)
	if err != nil {
		fmt.Println("memCacheRepository.SetBulk err", err.Error())
	}
	// Gộp kết quả từ Memory Cache và Redis Cache
	for key, val := range rsRedis {
		rsMem[key] = val
	}

	return rsMem, notfoundKeys, nil
}

func (v *CacheVisitor) Set(ctx context.Context, key string, entry interface{}) error {
	bb, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if err = v.memCacheRepository.Set(ctx, key, entry); err != nil {
		return err
	}
	if entry == nil {
		return nil
	}
	return v.redisCacheRepository.SetString(ctx, key, string(bb))
}

func (v *CacheVisitor) SetString(ctx context.Context, key string, entry string) error {
	if err := v.memCacheRepository.Set(ctx, key, entry); err != nil {
		return err
	}
	return v.redisCacheRepository.SetString(ctx, key, entry)
}

func (v *CacheVisitor) SetBulk(ctx context.Context, entries map[string]interface{}) error {
	if len(entries) == 0 {
		return nil
	}
	if err := v.memCacheRepository.SetBulk(ctx, entries); err != nil {
		return err
	}
	return v.redisCacheRepository.SetBulk(ctx, entries)
}

func (v *CacheVisitor) Delete(ctx context.Context, key string, opt ...bool) error {
	isDelMem := true
	isDelRedis := true
	if len(opt) > 0 {
		isDelMem = opt[0]
	}
	if len(opt) > 1 {
		isDelRedis = opt[1]
	}
	if isDelMem {
		if err := v.memCacheRepository.Delete(ctx, key); err != nil {
			return err
		}
	}
	if isDelRedis {
		return v.redisCacheRepository.Delete(ctx, key)
	}
	return nil
}

func (v *CacheVisitor) Reset(ctx context.Context) error {
	if err := v.memCacheRepository.Reset(ctx); err != nil {
		return err
	}
	return v.redisCacheRepository.Reset(ctx)
}

// Hàm helper để đảm bảo giá trị đúng kiểu (con trỏ hoặc giá trị trực tiếp)
func (v *CacheVisitor) ensureCorrectType(value interface{}) (interface{}, error) {
	// Nếu không có model định nghĩa từ trước thì trả về luôn giá trị
	if v.out == nil {
		return value, nil
	}

	// Kiểm tra nếu giá trị là con trỏ đến kiểu `v.out`, trả về trực tiếp
	if reflect.TypeOf(value) == reflect.TypeOf(v.out) {
		return value, nil
	}

	// Kiểm tra nếu giá trị là giá trị trực tiếp của kiểu `v.out`, chuyển thành con trỏ
	if reflect.TypeOf(value) == reflect.TypeOf(reflect.ValueOf(v.out).Elem().Interface()) {
		newVal := reflect.New(reflect.TypeOf(value))
		newVal.Elem().Set(reflect.ValueOf(value))
		return newVal.Interface(), nil
	}

	// Nếu dữ liệu lấy từ Redis là một chuỗi (JSON), giải mã nó thành đối tượng tương ứng
	if strVal, ok := value.(string); ok {
		if v.out != nil {
			// Giải mã chuỗi JSON thành đối tượng Go
			err := json.Unmarshal([]byte(strVal), v.out)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal cache data: %w", err)
			}
			return v.out, nil
		}
	}

	// Trường hợp không hỗ trợ
	return nil, fmt.Errorf("unsupported cache data type: %T", value)
}
