package handlers

import (
	"net/http"
	"net/url"
	"strings"
)

// originAllowed reports whether an Origin header may talk to the service: same host as the
// request (including the nginx proxy, which forwards Host), or one listed in allowed.
// Requests without Origin (curl, tests, server-to-server) are always allowed.
func originAllowed(r *http.Request, allowed []string) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}
	// Local development: Vite dev server and localhost variants.
	hostname := u.Hostname()
	if hostname == "localhost" || hostname == "127.0.0.1" {
		return true
	}
	for _, a := range allowed {
		if a == "*" || strings.EqualFold(strings.TrimRight(a, "/"), origin) {
			return true
		}
	}
	return false
}

// CORSMiddleware adds CORS headers for allowed origins and handles preflight requests.
func CORSMiddleware(next http.Handler, allowed []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" && originAllowed(r, allowed) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
