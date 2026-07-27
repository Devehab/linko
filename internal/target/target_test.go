package target

import "testing"

func TestParseValid(t *testing.T) {
	cases := []struct {
		in      string
		service string
		port    int
	}{
		{"3000", "http://localhost:3000", 3000},
		{" 3000 ", "http://localhost:3000", 3000},
		{"80", "http://localhost:80", 80},
		{"65535", "http://localhost:65535", 65535},
		{":3000", "http://localhost:3000", 3000},
		{"localhost:3000", "http://localhost:3000", 3000},
		{"127.0.0.1:8080", "http://127.0.0.1:8080", 8080},
		{"0.0.0.0:5000", "http://0.0.0.0:5000", 5000},
		{"http://localhost:3000", "http://localhost:3000", 3000},
		{"https://127.0.0.1:8443", "https://127.0.0.1:8443", 8443},
		{"tcp://localhost:22", "tcp://localhost:22", 22},
	}
	for _, c := range cases {
		service, port, err := Parse(c.in)
		if err != nil {
			t.Errorf("Parse(%q) error = %v", c.in, err)
			continue
		}
		if service != c.service {
			t.Errorf("Parse(%q) service = %q, want %q", c.in, service, c.service)
		}
		if port != c.port {
			t.Errorf("Parse(%q) port = %d, want %d", c.in, port, c.port)
		}
	}
}

func TestParseInvalid(t *testing.T) {
	for _, in := range []string{
		"",
		"   ",
		"0",
		"70000",
		"-1",
		"abc",
		"list",
		"init",
		"status",
		"doctor",
		"remove",
		"localhost",
		"localhost:abc",
		"ftp://localhost:21",
	} {
		if service, port, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) = (%q, %d, nil), want an error", in, service, port)
		}
	}
}

func TestLooksRejectsSubcommands(t *testing.T) {
	for _, name := range []string{"init", "start", "list", "remove", "status", "doctor", "help", "completion"} {
		if Looks(name) {
			t.Errorf("Looks(%q) = true, want false — it would shadow a subcommand", name)
		}
	}
	for _, ok := range []string{"3000", ":8080", "localhost:3000", "http://localhost:3000"} {
		if !Looks(ok) {
			t.Errorf("Looks(%q) = false, want true", ok)
		}
	}
}

func TestParseIPv6(t *testing.T) {
	service, port, err := Parse("[::1]:3000")
	if err != nil {
		t.Fatalf("Parse ipv6 error = %v", err)
	}
	if port != 3000 {
		t.Fatalf("port = %d", port)
	}
	if service != "http://[::1]:3000" {
		t.Fatalf("service = %q", service)
	}
}
