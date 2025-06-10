// pkg/result/result.go
package result

import (
	"container/list"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/config"
	"github.com/fatih/color"
	"log"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type Result struct {
	URL         string
	Content     string
	Error       error
	StatusCode  int
	FileSize    int64
	ContentType string
}

// LRU-based ResponseMap with size limit
type ResponseMap struct {
	shards   [256]responseShard
	maxItems int
}

type responseShard struct {
	mu        sync.RWMutex
	responses map[uint64]*list.Element
	lru       *list.List
	maxItems  int
}

type cacheEntry struct {
	hash uint64
	key  string
}

func fnv1aHash(data string) uint64 {
	var hash uint64 = 0xcbf29ce484222325 // FNV offset basis

	for i := 0; i < len(data); i++ {
		hash ^= uint64(data[i])
		hash *= 0x100000001b3 // FNV prime
	}

	return hash
}

func NewResponseMap() *ResponseMap {
	rm := &ResponseMap{
		maxItems: 10000, // Limit total items across all shards
	}

	itemsPerShard := rm.maxItems / 256
	for i := range rm.shards {
		rm.shards[i].responses = make(map[uint64]*list.Element, itemsPerShard)
		rm.shards[i].lru = list.New()
		rm.shards[i].maxItems = itemsPerShard
	}
	return rm
}

func (rm *ResponseMap) getShard(key string) *responseShard {
	return &rm.shards[fnv1aHash(key)&0xFF]
}

func (rm *ResponseMap) isNewResponse(host string, size int64) bool {
	key := host + ":" + strconv.FormatInt(size, 10)
	shard := rm.getShard(key)
	hash := fnv1aHash(key)

	shard.mu.Lock()
	defer shard.mu.Unlock()

	// Check if exists
	if elem, exists := shard.responses[hash]; exists {
		// Move to front (LRU)
		shard.lru.MoveToFront(elem)
		return false
	}

	// Add new entry
	entry := &cacheEntry{hash: hash, key: key}
	elem := shard.lru.PushFront(entry)
	shard.responses[hash] = elem

	// Evict oldest if over limit
	if shard.lru.Len() > shard.maxItems {
		oldest := shard.lru.Back()
		if oldest != nil {
			oldEntry := oldest.Value.(*cacheEntry)
			delete(shard.responses, oldEntry.hash)
			shard.lru.Remove(oldest)
		}
	}

	return true
}

func extractHost(urlStr string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return urlStr
	}
	return parsedURL.Host
}

var tracker = NewResponseMap()

// String pool for commonly used strings
var stringPool = sync.Pool{
	New: func() interface{} {
		return new(strings.Builder)
	},
}

func ProcessResult(result Result, cfg config.Config, markers []string) {
	// Defer cleanup to ensure memory is freed
	defer func() {
		result.Content = "" // Always clear content at end
	}()

	// Early exit for errors
	if result.Error != nil {
		if cfg.Verbose {
			log.Printf("Error processing %s: %v\n", result.URL, result.Error)
		}
		return
	}

	// Check if content type is disallowed first
	disallowedContentTypes := strings.ToLower(cfg.DisallowedContentTypes)
	if disallowedContentTypes != "" {
		disallowedContentTypesList := strings.Split(disallowedContentTypes, ",")
		if isDisallowedContentType(strings.ToLower(result.ContentType), disallowedContentTypesList) {
			return
		}
	}

	// For large content, process in chunks to avoid keeping entire content in memory
	if len(result.Content) > 1024*1024 { // 1MB threshold
		if !processLargeContent(result, cfg, markers) {
			return
		}
	} else {
		// Regular processing for smaller content
		if !processRegularContent(result, cfg, markers) {
			return
		}
	}

	// Check duplicate responses
	host := extractHost(result.URL)
	if !cfg.DisableDuplicateCheck {
		if !tracker.isNewResponse(host, result.FileSize) {
			if cfg.Verbose {
				log.Printf("Skipped duplicate response size %d for host %s\n", result.FileSize, host)
			}
			return
		}
	}

	// Output results
	outputResults(result, cfg, markers)
}

func processLargeContent(result Result, cfg config.Config, markers []string) bool {
	// Check disallowed strings in chunks
	disallowedContentStrings := strings.ToLower(cfg.DisallowedContentStrings)
	if disallowedContentStrings != "" {
		disallowedContentStringsList := strings.Split(disallowedContentStrings, ",")

		// Process in 64KB chunks
		chunkSize := 65536
		for i := 0; i < len(result.Content); i += chunkSize {
			end := i + chunkSize
			if end > len(result.Content) {
				end = len(result.Content)
			}

			chunk := strings.ToLower(result.Content[i:end])
			for _, disallowed := range disallowedContentStringsList {
				if disallowed != "" && strings.Contains(chunk, strings.ToLower(disallowed)) {
					return false
				}
			}
		}
	}

	// Check markers in chunks
	if len(markers) > 0 {
		for _, marker := range markers {
			if strings.HasPrefix(marker, "regex:") {
				regex := strings.TrimPrefix(marker, "regex:")
				if match, _ := regexp.MatchString(regex, result.Content); match {
					return true
				}
			} else {
				// For non-regex markers, check in chunks
				chunkSize := 65536
				for i := 0; i < len(result.Content); i += chunkSize {
					end := i + chunkSize
					if end > len(result.Content) {
						end = len(result.Content)
					}

					if strings.Contains(result.Content[i:end], marker) {
						return true
					}
				}
			}
		}
		return false // No marker found
	}

	return checkRules(result, cfg)
}

func processRegularContent(result Result, cfg config.Config, markers []string) bool {
	// Check if content contains disallowed strings
	disallowedContentStrings := strings.ToLower(cfg.DisallowedContentStrings)
	if disallowedContentStrings != "" {
		disallowedContentStringsList := strings.Split(disallowedContentStrings, ",")
		if containsDisallowedStringInContent(result.Content, disallowedContentStringsList) {
			return false
		}
	}

	markerFound := false
	hasMarkers := len(markers) > 0

	if hasMarkers {
		for _, marker := range markers {
			if strings.HasPrefix(marker, "regex:") == false && strings.Contains(result.Content, marker) {
				markerFound = true
				break
			}

			if strings.HasPrefix(marker, "regex:") {
				regex := strings.TrimPrefix(marker, "regex:")
				if match, _ := regexp.MatchString(regex, result.Content); match {
					markerFound = true
					break
				}
			}
		}

		if !markerFound {
			return false
		}
	}

	return checkRules(result, cfg)
}

func checkRules(result Result, cfg config.Config) bool {
	rulesMatched := 0
	rulesCount := 0

	if cfg.HTTPStatusCodes != "" {
		rulesCount++
	}

	if cfg.MinContentSize > 0 {
		rulesCount++
	}

	if cfg.ContentTypes != "" {
		rulesCount++
	}

	if cfg.HTTPStatusCodes != "" {
		allowedHttpStatusesList := strings.Split(cfg.HTTPStatusCodes, ",")
		for _, allowedHttpStatusString := range allowedHttpStatusesList {
			allowedStatus, err := strconv.Atoi(strings.TrimSpace(allowedHttpStatusString))
			if err != nil {
				log.Printf("Error converting status code '%s' to integer: %v", allowedHttpStatusString, err)
				continue
			}
			if result.StatusCode == allowedStatus {
				rulesMatched++
				break
			}
		}
	}

	if cfg.MinContentSize > 0 && result.FileSize >= cfg.MinContentSize {
		rulesMatched++
	}

	if cfg.ContentTypes != "" {
		allowedContentTypes := strings.ToLower(cfg.ContentTypes)
		allowedContentTypesList := strings.Split(allowedContentTypes, ",")
		resultContentType := strings.ToLower(result.ContentType)
		for _, allowedContentTypeString := range allowedContentTypesList {
			if strings.Contains(resultContentType, allowedContentTypeString) {
				rulesMatched++
				break
			}
		}
	}

	return rulesCount == 0 || (rulesCount > 0 && rulesMatched == rulesCount)
}

func outputResults(result Result, cfg config.Config, markers []string) {
	color.Red("\n[!]\tMatch found in %s", result.URL)

	// Find which marker matched (if any)
	usedMarker := ""
	for _, marker := range markers {
		if strings.HasPrefix(marker, "regex:") == false && strings.Contains(result.Content, marker) {
			usedMarker = marker
			break
		}

		if strings.HasPrefix(marker, "regex:") {
			regex := strings.TrimPrefix(marker, "regex:")
			if match, _ := regexp.MatchString(regex, result.Content); match {
				usedMarker = marker
				break
			}
		}
	}

	if usedMarker != "" {
		color.Red("\tMarkers check: passed (%s)", usedMarker)
	}

	color.Red("\tRules check: passed (S: %d, FS: %d, CT: %s)",
		result.StatusCode, result.FileSize, result.ContentType)

	// Show limited content preview
	content := strings.ReplaceAll(result.Content, "\n", "")
	if len(content) > 150 {
		color.Green("\n[!]\tBody: %s\n", content[:150])
	} else {
		color.Green("\n[!]\tBody: %s\n", content)
	}

	if cfg.Verbose {
		log.Printf("Processed: %s (Status: %d, Size: %d bytes, Type: %s)\n",
			result.URL, result.StatusCode, result.FileSize, result.ContentType)
	}
}

func containsDisallowedStringInContent(contentBody string, disallowedContentStringsList []string) bool {
	if len(disallowedContentStringsList) == 0 {
		return false
	}

	lowerContent := strings.ToLower(contentBody)
	for _, disallowedContentString := range disallowedContentStringsList {
		if disallowedContentString == "" {
			continue
		}

		if strings.Contains(lowerContent, strings.ToLower(disallowedContentString)) {
			return true
		}
	}

	return false
}

func isDisallowedContentType(contentType string, disallowedContentTypesList []string) bool {
	if len(disallowedContentTypesList) == 0 {
		return false
	}

	for _, disallowedContentType := range disallowedContentTypesList {
		if disallowedContentType == "" {
			continue
		}

		if strings.Contains(contentType, strings.TrimSpace(disallowedContentType)) {
			return true
		}
	}

	return false
}
