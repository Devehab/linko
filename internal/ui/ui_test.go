package ui

import (
	"bytes"
	"strings"
	"testing"
)

func TestColourCanBeDisabled(t *testing.T) {
	old := useColor
	defer SetColor(old)

	SetColor(false)
	if got := Green("ok"); got != "ok" {
		t.Fatalf("Green() = %q with colour off", got)
	}

	SetColor(true)
	if got := Green("ok"); !strings.Contains(got, "\x1b[32m") {
		t.Fatalf("Green() = %q with colour on", got)
	}
}

func TestTableAligns(t *testing.T) {
	old := useColor
	defer SetColor(old)
	SetColor(false)

	var buf bytes.Buffer
	Table(&buf, []string{"NAME", "URL"}, [][]string{
		{"crm", "https://crm.demo.example.com"},
		{"a", "https://a.demo.example.com"},
	})
	out := buf.String()

	if !strings.Contains(out, "NAME") || !strings.Contains(out, "crm") {
		t.Fatalf("table output missing content:\n%s", out)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (header + 2 rows):\n%s", len(lines), out)
	}
	// Columns must line up: the URL starts at the same offset on both rows.
	if strings.Index(lines[1], "https://") != strings.Index(lines[2], "https://") {
		t.Fatalf("columns are not aligned:\n%s", out)
	}
}

func TestAskUsesDefaultOnEmptyInput(t *testing.T) {
	p := NewPrompterFrom(strings.NewReader("\n"))
	got, err := p.Ask("Domain:", "example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got != "example.com" {
		t.Fatalf("Ask() = %q, want the default", got)
	}
}

func TestAskTrimsInput(t *testing.T) {
	p := NewPrompterFrom(strings.NewReader("  example.org  \n"))
	got, err := p.Ask("Domain:", "example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got != "example.org" {
		t.Fatalf("Ask() = %q", got)
	}
}

func TestAskRequiredRetriesUntilValid(t *testing.T) {
	p := NewPrompterFrom(strings.NewReader("\nBAD\ngood\n"))
	got, err := p.AskRequired("Name:", "", func(s string) error {
		if s != "good" {
			return errNotGood
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "good" {
		t.Fatalf("AskRequired() = %q", got)
	}
}

var errNotGood = errTest("not good")

type errTest string

func (e errTest) Error() string { return string(e) }

func TestConfirm(t *testing.T) {
	cases := []struct {
		in   string
		def  bool
		want bool
	}{
		{"y\n", false, true},
		{"YES\n", false, true},
		{"n\n", true, false},
		{"no\n", true, false},
		{"\n", true, true},
		{"\n", false, false},
		{"maybe\n", true, true},
	}
	for _, c := range cases {
		p := NewPrompterFrom(strings.NewReader(c.in))
		if got := p.Confirm("Continue?", c.def); got != c.want {
			t.Errorf("Confirm(%q, def=%v) = %v, want %v", c.in, c.def, got, c.want)
		}
	}
}

func TestChoose(t *testing.T) {
	p := NewPrompterFrom(strings.NewReader("2\n"))
	got, err := p.Choose("Pick:", []string{"a", "b", "c"})
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 {
		t.Fatalf("Choose() = %d, want 1 (zero-based)", got)
	}
}

func TestChooseRejectsOutOfRange(t *testing.T) {
	p := NewPrompterFrom(strings.NewReader("9\nx\n1\n"))
	got, err := p.Choose("Pick:", []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if got != 0 {
		t.Fatalf("Choose() = %d, want 0", got)
	}
}

func TestChooseGivesUp(t *testing.T) {
	p := NewPrompterFrom(strings.NewReader("9\n9\n9\n9\n9\n"))
	if _, err := p.Choose("Pick:", []string{"a", "b"}); err == nil {
		t.Fatal("Choose() = nil error after five bad answers")
	}
}
