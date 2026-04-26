package auth

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// rateLimiter holds two maps of token-bucket limiters: one keyed by phone
// number, one by source IP. Per-key buckets are created lazily on first
// use and aged out by sweep().
//
// IMPORTANT: this limiter is per-process. Across multiple Fly machines an
// attacker can multiply their effective allowance by the number of
// instances. README documents this gap; compensating controls are Twilio
// Verify's own per-phone limits + fraud detection + cost monitoring.
// Replace with a shared backend (Postgres, Redis) if abuse materializes.
type rateLimiter struct {
	mu      sync.Mutex
	byPhone map[string]*entry
	byIP    map[string]*entry
}

type entry struct {
	lim      *rate.Limiter
	lastSeen time.Time
}

// Limits derived from the threat model: SMS is the primary cost-and-abuse
// vector. 3 attempts per phone per 10 minutes is enough for a real user
// with a typo'd code; 10 per IP per 10 minutes is enough for two devices
// on the same NAT.
const (
	phoneRefillEvery = 10 * time.Minute / 3 // ~3.33 minutes per token
	phoneBurst       = 3
	ipRefillEvery    = 10 * time.Minute / 10
	ipBurst          = 10
	bucketIdleTTL    = 30 * time.Minute
)

func newRateLimiter() *rateLimiter {
	rl := &rateLimiter{
		byPhone: map[string]*entry{},
		byIP:    map[string]*entry{},
	}
	go rl.sweepLoop()
	return rl
}

func (rl *rateLimiter) allowPhone(phone string) bool {
	return rl.allow(rl.byPhone, phone, rate.Every(phoneRefillEvery), phoneBurst)
}

func (rl *rateLimiter) allowIP(ip string) bool {
	if ip == "" {
		// No IP? Fail open here — we still have the per-phone bucket
		// catching abuse, and refusing requests with no resolvable IP
		// would break legitimate clients behind weird proxies.
		return true
	}
	return rl.allow(rl.byIP, ip, rate.Every(ipRefillEvery), ipBurst)
}

func (rl *rateLimiter) allow(m map[string]*entry, key string, every rate.Limit, burst int) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	e, ok := m[key]
	if !ok {
		e = &entry{lim: rate.NewLimiter(every, burst)}
		m[key] = e
	}
	e.lastSeen = time.Now()
	return e.lim.Allow()
}

// sweepLoop periodically removes idle buckets so memory doesn't grow
// unbounded under attack. Held under the same mutex as allow(), so
// serialization is fine for our request volume.
func (rl *rateLimiter) sweepLoop() {
	t := time.NewTicker(bucketIdleTTL)
	defer t.Stop()
	for range t.C {
		rl.sweep()
	}
}

func (rl *rateLimiter) sweep() {
	cutoff := time.Now().Add(-bucketIdleTTL)
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for k, e := range rl.byPhone {
		if e.lastSeen.Before(cutoff) {
			delete(rl.byPhone, k)
		}
	}
	for k, e := range rl.byIP {
		if e.lastSeen.Before(cutoff) {
			delete(rl.byIP, k)
		}
	}
}
