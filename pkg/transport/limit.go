package transport

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// LimitAuth bounds bcrypt work per source IP. Forwarded headers are deliberately
// ignored: trusting them requires a separately configured trusted proxy.
func LimitAuth(next http.Handler) http.Handler {
	type window struct {
		start time.Time
		count int
	}
	var mu sync.Mutex
	clients := make(map[string]window)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		now := time.Now()
		mu.Lock()
		entry := clients[ip]
		if now.Sub(entry.start) >= time.Minute {
			entry = window{start: now}
		}
		allowed := entry.count < 10
		if allowed {
			entry.count++
		}
		if len(clients) >= 10000 {
			for key, value := range clients {
				if now.Sub(value.start) >= time.Minute {
					delete(clients, key)
				}
			}
		}
		if _, exists := clients[ip]; !exists && len(clients) >= 10000 {
			allowed = false
		} else {
			clients[ip] = entry
		}
		mu.Unlock()
		if !allowed {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "too many authentication attempts", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
