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

func ProcessResult(result Result, cfg config.Config, markers []string) {
	if result.Error != nil {
		if cfg.Verbose {
			log.Printf("Error processing %s: %v\n", result.URL, result.Error)
		}
		return
	}

	// Check if content type is disallowed first
	DisallowedContentTypes := strings.ToLower(cfg.DisallowedContentTypes)
	DisallowedContentTypesList := strings.Split(DisallowedContentTypes, ",")
	if isDisallowedContentType(result.ContentType, DisallowedContentTypesList) {
		return
	}

	// Check if content contains disallowed strings
	DisallowedContentStrings := strings.ToLower(cfg.DisallowedContentStrings)
	DisallowedContentStringsList := strings.Split(DisallowedContentStrings, ",")
	if containsDisallowedStringInContent(result.Content, DisallowedContentStringsList) {
		return
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
		AllowedHttpStatusesList := strings.Split(cfg.HTTPStatusCodes, ",")
		for _, AllowedHttpStatusString := range AllowedHttpStatusesList {
			allowedStatus, err := strconv.Atoi(strings.TrimSpace(AllowedHttpStatusString))
			if err != nil {
				log.Printf("Error converting status code '%s' to integer: %v", AllowedHttpStatusString, err)
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
		AllowedContentTypes := strings.ToLower(cfg.ContentTypes)
		AllowedContentTypesList := strings.Split(AllowedContentTypes, ",")
		ResultContentType := strings.ToLower(result.ContentType)
		for _, AllowedContentTypeString := range AllowedContentTypesList {
			if strings.Contains(ResultContentType, AllowedContentTypeString) {
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
		return
	}

	host := extractHost(result.URL)
	if !cfg.DisableDuplicateCheck {
		if !tracker.isNewResponse(host, result.FileSize) {
			if cfg.Verbose {
				log.Printf("Skipped duplicate response size %d for host %s\n", result.FileSize, host)
			}
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

	content := result.Content
	content = strings.ReplaceAll(content, "\n", "")

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

func containsDisallowedStringInContent(contentBody string, DisallowedContentStringsList []string) bool {
	if len(DisallowedContentStringsList) == 0 {
		return false
	}

	for _, disallowedContentString := range DisallowedContentStringsList {
		if disallowedContentString == "" {
			continue
		}

		if strings.Contains(contentBody, disallowedContentString) {
			return true
		}
	}

	return false
}

func isDisallowedContentType(contentType string, DisallowedContentTypesList []string) bool {

	if len(DisallowedContentTypesList) == 0 {
		return false
	}

	for _, disallowedContentType := range DisallowedContentTypesList {
		if disallowedContentType == "" {
			continue
		}

		if strings.Contains(contentType, disallowedContentType) {
			return true
		}
	}

	return false

}
