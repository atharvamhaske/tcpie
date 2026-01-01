package ratelimiter

import (
	"log"
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
		Rate:       rate,
		LastRefill: time.Now(),
		Mutex:      &sync.Mutex{},
	}
}

// this method puts tokens in bucket
func (tb *TokenBucket) refillBucket() {
	log.Println("Refilling the bucket")
	now := time.Now()

	last := time.Since(tb.LastRefill)

	tokenstoAdd := (last.Milliseconds() * tb.Rate) / 1000
	log.Printf("adding %f tokens to bucket at %v", float64(tb.Tokens+tokenstoAdd), now)

	tb.Tokens = int64(math.Min(float64(tb.Tokens+tokenstoAdd), float64(tb.MaxTokens)))

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
