package domain

import (
	"fmt"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/config"
	"math/rand"
	"regexp"
	"strings"

	"github.com/dsecuredcom/dynamic-file-searcher/pkg/utils"
)

// GetDomains returns a list of domains from a file or a single domain
func GetDomains(domainsFile, singleDomain string) []string {
	if domainsFile != "" {
		return utils.ReadLines(domainsFile)
	}
	return []string{singleDomain}
}

// GenerateURLs generates all possible URLs from given domains and paths
func GenerateURLs(domains, paths []string, cfg *config.Config) []string {
	var allURLs []string
	for _, domain := range domains {
		for _, path := range paths {
			allURLs = append(allURLs, fmt.Sprintf("https://%s/%s", domain, path))

			words := splitDomain(domain)
			for _, word := range words {
				allURLs = append(allURLs, fmt.Sprintf("https://%s/%s/%s", domain, word, path))

				// Add base paths if configured
				for _, basePath := range cfg.BasePaths {
					allURLs = append(allURLs, fmt.Sprintf("https://%s/%s%s/%s", domain, basePath, word, path))
				}
			}
		}
	}

	// Shuffle the URLs
	rand.Shuffle(len(allURLs), func(i, j int) {
		allURLs[i], allURLs[j] = allURLs[j], allURLs[i]
	})

	return allURLs
}

func splitDomain(domain string) []string {
	if isIPAddress(domain) {
		return []string{}
	}

	parts := strings.Split(domain, ".")
	results := make(map[string]bool)

	results[domain] = true

	filteredParts := filterParts(parts)

	subdomainParts := filteredParts[:len(filteredParts)-2]
	domainParts := filteredParts[len(filteredParts)-2:]

	subdomain := strings.Join(subdomainParts, ".")
	results[subdomain] = true
	for _, part := range subdomainParts {
		results[part] = true
		results[part+"1"] = true
		results[part+"123"] = true
	}

	domain = strings.Join(domainParts, ".")
	results[domain] = true
	results[domainParts[0]] = true
	results[domainParts[0]+"1"] = true
	results[domainParts[0]+"123"] = true

	baseWords := []string{subdomain}
	baseWords = append(baseWords, subdomainParts...)
	baseWords = append(baseWords, domainParts[0])

	extendedWords := generateExtendedWords(baseWords)
	for _, word := range extendedWords {
		results[word] = true
	}

	finalResults := make([]string, 0, len(results))
	for word := range results {
		finalResults = append(finalResults, word)
	}

	return finalResults
}

func filterParts(parts []string) []string {
	var filteredParts []string
	for _, part := range parts {
		if !isRegion(part) {
			if isEnvironment(part) {
				envParts := strings.Split(part, "-")
				if len(envParts) > 1 {
					filteredParts = append(filteredParts, envParts[len(envParts)-1])
				}
			} else {
				filteredParts = append(filteredParts, part)
			}
		}
	}
	return filteredParts
}

func isRegion(s string) bool {
	regionPattern := regexp.MustCompile(`^[a-z]{2}-[a-z]+-\d+$`)
	return regionPattern.MatchString(s)
}

func isEnvironment(s string) bool {
	envPattern := regexp.MustCompile(`^(prod|dev|stage|test|qa|stg|uat)$`)
	return envPattern.MatchString(s)
}

func generateExtendedWords(baseWords []string) []string {
	suffixes := []string{"qa", "dev", "stage", "prod", "test", "stg", "uat", "admin", "adm", "backup", "bak", "old", "new"}
	var extendedWords []string
	for _, word := range baseWords {

		if word == "" {
			continue
		}

		if !strings.Contains(word, ".") {
			for _, suffix := range suffixes {
				extendedWords = append(extendedWords, word+"-"+suffix, word+suffix)
			}
		}
	}
	return extendedWords
}

func isIPAddress(s string) bool {
	ipPattern := regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
	return ipPattern.MatchString(s)
}
