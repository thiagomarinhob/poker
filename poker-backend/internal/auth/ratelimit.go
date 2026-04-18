package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	maxLoginAttempts = 5
	rateLimitWindow  = 15 * time.Minute
)

// checkRateLimit increments the attempt counter for ip.
// Fails open on Redis errors to avoid blocking legitimate users.
func checkRateLimit(ctx context.Context, rdb *redis.Client, ip string) error {
	key := fmt.Sprintf("ratelimit:login:%s", ip)

	count, err := rdb.Incr(ctx, key).Result()
	if err != nil {
		return nil
	}
	if count == 1 {
		rdb.Expire(ctx, key, rateLimitWindow)
	}
	if count > maxLoginAttempts {
		return ErrRateLimited
	}
	return nil
}
