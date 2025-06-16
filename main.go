// main.go
package main

import (
	"context"
	"fmt"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/config"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/domain"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/fasthttp"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/http"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/result"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/utils"
	"github.com/fatih/color"
	"golang.org/x/time/rate"
	"math/rand"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	urlBufferSize    = 250
	resultBufferSize = 35
)

var builderPool = sync.Pool{
	New: func() interface{} {
		return &strings.Builder{}
	},
}

func main() {
	// More aggressive GC settings for low memory
	debug.SetGCPercent(50)                   // Reduced from 75
	debug.SetMemoryLimit(1400 * 1024 * 1024) // 1,4GB limit

	var markers []string

	cfg := config.ParseFlags()

	initialDomains := domain.GetDomains(cfg.DomainsFile, cfg.Domain)
	paths := utils.ReadLines(cfg.PathsFile)
	if cfg.MarkersFile != "" {
		markers = utils.ReadLines(cfg.MarkersFile)
	}

	// Create rate limiter with burst of 1
	limiter := rate.NewLimiter(rate.Limit(cfg.Concurrency), cfg.Concurrency)

	validateInput(initialDomains, paths, markers)

	rand.Seed(time.Now().UnixNano())

	printInitialInfo(cfg, initialDomains, paths)

	resultBufferSize := cfg.Concurrency * 3 // Give breathing room
	if resultBufferSize < 100 {
		resultBufferSize = 120
	}

	urlChan := make(chan string, urlBufferSize)
	resultsChan := make(chan result.Result, resultBufferSize)

	// Memory-bounded semaphore for active requests
	requestSemaphore := make(chan struct{}, cfg.Concurrency)

	var client interface {
		MakeRequest(url string) result.Result
	}

	if cfg.FastHTTP {
		client = fasthttp.NewClient(cfg)
	} else {
		client = http.NewClient(cfg)
	}

	var processedCount int64
	var totalURLs int64

	// Start URL generation with better backpressure
	go generateURLsWithBackpressure(initialDomains, paths, cfg, urlChan, &totalURLs)

	var wg sync.WaitGroup
	// Reduce workers to match concurrency limit
	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go worker(urlChan, resultsChan, &wg, client, &processedCount, limiter, requestSemaphore)
	}

	done := make(chan bool)
	go trackProgress(&processedCount, &totalURLs, done)

	go func() {
		wg.Wait()
		close(resultsChan)
		done <- true
	}()

	// Process results with periodic GC
	resultCount := 0
	for res := range resultsChan {
		result.ProcessResult(res, cfg, markers)
		resultCount++

		// Force GC every 100 results to free memory
		if resultCount%100 == 0 {
			runtime.GC()
		}
	}

	color.Green("\n[✔] Scan completed.")
}

func validateInput(initialDomains, paths, markers []string) {
	if len(initialDomains) == 0 {
		color.Red("[✘] Error: The domain list is empty. Please provide at least one domain.")
		os.Exit(1)
	}

	if len(paths) == 0 {
		color.Red("[✘] Error: The path list is empty. Please provide at least one path.")
		os.Exit(1)
	}

	if len(markers) == 0 {
		color.Yellow("[!] Warning: The marker list is empty. The scan will just use the size filter which might not be very useful.")
	}
}

func printInitialInfo(cfg config.Config, initialDomains, paths []string) {
	color.Cyan("[i] Scanning %d domains with %d paths", len(initialDomains), len(paths))
	color.Cyan("[i] Minimum file size to detect: %d bytes", cfg.MinContentSize)
	color.Cyan("[i] Filtering for HTTP status code: %s", cfg.HTTPStatusCodes)

	if len(cfg.ExtraHeaders) > 0 {
		color.Cyan("[i] Using extra headers:")
		for key, value := range cfg.ExtraHeaders {
			color.Cyan("  %s: %s", key, value)
		}
	}
}

// Improved URL generation with proper backpressure
func generateURLsWithBackpressure(domains, paths []string, cfg config.Config, urlChan chan<- string, totalURLs *int64) {
	defer close(urlChan)

	// Don't pre-calculate total URLs to save memory
	// Process domains one by one
	for _, d := range domains {
		// Check memory before processing each domain
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		if m.Alloc > 900*1024*1024 { // If approaching 900MB
			runtime.GC()
			time.Sleep(100 * time.Millisecond) // Give GC time to work
		}

		streamURLsForDomainWithBackpressure(d, paths, &cfg, urlChan, totalURLs)
	}
}

// Stream URLs with proper backpressure (blocking sends)
func streamURLsForDomainWithBackpressure(domainD string, paths []string, cfg *config.Config, urlChan chan<- string, totalURLs *int64) {
	proto := "https"
	if cfg.ForceHTTPProt {
		proto = "http"
	}

	d := strings.TrimSuffix(
		strings.TrimPrefix(
			strings.TrimPrefix(domainD, "https://"),
			"http://"),
		"/",
	)

	b := builderPool.Get().(*strings.Builder)
	defer func() {
		b.Reset()
		builderPool.Put(b)
	}()
	b.Grow(256)

	// Process paths one by one to reduce memory pressure
	for _, path := range paths {
		if strings.HasPrefix(path, "##") {
			continue
		}

		// Root path
		if !cfg.SkipRootFolderCheck {
			b.Reset()
			b.WriteString(proto)
			b.WriteString("://")
			b.WriteString(d)
			b.WriteString("/")
			b.WriteString(path)

			// Blocking send - will wait if channel is full
			urlChan <- b.String()
			atomic.AddInt64(totalURLs, 1)
		}

		// Base paths
		for _, base := range cfg.BasePaths {
			b.Reset()
			b.WriteString(proto)
			b.WriteString("://")
			b.WriteString(d)
			if !cfg.IgnoreBasePathSlash {
				b.WriteString("/")
			}
			b.WriteString(base)
			b.WriteString("/")
			b.WriteString(path)

			urlChan <- b.String()
			atomic.AddInt64(totalURLs, 1)
		}

		if cfg.DontGeneratePaths {
			continue
		}

		// Generate domain parts one by one
		domain.StreamDomainParts(d, cfg, func(word string) {
			if len(cfg.BasePaths) == 0 {
				b.Reset()
				b.WriteString(proto)
				b.WriteString("://")
				b.WriteString(d)
				b.WriteString("/")
				b.WriteString(word)
				b.WriteString("/")
				b.WriteString(path)

				urlChan <- b.String()
				atomic.AddInt64(totalURLs, 1)
			} else {
				for _, base := range cfg.BasePaths {
					b.Reset()
					b.WriteString(proto)
					b.WriteString("://")
					b.WriteString(d)
					b.WriteString("/")
					b.WriteString(base)
					b.WriteString("/")
					b.WriteString(word)
					b.WriteString("/")
					b.WriteString(path)

					urlChan <- b.String()
					atomic.AddInt64(totalURLs, 1)
				}
			}
		})
	}
}

func worker(urls <-chan string, results chan<- result.Result, wg *sync.WaitGroup, client interface {
	MakeRequest(url string) result.Result
}, processedCount *int64, limiter *rate.Limiter, semaphore chan struct{}) {
	defer wg.Done()

	for url := range urls {
		// Acquire semaphore
		semaphore <- struct{}{}

		// Wait for rate limiter
		err := limiter.Wait(context.Background())
		if err != nil {
			<-semaphore
			continue
		}

		res := client.MakeRequest(url)
		atomic.AddInt64(processedCount, 1)

		// FIX: NEVER DROP RESULTS - Block until processed
		results <- res // This will block if channel is full, ensuring no drops

		<-semaphore
		res.Content = "" // Clear memory
	}
}

func trackProgress(processedCount, totalURLs *int64, done chan bool) {
	start := time.Now()
	lastProcessed := int64(0)
	lastUpdate := start

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			now := time.Now()
			elapsed := now.Sub(start)
			currentProcessed := atomic.LoadInt64(processedCount)
			total := atomic.LoadInt64(totalURLs)

			// Calculate RPS
			intervalElapsed := now.Sub(lastUpdate)
			intervalProcessed := currentProcessed - lastProcessed
			rps := float64(intervalProcessed) / intervalElapsed.Seconds()

			// Memory stats
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			memMB := m.Alloc / 1024 / 1024

			if total > 0 {
				percentage := float64(currentProcessed) / float64(total) * 100
				estimatedTotal := float64(elapsed) / (float64(currentProcessed) / float64(total))
				remainingTime := time.Duration(estimatedTotal - float64(elapsed))
				fmt.Printf("\r%-100s", "")
				fmt.Printf("\rProgress: %.2f%% (%d/%d) | RPS: %.2f | Mem: %dMB | Elapsed: %s | ETA: %s",
					percentage, currentProcessed, total, rps, memMB,
					elapsed.Round(time.Second), remainingTime.Round(time.Second))
			} else {
				fmt.Printf("\r%-100s", "")
				fmt.Printf("\rProcessed: %d | RPS: %.2f | Mem: %dMB | Elapsed: %s",
					currentProcessed, rps, memMB, elapsed.Round(time.Second))
			}

			lastProcessed = currentProcessed
			lastUpdate = now
		}
	}
}
