package api

import (
	"context"
	"encoding/json"
	"net/http"

	"sgound/internal/auth"
	"sgound/internal/db"
)

func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var user *db.User

		// 1. Check JWT (Cookie or Authorization Header)
		token := ""
		if cookie, err := r.Cookie("sgound_token"); err == nil {
			token = cookie.Value
		} else {
			authHeader := r.Header.Get("Authorization")
			if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
				token = authHeader[7:]
			}
		}

		if token != "" {
			claims, err := auth.VerifyJWT(token, s.jwtSecret)
			if err == nil {
				user, _ = s.MasterDB.GetUserByUsername(claims.Username)
			}
		}

		// 2. Check API Key (External clients)
		if user == nil {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				apiKey = r.URL.Query().Get("apiKey")
			}
			if apiKey != "" {
				keyInfo, err := s.MasterDB.GetAPIKeyInfo(apiKey)
				if err == nil {
					if !keyInfo.IsActive {
						http.Error(w, "API Key is blocked", http.StatusForbidden)
						return
					}
					user, _ = s.MasterDB.GetUserByUsername(keyInfo.Username)
					if user != nil {
						// Extraer IP real (considerando proxies)
						ip := r.Header.Get("X-Forwarded-For")
						if ip == "" {
							ip = r.RemoteAddr
						}
						ua := r.Header.Get("User-Agent")
						// Actualizar uso en segundo plano
						go s.MasterDB.UpdateAPIKeyUsage(apiKey, ip, ua)
					}
				}
			}
		}

		if user == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
			return
		}

		// 3. Permission Guard for Mutation Methods
		if r.Method != http.MethodGet && user.Permissions == "r" {
			// Special case: we allow OPTIONS for CORS
			if r.Method != http.MethodOptions {
				http.Error(w, "Forbidden: Read-only access", http.StatusForbidden)
				return
			}
		}

		ctx := context.WithValue(r.Context(), userKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		
		// Improved CSP: Allowing essential external metadata and image sources (supporting root domains and subdomains).
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' unpkg.com; style-src 'self' 'unsafe-inline' unpkg.com fonts.googleapis.com; font-src 'self' data: fonts.gstatic.com; img-src 'self' data: http://*.itunes.apple.com https://*.itunes.apple.com https://coverartarchive.org https://*.coverartarchive.org https://last.fm https://*.last.fm https://wikimedia.org https://*.wikimedia.org https://upload.wikimedia.org; connect-src 'self' lrclib.net api.lyrics.ovh http://*.itunes.apple.com https://*.itunes.apple.com https://musicbrainz.org https://*.musicbrainz.org")

		if r.TLS != nil {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}
