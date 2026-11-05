package processor

import (
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// ============================================================
// In-memory mode — core dedup semantics
// ============================================================

func TestMemoryDeduplicator_FirstCallNotDuplicate(t *testing.T) {
	d := NewMemoryDeduplicator(time.Minute)
	defer d.Stop()

	if d.IsDuplicate("0xabc") {
		t.Fatal("first call should NOT be a duplicate")
	}
}

func TestMemoryDeduplicator_RepeatWithinTTLIsDuplicate(t *testing.T) {
	d := NewMemoryDeduplicator(time.Minute)
	defer d.Stop()

	h := "0xabc"
	d.IsDuplicate(h) // seed
	if !d.IsDuplicate(h) {
		t.Fatal("repeat within TTL should be a duplicate")
	}
}

func TestMemoryDeduplicator_ExpiredAfterTTL(t *testing.T) {
	d := NewMemoryDeduplicator(50 * time.Millisecond)
	defer d.Stop()

	h := "0xexpired"
	d.IsDuplicate(h) // seed

	// wait beyond the TTL
	time.Sleep(80 * time.Millisecond)

	if d.IsDuplicate(h) {
		t.Fatal("hash should be re-accepted after TTL expiry")
	}
	// re-seeded on the query above; the follow-up call should be duplicate again
	if !d.IsDuplicate(h) {
		t.Fatal("hash re-seeded should now be a duplicate again")
	}
}

func TestMemoryDeduplicator_DistinctHashesNoConflict(t *testing.T) {
	d := NewMemoryDeduplicator(time.Minute)
	defer d.Stop()

	for _, h := range []string{"0x1", "0x2", "0x3"} {
		if d.IsDuplicate(h) {
			t.Fatalf("hash %s should not be a duplicate on first call", h)
		}
	}
}

// ============================================================
// In-memory mode — cleanup of expired entries
// ============================================================

func TestMemoryDeduplicator_CleanupExpiresOldEntries(t *testing.T) {
	d := &Deduplicator{
		seen: map[string]time.Time{"0xold": time.Now().Add(-2 * time.Minute)},
		ttl:  time.Minute,
	}
	d.cleanup()

	if _, ok := d.seen["0xold"]; ok {
		t.Fatal("expired entry should be removed by cleanup")
	}
}

func TestMemoryDeduplicator_CleanupKeepsRecentEntries(t *testing.T) {
	recent := time.Now().Add(-30 * time.Second)
	d := &Deduplicator{
		seen: map[string]time.Time{"0xfresh": recent},
		ttl:  time.Minute,
	}
	d.cleanup()

	if _, ok := d.seen["0xfresh"]; !ok {
		t.Fatal("unexpired entry should survive cleanup")
	}
}

// ============================================================
// Mode routing: presence of redisClient decides the branch
// ============================================================

func TestDeduplicator_RoutesToMemoryWhenNoRedis(t *testing.T) {
	d := NewMemoryDeduplicator(time.Minute)
	defer d.Stop()

	d.IsDuplicate("0xroute")
	if !d.IsDuplicate("0xroute") {
		t.Fatal("memory dedup should mark a repeat as duplicate")
	}
}

func TestDeduplicator_RedisDegradeToMemoryOnError(t *testing.T) {
	// an unreachable address forces SetNX to error and exercise the fallback
	rc := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	d := &Deduplicator{
		redisClient: rc,
		redisTTL:    time.Minute,                // used by the Redis SETNX path
		ttl:         time.Minute,                // used by the in-memory fallback path
		seen:        make(map[string]time.Time), // must be init'd for the memory fallback
	}

	h := "0xdegrade"
	// the failed SetNX degrades to in-memory dedup: first call is not a duplicate
	if d.IsDuplicate(h) {
		t.Fatal("error path should degrade to memory and not flag first call as duplicate")
	}
	// after the fallback recorded the hash, a repeat should be flagged
	if !d.IsDuplicate(h) {
		t.Fatal("memory fallback should catch the repeat after error")
	}
}
