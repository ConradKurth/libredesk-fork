package cluster

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/zerodha/logf"
)

func discardLogger() *logf.Logger {
	lo := logf.New(logf.Opts{Writer: io.Discard})
	return &lo
}

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

func noop(context.Context) {}

// waitFor polls cond until it is true or the deadline passes.
func waitFor(cond func() bool, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return cond()
}

func TestNewStartsAsFollower(t *testing.T) {
	c := New(testRedis(t), "leader", time.Second, discardLogger())
	if c.IsLeader() {
		t.Fatal("a new coordinator must not report leadership")
	}
}

func TestElectsExactlyOneLeader(t *testing.T) {
	rdb := testRedis(t)
	const ttl = 300 * time.Millisecond
	c1 := New(rdb, "leader", ttl, discardLogger())
	c2 := New(rdb, "leader", ttl, discardLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c1.Run(ctx, noop)
	go c2.Run(ctx, noop)

	if !waitFor(func() bool { return c1.IsLeader() || c2.IsLeader() }, time.Second) {
		t.Fatal("no leader was elected")
	}
	// Give the follower ample opportunity to (wrongly) also acquire.
	time.Sleep(2 * ttl)
	if c1.IsLeader() == c2.IsLeader() {
		t.Fatalf("expected exactly one leader, got c1=%v c2=%v", c1.IsLeader(), c2.IsLeader())
	}
}

func TestLeaderRenewsAndKeepsLeadership(t *testing.T) {
	rdb := testRedis(t)
	const ttl = 300 * time.Millisecond
	c := New(rdb, "leader", ttl, discardLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx, noop)

	if !waitFor(c.IsLeader, time.Second) {
		t.Fatal("coordinator did not acquire leadership")
	}
	// Hold well past a single TTL; renewal must keep leadership alive.
	time.Sleep(3 * ttl)
	if !c.IsLeader() {
		t.Fatal("leader lost leadership despite active renewal")
	}
}

func TestFailoverWhenLeaderStops(t *testing.T) {
	rdb := testRedis(t)
	const ttl = 300 * time.Millisecond
	c1 := New(rdb, "leader", ttl, discardLogger())
	c2 := New(rdb, "leader", ttl, discardLogger())

	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	go c1.Run(ctx1, noop)
	if !waitFor(c1.IsLeader, time.Second) {
		t.Fatal("c1 did not become leader")
	}

	go c2.Run(ctx2, noop)
	// While c1 holds the lease, c2 must remain a follower.
	time.Sleep(ttl)
	if c2.IsLeader() {
		t.Fatal("c2 became leader while c1 was still leading")
	}

	// c1 steps down and releases the lease; c2 must take over promptly.
	cancel1()
	if !waitFor(c2.IsLeader, 2*time.Second) {
		t.Fatal("c2 did not take over after c1 stopped")
	}
}

func TestLeaderContextCancelledOnStop(t *testing.T) {
	c := New(testRedis(t), "leader", 300*time.Millisecond, discardLogger())

	ctx, cancel := context.WithCancel(context.Background())
	gotCtx := make(chan context.Context, 1)
	go c.Run(ctx, func(leaderCtx context.Context) { gotCtx <- leaderCtx })

	var leaderCtx context.Context
	select {
	case leaderCtx = <-gotCtx:
	case <-time.After(time.Second):
		t.Fatal("onElected was never called")
	}

	// Cancelling Run's context must cancel the leader-scoped context so jobs stop.
	cancel()
	select {
	case <-leaderCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("leader context was not cancelled after the coordinator stopped")
	}
}
