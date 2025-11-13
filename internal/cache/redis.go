package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"shuvoedward/Bible_project/internal/data"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
	PoolSize int
}

type RedisClient struct {
	client *redis.Client
	ttl    time.Duration
}

func NewRedisClient(cfg RedisConfig, ttl time.Duration) (*RedisClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: 5,

		// Timeouts
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,

		// Retry configuration
		MaxRetries:      3,
		MinRetryBackoff: 8 * time.Millisecond,
		MaxRetryBackoff: 512 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisClient{
		client: client,
		ttl:    ttl,
	}, nil
}

func (r *RedisClient) Close() error {
	return r.client.Close()
}

func (r *RedisClient) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return r.client.Ping(ctx).Err()
}

func (r *RedisClient) tokenKey(token string) string {
	return fmt.Sprintf("token:%s", token)
}

func (r *RedisClient) SetToken(token string, userID int64, activated bool) error {
	key := r.tokenKey(token)

	userData := fmt.Sprintf(`id:%d,activated:%t`, userID, activated)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := r.client.Set(ctx, key, userData, r.ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set token in Redis: %w", err)
	}

	return nil
}

func (r *RedisClient) GetForToken(token string) (string, error) {
	key := r.tokenKey(token)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	userData, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}

	return userData, err
}

func (r *RedisClient) DeleteToken(token string) error {
	key := r.tokenKey(token)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := r.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete token from Redis: %w", err)
	}

	return nil
}

// CacheVerses stores a Bible passage in Redis with a 24-hour TTL.
// This is a fire-and-forget operation - errors are returned but the caller
// typically logs them without failing the request.
func (r *RedisClient) CacheVerses(key string, passage *data.Passage) error {
	data, err := json.Marshal(passage)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = r.client.Set(ctx, "key", data, 24*time.Hour).Err()
	if err != nil {
		return fmt.Errorf("failed to cache verses: %w", err)
	}

	return nil
}

// GetCachedVerses retrieves a Bible passage from Redis cache.
// Returns:
//   - (*data.Passage, nil) on cache hit
//   - (nil, nil) on cache miss (key doesn't exist)
//   - (nil, error) on Redis connection/operational errors
func (r *RedisClient) GetCachedVerses(key string) (*data.Passage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	versesStr, err := r.client.Get(ctx, key).Result()
	// if err != nil {
	// 	if errors.Is(err, redis.Nil) {
	// 	}
	// 	return nil, nil // Cache miss
	// }
	// return nil, err // Real error

	// Cache miss - key doesn't exist in Redis (expected, not an error)
	if err == redis.Nil {
		return nil, nil
	}

	// Real Redis error (connection issues, timeout, etc.)
	if err != nil {
		return nil, err
	}

	passage := data.Passage{
		Verses: []data.VerseDetail{},
	}

	// Cache hit - deserialize the JSON data
	err = json.Unmarshal([]byte(versesStr), &passage)

	return &passage, err
}
