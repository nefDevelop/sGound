package main

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

func main() {
	const (
		concurrency = 50   // Usuarios simultáneos
		totalReqs   = 500  // Peticiones totales
		baseURL     = "http://localhost:8088/api"
	)

	fmt.Printf("🚀 Iniciando test de carga en %s\n", baseURL)
	fmt.Printf("👥 Concurrencia: %d | Total: %d\n\n", concurrency, totalReqs)

	start := time.Now()
	var wg sync.WaitGroup
	ch := make(chan int, totalReqs)

	// Llenar el canal de trabajo
	for i := 0; i < totalReqs; i++ {
		ch <- i
	}
	close(ch)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range ch {
				// Alternar entre lista de tracks y búsqueda
				endpoint := "/tracks"
				if time.Now().UnixNano()%2 == 0 {
					endpoint = "/search?q=a"
				}
				
				resp, err := http.Get(baseURL + endpoint)
				if err != nil {
					fmt.Printf("❌ Error: %v\n", err)
					continue
				}
				resp.Body.Close()
			}
		}()
	}

	wg.Wait()
	duration := time.Since(start)

	fmt.Printf("\n✅ Test completado en %v\n", duration)
	fmt.Printf("📊 Promedio: %v por petición\n", duration/time.Duration(totalReqs))
	fmt.Printf("📈 Rps: %.2f\n", float64(totalReqs)/duration.Seconds())
}
