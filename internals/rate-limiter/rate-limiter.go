package ratelimiter

import (
	"math"
	"sync"
	"time"
)

type TokenBucket struct {
	MaxTokens  int64
	Tokens     int64
	Rate       int64
	LastRefill time.Time
	Mutex      *sync.Mutex
}

func RateLimiter(rate, tokens int64) TokenBucket {
	return TokenBucket{
		MaxTokens:  tokens,
		Tokens:     tokens, // Start with full bucket
		Rate:       rate,
		LastRefill: time.Now(),
		Mutex:      &sync.Mutex{},
	}
}

// this method puts tokens in bucket
func (tb *TokenBucket) refillBucket() {
	now := time.Now()
	elapsed := now.Sub(tb.LastRefill)

	// Calculate tokens to add: rate is tokens per second
	// Use float64 to avoid integer division truncation
	secondsElapsed := elapsed.Seconds()
	tokensToAdd := secondsElapsed * float64(tb.Rate)

	// Add tokens (cap at MaxTokens)
	newTokens := float64(tb.Tokens) + tokensToAdd
	tb.Tokens = int64(math.Min(newTokens, float64(tb.MaxTokens)))

	tb.LastRefill = now
}

// method to check is request allowed or should be dropped
func (tb *TokenBucket) IsReqAllowed() bool {
	tb.Mutex.Lock()
	defer tb.Mutex.Unlock()

	tb.refillBucket()
	if tb.Tokens > 0 {
		tb.Tokens--
		return true
	}
	return false
}
