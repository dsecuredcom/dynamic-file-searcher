package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/dsecuredcom/dynamic-file-searcher/pkg/config"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/domain"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/http"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/result"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/utils"
	"github.com/schollz/progressbar/v3"
)

func main() {
	cfg := config.ParseFlags()

	domains := domain.GetDomains(cfg.DomainsFile, cfg.Domain)
	paths := utils.ReadLines(cfg.PathsFile)
	markers := utils.ReadLines(cfg.MarkersFile)

	rand.Seed(time.Now().UnixNano())

	allURLs := domain.GenerateURLs(domains, paths, &cfg)

	//fmt.Printf("%s\n", allURLs)
	//syscall.Exit(0)

	fmt.Printf("Scanning %d domains with %d paths\n", len(domains), len(paths))
	fmt.Printf("Generated %d URLs\n", len(allURLs))
	if len(cfg.BasePaths) > 0 {
		fmt.Printf("Using %d base paths\n", len(cfg.BasePaths))
	}
	fmt.Printf("Minimum file size to detect: %d bytes\n", cfg.MinFileSize)
	fmt.Printf("Filtering for HTTP status code: %d\n", cfg.HTTPStatusCode)

	if len(cfg.ExtraHeaders) > 0 {
		fmt.Println("Using extra headers:")
		for key, value := range cfg.ExtraHeaders {
			fmt.Printf("  %s: %s\n", key, value)
		}
	}

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

	fmt.Println("\nScan completed.")
}

func worker(urls <-chan string, results chan<- result.Result, wg *sync.WaitGroup, cfg config.Config, bar *progressbar.ProgressBar, client *http.Client, markers []string) {
	defer wg.Done()

	for url := range urls {
		res := client.MakeRequest(url)
		bar.Add(1)
		results <- res
	}
}
