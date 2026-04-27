package redis

import (
	"context"
	"fmt"
	"time"

	redisv9 "github.com/redis/go-redis/v9"
)

func Connect(ctx context.Context, redisURL string) (*redisv9.Client, error) {
	opts, err := redisv9.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis config: %w", err)
	}

	client := redisv9.NewClient(opts)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return client, nil
}
