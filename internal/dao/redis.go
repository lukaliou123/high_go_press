package dao

import (
	"context"
	"high-go-press/internal/biz"
	"high-go-press/pkg/logger"
	"strconv"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

type RedisRepo struct {
	client *redis.Client
}

func NewRedisRepo(client *redis.Client) biz.CounterRepo {
	return &RedisRepo{
		client: client,
	}
}

func NewRedisClient(addr, password string, db int) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// 测试连接
	ctx := context.Background()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	logger.Info("Connected to Redis successfully", zap.String("addr", addr))
	return rdb
}

func (r *RedisRepo) IncrementCounter(ctx context.Context, key string, increment int64) (int64, error) {
	result, err := r.client.IncrBy(ctx, key, increment).Result()
	if err != nil {
		logger.Error("Failed to increment counter",
			zap.String("key", key),
			zap.Int64("increment", increment),
			zap.Error(err))
		return 0, err
	}

	logger.Debug("Counter incremented successfully",
		zap.String("key", key),
		zap.Int64("increment", increment),
		zap.Int64("result", result))

	return result, nil
}

func (r *RedisRepo) GetCounter(ctx context.Context, key string) (int64, error) {
	result, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			// Key 不存在，返回 0
			return 0, nil
		}
		logger.Error("Failed to get counter",
			zap.String("key", key),
			zap.Error(err))
		return 0, err
	}

	count, err := strconv.ParseInt(result, 10, 64)
	if err != nil {
		logger.Error("Failed to parse counter value",
			zap.String("key", key),
			zap.String("value", result),
			zap.Error(err))
		return 0, err
	}

	return count, nil
}

func (r *RedisRepo) GetMultiCounters(ctx context.Context, keys []string) (map[string]int64, error) {
	if len(keys) == 0 {
		return make(map[string]int64), nil
	}

	// 使用 Pipeline 批量获取
	pipe := r.client.Pipeline()
	cmds := make(map[string]*redis.StringCmd)

	for _, key := range keys {
		cmds[key] = pipe.Get(ctx, key)
	}

	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		logger.Error("Failed to execute pipeline for multi get", zap.Error(err))
		return nil, err
	}

	result := make(map[string]int64)
	for key, cmd := range cmds {
		val, err := cmd.Result()
		if err != nil {
			if err == redis.Nil {
				result[key] = 0
			} else {
				logger.Error("Failed to get counter in batch",
					zap.String("key", key),
					zap.Error(err))
				continue
			}
		} else {
			count, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				logger.Error("Failed to parse counter value in batch",
					zap.String("key", key),
					zap.String("value", val),
					zap.Error(err))
				result[key] = 0
			} else {
				result[key] = count
			}
		}
	}

	return result, nil
}

func (r *RedisRepo) SetCounter(ctx context.Context, key string, value int64) error {
	err := r.client.Set(ctx, key, value, 0).Err()
	if err != nil {
		logger.Error("Failed to set counter",
			zap.String("key", key),
			zap.Int64("value", value),
			zap.Error(err))
		return err
	}

	logger.Debug("Counter set successfully",
		zap.String("key", key),
		zap.Int64("value", value))

	return nil
}
