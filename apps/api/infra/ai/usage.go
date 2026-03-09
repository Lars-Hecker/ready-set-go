package ai

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Checks if adding the max possible tokens exceeds the limit. If not, it reserves them.
// Sets a 40-day TTL to auto-expire old monthly keys.
var reserveScript = redis.NewScript(`
	local current = tonumber(redis.call("GET", KEYS[1]) or "0")
	local reserve_amount = tonumber(ARGV[1])
	local max_limit = tonumber(ARGV[2])

	if current + reserve_amount > max_limit then
		return -1 -- Denied: Quota exceeded
	end

	redis.call("INCRBY", KEYS[1], reserve_amount)
	redis.call("EXPIRE", KEYS[1], 3456000) -- 40 days
	return current + reserve_amount
`)

// Adjusts token count after generation completes (positive = charge more, negative = refund).
var adjustScript = redis.NewScript(`
	local amount = tonumber(ARGV[1])
	local result = redis.call("INCRBY", KEYS[1], amount)
	if amount > 0 then
		redis.call("EXPIRE", KEYS[1], 3456000) -- 40 days
	end
	return result
`)

// UsageTracker tracks per-user AI token usage with monthly auto-reset.
type UsageTracker struct {
	rdb          *redis.Client
	defaultQuota int64
}

// NewUsageTracker creates a new usage tracker.
func NewUsageTracker(rdb *redis.Client, defaultQuota int64) *UsageTracker {
	return &UsageTracker{
		rdb:          rdb,
		defaultQuota: defaultQuota,
	}
}

// UsageKey returns the Redis key for a user's monthly usage.
// Key format: ai:usage:{userID}:{YYYY-MM}
func UsageKey(userID string) string {
	return fmt.Sprintf("ai:usage:%s:%s", userID, time.Now().UTC().Format("2006-01"))
}

// Reserve attempts to reserve tokens for a request.
// Returns the usage key and new total if successful, or an error if quota exceeded.
// The returned key must be passed to Adjust to handle month boundaries correctly.
func (t *UsageTracker) Reserve(ctx context.Context, userID string, tokens int64, quota int64) (key string, total int64, err error) {
	if t.rdb == nil {
		return "", 0, nil
	}

	if quota == 0 {
		quota = t.defaultQuota
	}

	key = UsageKey(userID)
	result, err := reserveScript.Run(ctx, t.rdb, []string{key}, tokens, quota).Int64()
	if err != nil {
		return "", 0, fmt.Errorf("reserve tokens: %w", err)
	}

	if result == -1 {
		return "", 0, ErrQuotaExceeded
	}

	return key, result, nil
}

// Adjust modifies the token count after generation completes.
// Use negative values to refund, positive to charge extra.
// The key should be the one returned from Reserve.
func (t *UsageTracker) Adjust(ctx context.Context, key string, tokens int64) error {
	if t.rdb == nil || key == "" || tokens == 0 {
		return nil
	}

	_, err := adjustScript.Run(ctx, t.rdb, []string{key}, tokens).Int64()
	if err != nil {
		return fmt.Errorf("adjust tokens: %w", err)
	}

	return nil
}

// GetUsage returns the current monthly usage for a user.
func (t *UsageTracker) GetUsage(ctx context.Context, userID string) (int64, error) {
	if t.rdb == nil {
		return 0, nil
	}

	key := UsageKey(userID)
	result, err := t.rdb.Get(ctx, key).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, fmt.Errorf("get usage: %w", err)
	}

	return result, nil
}
