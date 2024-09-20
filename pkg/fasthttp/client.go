package fasthttp

import (
	"crypto/tls"
	"fmt"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/config"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/result"
	"github.com/fatih/color"
	"github.com/valyala/fasthttp"
	"math/rand"
	"strings"
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
	config config.Config
	client *fasthttp.Client
}

func NewClient(cfg config.Config) *Client {
	return &Client{
		config: cfg,
		client: &fasthttp.Client{
			ReadTimeout:                   cfg.Timeout,
			WriteTimeout:                  cfg.Timeout,
			DisableHeaderNamesNormalizing: true, // Prevent automatic header modifications
			TLSConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}
}

func (c *Client) MakeRequest(url string) result.Result {
	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)

	req.SetRequestURI(url)
	req.Header.SetMethod(fasthttp.MethodGet)
	req.Header.Set("Connection", "keep-alive")
	req.Header.SetProtocol("HTTP/1.1")

	randomizeRequest(req)
	for key, value := range c.config.ExtraHeaders {
		req.Header.Set(key, value)
	}

	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	client := &fasthttp.Client{
		ReadTimeout:  c.config.Timeout,
		WriteTimeout: c.config.Timeout,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	err := client.DoTimeout(req, resp, c.config.Timeout)
	if err != nil {
		if c.config.ShowFetchTimeoutErrors && err == fasthttp.ErrTimeout {
			color.Red("\n[!]\tTimeout based detection: 'Url: %s Error: %s", url, err)
		}

		return result.Result{URL: url, Error: fmt.Errorf("error fetching: %w", err)}
	}

	body := resp.Body()
	bodySize := len(body)

	if int64(bodySize) > c.config.MaxContentSize {
		body = body[:c.config.MaxContentSize]
	}

	return result.Result{
		URL:         url,
		Content:     string(body),
		StatusCode:  resp.StatusCode(),
		FileSize:    int64(bodySize),
		ContentType: string(resp.Header.Peek("Content-Type")),
	}
}

func randomizeRequest(req *fasthttp.Request) {
	req.Header.Set("User-Agent", getRandomUserAgent())
	req.Header.Set("Accept-Language", getRandomAcceptLanguage())

	referer := getReferer(req.URI().String())
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
