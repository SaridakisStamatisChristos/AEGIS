package redaction

import (
	"regexp"
)

// Common PII/secrets patterns
var (
	EmailPattern      = regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`)
	PhoneUSPattern    = regexp.MustCompile(`\b\d{3}[-.]?\d{3}[-.]?\d{4}\b`)
	PhoneIntlPattern  = regexp.MustCompile(`\+\d{1,3}[-.\s]?\(?\d{1,4}\)?[-.\s]?\d{1,4}[-.\s]?\d{1,9}`)
	CreditCardPattern = regexp.MustCompile(`\b\d{4}[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}\b`)
	SSNPattern        = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)

	// API key patterns (heuristic)
	GenericKeyPattern = regexp.MustCompile(`(?i)(api[_-]?key|apikey|access[_-]?token|secret[_-]?key|password)["\s:=]+([A-Za-z0-9_\-]{20,})`)
	AWSKeyPattern     = regexp.MustCompile(`AKIA[0-9A-Z]{16}`)
	JWTPattern        = regexp.MustCompile(`eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*`)

	// IP addresses (optional, for certain use cases)
	IPv4Pattern = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
)

// DefaultPatterns returns the default set of redaction patterns
func DefaultPatterns() []*regexp.Regexp {
	return []*regexp.Regexp{
		EmailPattern,
		PhoneUSPattern,
		PhoneIntlPattern,
		CreditCardPattern,
		SSNPattern,
		GenericKeyPattern,
		AWSKeyPattern,
		JWTPattern,
	}
}

// PatternNames maps patterns to human-readable names
var PatternNames = map[*regexp.Regexp]string{
	EmailPattern:      "EMAIL",
	PhoneUSPattern:    "PHONE",
	PhoneIntlPattern:  "PHONE_INTL",
	CreditCardPattern: "CC",
	SSNPattern:        "SSN",
	GenericKeyPattern: "API_KEY",
	AWSKeyPattern:     "AWS_KEY",
	JWTPattern:        "JWT",
	IPv4Pattern:       "IPV4",
}

func GetPatternName(pattern *regexp.Regexp) string {
	if name, ok := PatternNames[pattern]; ok {
		return name
	}
	return "REDACTED"
}
