package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"
)

func main() {
	port := os.Getenv("SGOUND_PORT")
	if port == "" {
		port = "8080"
	}
	baseURL := fmt.Sprintf("http://localhost:%s/api", port)
	
	const (
		concurrency = 200
		totalReqs   = 10000
		username    = "user"
		password    = "useruser"
	)

	fmt.Printf("🚀 Preparando TEST DE CARGA en %s\n", baseURL)

	// 1. Obtener Token JWT
	token, err := getAuthToken(baseURL, username, password)
	if err != nil {
		fmt.Printf("❌ Error de Auth: %v. Intentando registrar usuario...\n", err)
		if err := registerUser(baseURL, username, password); err != nil {
			fmt.Printf("❌ Error fatal: no se pudo registrar usuario: %v\n", err)
			return
		}
		token, _ = getAuthToken(baseURL, username, password)
	}

	if token == "" {
		fmt.Println("❌ No se pudo obtener token JWT. Abortando.")
		return
	}

	fmt.Printf("🔑 Token obtenido. Iniciando test con %d usuarios...\n\n", concurrency)

	latencies := make([]time.Duration, 0, totalReqs)
	var mu sync.Mutex
	errors := 0
	
	start := time.Now()
	var wg sync.WaitGroup
	ch := make(chan int, totalReqs)

	for i := 0; i < totalReqs; i++ {
		ch <- i
	}
	close(ch)

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range ch {
				reqStart := time.Now()
				
				// Mezclar endpoints: 70% tracks, 30% search
				endpoint := "/tracks"
				if time.Now().UnixNano()%3 == 0 {
					endpoint = "/search?q=a"
				}
				
				req, _ := http.NewRequest("GET", baseURL+endpoint, nil)
				req.Header.Set("Authorization", "Bearer "+token)
				
				resp, err := client.Do(req)
				duration := time.Since(reqStart)

				mu.Lock()
				if err != nil || resp.StatusCode != http.StatusOK {
					errors++
				} else {
					latencies = append(latencies, duration)
				}
				mu.Unlock()

				if err == nil {
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
				}
			}
		}()
	}

	wg.Wait()
	totalDuration := time.Since(start)

	if len(latencies) == 0 {
		fmt.Println("❌ Todas las peticiones fallaron.")
		return
	}

	// Calcular estadísticas
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	p50 := latencies[len(latencies)/2]
	p95 := latencies[int(float64(len(latencies))*0.95)]
	p99 := latencies[int(float64(len(latencies))*0.99)]

	fmt.Printf("✅ Test Completado en %v\n", totalDuration)
	fmt.Printf("📊 Total Reqs: %d | Errores: %d\n", totalReqs, errors)
	fmt.Printf("📈 RPS: %.2f\n", float64(totalReqs)/totalDuration.Seconds())
	fmt.Printf("🎯 Latencia P50: %v\n", p50)
	fmt.Printf("🎯 Latencia P95: %v\n", p95)
	fmt.Printf("🎯 Latencia P99: %v\n", p99)
}

func getAuthToken(baseURL, user, pass string) (string, error) {
	payload, _ := json.Marshal(map[string]string{
		"username": user,
		"password": pass,
	})
	resp, err := http.Post(baseURL+"/auth/login", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	var result struct {
		Token string `json:"token"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Token, nil
}

func registerUser(baseURL, user, pass string) error {
	payload, _ := json.Marshal(map[string]string{
		"username": user,
		"password": pass,
	})
	resp, err := http.Post(baseURL+"/auth/setup", "application/json", bytes.NewBuffer(payload))
	if err == nil {
		resp.Body.Close()
	}
	return err
}
