package result

import (
	"fmt"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/config"
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

	if result.StatusCode != cfg.HTTPStatusCode {
		if cfg.Verbose {
			log.Printf("Skipping %s: Status code %d (expected %d)\n", result.URL, result.StatusCode, cfg.HTTPStatusCode)
		}
		return
	}

	markerFound := false
	for _, marker := range markers {
		if strings.Contains(result.Content, marker) {
			fmt.Printf("\nFound marker '%s' in %s\n", marker, result.URL)
			markerFound = true
			break
		}
	}

	if markerFound {
		return
	}

	if result.FileSize >= cfg.MinFileSize {
		fmt.Printf("\nFound large file (%d bytes, type: %s) at %s\n", result.FileSize, result.ContentType, result.URL)
		return
	}

	if cfg.Verbose && !markerFound {
		log.Printf("Processed: %s (Status: %d, Size: %d bytes, Type: %s)\n",
			result.URL, result.StatusCode, result.FileSize, result.ContentType)
	}
}
