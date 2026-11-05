package processor

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Deduplicator deduplicates transactions
type Deduplicator struct {
	// memory mode
	mu   sync.RWMutex
	seen map[string]time.Time
	ttl  time.Duration

	// Redis mode (optional)
	redisClient *redis.Client
	redisTTL    time.Duration

	// cleanup control
	stopCleanup chan struct{}
}

// NewMemoryDeduplicator creates an in-memory deduplicator
func NewMemoryDeduplicator(ttl time.Duration) *Deduplicator {
	d := &Deduplicator{
		seen:        make(map[string]time.Time),
		ttl:         ttl,
		stopCleanup: make(chan struct{}),
	}
	go d.cleanupLoop()
	return d
}

// NewRedisDeduplicator creates a Redis-backed deduplicator
func NewRedisDeduplicator(redisClient *redis.Client, ttl time.Duration) *Deduplicator {
	return &Deduplicator{
		redisClient: redisClient,
		redisTTL:    ttl,
		ttl:         ttl, // keep in-memory TTL in sync so the fallback path still dedups
		stopCleanup: make(chan struct{}),
	}
}

// IsDuplicate reports whether the transaction has already been processed.
// It returns true if the transaction is a duplicate and should be skipped.
func (d *Deduplicator) IsDuplicate(txHash string) bool {
	if d.redisClient != nil {
		return d.isDuplicateRedis(txHash)
	}
	return d.isDuplicateMemory(txHash)
}

// isDuplicateMemory performs in-memory deduplication
func (d *Deduplicator) isDuplicateMemory(txHash string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	if lastSeen, ok := d.seen[txHash]; ok {
		if now.Sub(lastSeen) < d.ttl {
			return true // within TTL, treat as duplicate
		}
	}
	d.seen[txHash] = now
	return false
}

// isDuplicateRedis performs Redis-based deduplication (SETNX)
func (d *Deduplicator) isDuplicateRedis(txHash string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// SETNX: sets the key only if absent; returns true on first appearance
	ok, err := d.redisClient.SetNX(ctx, "txradar:dedup:"+txHash, 1, d.redisTTL).Result()
	if err != nil {
		// On Redis errors, degrade to memory mode to avoid blocking the main flow
		//TODO: consider logging the error for monitoring
		return d.isDuplicateMemory(txHash)
	}
	return !ok // SETNX returns false when the key exists → duplicate
}

// cleanupLoop periodically removes expired entries (memory mode only)
func (d *Deduplicator) cleanupLoop() {
	if d.redisClient != nil {
		return // Redis mode is auto-cleaned via TTL
	}

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopCleanup:
			return
		case <-ticker.C:
			d.cleanup()
		}
	}
}

// cleanup removes expired entries (memory mode only)
func (d *Deduplicator) cleanup() {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	for hash, lastSeen := range d.seen {
		if now.Sub(lastSeen) > d.ttl {
			delete(d.seen, hash)
		}
	}
}

// Stop terminates the cleanup goroutine
func (d *Deduplicator) Stop() {
	close(d.stopCleanup)
}
