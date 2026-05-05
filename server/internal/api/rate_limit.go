package api

import (
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
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
	peer := peerHost(r.RemoteAddr)
	if trustedForwarder(peer) {
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			if ip := firstForwardedIP(xri); ip != "" {
				return ip
			}
		}
		if os.Getenv("PROIDENTITY_TRUST_X_FORWARDED_FOR") == "1" {
			xff := r.Header.Get("X-Forwarded-For")
			if ip := firstForwardedIP(xff); ip != "" {
				return ip
			}
		}
	}
	if peer == "" {
		return r.RemoteAddr
	}
	return peer
}

func peerHost(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return addr
	}
	return host
}

func firstForwardedIP(xff string) string {
	first := xff
	if i := strings.Index(first, ","); i >= 0 {
		first = first[:i]
	}
	first = strings.TrimSpace(first)
	if net.ParseIP(first) == nil {
		return ""
	}
	return first
}

func trustedForwarder(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() && os.Getenv("PROIDENTITY_TRUST_LOOPBACK_PROXY") == "1" {
		return true
	}
	for _, raw := range strings.Split(os.Getenv("PROIDENTITY_TRUSTED_PROXIES"), ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if trustedIP := net.ParseIP(raw); trustedIP != nil {
			if trustedIP.Equal(ip) {
				return true
			}
			continue
		}
		_, cidr, err := net.ParseCIDR(raw)
		if err == nil && cidr.Contains(ip) {
			return true
		}
	}
	return false
}
