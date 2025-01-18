package domain

import (
	"fmt"
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

	if strings.HasPrefix(host, "http://") {
		host = strings.TrimPrefix(host, "http://")
	}

	if strings.HasPrefix(host, "https://") {
		host = strings.TrimPrefix(host, "https://")
	}

	host = strings.Split(host, "/")[0]

	if ipv4Regex.MatchString(host) || ipv6Regex.MatchString(host) {
		return []string{}
	}

	// This is a super naive but effective approach
	// 1. remove port in case it exists
	host = strings.Split(host, ":")[0]
	// Remove everything that looks like an ip (1-1-1-1, 1.1.1.1)
	host = ipPartRegex.ReplaceAllString(host, "")
	// Remove everything that looks like a hash
	host = md5Regex.ReplaceAllString(host, "")
	// Remove the top level domain
	host = removeTLD(host)
	// Remove regional parts, those are usually not interesting
	host = regionPartRegex.ReplaceAllString(host, "")

	// Standardize the relevant host part
	host = strings.ReplaceAll(host, "--", "-")
	host = strings.ReplaceAll(host, "..", ".")
	host = strings.ReplaceAll(host, "__", "_")

	// Separate the host into parts
	parts := strings.Split(host, ".")

	// if first part is www, remove it:
	if len(parts) > 0 && parts[0] == "www" {
		parts = parts[1:]
	}

	// If the host depth is set, only take the first n parts
	if cfg.HostDepth > 0 && len(parts) >= cfg.HostDepth {
		parts = parts[:cfg.HostDepth]
	}

	// We use a map to avoid duplicates
	relevantParts := make(map[string]bool)

	for i := 0; i < len(parts); i++ {
		relevantParts[parts[i]] = true

		subParts := strings.FieldsFunc(parts[i], func(r rune) bool {
			return r == '-' || r == '_'
		})

		// Add each subpart
		for _, subPart := range subParts {
			relevantParts[subPart] = true
		}
	}

	var result []string
	for part := range relevantParts {
		// Drop parts that are purely numeric
		if _, err := strconv.Atoi(part); err == nil {
			continue
		}

		// If part is just a single character, skip it
		if len(part) == 1 {
			continue
		}

		// If part matches the envRegex, remove this env string and add the rest to results
		if envRegex.MatchString(part) {
			result = append(result, strings.ReplaceAll(part, envRegex.FindString(part), ""))
		}

		// If part ends with a number, remove the numeric suffix and add the rest
		if suffixNumberRegex.MatchString(part) {
			result = append(result, strings.ReplaceAll(part, suffixNumberRegex.FindString(part), ""))
		}

		result = append(result, part)
	}

	// If appending env words is allowed, add them
	if cfg.NoEnvAppending == false {
		for _, part := range result {
			// Skip base word if it is not purely alpha
			if onlyAlphaRegex.MatchString(part) == false {
				continue
			}

			// Some sanity checks to prevent adding too many env words
			shouldBeAdded := true
			for _, env := range cfg.AppendEnvList {
				if strings.HasSuffix(part, env) {
					shouldBeAdded = false
					break
				}
			}

			if shouldBeAdded {
				for _, env := range cfg.AppendEnvList {
					// If part already *contains* env, skip
					if strings.Contains(part, env) {
						continue
					}
					result = append(result, part+env)
					result = append(result, part+"-"+env)
					result = append(result, part+"_"+env)
					result = append(result, part+"/"+env)
				}
			}
		}
	}

	// If removing environment suffixes is enabled
	if cfg.EnvRemoving {
		for _, part := range result {
			// Skip if not purely alpha
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

	// Clean things up
	for i := 0; i < len(result); i++ {
		result[i] = strings.TrimRight(result[i], ".-_")
		result[i] = strings.TrimLeft(result[i], ".-_")

		if result[i] == "" {
			result = append(result[:i], result[i+1:]...)
			i--
		}
	}

	// Make list unique
	result = makeUniqueList(result)

	if cfg.AppendByPassesToWords {
		for _, part := range result {
			for _, bypass := range byPassCharacters {
				result = append(result, part+bypass)
			}
		}
	}

	return result
}

func makeUniqueList(input []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range input {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func GetDomains(domainsFile, singleDomain string) []string {
	if domainsFile != "" {
		allLines := utils.ReadLines(domainsFile)
		var validDomains []string
		for _, line := range allLines {
			trimmedLine := strings.TrimSpace(line)
			if trimmedLine != "" && !strings.HasPrefix(trimmedLine, "#") {
				validDomains = append(validDomains, trimmedLine)
			}
		}
		validDomains = utils.ShuffleStrings(validDomains)
		return validDomains
	}
	return []string{singleDomain}
}

func GenerateURLs(domains, paths []string, cfg *config.Config) ([]string, int) {
	var domainProtocols []domainProtocol

	var proto = "https"

	for _, d := range domains {
		if cfg.ForceHTTPProt {
			proto = "http"
		} else {
			proto = "https"
		}

		if strings.HasPrefix(d, "http://") {
			proto = "http"
			d = strings.TrimPrefix(d, "http://")
		}

		if strings.HasPrefix(d, "https://") {
			proto = "https"
			d = strings.TrimPrefix(d, "https://")
		}

		d = strings.TrimSuffix(d, "/")

		domainProtocols = append(domainProtocols, domainProtocol{domain: d, protocol: proto})
	}

	var allURLs []string
	for _, dp := range domainProtocols {
		for _, path := range paths {
			if strings.HasPrefix(path, "##") {
				continue
			}
			if !cfg.SkipRootFolderCheck {
				allURLs = append(allURLs, fmt.Sprintf("%s://%s/%s", dp.protocol, dp.domain, path))
			}
			if len(cfg.BasePaths) > 0 {
				for _, basePath := range cfg.BasePaths {
					allURLs = append(allURLs, fmt.Sprintf("%s://%s/%s/%s", dp.protocol, dp.domain, basePath, path))
				}
			}
			if cfg.DontGeneratePaths {
				continue
			}

			words := splitDomain(dp.domain, cfg)

			if len(cfg.BasePaths) == 0 {
				for _, word := range words {
					allURLs = append(allURLs, fmt.Sprintf("%s://%s/%s/%s", dp.protocol, dp.domain, word, path))
				}
			} else {
				for _, word := range words {
					for _, basePath := range cfg.BasePaths {
						allURLs = append(allURLs, fmt.Sprintf("%s://%s/%s/%s/%s", dp.protocol, dp.domain, basePath, word, path))
					}
				}
			}
		}
	}

	allURLs = utils.ShuffleStrings(allURLs)

	return allURLs, len(domainProtocols)
}

func removeTLD(host string) string {
	host = strings.ToLower(host)
	parts := strings.Split(host, ".")

	for i := 0; i < len(parts); i++ {
		potentialTLD := strings.Join(parts[i:], ".")
		for _, tld := range commonTLDs {
			if potentialTLD == tld {
				return strings.Join(parts[:i], ".")
			}
		}
	}

	return host
}
