package api

import (
	"net/http"
	"sync"
	"time"

	"proidentity/internal/requestip"
)

type rateBucket struct {
	WindowStart time.Time
	Count       int
}

var (
	rateMu      sync.Mutex
	rateBuckets = map[string]rateBucket{}
)

func rateLimit(maxRequests int, window time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			now := time.Now()
			key := r.URL.Path + "|" + remoteAddr(r)

			rateMu.Lock()
			if len(rateBuckets) > 10000 {
				rateBuckets = map[string]rateBucket{}
			}
			for k, b := range rateBuckets {
				if now.Sub(b.WindowStart) > 2*window {
					delete(rateBuckets, k)
				}
			}
			b := rateBuckets[key]
			if b.WindowStart.IsZero() || now.Sub(b.WindowStart) >= window {
				b = rateBucket{WindowStart: now}
			}
			b.Count++
			rateBuckets[key] = b
			limited := b.Count > maxRequests
			rateMu.Unlock()

			if limited {
				jsonError(w, http.StatusTooManyRequests, "too many requests")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func remoteAddr(r *http.Request) string {
	return requestip.ClientIP(r)
}
