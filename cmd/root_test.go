package cmd

import (
	"strings"
	"testing"
)

func names(t *testing.T) map[string]bool {
	t.Helper()
	return commandNames(NewRootCmd())
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestCommandTreeHasEveryCommand(t *testing.T) {
	known := names(t)
	for _, want := range []string{"init", "start", "list", "remove", "status", "doctor"} {
		if !known[want] {
			t.Errorf("command %q is missing from the tree", want)
		}
	}
	for _, alias := range []string{"ls", "rm", "run"} {
		if !known[alias] {
			t.Errorf("alias %q is not registered", alias)
		}
	}
}

func TestNormalizeArgsInsertsStart(t *testing.T) {
	known := names(t)
	cases := []struct {
		in   []string
		want []string
	}{
		{[]string{"3000"}, []string{"start", "3000"}},
		{[]string{"3000", "--name", "crm"}, []string{"start", "3000", "--name", "crm"}},
		{[]string{"8080", "-n", "api", "--keep"}, []string{"start", "8080", "-n", "api", "--keep"}},
		{[]string{"localhost:3000"}, []string{"start", "localhost:3000"}},
		{[]string{":3000"}, []string{"start", ":3000"}},
		{[]string{"http://127.0.0.1:5000"}, []string{"start", "http://127.0.0.1:5000"}},
	}
	for _, c := range cases {
		got := NormalizeArgs(c.in, known)
		if !equal(got, c.want) {
			t.Errorf("NormalizeArgs(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNormalizeArgsLeavesCommandsAlone(t *testing.T) {
	known := names(t)
	cases := [][]string{
		{},
		{"init"},
		{"start", "3000"},
		{"list"},
		{"ls", "--remote"},
		{"remove", "crm"},
		{"rm", "--all"},
		{"status"},
		{"doctor"},
		{"help"},
		{"completion", "zsh"},
		{"--version"},
		{"--help"},
		{"-h"},
	}
	for _, in := range cases {
		got := NormalizeArgs(in, known)
		if !equal(got, in) {
			t.Errorf("NormalizeArgs(%v) = %v, want it unchanged", in, got)
		}
	}
}

func TestNormalizeArgsLeavesUnknownWordsAlone(t *testing.T) {
	known := names(t)
	// Not a port and not a command: cobra should produce its own error.
	in := []string{"frobnicate"}
	if got := NormalizeArgs(in, known); !equal(got, in) {
		t.Fatalf("NormalizeArgs(%v) = %v", in, got)
	}
}

func TestHelpMentionsThePortShortcut(t *testing.T) {
	root := NewRootCmd()
	if !strings.Contains(root.Long, "linko 3000") {
		t.Fatal("the root help text does not document the `linko 3000` shortcut")
	}
}

func TestExpandBase(t *testing.T) {
	cases := map[string]string{
		"demo":             "demo.example.com",
		"demo.example.com": "demo.example.com",
		"":                 "example.com",
		"@":                "example.com",
		"a.b":              "a.b.example.com",
	}
	for in, want := range cases {
		if got := expandBase(in, "example.com"); got != want {
			t.Errorf("expandBase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateBase(t *testing.T) {
	if err := validateBase("demo", "example.com"); err != nil {
		t.Errorf("validateBase(demo) = %v", err)
	}
	if err := validateBase("demo.other.com", "example.com"); err != nil {
		// "demo.other.com" is expanded to demo.other.com.example.com, which is valid.
		t.Errorf("validateBase = %v", err)
	}
	if err := validateBase("DEMO", "example.com"); err == nil {
		t.Error("validateBase accepted an uppercase label")
	}
}

func TestDefaultTunnelName(t *testing.T) {
	if got := defaultTunnelName("ibtkrgo.com"); got != "ibtkrgo-linko-tunnel" {
		t.Fatalf("defaultTunnelName = %q", got)
	}
	if got := defaultTunnelName("example.co.uk"); got != "example-linko-tunnel" {
		t.Fatalf("defaultTunnelName = %q", got)
	}
}

func TestIsConnectedLine(t *testing.T) {
	connected := []string{
		"2026-07-27T10:00:00Z INF Registered tunnel connection connIndex=0",
		"INF connection established",
	}
	for _, l := range connected {
		if !isConnectedLine(l) {
			t.Errorf("isConnectedLine(%q) = false", l)
		}
	}
	if isConnectedLine("INF Starting tunnel") {
		t.Error("isConnectedLine matched an unrelated line")
	}
}

func TestFirstLine(t *testing.T) {
	if got := firstLine("cloudflared version 2024.1.0\nbuilt on x"); got != "cloudflared version 2024.1.0" {
		t.Fatalf("firstLine = %q", got)
	}
	if got := firstLine(" single "); got != "single" {
		t.Fatalf("firstLine = %q", got)
	}
}

func TestPlural(t *testing.T) {
	if got := plural(1, "route"); got != "1 route" {
		t.Fatalf("plural(1) = %q", got)
	}
	if got := plural(3, "route"); got != "3 routes" {
		t.Fatalf("plural(3) = %q", got)
	}
	if got := plural(0, "route"); got != "0 routes" {
		t.Fatalf("plural(0) = %q", got)
	}
}
