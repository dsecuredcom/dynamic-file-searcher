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

type DebugStats struct {
	urlsGenerated   int64
	requestsSent    int64
	resultsReceived int64
	resultsDropped  int64
	errors          int64
	successes       int64
	channelFull     int64
	startTime       time.Time
}

var debugStats = DebugStats{
	startTime: time.Now(),
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

	resultBufferSize := cfg.Concurrency * 2 // Give breathing room
	if resultBufferSize < 100 {
		resultBufferSize = 100
	}

	color.Cyan("[i] Result buffer size: %d (concurrency: %d)", resultBufferSize, cfg.Concurrency)
	color.Cyan("[i] Rate limiter burst: %d", cfg.Concurrency)

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
		go workerWithStats(urlChan, resultsChan, &wg, client, &processedCount, limiter, requestSemaphore)
	}

	done := make(chan bool)
	go trackProgressWithStats(&processedCount, &totalURLs, done)

	go func() {
		wg.Wait()
		close(resultsChan)
		done <- true
	}()

	// Process results with integrated debugging
	processResultsWithIntegratedStats(resultsChan, cfg, markers)

	// Print final statistics
	printFinalStats()

	color.Green("\n[✔] Scan completed.")
}

func workerWithStats(urls <-chan string, results chan<- result.Result, wg *sync.WaitGroup,
	client interface {
		MakeRequest(url string) result.Result
	},
	processedCount *int64, limiter *rate.Limiter, semaphore chan struct{}) {
	defer wg.Done()

	for url := range urls {
		// Acquire semaphore
		semaphore <- struct{}{}
		atomic.AddInt64(&debugStats.requestsSent, 1)

		err := limiter.Wait(context.Background())
		if err != nil {
			<-semaphore
			atomic.AddInt64(&debugStats.errors, 1)
			continue
		}

		res := client.MakeRequest(url)
		atomic.AddInt64(processedCount, 1)

		// Track success/error
		if res.Error != nil {
			atomic.AddInt64(&debugStats.errors, 1)
		} else {
			atomic.AddInt64(&debugStats.successes, 1)
		}

		// FIXED: Never drop results - always block until processed
		select {
		case results <- res:
			// Result successfully sent
		default:
			// Channel is full - this indicates backpressure
			atomic.AddInt64(&debugStats.channelFull, 1)
			results <- res // Block until space available
		}

		<-semaphore

		// Clear result content
		res.Content = ""
	}
}

// Enhanced result processing with integrated statistics
func processResultsWithIntegratedStats(resultsChan <-chan result.Result, cfg config.Config, markers []string) {
	resultCount := 0
	lastStatsUpdate := time.Now()

	for res := range resultsChan {
		atomic.AddInt64(&debugStats.resultsReceived, 1)
		result.ProcessResult(res, cfg, markers)
		resultCount++

		// Print stats every 1000 results or every 10 seconds
		now := time.Now()
		if resultCount%1000 == 0 || now.Sub(lastStatsUpdate) > 10*time.Second {
			printIntermediateStats()
			lastStatsUpdate = now
		}

		// Force GC every 100 results to free memory
		if resultCount%100 == 0 {
			runtime.GC()
		}
	}
}

// Print intermediate statistics during processing
func printIntermediateStats() {
	sent := atomic.LoadInt64(&debugStats.requestsSent)
	received := atomic.LoadInt64(&debugStats.resultsReceived)
	errors := atomic.LoadInt64(&debugStats.errors)
	successes := atomic.LoadInt64(&debugStats.successes)
	channelFull := atomic.LoadInt64(&debugStats.channelFull)

	if sent > 0 {
		dropRate := float64(sent-received) / float64(sent) * 100
		errorRate := float64(errors) / float64(sent) * 100
		successRate := float64(successes) / float64(sent) * 100

		color.Yellow("\n[STATS] Sent: %d | Received: %d | Success: %d (%.1f%%) | Errors: %d (%.1f%%) | Channel full: %d | Drop rate: %.2f%%",
			sent, received, successes, successRate, errors, errorRate, channelFull, dropRate)

		// Warning if drop rate is high
		if dropRate > 1.0 {
			color.Red("[WARNING] High drop rate detected! Consider increasing concurrency buffer or reducing concurrency.")
		}

		// Warning if channel is frequently full
		if channelFull > 100 {
			color.Red("[WARNING] Result channel frequently full (%d times). Results may be delayed.", channelFull)
		}
	}
}

// Enhanced progress tracking with statistics
func trackProgressWithStats(processedCount, totalURLs *int64, done chan bool) {
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

			// Get current stats
			sent := atomic.LoadInt64(&debugStats.requestsSent)
			received := atomic.LoadInt64(&debugStats.resultsReceived)
			errors := atomic.LoadInt64(&debugStats.errors)

			if total > 0 {
				percentage := float64(currentProcessed) / float64(total) * 100
				estimatedTotal := float64(elapsed) / (float64(currentProcessed) / float64(total))
				remainingTime := time.Duration(estimatedTotal - float64(elapsed))

				dropRate := float64(0)
				if sent > 0 {
					dropRate = float64(sent-received) / float64(sent) * 100
				}

				fmt.Printf("\r%-120s", "")
				fmt.Printf("\rProgress: %.1f%% (%d/%d) | RPS: %.1f | Mem: %dMB | Errors: %d | Drops: %.1f%% | ETA: %s",
					percentage, currentProcessed, total, rps, memMB, errors, dropRate,
					remainingTime.Round(time.Second))
			} else {
				dropRate := float64(0)
				if sent > 0 {
					dropRate = float64(sent-received) / float64(sent) * 100
				}

				fmt.Printf("\r%-120s", "")
				fmt.Printf("\rProcessed: %d | RPS: %.1f | Mem: %dMB | Errors: %d | Drops: %.1f%% | Elapsed: %s",
					currentProcessed, rps, memMB, errors, dropRate, elapsed.Round(time.Second))
			}

			lastProcessed = currentProcessed
			lastUpdate = now
		}
	}
}

// Print comprehensive final statistics
func printFinalStats() {
	elapsed := time.Since(debugStats.startTime)
	generated := atomic.LoadInt64(&debugStats.urlsGenerated)
	sent := atomic.LoadInt64(&debugStats.requestsSent)
	received := atomic.LoadInt64(&debugStats.resultsReceived)
	errors := atomic.LoadInt64(&debugStats.errors)
	successes := atomic.LoadInt64(&debugStats.successes)
	channelFull := atomic.LoadInt64(&debugStats.channelFull)

	color.Cyan("\n" + strings.Repeat("=", 60))
	color.Cyan("FINAL STATISTICS")
	color.Cyan(strings.Repeat("=", 60))

	color.Green("Execution time: %s", elapsed.Round(time.Second))
	color.Green("URLs generated: %d", generated)
	color.Green("Requests sent: %d", sent)
	color.Green("Results received: %d", received)
	color.Green("Successful requests: %d", successes)
	color.Green("Failed requests: %d", errors)

	if sent > 0 {
		dropRate := float64(sent-received) / float64(sent) * 100
		errorRate := float64(errors) / float64(sent) * 100
		successRate := float64(successes) / float64(sent) * 100
		avgRPS := float64(sent) / elapsed.Seconds()

		color.Yellow("Success rate: %.2f%%", successRate)
		color.Yellow("Error rate: %.2f%%", errorRate)
		color.Yellow("Drop rate: %.2f%%", dropRate)
		color.Yellow("Average RPS: %.2f", avgRPS)
		color.Yellow("Channel full events: %d", channelFull)

		// Health assessment
		if dropRate > 5.0 {
			color.Red("❌ HIGH DROP RATE - Results are being lost!")
			color.Red("   Recommendation: Increase result buffer size or reduce concurrency")
		} else if dropRate > 1.0 {
			color.Yellow("⚠️  MODERATE DROP RATE - Some results may be lost")
		} else {
			color.Green("✅ LOW DROP RATE - System performing well")
		}

		if errorRate > 20.0 {
			color.Red("❌ HIGH ERROR RATE - Check network connectivity or target server")
		} else if errorRate > 5.0 {
			color.Yellow("⚠️  MODERATE ERROR RATE - Some requests failing")
		} else {
			color.Green("✅ LOW ERROR RATE - Network performance good")
		}
	}

	color.Cyan(strings.Repeat("=", 60))
}

// Enhanced URL generation with counting
func generateURLsWithBackpressure(domains, paths []string, cfg config.Config, urlChan chan<- string, totalURLs *int64) {
	defer close(urlChan)

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

// Enhanced URL streaming with statistics
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

			urlChan <- b.String()
			atomic.AddInt64(totalURLs, 1)
			atomic.AddInt64(&debugStats.urlsGenerated, 1)
		}

		// Base paths
		for _, base := range cfg.BasePaths {
			b.Reset()
			b.WriteString(proto)
			b.WriteString("://")
			b.WriteString(d)
			b.WriteString("/")
			b.WriteString(base)
			b.WriteString("/")
			b.WriteString(path)

			urlChan <- b.String()
			atomic.AddInt64(totalURLs, 1)
			atomic.AddInt64(&debugStats.urlsGenerated, 1)
		}

		if cfg.DontGeneratePaths {
			continue
		}

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
				atomic.AddInt64(&debugStats.urlsGenerated, 1)
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
					atomic.AddInt64(&debugStats.urlsGenerated, 1)
				}
			}
		})
	}
}

// Keep existing functions unchanged
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
