package gateway

import (
	"encoding/json"
	"net/http"

	"sgound/internal/db"
)

// SubsonicGateway actúa como el patrón Adapter para recibir tráfico OpenSubsonic
// y derivarlo al sistema interno o devolver respuestas formateadas
type SubsonicGateway struct {
	DB        *db.MasterDatabase
	MusicPath string // raíz de la biblioteca (jail de stream/download)
	CachePath string // caché de portadas (covers/<id>.jpg)
}

func NewSubsonicGateway(database *db.MasterDatabase, musicPath, cachePath string) *SubsonicGateway {
	return &SubsonicGateway{DB: database, MusicPath: musicPath, CachePath: cachePath}
}

// RegisterRoutes monta los endpoints OpenSubsonic en el multiplexor principal.
// De momento: ping, licencia, carpetas, navegación, búsqueda, stream y portadas.
func (g *SubsonicGateway) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/rest/ping.view", g.handlePing)
	mux.HandleFunc("/rest/getLicense.view", g.handleGetLicense)
	mux.HandleFunc("/rest/getMusicFolders.view", g.handleGetMusicFolders)
	mux.HandleFunc("/rest/getArtists.view", g.handleGetArtists)
	mux.HandleFunc("/rest/getArtist.view", g.handleGetArtist)
	mux.HandleFunc("/rest/getAlbum.view", g.handleGetAlbum)
	mux.HandleFunc("/rest/getSong.view", g.handleGetSong)
	mux.HandleFunc("/rest/search3.view", g.handleSearch3)
	mux.HandleFunc("/rest/stream.view", g.handleStream)
	mux.HandleFunc("/rest/download.view", g.handleDownload)
	mux.HandleFunc("/rest/getCoverArt.view", g.handleGetCoverArt)
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
