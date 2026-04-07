package main

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

func main() {
	const (
		concurrency = 200    // Usuarios simultáneos (Estábamos en 50)
		totalReqs   = 10000  // Peticiones totales (Estábamos en 500)
		baseURL     = "http://localhost:8088/api"
	)

	fmt.Printf("🔥 Iniciando STRESS TEST en %s\n", baseURL)
	fmt.Printf("👥 Concurrencia: %d | Total Reqs: %d\n\n", concurrency, totalReqs)

	latencies := make([]time.Duration, totalReqs)
	var mu sync.Mutex
	idx := 0

	start := time.Now()
	var wg sync.WaitGroup
	ch := make(chan int, totalReqs)

	for i := 0; i < totalReqs; i++ {
		ch <- i
	}
	close(ch)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range ch {
				reqStart := time.Now()
				
				endpoint := "/tracks"
				if time.Now().UnixNano()%2 == 0 {
					endpoint = "/search?q=a"
				}
				
				resp, err := http.Get(baseURL + endpoint)
				reqDuration := time.Since(reqStart)
				
				mu.Lock()
				latencies[idx] = reqDuration
				idx++
				mu.Unlock()

				if err != nil {
					continue
				}
				resp.Body.Close()
			}
		}()
	}

	wg.Wait()
	totalDuration := time.Since(start)

	// Calcular estadísticas
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	p50 := latencies[totalReqs/2]
	p95 := latencies[int(float64(totalReqs)*0.95)]
	p99 := latencies[int(float64(totalReqs)*0.99)]

	fmt.Printf("✅ Stress Test Completado en %v\n", totalDuration)
	fmt.Printf("📈 RPS: %.2f\n", float64(totalReqs)/totalDuration.Seconds())
	fmt.Printf("📊 Latencia Media: %v\n", totalDuration/time.Duration(totalReqs))
	fmt.Printf("🎯 P50: %v\n", p50)
	fmt.Printf("🎯 P95: %v\n", p95)
	fmt.Printf("🎯 P99: %v\n", p99)
}
