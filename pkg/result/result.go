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

	rulesCount := 0

	if cfg.HTTPStatusCode > 0 {
		rulesCount++
	}

	if cfg.MinFileSize > 0 {
		rulesCount++
	}

	if cfg.ContentType != "" {
		rulesCount++
	}

	rulesMatchd := 0

	if result.StatusCode == cfg.HTTPStatusCode {
		rulesMatchd++
	}

	if cfg.MinFileSize > 0 && result.FileSize >= cfg.MinFileSize {
		rulesMatchd++
	}

	if cfg.ContentType != "" && strings.Contains(result.ContentType, cfg.ContentType) {
		rulesMatchd++
	}

	if rulesMatchd == rulesCount {
		fmt.Printf("\nFound based on rules: 'S: %d, FS: %d', CT: %s in %s\n", result.StatusCode, result.FileSize, result.ContentType, result.URL)
	}

	if cfg.Verbose && !markerFound {
		log.Printf("Processed: %s (Status: %d, Size: %d bytes, Type: %s)\n",
			result.URL, result.StatusCode, result.FileSize, result.ContentType)
	}
}
