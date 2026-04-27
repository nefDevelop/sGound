package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
	"sync"
	"encoding/json"
	"bytes"
)

func main() {
	port := os.Getenv("SGOUND_PORT")
	if port == "" { port = "8088" }
	baseURL := fmt.Sprintf("http://localhost:%s/api", port)
	
	const (
		concurrency = 10
		username    = "user"
		password    = "useruser"
	)

	fmt.Println("🎵 Iniciando Benchmark de Transcodificación (FFmpeg)...")

	// 1. Auth
	token, _ := getAuthToken(baseURL, username, password)
	if token == "" {
		fmt.Println("❌ Error de Auth")
		return
	}

	// 2. Obtener un ID de track real
	trackID := getFirstTrackID(baseURL, token)
	if trackID == "" {
		fmt.Println("❌ No se encontraron canciones en la librería")
		return
	}

	fmt.Printf("🎯 Probando con Track ID: %s (Simulando %d usuarios)\n", trackID, concurrency)

	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			reqStart := time.Now()
			req, _ := http.NewRequest("GET", fmt.Sprintf("%s/stream/%s?bitrate=128k", baseURL, trackID), nil)
			req.Header.Set("Authorization", "Bearer "+token)
			
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				fmt.Printf("[%d] ❌ Error: %v\n", id, err)
				return
			}
			defer resp.Body.Close()

			// Medir TTFB (Time to First Byte)
			buf := make([]byte, 1)
			_, err = resp.Body.Read(buf)
			ttfb := time.Since(reqStart)

			if err != nil {
				fmt.Printf("[%d] ❌ Fallo al leer primer byte: %v\n", id, err)
				return
			}

			// Leer los primeros 512KB para simular buffer de reproducción
			io.CopyN(io.Discard, resp.Body, 512*1024)
			totalTime := time.Since(reqStart)

			fmt.Printf("[%d] ✅ TTFB: %v | 512KB leídos en %v\n", id, ttfb, totalTime)
		}(i)
	}

	wg.Wait()
	fmt.Printf("\n🏁 Benchmark finalizado en %v\n", time.Since(start))
}

func getAuthToken(baseURL, user, pass string) (string, error) {
	payload, _ := json.Marshal(map[string]string{"username": user, "password": pass})
	resp, err := http.Post(baseURL+"/auth/login", "application/json", bytes.NewBuffer(payload))
	if err != nil || resp.StatusCode != 200 { return "", err }
	var res struct { Token string }
	json.NewDecoder(resp.Body).Decode(&res)
	return res.Token, nil
}

func getFirstTrackID(baseURL, token string) string {
	req, _ := http.NewRequest("GET", baseURL+"/tracks?limit=1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil { return "" }
	var res []struct { ID string }
	json.NewDecoder(resp.Body).Decode(&res)
	if len(res) > 0 { return res[0].ID }
	return ""
}
