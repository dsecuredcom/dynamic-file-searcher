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

type Config struct {
	DomainsFile              string
	Domain                   string
	PathsFile                string
	MarkersFile              string
	BasePathsFile            string
	Concurrency              int
	Timeout                  time.Duration
	Verbose                  bool
	ProxyURL                 *url.URL
	ExtraHeaders             map[string]string
	FastHTTP                 bool
	ForceHTTPProt            bool
	HostDepth                int
	AppendByPassesToWords    bool
	SkipRootFolderCheck      bool
	BasePaths                []string
	DontGeneratePaths        bool
	NoEnvAppending           bool
	MinContentSize           int64
	MaxContentRead           int64
	HTTPStatusCode           int
	ContentTypes             string
	DisallowedContentTypes   string
	DisallowedContentStrings string
}

func ParseFlags() Config {
	cfg := Config{
		ExtraHeaders: make(map[string]string),
	}
	flag.StringVar(&cfg.DomainsFile, "domains", "", "File containing list of domains")
	flag.StringVar(&cfg.Domain, "domain", "", "Single domain to scan")
	flag.StringVar(&cfg.PathsFile, "paths", "", "File containing list of paths")
	flag.StringVar(&cfg.MarkersFile, "markers", "", "File containing list of markers")
	flag.StringVar(&cfg.BasePathsFile, "base-paths", "", "File containing list of base paths")
	flag.IntVar(&cfg.Concurrency, "concurrency", 10, "Number of concurrent requests")
	flag.IntVar(&cfg.HostDepth, "host-depth", 6, "How many sub-subdomains to use for path generation (e.g., 2 = test1-abc & test2 [based on test1-abc.test2.test3.example.com])")
	flag.BoolVar(&cfg.DontGeneratePaths, "dont-generate-paths", false, "If true, only the base paths (or nothing) will be used for scanning")
	flag.DurationVar(&cfg.Timeout, "timeout", 12*time.Second, "Timeout for each request")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "Verbose output")
	flag.BoolVar(&cfg.SkipRootFolderCheck, "skip-root-folder-check", false, "Prevents checking https://domain/PATH")
	flag.BoolVar(&cfg.AppendByPassesToWords, "append-bypasses-to-words", false, "Append bypasses to words (admin -> admin; -> admin..;)")
	flag.BoolVar(&cfg.FastHTTP, "use-fasthttp", false, "Use fasthttp instead of net/http")
	flag.BoolVar(&cfg.ForceHTTPProt, "force-http", false, "Force the usage of http:// instead of https://")
	flag.BoolVar(&cfg.NoEnvAppending, "dont-append-envs", false, "Prevent appending environment variables to requests (-qa, ...)")
	flag.StringVar(&cfg.ContentTypes, "content-types", "", "Content-Type header values to filter (csv allowed, e.g. json,octet)")
	flag.StringVar(&cfg.DisallowedContentStrings, "disallowed-content-strings", "", "If this string is present in the response body, the request will be considered as inrelevant (csv allowed, e.g. '<html>,<body>'")
	flag.StringVar(&cfg.DisallowedContentTypes, "disallowed-content-types", "", "Content-Type header value to filter out (csv allowed, e.g. json,octet)")
	flag.Int64Var(&cfg.MinContentSize, "min-content-size", 0, "Minimum file size to detect (in bytes)")
	flag.Int64Var(&cfg.MaxContentRead, "max-content-read", 5*1024*1024, "Maximum size of content to read for marker checking (in bytes)")
	flag.IntVar(&cfg.HTTPStatusCode, "http-status", 0, "HTTP status code to filter")

	var proxyURLStr string
	flag.StringVar(&proxyURLStr, "proxy", "", "Proxy URL (e.g., http://127.0.0.1:8080)")

	var extraHeaders string
	flag.StringVar(&extraHeaders, "headers", "", "Extra headers to add to each request (format: 'Header1:Value1,Header2:Value2')")

	flag.Parse()

	if (cfg.DomainsFile == "" && cfg.Domain == "") && cfg.PathsFile == "" {
		fmt.Println("Please provide either -domains file or -domain, along with -paths")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if (cfg.DomainsFile != "" || cfg.Domain != "") && cfg.PathsFile != "" && cfg.MarkersFile == "" && noRulesSpecified(cfg) {
		fmt.Println("If you provide -domains or -domain and -paths, you must provide at least one of -markers, -http-status, -content-types, -min-content-size, or -disallowed-content-types")
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

func noRulesSpecified(cfg Config) bool {
	noRules := true

	if cfg.HTTPStatusCode > 0 {
		noRules = false
	}

	if cfg.MinContentSize > 0 {
		noRules = false
	}

	if cfg.ContentTypes != "" {
		noRules = false
	}

	if cfg.DisallowedContentTypes != "" {
		noRules = false
	}

	return noRules
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
