// Package validation provides small, explicit input validation helpers.
//
// Validation is deliberately conservative: inputs are trimmed, control
// characters are rejected, and every free-text field has a length limit.
package validation

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/6-slx-6/reliefmesh/apps/api/internal/apperr"
)

// Errors collects per-field validation messages.
type Errors map[string]string

// Add records a message for field if none is recorded yet.
func (e Errors) Add(field, msg string) {
	if _, exists := e[field]; !exists {
		e[field] = msg
	}
}

// Has reports whether field has an error.
func (e Errors) Has(field string) bool { _, ok := e[field]; return ok }

// Err returns an *apperr.Error when any message was recorded.
func (e Errors) Err() error {
	if len(e) == 0 {
		return nil
	}
	return apperr.Validation(map[string]string(e))
}

// CleanText trims surrounding whitespace and normalizes line endings.
func CleanText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.TrimSpace(s)
}

// hasForbiddenControl reports control characters other than tab and newline
// (and, for single-line fields, newline too).
func hasForbiddenControl(s string, multiline bool) bool {
	for _, r := range s {
		if r == '\t' || (multiline && r == '\n') {
			continue
		}
		if unicode.IsControl(r) || r == ' ' || r == ' ' {
			return true
		}
	}
	return false
}

// Text validates a free-text value. Use min=0 for optional fields.
func (e Errors) Text(field, value string, min, max int, multiline bool) {
	n := utf8.RuneCountInString(value)
	switch {
	case !utf8.ValidString(value):
		e.Add(field, "Contains invalid characters.")
	case hasForbiddenControl(value, multiline):
		e.Add(field, "Contains invalid control characters.")
	case min > 0 && n == 0:
		e.Add(field, "This field is required.")
	case n < min:
		e.Add(field, fmt.Sprintf("Must be at least %d characters.", min))
	case n > max:
		e.Add(field, fmt.Sprintf("Must be at most %d characters.", max))
	}
}

// OneOf validates that value is one of allowed.
func (e Errors) OneOf(field, value string, allowed []string) {
	if !slices.Contains(allowed, value) {
		e.Add(field, "Must be one of: "+strings.Join(allowed, ", ")+".")
	}
}

// IntRange validates an optional integer.
func (e Errors) IntRange(field string, v *int, min, max int) {
	if v != nil && (*v < min || *v > max) {
		e.Add(field, fmt.Sprintf("Must be between %d and %d.", min, max))
	}
}

// TimeRange validates that an optional time lies within a sane window.
func (e Errors) TimeRange(field string, t *time.Time, notBefore, notAfter time.Time) {
	if t == nil {
		return
	}
	if t.Before(notBefore) || t.After(notAfter) {
		e.Add(field, "Date is outside the allowed range.")
	}
}

var tagPattern = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N}_\-]{0,31}$`)

// Tags normalizes and validates a tag list (lowercase, unique, max 10).
func (e Errors) Tags(field string, tags []string) []string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" {
			continue
		}
		if !tagPattern.MatchString(t) {
			e.Add(field, "Tags may contain letters, digits, '-' and '_' (max 32 characters).")
			return nil
		}
		if !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	if len(out) > 10 {
		e.Add(field, "At most 10 tags are allowed.")
		return nil
	}
	return out
}

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._@\-]{2,63}$`)

// Username validates and normalizes a login name (lowercase).
func (e Errors) Username(field, value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	if !usernamePattern.MatchString(v) {
		e.Add(field, "Use 3-64 characters: lowercase letters, digits, '.', '_', '-' or '@'.")
	}
	return v
}
