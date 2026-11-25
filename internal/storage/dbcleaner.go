package storage

import (
	"context"
	"log"
	"time"
)

// RetentionManager manages data retention by periodically
// cleaning up expired data.
type RetentionManager struct {
	cleaner           Cleaner
	postgresRetention time.Duration // how long to retain PostgreSQL data
	cleanupInterval   time.Duration // interval between cleanups

	stopChan chan struct{}
}

// NewRetentionManager creates a retention manager.
func NewRetentionManager(cleaner Cleaner, postgresRetention time.Duration) *RetentionManager {
	return &RetentionManager{
		cleaner:           cleaner,
		postgresRetention: postgresRetention,
		cleanupInterval:   1 * time.Hour,
		stopChan:          make(chan struct{}),
	}
}

// Start starts the periodic cleanup loop.
func (r *RetentionManager) Start(ctx context.Context) {
	go r.cleanupLoop(ctx)
	log.Printf("Data retention manager started (retention %v)", r.postgresRetention)
}

// cleanupLoop periodically cleans up expired data.
func (r *RetentionManager) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(r.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopChan:
			return
		case <-ticker.C:
			r.cleanup(ctx)
		}
	}
}

// cleanup deletes expired data.
func (r *RetentionManager) cleanup(ctx context.Context) {
	cutoff := time.Now().Add(-r.postgresRetention)
	result, err := r.cleaner.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		log.Printf("Failed to clean up expired data: %v", err)
		return
	}

	log.Printf("Cleaned up %d expired records (older than %v)", result, cutoff)
}

// Stop stops the cleanup loop.
func (r *RetentionManager) Stop() {
	close(r.stopChan)
}
