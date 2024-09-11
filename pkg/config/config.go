package config

import (
	"bufio"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// Config holds all configuration options
type Config struct {
	DomainsFile    string
	Domain         string
	PathsFile      string
	MarkersFile    string
	BasePathsFile  string
	Concurrency    int
	Timeout        time.Duration
	Verbose        bool
	ProxyURL       *url.URL
	ExtraHeaders   map[string]string
	MinFileSize    int64
	MaxContentSize int64
	HTTPStatusCode int
	BasePaths      []string
}

// ParseFlags parses command line flags and returns a Config struct
func ParseFlags() Config {
	cfg := Config{
		ExtraHeaders: make(map[string]string), // Initialize the map here
	}
	flag.StringVar(&cfg.DomainsFile, "domains", "", "File containing list of domains")
	flag.StringVar(&cfg.Domain, "domain", "", "Single domain to scan")
	flag.StringVar(&cfg.PathsFile, "paths", "", "File containing list of paths")
	flag.StringVar(&cfg.MarkersFile, "markers", "", "File containing list of markers")
	flag.StringVar(&cfg.BasePathsFile, "base-paths", "", "File containing list of base paths")
	flag.IntVar(&cfg.Concurrency, "concurrency", 10, "Number of concurrent requests")
	flag.DurationVar(&cfg.Timeout, "timeout", 12*time.Second, "Timeout for each request")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "Verbose output")
	flag.Int64Var(&cfg.MinFileSize, "min-size", 0, "Minimum file size to detect (in bytes)")
	flag.Int64Var(&cfg.MaxContentSize, "max-content-size", 5*1024*1024, "Maximum size of content to read for marker checking (in bytes)")
	flag.IntVar(&cfg.HTTPStatusCode, "status", 200, "HTTP status code to filter")

	var proxyURLStr string
	flag.StringVar(&proxyURLStr, "proxy", "", "Proxy URL (e.g., http://127.0.0.1:8080)")

	var extraHeaders string
	flag.StringVar(&extraHeaders, "headers", "", "Extra headers to add to each request (format: 'Header1:Value1,Header2:Value2')")

	flag.Parse()

	if (cfg.DomainsFile == "" && cfg.Domain == "") || cfg.PathsFile == "" || cfg.MarkersFile == "" {
		fmt.Println("Please provide either -domains file or -domain, along with -paths and -markers files")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if proxyURLStr != "" {
		proxyURL, err := url.Parse(proxyURLStr)
		if err != nil {
			fmt.Printf("Invalid proxy URL: %v\n", err)
			os.Exit(1)
		}
		cfg.ProxyURL = proxyURL
	}

	if extraHeaders != "" {
		headers := strings.Split(extraHeaders, ",")
		for _, header := range headers {
			parts := strings.SplitN(header, ":", 2)
			if len(parts) == 2 {
				cfg.ExtraHeaders[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
	}

	if cfg.BasePathsFile != "" {
		var err error
		cfg.BasePaths, err = readBasePaths(cfg.BasePathsFile)
		if err != nil {
			fmt.Printf("Error reading base paths file: %v\n", err)
			os.Exit(1)
		}
	}

	return cfg
}

func readBasePaths(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var basePaths []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		path := strings.TrimSpace(scanner.Text())
		if path != "" {
			basePaths = append(basePaths, path)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return basePaths, nil
}
