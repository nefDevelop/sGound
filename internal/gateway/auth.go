package gateway

import (
	"encoding/hex"
	"net/http"
	"strings"

	"sgound/internal/auth"
	"sgound/internal/db"
)

// authenticate aplica las reglas del protocolo y devuelve el usuario.
// Escribe el error y devuelve false si la petición no se autoriza.
func (g *SubsonicGateway) authenticate(w http.ResponseWriter, r *http.Request) (*db.User, bool) {
	q := r.URL.Query()

	if q.Get("v") == "" || q.Get("c") == "" {
		writeErr(w, r, errMissingParam, "Faltan los parámetros v y/o c.")
		return nil, false
	}

	apiKey := q.Get("apiKey")
	hasP := q.Get("p") != ""
	hasT := q.Get("t") != "" || q.Get("s") != ""

	// apiKey no admite p/t/s a la vez (u se tolera: muchos clientes lo envían).
	if apiKey != "" && (hasP || hasT) {
		writeErr(w, r, errConflictingAuth, "apiKey no admite p/t/s a la vez.")
		return nil, false
	}

	if apiKey != "" {
		info, err := g.DB.GetAPIKeyInfo(apiKey)
		if err != nil {
			writeErr(w, r, errBadAPIKey, "API key inválida.")
			return nil, false
		}
		if !info.IsActive {
			writeErr(w, r, errNotAuthorized, "API key bloqueada.")
			return nil, false
		}
		user, err := g.DB.GetUserByUsername(info.Username)
		if err != nil || user == nil {
			writeErr(w, r, errBadAPIKey, "API key sin usuario.")
			return nil, false
		}
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.RemoteAddr
		}
		go g.DB.UpdateAPIKeyUsage(apiKey, ip, r.Header.Get("User-Agent"))
		return user, true
	}

	username := q.Get("u")
	if username == "" {
		writeErr(w, r, errBadCredentials, "Falta el usuario.")
		return nil, false
	}
	user, err := g.DB.GetUserByUsername(username)
	if err != nil || user == nil {
		writeErr(w, r, errBadCredentials, "Usuario o contraseña incorrectos.")
		return nil, false
	}

	// Token t/s: no verificable contra hashes argon2 -> 41 según protocolo.
	if hasT {
		writeErr(w, r, errTokenNoSupport, "Autenticación por token no soportada; usa contraseña o API key.")
		return nil, false
	}
	if !hasP {
		writeErr(w, r, errBadCredentials, "Falta la contraseña.")
		return nil, false
	}

	pass := q.Get("p")
	if strings.HasPrefix(pass, "enc:") {
		raw, err := hex.DecodeString(strings.TrimPrefix(pass, "enc:"))
		if err != nil {
			writeErr(w, r, errBadCredentials, "Contraseña hex inválida.")
			return nil, false
		}
		pass = string(raw)
	}
	ok, err := auth.VerifyPassword(pass, user.PasswordHash)
	if err != nil || !ok {
		writeErr(w, r, errBadCredentials, "Usuario o contraseña incorrectos.")
		return nil, false
	}
	return user, true
}
