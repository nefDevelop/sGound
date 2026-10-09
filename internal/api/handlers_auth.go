package api

import (
	"encoding/json"
	"net/http"

	"sgound/internal/auth"
	"sgound/internal/db"
)

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	user, err := s.MasterDB.GetUserByUsername(creds.Username)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	match, err := auth.VerifyPassword(creds.Password, user.PasswordHash)
	if err != nil || !match {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := auth.GenerateJWT(user.Username, s.jwtSecret)
	if err != nil {
		http.Error(w, "Token generation failed", http.StatusInternalServerError)
		return
	}

	// Set as cookie for web sessions
	http.SetCookie(w, &http.Cookie{
		Name:     "sgound_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.Config.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"token":       token,
		"username":    user.Username,
		"permissions": user.Permissions,
	})
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	username := r.URL.Query().Get("username")

	if username == "" || username == "check" {
		count, _ := s.MasterDB.GetUserCount()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"setupRequired": count == 0,
			"global":        true,
		})
		return
	}

	// Check if user is in AllowedUsers in config
	perms, allowed := s.Config.AllowedUsers[username]
	if !allowed {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "User not authorized in server config"})
		return
	}

	user, _ := s.MasterDB.GetUserByUsername(username)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"allowed":       true,
		"setupRequired": user == nil,
		"permissions":   perms,
	})
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	// Check if allowed
	perms, allowed := s.Config.AllowedUsers[body.Username]
	if !allowed {
		http.Error(w, "User not authorized in server config", http.StatusForbidden)
		return
	}

	user, _ := s.MasterDB.GetUserByUsername(body.Username)
	if user != nil {
		http.Error(w, "User already exists", http.StatusForbidden)
		return
	}

	hash, _ := auth.HashPassword(body.Password)

	// Create user with perms from config
	err := s.MasterDB.CreateUser(body.Username, hash, perms, "", "*")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Automatic Session: Set JWT cookie so user lands directly in dashboard
	token, err := auth.GenerateJWT(body.Username, s.jwtSecret)
	if err == nil {
		http.SetCookie(w, &http.Cookie{
			Name:     "sgound_token",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			Secure:   s.Config.CookieSecure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   86400 * 30,
		})
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "success",
		"username":    body.Username,
		"permissions": perms,
	})
}

func (s *Server) handleAPIKeys(w http.ResponseWriter, r *http.Request) {
	user, _ := r.Context().Value(userKey).(*db.User)
	if user == nil {
		return
	}

	if r.Method == http.MethodGet {
		keys, _ := s.MasterDB.GetAPIKeys(user.Username)
		json.NewEncoder(w).Encode(keys)
		return
	}

	if r.Method == http.MethodPost {
		var body struct {
			Label       string `json:"label"`
			Permissions string `json:"permissions"` // "r" or "rw"
		}
		json.NewDecoder(r.Body).Decode(&body)

		// Guard: Cannot grant more than you have
		if user.Permissions == "r" && body.Permissions == "rw" {
			http.Error(w, "Cannot create RW key with R user account", http.StatusForbidden)
			return
		}

		key := auth.GenerateAPIKey()
		err := s.MasterDB.CreateAPIKey(key, user.Username, body.Permissions, body.Label)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"key": key, "label": body.Label, "permissions": body.Permissions})
		return
	}

	if r.Method == http.MethodDelete {
		key := r.URL.Query().Get("key")
		s.MasterDB.DeleteAPIKey(key, user.Username)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method == http.MethodPatch {
		var body struct {
			Key    string `json:"key"`
			Active bool   `json:"active"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}

		err := s.MasterDB.ToggleAPIKeyStatus(body.Key, user.Username, body.Active)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}
}
