package server

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/sessions"
	"github.com/rbcervilla/redisstore/v8"
	"github.com/sirupsen/logrus"
)

// RedisConfig represents the configuration for Redis
type RedisConfig struct {
	Host      string
	Port      int
	Password  string
	DB        int
	KeyPrefix string
	Timeout   time.Duration
}

// RedisClient represents a Redis client for session management
type RedisClient struct {
	client    *redis.Client
	log       *logrus.Logger
	keyPrefix string
	ctx       context.Context
}

// NewRedisClient creates a new Redis client
func NewRedisClient(config RedisConfig) (*RedisClient, error) {
	// Set default timeout if not specified
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", config.Host, config.Port),
		Password: config.Password,
		DB:       config.DB,
	})

	// Ping Redis to check the connection
	if _, err := client.Ping(ctx).Result(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisClient{
		client:    client,
		log:       logrus.StandardLogger(),
		keyPrefix: config.KeyPrefix,
		ctx:       ctx,
	}, nil
}

// Close closes the Redis connection
func (r *RedisClient) Close() error {
	return r.client.Close()
}

// CreateSessionStore creates a new Redis session store
func (r *RedisClient) CreateSessionStore(sessionKey string) (sessions.Store, error) {
	// Create a new Redis store
	store, err := redisstore.NewRedisStore(context.Background(), r.client)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis session store: %w", err)
	}

	// Set session store options
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   86400, // 24 hours
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	})

	// Set the key prefix
	store.KeyPrefix(r.keyPrefix)

	return store, nil
}

// GetUserSessionsCount retrieves the number of active sessions for a user
func (r *RedisClient) GetUserSessionsCount(userID string) (int, error) {
	pattern := fmt.Sprintf("%ssession:%s:*", r.keyPrefix, userID)
	keys, _, err := r.client.Scan(context.Background(), 0, pattern, 0).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to count user sessions: %w", err)
	}

	return len(keys), nil
}

// GetTunnelCount retrieves the number of active tunnels for a user
func (r *RedisClient) GetTunnelCount(userID string) (int, error) {
	key := fmt.Sprintf("%stunnels:%s", r.keyPrefix, userID)
	count, err := r.client.Get(context.Background(), key).Result()
	if err == redis.Nil {
		return 0, nil
	} else if err != nil {
		return 0, fmt.Errorf("failed to get tunnel count: %w", err)
	}

	countInt, err := strconv.Atoi(count)
	if err != nil {
		return 0, fmt.Errorf("invalid tunnel count: %w", err)
	}

	return countInt, nil
}

// IncrementTunnelCount increments the tunnel count for a user
func (r *RedisClient) IncrementTunnelCount(userID string) (int, error) {
	key := fmt.Sprintf("%stunnels:%s", r.keyPrefix, userID)
	count, err := r.client.Incr(context.Background(), key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment tunnel count: %w", err)
	}

	return int(count), nil
}

// DecrementTunnelCount decrements the tunnel count for a user
func (r *RedisClient) DecrementTunnelCount(userID string) (int, error) {
	key := fmt.Sprintf("%stunnels:%s", r.keyPrefix, userID)
	count, err := r.client.Decr(context.Background(), key).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to decrement tunnel count: %w", err)
	}

	// Ensure count doesn't go below 0
	if count < 0 {
		r.client.Set(context.Background(), key, 0, 0)
		return 0, nil
	}

	return int(count), nil
}

// SetTunnelExpiry sets a TTL for a tunnel
func (r *RedisClient) SetTunnelExpiry(tunnelID string, duration time.Duration) error {
	key := fmt.Sprintf("%stunnel:%s", r.keyPrefix, tunnelID)

	// Create the tunnel entry if it doesn't exist
	_, err := r.client.SetNX(context.Background(), key, time.Now().Format(time.RFC3339), duration).Result()
	if err != nil {
		return fmt.Errorf("failed to set tunnel expiry: %w", err)
	}

	return nil
}

// IsTunnelExpired checks if a tunnel is expired
func (r *RedisClient) IsTunnelExpired(tunnelID string) (bool, error) {
	key := fmt.Sprintf("%stunnel:%s", r.keyPrefix, tunnelID)

	// Check if the tunnel exists
	_, err := r.client.Get(context.Background(), key).Result()
	if err == redis.Nil {
		return true, nil
	} else if err != nil {
		return false, fmt.Errorf("failed to check tunnel expiry: %w", err)
	}

	return false, nil
}

// CreateSessionStore creates a new Redis session store
func CreateSessionStore(sessionKey string) (sessions.Store, error) {
	// Create a client
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Create a new redis store
	store, err := redisstore.NewRedisStore(context.Background(), client)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis store: %w", err)
	}

	// Set keys for the store
	if sessionKey != "" {
		store.KeyPrefix(sessionKey)
	} else {
		store.KeyPrefix("session_")
	}

	// Set session options
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   60 * 60 * 24, // 1 day
		HttpOnly: true,
	})

	return store, nil
}
