package result

import (
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/config"
	"github.com/fatih/color"
	"log"
	"regexp"
	"strconv"
	"strings"
)

type Result struct {
	URL         string
	Content     string
	Error       error
	StatusCode  int
	FileSize    int64
	ContentType string
}

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
