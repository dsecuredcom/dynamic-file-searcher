// pkg/http/client.go
package http

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dsecuredcom/dynamic-file-searcher/pkg/config"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/result"
)

var baseUserAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:89.0) Gecko/20100101 Firefox/89.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1.1 Safari/605.1.15",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36 Edg/91.0.864.59",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
}

var acceptLanguages = []string{
	"en-US,en;q=0.9", "en-GB,en;q=0.8", "es-ES,es;q=0.9",
	"fr-FR,fr;q=0.9", "de-DE,de;q=0.8", "it-IT,it;q=0.9",
}

// Buffer pool for reading responses
var bufferPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 32*1024) // 32KB buffers
		return &buf
	},
}

type Client struct {
	httpClient *http.Client
	config     config.Config
}

func NewClient(cfg config.Config) *Client {
	// Enhanced transport with better connection pooling
	transport := &http.Transport{
		MaxIdleConns:        cfg.Concurrency * 2,
		MaxIdleConnsPerHost: cfg.Concurrency,
		MaxConnsPerHost:     cfg.Concurrency * 2,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  true, // We handle partial content anyway
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ResponseHeaderTimeout: cfg.Timeout,
	}

	if cfg.ProxyURL != nil {
		transport.Proxy = http.ProxyURL(cfg.ProxyURL)
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout + 3*time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return &Client{
		httpClient: client,
		config:     cfg,
	}
}

func (c *Client) MakeRequest(url string) result.Result {
	ctx, cancel := context.WithTimeout(context.Background(), c.config.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return result.Result{URL: url, Error: fmt.Errorf("error creating request: %w", err)}
	}

	randomizeRequest(req)

	for key, value := range c.config.ExtraHeaders {
		req.Header.Set(key, value)
	}

	req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", c.config.MaxContentRead-1))
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return result.Result{URL: url, Error: fmt.Errorf("error fetching: %w", err)}
	}
	defer resp.Body.Close()

	// Use buffer from pool
	bufPtr := bufferPool.Get().(*[]byte)
	defer bufferPool.Put(bufPtr)

	// Read with limited reader to prevent excessive memory usage
	limitedReader := io.LimitReader(resp.Body, c.config.MaxContentRead)

	// Read efficiently using the pooled buffer
	content := make([]byte, 0, c.config.MaxContentRead)
	buf := *bufPtr
	totalRead := 0

	for {
		n, err := limitedReader.Read(buf)
		if n > 0 {
			content = append(content, buf[:n]...)
			totalRead += n
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return result.Result{URL: url, Error: fmt.Errorf("error reading body: %w", err)}
		}
	}

	var totalSize int64
	if contentRange := resp.Header.Get("Content-Range"); contentRange != "" {
		parts := strings.Split(contentRange, "/")
		if len(parts) == 2 {
			totalSize, _ = strconv.ParseInt(parts[1], 10, 64)
		}
	} else if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
		totalSize, _ = strconv.ParseInt(contentLength, 10, 64)
	} else {
		totalSize = int64(totalRead)
	}

	return result.Result{
		URL:         url,
		Content:     string(content),
		StatusCode:  resp.StatusCode,
		FileSize:    totalSize,
		ContentType: resp.Header.Get("Content-Type"),
	}
}

func randomizeRequest(req *http.Request) {
	req.Header.Set("User-Agent", getRandomUserAgent())
	req.Header.Set("Accept-Language", getRandomAcceptLanguage())

	referer := getReferer(req.URL.String())
	req.Header.Set("Referer", referer)
	req.Header.Set("Origin", referer)
	req.Header.Set("Accept", "*/*")

	if rand.Float32() < 0.5 {
		req.Header.Set("DNT", "1")
	}
	if rand.Float32() < 0.3 {
		req.Header.Set("Upgrade-Insecure-Requests", "1")
	}
}

func getRandomUserAgent() string {
	baseUA := baseUserAgents[rand.Intn(len(baseUserAgents))]
	parts := strings.Split(baseUA, " ")

	for i, part := range parts {
		if strings.Contains(part, "/") {
			versionParts := strings.Split(part, "/")
			if len(versionParts) == 2 {
				version := strings.Split(versionParts[1], ".")
				if len(version) > 2 {
					version[2] = fmt.Sprintf("%d", rand.Intn(100))
					versionParts[1] = strings.Join(version, ".")
				}
			}
			parts[i] = strings.Join(versionParts, "/")
		}
	}

	return strings.Join(parts, " ")
}

func getRandomAcceptLanguage() string {
	return acceptLanguages[rand.Intn(len(acceptLanguages))]
}

func getReferer(url string) string {
	return url
}
