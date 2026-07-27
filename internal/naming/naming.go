// Package naming generates and validates DNS subdomain labels.
package naming

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

// Alphabets deliberately exclude look-alike characters (0/o, 1/l/i).
const (
	letters = "abcdefghjkmnpqrstuvwxyz"
	alnum   = "abcdefghjkmnpqrstuvwxyz23456789"
)

// DefaultLength is the length of a generated subdomain label.
const DefaultLength = 5

// Random returns a random DNS label of n characters starting with a letter.
func Random(n int) (string, error) {
	if n < 1 {
		n = DefaultLength
	}
	var sb strings.Builder
	first, err := pick(letters)
	if err != nil {
		return "", err
	}
	sb.WriteByte(first)
	for i := 1; i < n; i++ {
		ch, err := pick(alnum)
		if err != nil {
			return "", err
		}
		sb.WriteByte(ch)
	}
	return sb.String(), nil
}

func pick(set string) (byte, error) {
	idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(set))))
	if err != nil {
		return 0, fmt.Errorf("generating random name: %w", err)
	}
	return set[idx.Int64()], nil
}

// ValidateLabel checks a single DNS label against RFC 1123 rules.
func ValidateLabel(s string) error {
	if s == "" {
		return errors.New("name cannot be empty")
	}
	if len(s) > 63 {
		return errors.New("name cannot be longer than 63 characters")
	}
	if s != strings.ToLower(s) {
		return errors.New("name must be lowercase")
	}
	if s[0] == '-' || s[len(s)-1] == '-' {
		return errors.New("name cannot start or end with a hyphen")
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= '0' && c <= '9':
		case c == '-':
		default:
			return fmt.Errorf("name contains an invalid character %q (use a-z, 0-9 and -)", string(c))
		}
	}
	return nil
}

// ValidateHostname checks a dotted hostname label by label.
func ValidateHostname(s string) error {
	s = strings.TrimSuffix(strings.TrimSpace(s), ".")
	if s == "" {
		return errors.New("hostname cannot be empty")
	}
	if len(s) > 253 {
		return errors.New("hostname cannot be longer than 253 characters")
	}
	parts := strings.Split(s, ".")
	if len(parts) < 2 {
		return fmt.Errorf("%q does not look like a domain name", s)
	}
	for _, p := range parts {
		if err := ValidateLabel(p); err != nil {
			return err
		}
	}
	return nil
}
