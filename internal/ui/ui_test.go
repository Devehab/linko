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

func TestLinkWrapsInOSC8WhenSupported(t *testing.T) {
	oldColor, oldLinks := useColor, hyperlinkCapable
	defer func() { useColor, hyperlinkCapable = oldColor, oldLinks }()

	url := "https://ali.example.com"

	useColor, hyperlinkCapable = true, true
	got := Link(url, url)
	if !strings.HasPrefix(got, "\x1b]8;;"+url+"\x1b\\") {
		t.Fatalf("Link() did not open an OSC 8 sequence: %q", got)
	}
	if !strings.HasSuffix(got, "\x1b]8;;\x1b\\") {
		t.Fatalf("Link() did not close the OSC 8 sequence: %q", got)
	}
	if !strings.Contains(got, url) {
		t.Fatal("the URL must stay visible in the label")
	}
}

func TestLinkIsPlainWhenUnsupported(t *testing.T) {
	oldColor, oldLinks := useColor, hyperlinkCapable
	defer func() { useColor, hyperlinkCapable = oldColor, oldLinks }()

	url := "https://ali.example.com"

	// Terminal cannot follow OSC 8 — emit nothing it would render as garbage.
	useColor, hyperlinkCapable = true, false
	if got := Link(url, url); got != url {
		t.Fatalf("Link() = %q on a terminal without OSC 8, want the bare text", got)
	}

	// Piped output or --no-color: never emit escapes.
	useColor, hyperlinkCapable = false, true
	if got := Link(url, url); got != url {
		t.Fatalf("Link() = %q with colour off, want the bare text", got)
	}
}

func TestDetectHyperlinks(t *testing.T) {
	t.Setenv("VTE_VERSION", "")
	for _, env := range []string{"KITTY_WINDOW_ID", "WT_SESSION", "KONSOLE_VERSION",
		"ALACRITTY_WINDOW_ID", "DOMTERM", "CONTOUR_PROFILE"} {
		t.Setenv(env, "")
	}

	t.Setenv("TERM_PROGRAM", "iTerm.app")
	if !detectHyperlinks() {
		t.Error("iTerm2 supports OSC 8")
	}

	t.Setenv("TERM_PROGRAM", "vscode")
	if !detectHyperlinks() {
		t.Error("the VS Code terminal supports OSC 8")
	}

	// Terminal.app linkifies bare URLs itself, but ignores OSC 8 — emitting it
	// there would only add invisible noise.
	t.Setenv("TERM_PROGRAM", "Apple_Terminal")
	if detectHyperlinks() {
		t.Error("Terminal.app does not support OSC 8")
	}
	if !AppleTerminal() {
		t.Error("AppleTerminal() should detect Terminal.app")
	}

	t.Setenv("TERM_PROGRAM", "")
	if detectHyperlinks() {
		t.Error("an unknown terminal must not be assumed to support OSC 8")
	}

	t.Setenv("KITTY_WINDOW_ID", "1")
	if !detectHyperlinks() {
		t.Error("kitty supports OSC 8")
	}
	t.Setenv("KITTY_WINDOW_ID", "")

	t.Setenv("VTE_VERSION", "6003")
	if !detectHyperlinks() {
		t.Error("VTE 0.60 supports OSC 8")
	}
	t.Setenv("VTE_VERSION", "4800")
	if detectHyperlinks() {
		t.Error("VTE older than 0.50 does not support OSC 8")
	}
}

func TestClickHint(t *testing.T) {
	oldColor, oldLinks := useColor, hyperlinkCapable
	defer func() { useColor, hyperlinkCapable = oldColor, oldLinks }()

	useColor, hyperlinkCapable = true, true
	if ClickHint() == "" {
		t.Error("a hint is useful even when links are clickable")
	}

	useColor = false
	if ClickHint() != "" {
		t.Error("no hint when output is not a terminal")
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
