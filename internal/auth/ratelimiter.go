package auth

import (
	"sync"
	"time"
)

// LoginAttempt tracks failed login attempts for rate limiting
type LoginAttempt struct {
	Count     int
	FirstFail time.Time
	LastFail  time.Time
	LockedUntil time.Time
}

// RateLimiter prevents brute force attacks on PIN login
type RateLimiter struct {
	mu       sync.RWMutex
	attempts map[string]*LoginAttempt // client IP -> attempts
	maxAttempts int
	lockoutDuration time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		attempts:        make(map[string]*LoginAttempt),
		maxAttempts:     5,
		lockoutDuration: 30 * time.Second, // Initial lockout, doubles each time
	}
}

// RecordFailedAttempt records a failed login attempt for the given client
func (r *RateLimiter) RecordFailedAttempt(clientIP string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	attempt, exists := r.attempts[clientIP]

	if !exists {
		// First failed attempt
		r.attempts[clientIP] = &LoginAttempt{
			Count:     1,
			FirstFail: now,
			LastFail:  now,
		}
		return
	}

	// Reset counter if it's been more than 1 hour since first failure
	if now.Sub(attempt.FirstFail) > time.Hour {
		attempt.Count = 1
		attempt.FirstFail = now
		attempt.LastFail = now
		attempt.LockedUntil = time.Time{}
		return
	}

	// Increment failure count
	attempt.Count++
	attempt.LastFail = now

	// Apply lockout after max attempts
	if attempt.Count >= r.maxAttempts {
		// Progressive backoff: 30s, 60s, 120s, etc.
		lockoutDuration := r.lockoutDuration
		if attempt.Count > r.maxAttempts {
			// Double the lockout duration for each additional failure
			multiplier := 1 << (attempt.Count - r.maxAttempts)
			lockoutDuration = lockoutDuration * time.Duration(multiplier)

			// Cap at 10 minutes
			if lockoutDuration > 10*time.Minute {
				lockoutDuration = 10*time.Minute
			}
		}

		attempt.LockedUntil = now.Add(lockoutDuration)
	}
}

// IsLocked returns whether the client is currently rate limited
func (r *RateLimiter) IsLocked(clientIP string) (bool, time.Duration) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	attempt, exists := r.attempts[clientIP]
	if !exists {
		return false, 0
	}

	now := time.Now()

	// Check if lockout has expired
	if !attempt.LockedUntil.IsZero() && now.After(attempt.LockedUntil) {
		return false, 0
	}

	// Check if client is currently locked
	if !attempt.LockedUntil.IsZero() && now.Before(attempt.LockedUntil) {
		remaining := attempt.LockedUntil.Sub(now)
		return true, remaining
	}

	return false, 0
}

// ClearAttempts clears failed attempts for a client (on successful login)
func (r *RateLimiter) ClearAttempts(clientIP string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.attempts, clientIP)
}

// CleanupExpired removes old attempt records (call periodically)
func (r *RateLimiter) CleanupExpired() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for ip, attempt := range r.attempts {
		// Remove attempts older than 1 hour that aren't locked
		if now.Sub(attempt.FirstFail) > time.Hour &&
		   (attempt.LockedUntil.IsZero() || now.After(attempt.LockedUntil)) {
			delete(r.attempts, ip)
		}
	}
}