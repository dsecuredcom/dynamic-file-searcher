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

func StreamDomainParts(host string, cfg *config.Config, callback func(string)) {
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
		return
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

	// Use smaller map to track sent parts - with initial capacity hint
	sent := make(map[string]struct{}, len(parts)*3) // Estimate ~3 variants per part

	// Helper to send unique parts
	sendUnique := func(part string) {
		if _, exists := sent[part]; !exists && part != "" && len(part) > 1 {
			sent[part] = struct{}{}
			callback(part)
		}
	}

	// Process each part
	for _, part := range parts {
		// Skip purely numeric parts
		if _, err := strconv.Atoi(part); err == nil {
			continue
		}

		// Skip single characters
		if len(part) <= 1 {
			continue
		}

		// Process base part
		sendUnique(part)

		// Split by separators - but limit depth to avoid explosion
		subParts := strings.FieldsFunc(part, func(r rune) bool {
			return r == '-' || r == '_'
		})

		// Limit subparts to avoid memory explosion
		if len(subParts) > 10 {
			subParts = subParts[:10]
		}

		for _, subPart := range subParts {
			sendUnique(subPart)
		}

		// If part matches environment pattern, add version without it
		if envRegex.MatchString(part) {
			cleaned := strings.TrimSuffix(part, envRegex.FindString(part))
			sendUnique(cleaned)
		}

		// If part ends with numbers, add version without numbers
		if suffixNumberRegex.MatchString(part) {
			cleaned := strings.TrimSuffix(part, suffixNumberRegex.FindString(part))
			sendUnique(cleaned)
		}

		// Add short prefixes - but limit to avoid too many variants
		if len(sent) < 100 { // Limit total variants
			if len(part) >= 3 {
				sendUnique(part[:3])
			}
			if len(part) >= 4 {
				sendUnique(part[:4])
			}
		}
	}

	// Process environment variants if enabled - but with limits
	if !cfg.NoEnvAppending && len(sent) < 200 {
		// Create a slice of parts to iterate (to avoid modifying map while iterating)
		partsToProcess := make([]string, 0, len(sent))
		for sentPart := range sent {
			if onlyAlphaRegex.MatchString(sentPart) {
				partsToProcess = append(partsToProcess, sentPart)
			}
		}

		// Limit parts to process
		if len(partsToProcess) > 20 {
			partsToProcess = partsToProcess[:20]
		}

		for _, sentPart := range partsToProcess {
			// Skip if already ends with env suffix
			hasEnvSuffix := false
			for _, env := range cfg.AppendEnvList {
				if strings.HasSuffix(sentPart, env) {
					hasEnvSuffix = true
					break
				}
			}

			if !hasEnvSuffix {
				// Limit env variants
				maxEnvs := 3
				if len(cfg.AppendEnvList) < maxEnvs {
					maxEnvs = len(cfg.AppendEnvList)
				}

				for i := 0; i < maxEnvs; i++ {
					env := cfg.AppendEnvList[i]
					if !strings.Contains(sentPart, env) {
						callback(sentPart + env)
						callback(sentPart + "-" + env)
						// Skip underscore variant to reduce combinations
						// callback(sentPart + "_" + env)
						// Skip slash variant
						// callback(sentPart + "/" + env)
					}
				}
			}
		}
	}

	// Remove environment suffixes if enabled
	if cfg.EnvRemoving {
		for sentPart := range sent {
			if onlyAlphaRegex.MatchString(sentPart) {
				for _, env := range cfg.AppendEnvList {
					if strings.HasSuffix(sentPart, env) {
						cleaned := strings.TrimSuffix(sentPart, env)
						callback(cleaned)
						break
					}
				}
			}
		}
	}

	// Add bypass characters if enabled - but limit them
	if cfg.AppendByPassesToWords && len(sent) < 50 {
		// Create a slice of current parts to avoid modifying map during iteration
		currentParts := make([]string, 0, len(sent))
		for part := range sent {
			currentParts = append(currentParts, part)
		}

		// Limit parts for bypass
		if len(currentParts) > 10 {
			currentParts = currentParts[:10]
		}

		for _, part := range currentParts {
			// Only add first bypass character to reduce combinations
			if len(byPassCharacters) > 0 {
				callback(part + byPassCharacters[0])
			}
		}
	}
}

// GetRelevantDomainParts - backward compatibility wrapper
func GetRelevantDomainParts(host string, cfg *config.Config) []string {
	var result []string
	StreamDomainParts(host, cfg, func(part string) {
		result = append(result, part)
	})
	return result
}

func GetDomains(domainsFile, singleDomain string) []string {
	if domainsFile != "" {
		allLines := utils.ReadLines(domainsFile)
		validDomains := make([]string, 0, len(allLines))

		for _, line := range allLines {
			trimmedLine := strings.TrimSpace(line)
			if trimmedLine != "" && !strings.HasPrefix(trimmedLine, "#") {
				validDomains = append(validDomains, trimmedLine)
			}
		}

		// Don't shuffle to maintain predictable memory usage
		// validDomains = utils.ShuffleStrings(validDomains)
		return validDomains
	}

	return []string{singleDomain}
}

func removeTLD(host string) string {
	host = strings.ToLower(host)
	parts := strings.Split(host, ".")

	for i := 0; i < len(parts); i++ {
		potentialTLD := strings.Join(parts[i:], ".")
		if _, exists := commonTLDsMap[potentialTLD]; exists {
			return strings.Join(parts[:i], ".")
		}
	}

	return host
}
