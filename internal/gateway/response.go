package gateway

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
)

// Versión del protocolo que declaramos hablar.
const (
	subsonicVersion = "1.16.1"
	serverType      = "sgound"
	serverVersion   = "1.0.0"
)

// Códigos de error del protocolo.
const (
	errGeneric        = 0
	errMissingParam   = 10
	errClientTooOld   = 20
	errServerTooOld   = 30
	errBadCredentials = 40
	errTokenNoSupport = 41
	errAuthUnsupported = 42
	errConflictingAuth = 43
	errBadAPIKey      = 44
	errNotAuthorized  = 50
	errNotFound       = 70
)

type subError struct {
	XMLName xml.Name `xml:"error" json:"-"`
	Code    int      `xml:"code,attr" json:"code"`
	Message string   `xml:"message,attr,omitempty" json:"message,omitempty"`
}

// envelope es la respuesta raíz. Inner lleva el payload del endpoint
// ya serializado (cada endpoint define su propio elemento raíz).
type envelope struct {
	XMLName       xml.Name  `xml:"subsonic-response" json:"-"`
	Xmlns         string    `xml:"xmlns,attr" json:"-"`
	Status        string    `xml:"status,attr" json:"status"`
	Version       string    `xml:"version,attr" json:"version"`
	Type          string    `xml:"type,attr" json:"type"`
	ServerVersion string    `xml:"serverVersion,attr" json:"serverVersion"`
	OpenSubsonic  bool      `xml:"openSubsonic,attr" json:"openSubsonic"`
	Error         *subError `xml:"error,omitempty" json:"error,omitempty"`
	Inner         []byte    `xml:",innerxml" json:"-"`
}

// writeOK serializa el payload según f=json|xml (por defecto xml).
func writeOK(w http.ResponseWriter, r *http.Request, endpoint string, xmlPayload any, jsonPayload any) {
	if r.URL.Query().Get("f") == "json" {
		base := map[string]any{
			"status":        "ok",
			"version":       subsonicVersion,
			"type":          serverType,
			"serverVersion": serverVersion,
			"openSubsonic":  true,
		}
		if endpoint != "" && jsonPayload != nil {
			base[endpoint] = jsonPayload
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{"subsonic-response": base})
		return
	}
	env := envelope{
		Xmlns:         "http://subsonic.org/restapi",
		Status:        "ok",
		Version:       subsonicVersion,
		Type:          serverType,
		ServerVersion: serverVersion,
		OpenSubsonic:  true,
	}
	if xmlPayload != nil {
		raw, err := xml.Marshal(xmlPayload)
		if err != nil {
			writeErr(w, r, errGeneric, "Error serializando la respuesta.")
			return
		}
		env.Inner = raw
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(env)
}

// writeErr serializa un fallo con su código de protocolo.
func writeErr(w http.ResponseWriter, r *http.Request, code int, msg string) {
	if r.URL.Query().Get("f") == "json" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]any{"subsonic-response": map[string]any{
			"status":        "failed",
			"version":       subsonicVersion,
			"type":          serverType,
			"serverVersion": serverVersion,
			"openSubsonic":  true,
			"error":         map[string]any{"code": code, "message": msg},
		}})
		return
	}
	env := envelope{
		Xmlns:        "http://subsonic.org/restapi",
		Status:       "failed",
		Version:      subsonicVersion,
		Type:         serverType,
		ServerVersion: serverVersion,
		OpenSubsonic: true,
		Error:        &subError{Code: code, Message: msg},
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(env)
}
