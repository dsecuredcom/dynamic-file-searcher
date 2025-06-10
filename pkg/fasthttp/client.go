// pkg/fasthttp/client.go
package fasthttp

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/config"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/result"
	"github.com/valyala/fasthttp"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"
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

// Response pool to reuse response objects
var responsePool = sync.Pool{
	New: func() interface{} {
		return &fasthttp.Response{}
	},
}

// Request pool to reuse request objects
var requestPool = sync.Pool{
	New: func() interface{} {
		return &fasthttp.Request{}
	},
}

type Client struct {
	config config.Config
	client *fasthttp.Client
}

func NewClient(cfg config.Config) *Client {
	return &Client{
		config: cfg,
		client: &fasthttp.Client{
			ReadTimeout:                   cfg.Timeout,
			WriteTimeout:                  cfg.Timeout,
			DisablePathNormalizing:        true,
			DisableHeaderNamesNormalizing: true,
			MaxConnsPerHost:               cfg.Concurrency * 2, // Connection pooling
			MaxIdleConnDuration:           90 * time.Second,
			MaxConnDuration:               10 * time.Minute,
			MaxResponseBodySize:           int(cfg.MaxContentRead),
			TLSConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
}

func (c *Client) MakeRequest(url string) result.Result {
	// Get request from pool
	req := requestPool.Get().(*fasthttp.Request)
	defer func() {
		req.Reset()
		requestPool.Put(req)
	}()

	req.SetRequestURI(url)
	req.URI().DisablePathNormalizing = true
	req.Header.DisableNormalizing()
	req.Header.SetMethod(fasthttp.MethodGet)
	req.Header.Set("Connection", "keep-alive")
	req.Header.SetProtocol("HTTP/1.1")
	req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", c.config.MaxContentRead-1))

	randomizeRequest(req)
	for key, value := range c.config.ExtraHeaders {
		req.Header.Set(key, value)
	}

	// Get response from pool
	resp := responsePool.Get().(*fasthttp.Response)
	defer func() {
		resp.Reset()
		responsePool.Put(resp)
	}()

	// Use the pre-configured client with connection pooling
	err := c.client.DoRedirects(req, resp, 0)
	if err == fasthttp.ErrMissingLocation {
		return result.Result{URL: url, Error: fmt.Errorf("error fetching: %w", err)}
	}

	if err != nil {
		return result.Result{URL: url, Error: fmt.Errorf("error fetching: %w", err)}
	}

	// Get body efficiently
	body := resp.Body()

	var totalSize int64
	contentRange := resp.Header.Peek("Content-Range")
	if len(contentRange) > 0 {
		parts := bytes.Split(contentRange, []byte("/"))
		if len(parts) == 2 {
			totalSize, _ = strconv.ParseInt(string(parts[1]), 10, 64)
		}
	} else {
		totalSize = int64(len(body))
	}

	// Only copy the amount we need
	contentSize := int64(len(body))
	if contentSize > c.config.MaxContentRead {
		contentSize = c.config.MaxContentRead
	}

	// Make a copy of only what we need
	contentCopy := make([]byte, contentSize)
	copy(contentCopy, body[:contentSize])

	return result.Result{
		URL:         url,
		Content:     string(contentCopy),
		StatusCode:  resp.StatusCode(),
		FileSize:    totalSize,
		ContentType: string(resp.Header.Peek("Content-Type")),
	}
}

func randomizeRequest(req *fasthttp.Request) {
	req.Header.Set("User-Agent", getRandomUserAgent())
	req.Header.Set("Accept-Language", getRandomAcceptLanguage())

	referer := getReferer(req.URI().String())
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
