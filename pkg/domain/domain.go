package domain

import (
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/config"
	"github.com/dsecuredcom/dynamic-file-searcher/pkg/utils"
	"regexp"
	"strconv"
	"strings"
)

type domainProtocol struct {
	domain   string
	protocol string
}

var (
	ipv4Regex         = regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
	ipv6Regex         = regexp.MustCompile(`^([0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}$`)
	ipPartRegex       = regexp.MustCompile(`(\d{1,3}[-\.]\d{1,3}[-\.]\d{1,3}[-\.]\d{1,3})`)
	md5Regex          = regexp.MustCompile(`^[a-fA-F0-9]{32}$`)
	onlyAlphaRegex    = regexp.MustCompile(`^[a-z]+$`)
	suffixNumberRegex = regexp.MustCompile(`[\d]+$`)
	envRegex          = regexp.MustCompile(`(prod|qa|dev|testing|test|uat|stg|stage|staging|developement|production)$`)
	// Removed the hard-coded appendEnvList. Use cfg.AppendEnvList instead in splitDomain().
	regionPartRegex  = regexp.MustCompile(`(us-east|us-west|af-south|ap-east|ap-south|ap-northeast|ap-southeast|ca-central|eu-west|eu-north|eu-south|me-south|sa-east|us-east-1|us-east-2|us-west-1|us-west-2|af-south-1|ap-east-1|ap-south-1|ap-northeast-3|ap-northeast-2|ap-southeast-1|ap-southeast-2|ap-southeast-3|ap-northeast-1|ca-central-1|eu-central-1|eu-west-1|eu-west-2|eu-west-3|eu-north-1|eu-south-1|me-south-1|sa-east-1|useast1|useast2|uswest1|uswest2|afsouth1|apeast1|apsouth1|apnortheast3|apnortheast2|apsoutheast1|apsoutheast2|apsoutheast3|apnortheast1|cacentral1|eucentral1|euwest1|euwest2|euwest3|eunorth1|eusouth1|mesouth1|saeast1)`)
	byPassCharacters = []string{";", "..;"}
)

var commonTLDsMap map[string]struct{}

func init() {
	// Initialize the TLD map once at startup
	commonTLDsMap = make(map[string]struct{}, len(commonTLDs))
	for _, tld := range commonTLDs {
		commonTLDsMap[tld] = struct{}{}
	}
}

var commonTLDs = []string{
	// Multi-part TLDs
	"co.uk", "co.jp", "co.nz", "co.za", "com.au", "com.br", "com.cn", "com.mx", "com.tr", "com.tw",
	"edu.au", "edu.cn", "edu.hk", "edu.sg", "gov.uk", "net.au", "net.cn", "org.au", "org.uk",
	"ac.uk", "ac.nz", "ac.jp", "ac.kr", "ne.jp", "or.jp", "org.nz", "govt.nz", "sch.uk", "nhs.uk",

	// Generic TLDs (gTLDs)
	"com", "org", "net", "edu", "gov", "int", "mil", "aero", "biz", "cat", "coop", "info", "jobs",
	"mobi", "museum", "name", "pro", "tel", "travel", "xxx", "asia", "arpa",

	// New gTLDs
	"app", "dev", "io", "ai", "cloud", "digital", "online", "store", "tech", "site", "website",
	"blog", "shop", "agency", "expert", "software", "studio", "design", "education", "healthcare",

	// Country Code TLDs (ccTLDs)
	"ac", "ad", "ae", "af", "ag", "ai", "al", "am", "an", "ao", "aq", "ar", "as", "at", "au", "aw",
	"ax", "az", "ba", "bb", "bd", "be", "bf", "bg", "bh", "bi", "bj", "bm", "bn", "bo", "br", "bs",
	"bt", "bv", "bw", "by", "bz", "ca", "cc", "cd", "cf", "cg", "ch", "ci", "ck", "cl", "cm", "cn",
	"co", "cr", "cu", "cv", "cx", "cy", "cz", "de", "dj", "dk", "dm", "do", "dz", "ec", "ee", "eg",
	"er", "es", "et", "eu", "fi", "fj", "fk", "fm", "fo", "fr", "ga", "gb", "gd", "ge", "gf", "gg",
	"gh", "gi", "gl", "gm", "gn", "gp", "gq", "gr", "gs", "gt", "gu", "gw", "gy", "hk", "hm", "hn",
	"hr", "ht", "hu", "id", "ie", "il", "im", "in", "io", "iq", "ir", "is", "it", "je", "jm", "jo",
	"jp", "ke", "kg", "kh", "ki", "km", "kn", "kp", "kr", "kw", "ky", "kz", "la", "lb", "lc", "li",
	"lk", "lr", "ls", "lt", "lu", "lv", "ly", "ma", "mc", "md", "me", "mg", "mh", "mk", "ml", "mm",
	"mn", "mo", "mp", "mq", "mr", "ms", "mt", "mu", "mv", "mw", "mx", "my", "mz", "na", "nc", "ne",
	"nf", "ng", "ni", "nl", "no", "np", "nr", "nu", "nz", "om", "pa", "pe", "pf", "pg", "ph", "pk",
	"pl", "pm", "pn", "pr", "ps", "pt", "pw", "py", "qa", "re", "ro", "rs", "ru", "rw", "sa", "sb",
	"sc", "sd", "se", "sg", "sh", "si", "sj", "sk", "sl", "sm", "sn", "so", "sr", "st", "su", "sv",
	"sy", "sz", "tc", "td", "tf", "tg", "th", "tj", "tk", "tl", "tm", "tn", "to", "tp", "tr", "tt",
	"tv", "tw", "tz", "ua", "ug", "uk", "us", "uy", "uz", "va", "vc", "ve", "vg", "vi", "vn", "vu",
	"wf", "ws", "ye", "yt", "za", "zm", "zw",
}

func splitDomain(host string, cfg *config.Config) []string {
	// Strip protocol
	if strings.HasPrefix(host, "http://") {
		host = strings.TrimPrefix(host, "http://")
	}
	if strings.HasPrefix(host, "https://") {
		host = strings.TrimPrefix(host, "https://")
	}

	// Get just the domain part
	host = strings.Split(host, "/")[0]

	// Skip IP addresses
	if ipv4Regex.MatchString(host) || ipv6Regex.MatchString(host) {
		return nil
	}

	// Remove port if present
	host = strings.Split(host, ":")[0]

	// Remove IP-like parts
	host = ipPartRegex.ReplaceAllString(host, "")

	// Remove hash-like parts
	host = md5Regex.ReplaceAllString(host, "")

	// Remove TLD
	host = removeTLD(host)

	// Remove regional parts
	host = regionPartRegex.ReplaceAllString(host, "")

	// Standardize separators
	host = strings.ReplaceAll(host, "--", "-")
	host = strings.ReplaceAll(host, "..", ".")
	host = strings.ReplaceAll(host, "__", "_")

	// Split into parts by dot
	parts := strings.Split(host, ".")

	// Remove "www" if it's the first part
	if len(parts) > 0 && parts[0] == "www" {
		parts = parts[1:]
	}

	// Limit host depth if configured
	if cfg.HostDepth > 0 && len(parts) >= cfg.HostDepth {
		parts = parts[:cfg.HostDepth]
	}

	// Pre-allocate the map with a reasonable capacity
	estimatedCapacity := len(parts) * 3 // Rough estimate for parts and subparts
	relevantParts := make(map[string]struct{}, estimatedCapacity)

	// Process each part
	for _, part := range parts {
		relevantParts[part] = struct{}{}

		// Split by separators
		subParts := strings.FieldsFunc(part, func(r rune) bool {
			return r == '-' || r == '_'
		})

		// Add each subpart
		for _, subPart := range subParts {
			relevantParts[subPart] = struct{}{}
		}
	}

	// Estimate final result size
	estimatedResultSize := len(relevantParts)
	if !cfg.NoEnvAppending {
		// If we'll be adding env variants, estimate additional capacity
		estimatedResultSize += len(relevantParts) * len(cfg.AppendEnvList) * 4
	}

	// Allocate result slice with appropriate capacity
	result := make([]string, 0, estimatedResultSize)

	// Process each relevant part
	for part := range relevantParts {
		// Skip purely numeric parts
		if _, err := strconv.Atoi(part); err == nil {
			continue
		}

		// Skip single characters
		if len(part) <= 1 {
			continue
		}

		// If part matches environment pattern, add a version without it
		if envRegex.MatchString(part) {
			result = append(result, strings.TrimSuffix(part, envRegex.FindString(part)))
		}

		// If part ends with numbers, add a version without the numbers
		if suffixNumberRegex.MatchString(part) {
			result = append(result, strings.TrimSuffix(part, suffixNumberRegex.FindString(part)))
		}

		// Add the original part
		result = append(result, part)
	}

	// Add environment variants if enabled
	if !cfg.NoEnvAppending {
		baseLength := len(result)
		for i := 0; i < baseLength; i++ {
			part := result[i]
			// Skip parts that aren't purely alphabetic
			if !onlyAlphaRegex.MatchString(part) {
				continue
			}

			// Skip if part already ends with an environment suffix
			shouldBeAdded := true
			for _, env := range cfg.AppendEnvList {
				if strings.HasSuffix(part, env) {
					shouldBeAdded = false
					break
				}
			}

			if shouldBeAdded {
				for _, env := range cfg.AppendEnvList {
					// Skip if part already contains the environment name
					if strings.Contains(part, env) {
						continue
					}

					// Add variants with different separators
					result = append(result, part+env)
					result = append(result, part+"-"+env)
					result = append(result, part+"_"+env)
					result = append(result, part+"/"+env)
				}
			}
		}
	}

	// Remove environment suffixes if enabled
	if cfg.EnvRemoving {
		baseLength := len(result)
		for i := 0; i < baseLength; i++ {
			part := result[i]
			// Skip parts that aren't purely alphabetic
			if !onlyAlphaRegex.MatchString(part) {
				continue
			}

			// If the part ends with a known env word, produce a version with that suffix trimmed
			for _, env := range cfg.AppendEnvList {
				if strings.HasSuffix(part, env) {
					result = append(result, strings.TrimSuffix(part, env))
					break
				}
			}
		}
	}

	// Clean up results (trim separators)
	cleanedResult := make([]string, 0, len(result))
	for _, item := range result {
		trimmed := strings.Trim(item, ".-_")
		if trimmed != "" {
			cleanedResult = append(cleanedResult, trimmed)
		}
	}

	// Add short prefixes (3 and 4 character) for common patterns
	baseLength := len(cleanedResult)
	additionalItems := make([]string, 0, baseLength*2)
	for i := 0; i < baseLength; i++ {
		word := cleanedResult[i]
		if len(word) >= 3 {
			additionalItems = append(additionalItems, word[:3])
		}
		if len(word) >= 4 {
			additionalItems = append(additionalItems, word[:4])
		}
	}

	// Combine all items
	result = append(cleanedResult, additionalItems...)

	// Deduplicate
	result = makeUniqueList(result)

	// Add bypass character variants if enabled
	if cfg.AppendByPassesToWords {
		baseLength := len(result)
		bypassVariants := make([]string, 0, baseLength*len(byPassCharacters))

		for i := 0; i < baseLength; i++ {
			for _, bypass := range byPassCharacters {
				bypassVariants = append(bypassVariants, result[i]+bypass)
			}
		}

		result = append(result, bypassVariants...)
	}

	return result
}

func GetRelevantDomainParts(host string, cfg *config.Config) []string {
	return splitDomain(host, cfg)
}

func makeUniqueList(input []string) []string {
	// Use a map for deduplication
	seen := make(map[string]struct{}, len(input))
	result := make([]string, 0, len(input))

	for _, item := range input {
		if _, exists := seen[item]; !exists {
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}

	return result
}

func GetDomains(domainsFile, singleDomain string) []string {
	if domainsFile != "" {
		allLines := utils.ReadLines(domainsFile)
		// Pre-allocate with a capacity based on the number of lines
		validDomains := make([]string, 0, len(allLines))

		for _, line := range allLines {
			trimmedLine := strings.TrimSpace(line)
			if trimmedLine != "" && !strings.HasPrefix(trimmedLine, "#") {
				validDomains = append(validDomains, trimmedLine)
			}
		}

		validDomains = utils.ShuffleStrings(validDomains)
		return validDomains
	}

	// Return single domain as a slice
	return []string{singleDomain}
}

func removeTLD(host string) string {
	host = strings.ToLower(host)
	parts := strings.Split(host, ".")

	// Iterate through possible multi-part TLDs
	for i := 0; i < len(parts); i++ {
		potentialTLD := strings.Join(parts[i:], ".")
		if _, exists := commonTLDsMap[potentialTLD]; exists {
			return strings.Join(parts[:i], ".")
		}
	}

	return host
}
