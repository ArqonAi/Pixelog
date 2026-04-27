package privacy

import (
	"regexp"
	"strings"
)

// Filter strips secrets, credentials, and sensitive data from content
// before it is stored in indexes or embeddings.
type Filter struct {
	patterns     []*patternRule
	customRedact []string // user-defined additional patterns
}

type patternRule struct {
	name    string
	regex   *regexp.Regexp
	replace string
}

// NewFilter creates a privacy filter with production-grade secret detection patterns.
func NewFilter() *Filter {
	f := &Filter{}
	f.patterns = defaultPatterns()
	return f
}

// AddCustomPattern adds a user-defined regex pattern for redaction.
func (f *Filter) AddCustomPattern(name, pattern, replacement string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return err
	}
	f.patterns = append(f.patterns, &patternRule{
		name:    name,
		regex:   re,
		replace: replacement,
	})
	return nil
}

// Scrub removes all detected secrets from the input string.
// Returns the scrubbed content and a count of redactions made.
func (f *Filter) Scrub(content string) (string, int) {
	redactions := 0
	result := content

	for _, p := range f.patterns {
		matches := p.regex.FindAllStringIndex(result, -1)
		if len(matches) > 0 {
			redactions += len(matches)
			result = p.regex.ReplaceAllString(result, p.replace)
		}
	}

	return result, redactions
}

// ContainsSecrets returns true if the content contains any detected secrets.
func (f *Filter) ContainsSecrets(content string) bool {
	for _, p := range f.patterns {
		if p.regex.MatchString(content) {
			return true
		}
	}
	return false
}

// DetectedSecrets returns a list of secret type names found in the content.
func (f *Filter) DetectedSecrets(content string) []string {
	var found []string
	seen := make(map[string]struct{})
	for _, p := range f.patterns {
		if p.regex.MatchString(content) {
			if _, ok := seen[p.name]; !ok {
				found = append(found, p.name)
				seen[p.name] = struct{}{}
			}
		}
	}
	return found
}

// ScrubLines processes content line by line, useful for large documents.
func (f *Filter) ScrubLines(content string) (string, int) {
	lines := strings.Split(content, "\n")
	totalRedactions := 0
	for i, line := range lines {
		scrubbed, count := f.Scrub(line)
		lines[i] = scrubbed
		totalRedactions += count
	}
	return strings.Join(lines, "\n"), totalRedactions
}

func defaultPatterns() []*patternRule {
	rules := []struct {
		name    string
		pattern string
		replace string
	}{
		// API Keys
		{"openai_api_key", `sk-[A-Za-z0-9_-]{20,}`, "[REDACTED:OPENAI_KEY]"},
		{"openrouter_api_key", `sk-or-v1-[A-Za-z0-9]{48,}`, "[REDACTED:OPENROUTER_KEY]"},
		{"anthropic_api_key", `sk-ant-[A-Za-z0-9_-]{20,}`, "[REDACTED:ANTHROPIC_KEY]"},
		{"google_api_key", `AIza[A-Za-z0-9_-]{35}`, "[REDACTED:GOOGLE_KEY]"},
		{"aws_access_key", `AKIA[A-Z0-9]{16}`, "[REDACTED:AWS_KEY]"},
		{"azure_key", `[A-Za-z0-9+/]{86}==`, "[REDACTED:AZURE_KEY]"},
		{"stripe_key", `(sk|pk)_(test|live)_[A-Za-z0-9]{20,}`, "[REDACTED:STRIPE_KEY]"},
		{"github_token", `(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{36,}`, "[REDACTED:GITHUB_TOKEN]"},
		{"gitlab_token", `glpat-[A-Za-z0-9_-]{20,}`, "[REDACTED:GITLAB_TOKEN]"},
		{"slack_token", `xox[bpas]-[A-Za-z0-9-]+`, "[REDACTED:SLACK_TOKEN]"},
		{"discord_token", `[MN][A-Za-z0-9]{23,}\.[A-Za-z0-9_-]{6}\.[A-Za-z0-9_-]{27,}`, "[REDACTED:DISCORD_TOKEN]"},
		{"twilio_key", `SK[a-f0-9]{32}`, "[REDACTED:TWILIO_KEY]"},
		{"sendgrid_key", `SG\.[A-Za-z0-9_-]{22}\.[A-Za-z0-9_-]{43}`, "[REDACTED:SENDGRID_KEY]"},
		{"npm_token", `npm_[A-Za-z0-9]{36}`, "[REDACTED:NPM_TOKEN]"},
		{"heroku_key", `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`, "[REDACTED:UUID]"},

		// Bearer tokens in headers
		{"bearer_token", `(?i)bearer\s+[A-Za-z0-9._~+/=-]{20,}`, "[REDACTED:BEARER]"},
		{"authorization_header", `(?i)authorization:\s*[A-Za-z]+\s+[A-Za-z0-9._~+/=-]{20,}`, "[REDACTED:AUTH_HEADER]"},

		// Private keys
		{"private_key_pem", `-----BEGIN\s+(RSA|EC|OPENSSH|DSA)?\s*PRIVATE KEY-----[\s\S]*?-----END\s+(RSA|EC|OPENSSH|DSA)?\s*PRIVATE KEY-----`, "[REDACTED:PRIVATE_KEY]"},

		// Connection strings
		{"postgres_url", `postgres(ql)?://[^\s"']+`, "[REDACTED:DB_URL]"},
		{"mysql_url", `mysql://[^\s"']+`, "[REDACTED:DB_URL]"},
		{"mongodb_url", `mongodb(\+srv)?://[^\s"']+`, "[REDACTED:DB_URL]"},
		{"redis_url", `redis(s)?://[^\s"']+`, "[REDACTED:DB_URL]"},

		// Passwords in common config formats
		{"password_assignment", `(?i)(password|passwd|pwd)\s*[:=]\s*["']?[^\s"'\n]{8,}["']?`, "[REDACTED:PASSWORD]"},
		{"secret_assignment", `(?i)(secret|token|api_key|apikey|auth_token)\s*[:=]\s*["']?[^\s"'\n]{8,}["']?`, "[REDACTED:SECRET]"},

		// Hex-encoded secrets (64+ chars, likely SHA-256 or similar)
		{"hex_secret", `(?i)(secret|key|token)\s*[:=]\s*["']?[a-fA-F0-9]{64,}["']?`, "[REDACTED:HEX_SECRET]"},

		// IPv4 with port (potential internal addresses)
		{"internal_ip_port", `\b(10\.\d{1,3}\.\d{1,3}\.\d{1,3}|172\.(1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3}|192\.168\.\d{1,3}\.\d{1,3}):\d{1,5}\b`, "[REDACTED:INTERNAL_ADDR]"},

		// AWS ARN
		{"aws_arn", `arn:aws:[a-zA-Z0-9_-]+:[a-z0-9-]*:\d{12}:[^\s"']+`, "[REDACTED:AWS_ARN]"},

		// JWT tokens (three base64 segments separated by dots)
		{"jwt_token", `eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`, "[REDACTED:JWT]"},
	}

	patterns := make([]*patternRule, 0, len(rules))
	for _, r := range rules {
		re, err := regexp.Compile(r.pattern)
		if err != nil {
			continue // skip invalid patterns silently
		}
		patterns = append(patterns, &patternRule{
			name:    r.name,
			regex:   re,
			replace: r.replace,
		})
	}
	return patterns
}
