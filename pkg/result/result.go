// pkg/result/result.go
package result

import (
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

type ResponseMap struct {
	shards [256]responseShard
}

type responseShard struct {
	mu        sync.RWMutex
	responses map[uint64]struct{}
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
	rm := &ResponseMap{}
	for i := range rm.shards {
		rm.shards[i].responses = make(map[uint64]struct{}, 64) // Reasonable initial capacity
	}
	return rm
}

func (rm *ResponseMap) getShard(key string) *responseShard {
	// Use first byte of hash as shard key for even distribution
	return &rm.shards[fnv1aHash(key)&0xFF]
}

// Improved response tracking with better collision avoidance
func (rm *ResponseMap) isNewResponse(host string, size int64) bool {
	// Create composite key
	key := host + ":" + strconv.FormatInt(size, 10)

	// Get the appropriate shard
	shard := rm.getShard(key)

	// Calculate full hash
	hash := fnv1aHash(key)

	// Check if response exists with minimal locking
	shard.mu.RLock()
	_, exists := shard.responses[hash]
	shard.mu.RUnlock()

	if exists {
		return false
	}

	// If not found, acquire write lock and check again
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if _, exists := shard.responses[hash]; exists {
		return false
	}

	// Add new entry
	shard.responses[hash] = struct{}{}
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
	// Early exit for errors
	if result.Error != nil {
		if cfg.Verbose {
			log.Printf("Error processing %s: %v\n", result.URL, result.Error)
		}
		// Clear content to free memory
		result.Content = ""
		return
	}

	// Check if content type is disallowed first
	disallowedContentTypes := strings.ToLower(cfg.DisallowedContentTypes)
	if disallowedContentTypes != "" {
		disallowedContentTypesList := strings.Split(disallowedContentTypes, ",")
		if isDisallowedContentType(strings.ToLower(result.ContentType), disallowedContentTypesList) {
			result.Content = "" // Free memory
			return
		}
	}

	// Check if content contains disallowed strings
	disallowedContentStrings := strings.ToLower(cfg.DisallowedContentStrings)
	if disallowedContentStrings != "" {
		disallowedContentStringsList := strings.Split(disallowedContentStrings, ",")
		if containsDisallowedStringInContent(result.Content, disallowedContentStringsList) {
			result.Content = "" // Free memory
			return
		}
	}

	markerFound := false
	hasMarkers := len(markers) > 0
	usedMarker := ""

	if hasMarkers {
		for _, marker := range markers {
			if strings.HasPrefix(marker, "regex:") == false && strings.Contains(result.Content, marker) {
				markerFound = true
				usedMarker = marker
				break
			}

			if strings.HasPrefix(marker, "regex:") {
				regex := strings.TrimPrefix(marker, "regex:")
				if match, _ := regexp.MatchString(regex, result.Content); match {
					markerFound = true
					usedMarker = marker
					break
				}
			}
		}
	}

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

	// Check content size
	if cfg.MinContentSize > 0 && result.FileSize >= cfg.MinContentSize {
		rulesMatched++
	}

	// Check content types
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

	// Determine if rules match
	rulesPass := rulesCount == 0 || (rulesCount > 0 && rulesMatched == rulesCount)

	// Final decision based on both markers and rules
	if (hasMarkers && !markerFound) || (rulesCount > 0 && !rulesPass) {
		// If we have markers but didn't find one, OR if we have rules but they didn't pass, skip
		if cfg.Verbose {
			log.Printf("Skipped: %s (Status: %d, Size: %d bytes, Type: %s)\n",
				result.URL, result.StatusCode, result.FileSize, result.ContentType)
		}
		result.Content = "" // Free memory
		return
	}

	host := extractHost(result.URL)
	if !cfg.DisableDuplicateCheck {
		if !tracker.isNewResponse(host, result.FileSize) {
			if cfg.Verbose {
				log.Printf("Skipped duplicate response size %d for host %s\n", result.FileSize, host)
			}
			result.Content = "" // Free memory
			return
		}
	}

	// If we get here, all configured conditions were met
	color.Red("\n[!]\tMatch found in %s", result.URL)
	if hasMarkers {
		color.Red("\tMarkers check: passed (%s)", usedMarker)
	}

	color.Red("\tRules check: passed (S: %d, FS: %d, CT: %s)",
		result.StatusCode, result.FileSize, result.ContentType)

	// Process content with string builder from pool
	sb := stringPool.Get().(*strings.Builder)
	defer func() {
		sb.Reset()
		stringPool.Put(sb)
	}()

	// Remove newlines efficiently
	for _, r := range result.Content {
		if r != '\n' {
			sb.WriteRune(r)
		}
	}
	content := sb.String()

	if len(content) > 150 {
		color.Green("\n[!]\tBody: %s\n", content[:150])
	} else {
		color.Green("\n[!]\tBody: %s\n", content)
	}

	if cfg.Verbose {
		log.Printf("Processed: %s (Status: %d, Size: %d bytes, Type: %s)\n",
			result.URL, result.StatusCode, result.FileSize, result.ContentType)
	}

	// Clear content to free memory
	result.Content = ""
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
