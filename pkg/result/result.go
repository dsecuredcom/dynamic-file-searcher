package result

import (
	"fmt"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/config"
	"log"
	"strings"
)

// Result represents the result of a single request
type Result struct {
	URL         string
	Content     string
	Error       error
	StatusCode  int
	FileSize    int64
	ContentType string
}

// ProcessResult processes a single Result
func ProcessResult(result Result, cfg config.Config, markers []string) {
	if result.Error != nil {
		if cfg.Verbose {
			log.Printf("Error processing %s: %v\n", result.URL, result.Error)
		}
		return
	}

	// Check HTTP status code
	if result.StatusCode != cfg.HTTPStatusCode {
		if cfg.Verbose {
			log.Printf("Skipping %s: Status code %d (expected %d)\n", result.URL, result.StatusCode, cfg.HTTPStatusCode)
		}
		return
	}

	// Check for markers in the partial content
	markerFound := false
	for _, marker := range markers {
		if strings.Contains(result.Content, marker) {
			fmt.Printf("\nFound marker '%s' in %s\n", marker, result.URL)
			markerFound = true
			break
		}
	}

	// Check file size
	if result.FileSize >= cfg.MinFileSize {
		fmt.Printf("\nFound large file (%d bytes, type: %s) at %s\n", result.FileSize, result.ContentType, result.URL)
		return
	}

	if cfg.Verbose && !markerFound {
		log.Printf("Processed: %s (Status: %d, Size: %d bytes, Type: %s)\n",
			result.URL, result.StatusCode, result.FileSize, result.ContentType)
	}
}
