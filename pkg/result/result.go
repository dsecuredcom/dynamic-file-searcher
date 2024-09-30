package result

import (
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/config"
	"github.com/fatih/color"
	"log"
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

	markerFound := false
	for _, marker := range markers {
		if strings.Contains(result.Content, marker) {
			color.Red("\n[!]\tFound marker '%s' in %s", marker, result.URL)
			if len(result.Content) > 150 {
				color.Green("\n[!]\tBody: %s\n", result.Content[:150])
			} else {
				color.Green("\n[!]\tBody: %s\n", result.Content)
			}
			markerFound = true
			break
		}
	}

	if markerFound {
		return
	}

	rulesCount := 0

	if cfg.HTTPStatusCode > 0 {
		rulesCount++
	}

	if cfg.MinFileSize > 0 {
		rulesCount++
	}

	if cfg.ContentTypes != "" {
		rulesCount++
	}

	rulesMatched := 0

	if result.StatusCode == cfg.HTTPStatusCode {
		rulesMatched++
	}

	if cfg.MinFileSize > 0 && result.FileSize >= cfg.MinFileSize {
		rulesMatched++
	}

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

	DisallowedContentTypes := strings.ToLower(cfg.DisallowedContentTypes)
	DisallowedContentTypesList := strings.Split(DisallowedContentTypes, ",")

	if isDisallowedContentType(result.ContentType, DisallowedContentTypesList) {
		return
	}

	DisallowedContentStrings := strings.ToLower(cfg.DisallowedContentStrings)
	DisallowedContentStringsList := strings.Split(DisallowedContentStrings, ",")

	if containsDisallowedStringInContent(result.Content, DisallowedContentStringsList) {
		return
	}

	if rulesMatched == rulesCount {
		color.Red("\n[!]\tFound based on rules: 'S: %d, FS: %d', CT: %s in %s", result.StatusCode, result.FileSize, result.ContentType, result.URL)
		if len(result.Content) > 150 {
			color.Green("\n[!]\tBody: %s\n", result.Content[:150])
		} else {
			color.Green("\n[!]\tBody: %s\n", result.Content)
		}
	}

	if cfg.Verbose && !markerFound {
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
