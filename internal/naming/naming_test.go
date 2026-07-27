package naming

import (
	"strings"
	"testing"
)

func TestRandomProducesValidLabels(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 500; i++ {
		s, err := Random(DefaultLength)
		if err != nil {
			t.Fatalf("Random() error = %v", err)
		}
		if len(s) != DefaultLength {
			t.Fatalf("len(%q) = %d, want %d", s, len(s), DefaultLength)
		}
		if err := ValidateLabel(s); err != nil {
			t.Fatalf("Random() produced an invalid label %q: %v", s, err)
		}
		if !strings.ContainsAny(s[:1], letters) {
			t.Fatalf("label %q does not start with a letter", s)
		}
		seen[s] = true
	}
	// 500 draws from ~23*31^4 possibilities should almost never collide much.
	if len(seen) < 450 {
		t.Fatalf("only %d unique labels out of 500 — randomness looks broken", len(seen))
	}
}

func TestRandomDefaultsLength(t *testing.T) {
	s, err := Random(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(s) != DefaultLength {
		t.Fatalf("len = %d, want %d", len(s), DefaultLength)
	}
}

func TestRandomExcludesConfusingCharacters(t *testing.T) {
	for i := 0; i < 300; i++ {
		s, err := Random(8)
		if err != nil {
			t.Fatal(err)
		}
		if strings.ContainsAny(s, "0o1li") {
			t.Fatalf("label %q contains a look-alike character", s)
		}
	}
}

func TestValidateLabel(t *testing.T) {
	valid := []string{"a", "crm", "api-v2", "x92ka", "a1", "my-long-name-123"}
	for _, s := range valid {
		if err := ValidateLabel(s); err != nil {
			t.Errorf("ValidateLabel(%q) = %v, want nil", s, err)
		}
	}

	invalid := []string{
		"",
		"-crm",
		"crm-",
		"CRM",
		"my_app",
		"my.app",
		"my app",
		"café",
		strings.Repeat("a", 64),
	}
	for _, s := range invalid {
		if err := ValidateLabel(s); err == nil {
			t.Errorf("ValidateLabel(%q) = nil, want an error", s)
		}
	}
}

func TestValidateHostname(t *testing.T) {
	valid := []string{"example.com", "demo.example.com", "a.b.c.example.com", "demo.example.com."}
	for _, s := range valid {
		if err := ValidateHostname(s); err != nil {
			t.Errorf("ValidateHostname(%q) = %v, want nil", s, err)
		}
	}

	invalid := []string{"", "example", "demo..example.com", "-demo.example.com", "DEMO.example.com"}
	for _, s := range invalid {
		if err := ValidateHostname(s); err == nil {
			t.Errorf("ValidateHostname(%q) = nil, want an error", s)
		}
	}
}
