package main

import (
	"fmt"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/config"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/domain"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/fasthttp"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/http"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/result"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/utils"
	"github.com/fatih/color"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

const (
	urlBufferSize = 15000
)

func main() {
	cfg := config.ParseFlags()

	initialDomains := domain.GetDomains(cfg.DomainsFile, cfg.Domain)
	paths := utils.ReadLines(cfg.PathsFile)
	markers := utils.ReadLines(cfg.MarkersFile)

	validateInput(initialDomains, paths, markers)

	rand.Seed(time.Now().UnixNano())

	printInitialInfo(cfg, initialDomains, paths)

	urlChan := make(chan string, urlBufferSize)
	resultsChan := make(chan result.Result, cfg.Concurrency)

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

	go generateURLs(initialDomains, paths, cfg, urlChan, &totalURLs)

	var wg sync.WaitGroup
	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go worker(urlChan, resultsChan, &wg, cfg, client, &processedCount)
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
	if cfg.PerformProtocolCheck {
		color.Cyan("[*] Checking protocols and blacklist for %d domains concurrently...", len(initialDomains))
	} else {
		color.Cyan("[*] Skipping protocol check, using HTTPS for all domains...")
	}

	if cfg.UseStaticWordSeparator {
		color.Cyan("[*] Static word separation is active")
		color.Cyan("[*] Loaded %d entries for word separation", len(cfg.StaticWords))
	} else {
		color.Cyan("[*] Using default word separation")
	}

	color.Cyan("[i] Scanning %d domains with %d paths", len(initialDomains), len(paths))
	color.Cyan("[i] Minimum file size to detect: %d bytes", cfg.MinFileSize)
	color.Cyan("[i] Filtering for HTTP status code: %d", cfg.HTTPStatusCode)

	if len(cfg.ExtraHeaders) > 0 {
		color.Cyan("[i] Using extra headers:")
		for key, value := range cfg.ExtraHeaders {
			color.Cyan("  %s: %s", key, value)
		}
	}
}

func generateURLs(initialDomains, paths []string, cfg config.Config, urlChan chan<- string, totalURLs *int64) {
	defer close(urlChan)

	for _, d := range initialDomains {
		domainURLs, _ := domain.GenerateURLs([]string{d}, paths, &cfg)
		atomic.AddInt64(totalURLs, int64(len(domainURLs)))
		for _, url := range domainURLs {
			urlChan <- url
		}
	}
}

func worker(urls <-chan string, results chan<- result.Result, wg *sync.WaitGroup, cfg config.Config, client interface {
	MakeRequest(url string) result.Result
}, processedCount *int64) {
	defer wg.Done()

	for url := range urls {
		res := client.MakeRequest(url)
		atomic.AddInt64(processedCount, 1)
		results <- res
	}
}

func trackProgress(processedCount, totalURLs *int64, done chan bool) {
	start := time.Now()
	for {
		select {
		case <-done:
			return
		default:
			processed := atomic.LoadInt64(processedCount)
			total := atomic.LoadInt64(totalURLs)
			if total > 0 {
				percentage := float64(processed) / float64(total) * 100
				elapsed := time.Since(start)
				estimatedTotal := float64(elapsed) / (float64(processed) / float64(total))
				remainingTime := time.Duration(estimatedTotal - float64(elapsed))

				fmt.Printf("\rProgress: %.2f%% (%d/%d) | Elapsed: %s | ETA: %s",
					percentage, processed, total, elapsed.Round(time.Second), remainingTime.Round(time.Second))
			} else {
				fmt.Printf("\rProcessed: %d | Elapsed: %s", processed, time.Since(start).Round(time.Second))
			}
			time.Sleep(time.Second)
		}
	}
}
