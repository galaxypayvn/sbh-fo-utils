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
