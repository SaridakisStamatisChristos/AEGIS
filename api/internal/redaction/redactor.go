package redaction

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/aegisrun/aegisrun/internal/contracts"
)

// Redactor handles PII/secrets redaction
type Redactor struct {
	patterns     []*regexp.Regexp
	maskStrategy contracts.MaskStrategy
}

func NewRedactor(patterns []*regexp.Regexp, strategy contracts.MaskStrategy) *Redactor {
	if len(patterns) == 0 {
		patterns = DefaultPatterns()
	}
	if strategy == "" {
		strategy = contracts.MaskRedact
	}

	return &Redactor{
		patterns:     patterns,
		maskStrategy: strategy,
	}
}

// RedactValue redacts sensitive data from a single value
func (r *Redactor) RedactValue(value interface{}) (interface{}, bool) {
	switch v := value.(type) {
	case string:
		return r.redactString(v)
	case map[string]interface{}:
		return r.redactMap(v)
	case []interface{}:
		return r.redactSlice(v)
	default:
		return value, false
	}
}

func (r *Redactor) redactString(s string) (string, bool) {
	redacted := false
	result := s

	for _, pattern := range r.patterns {
		if pattern.MatchString(result) {
			patternName := GetPatternName(pattern)
			result = pattern.ReplaceAllStringFunc(result, func(match string) string {
				redacted = true
				return r.mask(match, patternName)
			})
		}
	}

	return result, redacted
}

func (r *Redactor) redactMap(m map[string]interface{}) (map[string]interface{}, bool) {
	result := make(map[string]interface{})
	anyRedacted := false

	for k, v := range m {
		// Check if key itself should trigger redaction
		if r.isSensitiveKey(k) {
			result[k] = r.mask(fmt.Sprintf("%v", v), "SENSITIVE_FIELD")
			anyRedacted = true
			continue
		}

		redactedValue, wasRedacted := r.RedactValue(v)
		result[k] = redactedValue
		if wasRedacted {
			anyRedacted = true
		}
	}

	return result, anyRedacted
}

func (r *Redactor) redactSlice(slice []interface{}) ([]interface{}, bool) {
	result := make([]interface{}, len(slice))
	anyRedacted := false

	for i, v := range slice {
		redactedValue, wasRedacted := r.RedactValue(v)
		result[i] = redactedValue
		if wasRedacted {
			anyRedacted = true
		}
	}

	return result, anyRedacted
}

func (r *Redactor) isSensitiveKey(key string) bool {
	lowerKey := strings.ToLower(key)
	sensitiveKeys := []string{
		"password", "passwd", "pwd",
		"secret", "token", "api_key", "apikey",
		"private_key", "privatekey",
		"credit_card", "creditcard", "cc",
		"ssn", "social_security",
	}

	for _, sk := range sensitiveKeys {
		if strings.Contains(lowerKey, sk) {
			return true
		}
	}

	return false
}

func (r *Redactor) mask(value string, patternType string) string {
	switch r.maskStrategy {
	case contracts.MaskHash:
		hash := sha256.Sum256([]byte(value))
		return fmt.Sprintf("[REDACTED:%s:%s]", patternType, hex.EncodeToString(hash[:])[:8])

	case contracts.MaskTruncate:
		if len(value) <= 4 {
			return fmt.Sprintf("[REDACTED:%s]", patternType)
		}
		return fmt.Sprintf("%s****[REDACTED:%s]", value[:4], patternType)

	case contracts.MaskRedact:
		fallthrough
	default:
		return fmt.Sprintf("[REDACTED:%s]", patternType)
	}
}

// RedactJSON redacts sensitive data from JSON
func (r *Redactor) RedactJSON(data []byte) ([]byte, bool, error) {
	var obj interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, false, fmt.Errorf("unmarshal JSON: %w", err)
	}

	redacted, wasRedacted := r.RedactValue(obj)

	result, err := json.Marshal(redacted)
	if err != nil {
		return nil, false, fmt.Errorf("marshal redacted JSON: %w", err)
	}

	return result, wasRedacted, nil
}

// RedactMap is a convenience method for redacting map[string]interface{}
func (r *Redactor) RedactMap(m map[string]interface{}) (map[string]interface{}, bool) {
	return r.redactMap(m)
}
