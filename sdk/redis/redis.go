package redis

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrNotFound = errors.New("not found")
)

type Config struct {
	Address  string
	Password string
	DB       int
}

type RedisRepo struct {
	RDB *redis.Client
}

func NewRedisRepo(cfg Config) IRedisRepo {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Address,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     100,
		MinIdleConns: 10,
		PoolTimeout:  30 * time.Second,
	})

	return &RedisRepo{
		RDB: client,
	}
}

type IRedisRepo interface {
	GetRepo() *redis.Client
	SetNX(ctx context.Context, key, value string, exp time.Duration) (bool, error)
	Del(ctx context.Context, key string) error
	SetKey(ctx context.Context, key string, value interface{}, expire time.Duration) (err error)
	GetKey(ctx context.Context, key string) (value string, err error)
	SetHash(ctx context.Context, key string, value interface{}) (err error)
	GetHash(ctx context.Context, key string, res any) (err error)
	CheckExist(ctx context.Context, key string) (res bool, err error)
	GetHashByKey(ctx context.Context, key string, field string) (res string, err error)
	GetSet(ctx context.Context, key string, newValue any, expire time.Duration) (value string, err error)
	SetTTL(ctx context.Context, key string, expire time.Duration) error
	RunRedisScript(ctx context.Context, script, redisKey string) (interface{}, error)
	HMSet(ctx context.Context, key string, values map[string]interface{}) error
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	HMGet(ctx context.Context, key string, fields ...string) ([]string, error)
	HSet(ctx context.Context, key string, fields map[string]interface{}) error
	HDel(ctx context.Context, key string, fields ...string) error
	HExists(ctx context.Context, key string, field string) (bool, error)
	HLen(ctx context.Context, key string) (int64, error)
	HKeys(ctx context.Context, key string) ([]string, error)
	HVals(ctx context.Context, key string) ([]string, error)
	HIncrBy(ctx context.Context, key string, field string, increment int64) (int64, error)
	HIncrByFloat(ctx context.Context, key string, field string, increment float64) (float64, error)
	HGet(ctx context.Context, key string, field string) (string, error)
	HScan(ctx context.Context, key string, cursor uint64, match string, count int64) (uint64, []string, error)
	HSetNX(ctx context.Context, key string, field string, value interface{}) (bool, error)
}

func (r *RedisRepo) GetRepo() *redis.Client {
	return r.RDB
}

func (r *RedisRepo) SetNX(ctx context.Context, key, value string, exp time.Duration) (bool, error) {
	res := r.RDB.SetNX(ctx, key, value, exp)
	result, err := res.Result()
	if err != nil {
		return false, err
	}
	return result, nil
}

func (r *RedisRepo) Del(ctx context.Context, key string) error {
	err := r.RDB.Del(ctx, key).Err()
	if err != nil {
		return err
	}
	return nil
}

func (r *RedisRepo) CheckExist(ctx context.Context, key string) (res bool, err error) {
	exist, err := r.RDB.Exists(ctx, key).Result()
	if err != nil {
		return res, err
	}
	if exist == 0 {
		return false, nil
	} else {
		return true, nil
	}
}

func (r *RedisRepo) SetKey(ctx context.Context, key string, value interface{}, expire time.Duration) (err error) {
	err = r.RDB.Set(ctx, key, value, expire).Err()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrNotFound
		}

		return err
	}
	return err
}

func (r *RedisRepo) GetKey(ctx context.Context, key string) (value string, err error) {
	value, err = r.RDB.Get(ctx, key).Result()
	if err != nil {
		return "", err
	}
	return value, err
}

func (r *RedisRepo) SetHash(ctx context.Context, key string, value interface{}) (err error) {
	err = r.RDB.HSet(ctx, key, value).Err()
	if err != nil {
		return err
	}

	return nil
}

func (r *RedisRepo) GetHash(ctx context.Context, key string, res any) error {
	err := r.RDB.HGetAll(ctx, key).Scan(res)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrNotFound
		}

		return err
	}

	return nil
}

func (r *RedisRepo) GetHashByKey(ctx context.Context, key string, field string) (res string, err error) {
	res, err = r.RDB.HGet(ctx, key, field).Result()
	if err != nil {
		return res, err
	}

	return res, nil
}

func (r *RedisRepo) GetSet(ctx context.Context, key string, value any, expire time.Duration) (string, error) {
	curValue, err := r.RDB.GetSet(ctx, key, value).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", ErrNotFound
		}

		return "", err
	}

	return curValue, err
}

func (r *RedisRepo) SetTTL(ctx context.Context, key string, expire time.Duration) error {
	return r.RDB.Expire(ctx, key, expire).Err()
}

func (r *RedisRepo) RunRedisScript(ctx context.Context, script, redisKey string) (interface{}, error) {
	return redis.NewScript(script).Run(ctx, r.RDB, []string{redisKey}).Int()
}

// HMSet and HGetAll
func (r *RedisRepo) HMSet(ctx context.Context, key string, values map[string]interface{}) error {
	if len(values) == 0 {
		return nil
	}
	err := r.RDB.HMSet(ctx, key, values).Err()
	if err != nil {
		return err
	}
	return nil
}
func (r *RedisRepo) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	values, err := r.RDB.HGetAll(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return values, nil
}

// HMGet retrieves multiple fields from a hash stored at key.
func (r *RedisRepo) HMGet(ctx context.Context, key string, fields ...string) ([]string, error) {
	values, err := r.RDB.HMGet(ctx, key, fields...).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	// Convert interface{} to string
	strValues := make([]string, len(values))
	for i, v := range values {
		if v == nil {
			strValues[i] = ""
		} else {
			strValues[i] = v.(string)
		}
	}

	return strValues, nil
}

// HSet sets multiple fields in a hash stored at key.
func (r *RedisRepo) HSet(ctx context.Context, key string, fields map[string]interface{}) error {
	if len(fields) == 0 {
		return nil
	}
	err := r.RDB.HSet(ctx, key, fields).Err()
	if err != nil {
		return err
	}
	return nil
}

// HDel deletes one or more fields from a hash stored at key.
func (r *RedisRepo) HDel(ctx context.Context, key string, fields ...string) error {
	if len(fields) == 0 {
		return nil
	}
	err := r.RDB.HDel(ctx, key, fields...).Err()
	if err != nil {
		return err
	}
	return nil
}

// HExists checks if a field exists in a hash stored at key.
func (r *RedisRepo) HExists(ctx context.Context, key string, field string) (bool, error) {
	exists, err := r.RDB.HExists(ctx, key, field).Result()
	if err != nil {
		return false, err
	}
	return exists, nil
}

// HLen returns the number of fields in a hash stored at key.
func (r *RedisRepo) HLen(ctx context.Context, key string) (int64, error) {
	len, err := r.RDB.HLen(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	return len, nil
}

// HKeys returns all fields in a hash stored at key.
func (r *RedisRepo) HKeys(ctx context.Context, key string) ([]string, error) {
	keys, err := r.RDB.HKeys(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return keys, nil
}

// HVals returns all values in a hash stored at key.
func (r *RedisRepo) HVals(ctx context.Context, key string) ([]string, error) {
	values, err := r.RDB.HVals(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return values, nil
}

// HIncrBy increments the integer value of a field in a hash stored at key by the given increment.
func (r *RedisRepo) HIncrBy(ctx context.Context, key string, field string, increment int64) (int64, error) {
	res, err := r.RDB.HIncrBy(ctx, key, field, increment).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	return res, nil
}

// HIncrByFloat increments the float value of a field in a hash stored at key by the given increment.
func (r *RedisRepo) HIncrByFloat(ctx context.Context, key string, field string, increment float64) (float64, error) {
	res, err := r.RDB.HIncrByFloat(ctx, key, field, increment).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, ErrNotFound
		}
		return 0, err
	}
	return res, nil
}

// HGet retrieves the value of a field in a hash stored at key.
func (r *RedisRepo) HGet(ctx context.Context, key string, field string) (string, error) {
	res, err := r.RDB.HGet(ctx, key, field).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", ErrNotFound
		}
		return "", err
	}
	return res, nil
}

// HScan iterates over the fields and values in a hash stored at key.
func (r *RedisRepo) HScan(ctx context.Context, key string, cursor uint64, match string, count int64) (uint64, []string, error) {
	values, newCursor, err := r.RDB.HScan(ctx, key, cursor, match, count).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil, ErrNotFound
		}
		return 0, nil, err
	}
	return newCursor, values, nil
}

// HSetNX sets a field in a hash stored at key, only if the field does not already exist.
func (r *RedisRepo) HSetNX(ctx context.Context, key string, field string, value interface{}) (bool, error) {
	res, err := r.RDB.HSetNX(ctx, key, field, value).Result()
	if err != nil {
		return false, err
	}
	return res, nil
}
