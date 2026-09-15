// Package cluster provides Redis-based leader election so that, in a
// multi-instance deployment, singleton background jobs (periodic scanners,
// external pollers such as IMAP receivers, time-triggers and cleaners) run on
// exactly one instance at a time. Libredesk's web/WebSocket tier is safe to run
// on every replica (the Redis WS backplane fans out broadcasts), but those
// background jobs would duplicate their side-effects — double email sends,
// duplicate ticket ingestion, doubled notifications — if they ran on more than
// one instance. This package elects a single leader to run them, with automatic
// failover when the leader dies.
//
// It uses a single Redis key as a lease: SETNX to acquire, a compare-and-expire
// Lua script to renew, and a compare-and-delete Lua script to release. This is a
// lightweight single-Redis lease, not full Redlock; it assumes one Redis, which
// Libredesk already depends on. The design always fails safe: on any Redis error
// the holder steps down rather than risk two leaders.
package cluster

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/zerodha/logf"
)

// DefaultLeaseTTL is the lease lifetime when none is configured. If the leader
// dies, a standby claims leadership after at most this long. The lease is
// renewed every TTL/3 while the leader is healthy.
const DefaultLeaseTTL = 15 * time.Second

// renewScript extends the lease only if this instance still owns it. Returns 1
// on success, 0 if the key is missing or owned by someone else.
var renewScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("pexpire", KEYS[1], ARGV[2])
end
return 0
`)

// releaseScript deletes the lease only if this instance still owns it, so a
// stepping-down leader never deletes a lease another instance has since taken.
var releaseScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("del", KEYS[1])
end
return 0
`)

// Coordinator contends for leadership on a single Redis lease key.
type Coordinator struct {
	rdb *redis.Client
	key string
	id  string
	ttl time.Duration
	lo  *logf.Logger

	mu       sync.RWMutex
	isLeader bool
}

// New returns a Coordinator that contends for the lease at key. id is unique per
// process, so a lease can be attributed to its holder. A non-positive ttl falls
// back to DefaultLeaseTTL.
func New(rdb *redis.Client, key string, ttl time.Duration, lo *logf.Logger) *Coordinator {
	if ttl <= 0 {
		ttl = DefaultLeaseTTL
	}
	return &Coordinator{
		rdb: rdb,
		key: key,
		id:  uuid.NewString(),
		ttl: ttl,
		lo:  lo,
	}
}

// IsLeader reports whether this instance currently holds leadership.
func (c *Coordinator) IsLeader() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isLeader
}

func (c *Coordinator) setLeader(v bool) {
	c.mu.Lock()
	c.isLeader = v
	c.mu.Unlock()
}

// Run contends for leadership until ctx is cancelled. Each time this instance
// acquires the lease it calls onElected in the caller's goroutine with a
// leaderCtx that is cancelled the moment leadership is lost (renewal failure or
// ctx cancellation). Callers start their singleton jobs in onElected; those jobs
// must stop when leaderCtx is done. onElected may be called again on re-election,
// so it must be safe to invoke more than once over the process lifetime.
func (c *Coordinator) Run(ctx context.Context, onElected func(leaderCtx context.Context)) {
	retry := c.renewInterval()
	for {
		if ctx.Err() != nil {
			return
		}
		ok, err := c.rdb.SetNX(ctx, c.key, c.id, c.ttl).Result()
		switch {
		case err != nil:
			// Redis unreachable: stay a follower and retry. Never assume
			// leadership on error — that could produce two leaders.
			if ctx.Err() == nil {
				c.lo.Error("leader election: acquire failed", "error", err)
			}
		case ok:
			c.lo.Info("leader election: acquired leadership", "id", c.id)
			c.lead(ctx, onElected)
			c.lo.Info("leader election: lost leadership", "id", c.id)
		}
		if !sleep(ctx, retry) {
			return
		}
	}
}

// lead holds leadership: it starts the singleton jobs, renews the lease until it
// is lost or ctx is cancelled, and releases the lease on the way out so a standby
// can take over promptly rather than waiting for expiry.
func (c *Coordinator) lead(ctx context.Context, onElected func(context.Context)) {
	leaderCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	c.setLeader(true)
	defer c.setLeader(false)
	// Release with a fresh context so it still runs when ctx is already cancelled.
	defer c.release()

	onElected(leaderCtx)

	ticker := time.NewTicker(c.renewInterval())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ok, err := renewScript.Run(ctx, c.rdb, []string{c.key}, c.id, c.ttl.Milliseconds()).Bool()
			if err != nil {
				// Can't renew: step down. The lease will expire and another
				// instance can take over — stepping down now avoids split-brain.
				if ctx.Err() == nil {
					c.lo.Error("leader election: renew failed, stepping down", "error", err)
				}
				return
			}
			if !ok {
				// Lease no longer ours (expired and taken elsewhere). Step down.
				c.lo.Warn("leader election: lease lost, stepping down", "id", c.id)
				return
			}
		}
	}
}

func (c *Coordinator) release() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := releaseScript.Run(ctx, c.rdb, []string{c.key}, c.id).Err(); err != nil {
		c.lo.Error("leader election: release failed", "error", err)
	}
}

// renewInterval is how often the lease is renewed and how often acquisition is
// retried: a third of the TTL, so two renewals can fail before the lease expires.
func (c *Coordinator) renewInterval() time.Duration {
	d := c.ttl / 3
	if d <= 0 {
		d = time.Second
	}
	return d
}

// sleep waits for d or ctx cancellation. It returns false if ctx was cancelled.
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
