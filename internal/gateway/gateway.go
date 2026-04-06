package gateway

import (
	"encoding/json"
	"net/http"

	"sgound/internal/db"
)

// SubsonicGateway actúa como el patrón Adapter para recibir tráfico OpenSubsonic
// y derivarlo al sistema interno o devolver respuestas formateadas
type SubsonicGateway struct {
	DB *db.Database
}

func NewSubsonicGateway(database *db.Database) *SubsonicGateway {
	return &SubsonicGateway{DB: database}
}

// RegisterRoutes monta los endpoints "fake" legacy en el multiplexor principal
func (g *SubsonicGateway) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/rest/ping.view", g.handlePing)
	// Múltiples adiciones en el futuro:
	// mux.HandleFunc("/rest/getArtists.view", g.handleGetArtists)
}

// SubsonicResponseWrapper forma la estructura ridícula que exigen los clientes Subsonic
type SubsonicResponseWrapper struct {
	SubsonicResponse struct {
		Status        string `json:"status"`
		Version       string `json:"version"`
		Type          string `json:"type"`
		ServerVersion string `json:"serverVersion"`
	} `json:"subsonic-response"`
}

func (g *SubsonicGateway) handlePing(w http.ResponseWriter, r *http.Request) {
	// A futuro se lee: r.URL.Query().Get("u"), Get("v"), Get("t"), etc.

	resp := SubsonicResponseWrapper{}
	resp.SubsonicResponse.Status = "ok"
	resp.SubsonicResponse.Version = "1.16.1"
	resp.SubsonicResponse.Type = "sgound"
	resp.SubsonicResponse.ServerVersion = "1.0.0"

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
