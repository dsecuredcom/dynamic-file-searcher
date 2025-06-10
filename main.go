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
	urlBufferSize    = 750
	resultBufferSize = 25
)

const domainChunkSize = 100

var builderPool = sync.Pool{
	New: func() interface{} {
		return &strings.Builder{}
	},
}

func main() {
	debug.SetGCPercent(50)
	debug.SetMemoryLimit(1750 * 1024 * 1024)

	var markers []string

	cfg := config.ParseFlags()

	initialDomains := domain.GetDomains(cfg.DomainsFile, cfg.Domain)
	paths := utils.ReadLines(cfg.PathsFile)
	if cfg.MarkersFile != "" {
		markers = utils.ReadLines(cfg.MarkersFile)
	}

	limiter := rate.NewLimiter(rate.Limit(cfg.Concurrency), 1)

	validateInput(initialDomains, paths, markers)

	rand.Seed(time.Now().UnixNano())

	printInitialInfo(cfg, initialDomains, paths)

	urlChan := make(chan string, urlBufferSize)
	resultsChan := make(chan result.Result, resultBufferSize)

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

	// Start URL generation in a goroutine
	go generateURLsStreaming(initialDomains, paths, cfg, urlChan, &totalURLs)

	var wg sync.WaitGroup
	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go worker(urlChan, resultsChan, &wg, client, &processedCount, limiter)
	}

	done := make(chan bool)
	go trackProgress(&processedCount, &totalURLs, done)

	go func() {
		wg.Wait()
		close(resultsChan)
		done <- true
	}()

	for res := range resultsChan {
		result.ProcessResult(res, cfg, markers)
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

// New streaming URL generation
func generateURLsStreaming(domains, paths []string, cfg config.Config, urlChan chan<- string, totalURLs *int64) {
	defer close(urlChan)

	// Estimate upfront (optional)
	estimate := estimateURLCount(domains, paths, &cfg)
	*totalURLs = int64(estimate)

	// Chunked generation with occasional GC
	const chunkSize = 100
	for i := 0; i < len(domains); i += chunkSize {
		end := i + chunkSize
		if end > len(domains) {
			end = len(domains)
		}
		for _, d := range domains[i:end] {
			streamURLsForDomain(d, paths, &cfg, urlChan)
		}
		// Trigger GC if memory grows too much
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		if m.Alloc > 1750*1024*1024 {
			runtime.GC()
		}
	}
}

// Estimate URL count without generating them
func estimateURLCount(domains, paths []string, cfg *config.Config) int {
	count := 0

	// Sample estimation based on first domain
	if len(domains) > 0 {
		sampleDomain := domains[0]
		partsCount := 0

		domain.StreamDomainParts(sampleDomain, cfg, func(part string) {
			partsCount++
		})

		// Estimate per domain
		perDomainCount := 0
		for _, path := range paths {
			if strings.HasPrefix(path, "##") {
				continue
			}

			if !cfg.SkipRootFolderCheck {
				perDomainCount++
			}

			perDomainCount += len(cfg.BasePaths)

			if !cfg.DontGeneratePaths {
				if len(cfg.BasePaths) == 0 {
					perDomainCount += partsCount
				} else {
					perDomainCount += partsCount * len(cfg.BasePaths)
				}
			}
		}

		count = perDomainCount * len(domains)
	}

	return count
}

// Stream URLs for a single domain
func streamURLsForDomain(domainD string, paths []string, cfg *config.Config, urlChan chan<- string) {
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
			select {
			case urlChan <- b.String():
			default:
			}
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
			select {
			case urlChan <- b.String():
			default:
			}
		}

		if cfg.DontGeneratePaths {
			continue
		}

		domain.StreamDomainParts(d, cfg, func(word string) {
			b.Reset()
			b.WriteString(proto)
			b.WriteString("://")
			b.WriteString(d)
			b.WriteString("/")
			b.WriteString(word)
			b.WriteString("/")
			b.WriteString(path)
			select {
			case urlChan <- b.String():
			default:
			}
		})
	}
}

func worker(urls <-chan string, results chan<- result.Result, wg *sync.WaitGroup, client interface {
	MakeRequest(url string) result.Result
}, processedCount *int64, limiter *rate.Limiter) {
	defer wg.Done()

	for url := range urls {
		err := limiter.Wait(context.Background())
		if err != nil {
			continue
		}
		res := client.MakeRequest(url)
		atomic.AddInt64(processedCount, 1)

		// Non-blocking send to results
		select {
		case results <- res:
		case <-time.After(50 * time.Millisecond):
			// Skip if results channel is full
		}
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

			if total > 0 {
				percentage := float64(currentProcessed) / float64(total) * 100
				estimatedTotal := float64(elapsed) / (float64(currentProcessed) / float64(total))
				remainingTime := time.Duration(estimatedTotal - float64(elapsed))
				fmt.Printf("\r%-100s", "")
				fmt.Printf("\rProgress: %.2f%% (%d/%d) | RPS: %.2f | Elapsed: %s | ETA: %s",
					percentage, currentProcessed, total, rps,
					elapsed.Round(time.Second), remainingTime.Round(time.Second))
			} else {
				fmt.Printf("\r%-100s", "")
				fmt.Printf("\rProcessed: %d | RPS: %.2f | Elapsed: %s",
					currentProcessed, rps, elapsed.Round(time.Second))
			}

			lastProcessed = currentProcessed
			lastUpdate = now
		}
	}
}
