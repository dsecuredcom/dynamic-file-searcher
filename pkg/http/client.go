package http

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
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

type Client struct {
	httpClient *http.Client
	config     config.Config
}

func NewClient(cfg config.Config) *Client {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
	}

	if cfg.ProxyURL != nil {
		transport.Proxy = http.ProxyURL(cfg.ProxyURL)
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout + 3*time.Second,
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

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return result.Result{URL: url, Error: fmt.Errorf("error fetching: %w", err)}
	}
	defer resp.Body.Close()

	buffer := make([]byte, c.config.MaxContentSize)
	n, err := io.ReadFull(resp.Body, buffer)
	if err != nil && err != io.ErrUnexpectedEOF {
		return result.Result{URL: url, Error: fmt.Errorf("error reading body: %w", err)}
	}
	buffer = buffer[:n]

	remainingSize, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		return result.Result{URL: url, Error: fmt.Errorf("error reading remaining body: %w", err)}
	}

	totalSize := int64(n) + remainingSize

	return result.Result{
		URL:         url,
		Content:     string(buffer),
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
