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
			color.Green("\n[!]\tFound marker '%s' in %s", marker, result.URL)
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
		color.Green("\n[!]\tFound based on rules: 'S: %d, FS: %d', CT: %s in %s", result.StatusCode, result.FileSize, result.ContentType, result.URL)
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
