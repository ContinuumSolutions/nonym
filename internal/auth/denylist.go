package auth

import (
	"sync"
	"time"
)

// TokenDenylist manages invalidated JWT tokens for logout functionality
type TokenDenylist struct {
	mu            sync.RWMutex
	tokens        map[string]time.Time // tokenID -> expiry time
	cleanupTicker *time.Ticker
	stopCh        chan struct{}
}

// NewTokenDenylist creates a new token denylist with automatic cleanup
func NewTokenDenylist() *TokenDenylist {
	d := &TokenDenylist{
		tokens:        make(map[string]time.Time),
		cleanupTicker: time.NewTicker(1 * time.Hour),
		stopCh:        make(chan struct{}),
	}

	// Start background cleanup goroutine
	go d.cleanup()

	return d
}

// Add adds a token ID to the denylist with its expiry time
func (d *TokenDenylist) Add(tokenID string, expiresAt time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.tokens[tokenID] = expiresAt
}

// IsBlacklisted checks if a token ID is in the denylist
func (d *TokenDenylist) IsBlacklisted(tokenID string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	expiry, exists := d.tokens[tokenID]
	if !exists {
		return false
	}

	// Check if token has expired naturally (cleanup)
	if time.Now().After(expiry) {
		return false
	}

	return true
}

// cleanup runs periodically to remove expired tokens from the denylist
func (d *TokenDenylist) cleanup() {
	for {
		select {
		case <-d.cleanupTicker.C:
			d.removeExpired()
		case <-d.stopCh:
			return
		}
	}
}

// removeExpired removes naturally expired tokens from the denylist
func (d *TokenDenylist) removeExpired() {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	for tokenID, expiry := range d.tokens {
		if now.After(expiry) {
			delete(d.tokens, tokenID)
		}
	}
}

// Stop stops the background cleanup goroutine
func (d *TokenDenylist) Stop() {
	close(d.stopCh)
	d.cleanupTicker.Stop()
}

// Size returns the current size of the denylist (for debugging/monitoring)
func (d *TokenDenylist) Size() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.tokens)
}