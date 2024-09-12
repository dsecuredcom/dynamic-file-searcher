package main

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/dsecuredcom/dynamic-file-searcher/pkg/config"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/domain"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/http"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/result"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/utils"
	"github.com/fatih/color"
	"github.com/schollz/progressbar/v3"
)

func main() {
	cfg := config.ParseFlags()

	initialDomains := domain.GetDomains(cfg.DomainsFile, cfg.Domain)

	if len(initialDomains) == 0 {
		color.Red("[✘] Error: The domain list is empty. Please provide at least one domain.")
		os.Exit(1)
	}

	paths := utils.ReadLines(cfg.PathsFile)

	if len(paths) == 0 {
		color.Red("[✘] Error: The path list is empty. Please provide at least one path.")
		os.Exit(1)
	}

	markers := utils.ReadLines(cfg.MarkersFile)

	if len(markers) == 0 {
		color.Yellow("[!] Warning: The marker list is empty. The scan will just use the size filter which might not be very useful.")
	}

	rand.Seed(time.Now().UnixNano())

	color.Cyan("[*] Checking protocols and blacklist for %d domains concurrently...", len(initialDomains))
	start := time.Now()
	allURLs, validDomainCount := domain.GenerateURLs(initialDomains, paths, &cfg)
	elapsed := time.Since(start)

	color.Green("[✔] Protocol checks and blacklist filtering completed in %s", elapsed)
	color.Cyan("[i] Scanning %d domains (out of initial %d) with %d paths", validDomainCount, len(initialDomains), len(paths))
	color.Cyan("[i] Generated %d URLs", len(allURLs))

	blacklistedCount := len(initialDomains) - validDomainCount
	if blacklistedCount > 0 {
		color.Yellow("[!] Blacklisted %d domains due to detected protection mechanisms", blacklistedCount)
	}

	if len(cfg.BasePaths) > 0 {
		color.Cyan("[i] Using %d base paths", len(cfg.BasePaths))
	}

	color.Cyan("[i] Minimum file size to detect: %d bytes", cfg.MinFileSize)
	color.Cyan("[i] Filtering for HTTP status code: %d", cfg.HTTPStatusCode)

	if len(cfg.ExtraHeaders) > 0 {
		color.Cyan("[i] Using extra headers:")
		for key, value := range cfg.ExtraHeaders {
			color.Cyan("  %s: %s", key, value)
		}
	}

	fmt.Printf("%s", allURLs)
	syscall.Exit(1)

	results := make(chan result.Result)

	bar := progressbar.Default(int64(len(allURLs)))

	var wg sync.WaitGroup
	urlChan := make(chan string, cfg.Concurrency)

	client := http.NewClient(cfg)

	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go worker(urlChan, results, &wg, cfg, bar, client, markers)
	}

	go func() {
		for _, urli := range allURLs {
			urlChan <- urli
		}
		close(urlChan)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	for res := range results {
		result.ProcessResult(res, cfg, markers)
	}

	color.Green("\n[✔] Scan completed.")
}

func worker(urls <-chan string, results chan<- result.Result, wg *sync.WaitGroup, cfg config.Config, bar *progressbar.ProgressBar, client *http.Client, markers []string) {
	defer wg.Done()

	for url := range urls {
		res := client.MakeRequest(url)
		bar.Add(1)
		results <- res
	}
}
