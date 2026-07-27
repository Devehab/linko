// Package target turns what the user types on the command line into a
// cloudflared origin service URL.
package target

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// DefaultHost is used when the user only gives a port.
const DefaultHost = "localhost"

// Parse converts a user-supplied target into a service URL and, when known,
// the local port.
//
//	"3000"                  -> "http://localhost:3000", 3000
//	":3000"                 -> "http://localhost:3000", 3000
//	"localhost:3000"        -> "http://localhost:3000", 3000
//	"127.0.0.1:8080"        -> "http://127.0.0.1:8080", 8080
//	"http://localhost:3000" -> "http://localhost:3000", 3000
//	"https://127.0.0.1:8443"-> "https://127.0.0.1:8443", 8443
func Parse(s string) (service string, port int, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", 0, fmt.Errorf("no port or address given")
	}

	// Full URL form.
	if strings.Contains(s, "://") {
		u, uerr := url.Parse(s)
		if uerr != nil {
			return "", 0, fmt.Errorf("%q is not a valid address: %w", s, uerr)
		}
		switch u.Scheme {
		case "http", "https", "tcp", "unix", "ssh", "rdp":
		default:
			return "", 0, fmt.Errorf("unsupported scheme %q (use http, https, tcp, unix, ssh or rdp)", u.Scheme)
		}
		if u.Scheme == "unix" {
			return s, 0, nil
		}
		if u.Host == "" {
			return "", 0, fmt.Errorf("%q is missing a host", s)
		}
		p := 0
		if ps := u.Port(); ps != "" {
			p, err = parsePort(ps)
			if err != nil {
				return "", 0, err
			}
		}
		return strings.TrimSuffix(u.String(), "/"), p, nil
	}

	// Bare port.
	if p, perr := parsePort(s); perr == nil {
		return fmt.Sprintf("http://%s:%d", DefaultHost, p), p, nil
	} else if isAllDigits(s) {
		return "", 0, perr
	}

	// ":3000"
	if strings.HasPrefix(s, ":") {
		p, perr := parsePort(strings.TrimPrefix(s, ":"))
		if perr != nil {
			return "", 0, perr
		}
		return fmt.Sprintf("http://%s:%d", DefaultHost, p), p, nil
	}

	// "host:port"
	host, portStr, serr := net.SplitHostPort(s)
	if serr != nil {
		return "", 0, fmt.Errorf("%q is not a port or host:port address", s)
	}
	if host == "" {
		host = DefaultHost
	}
	p, perr := parsePort(portStr)
	if perr != nil {
		return "", 0, perr
	}
	if strings.Contains(host, ":") { // bare IPv6
		host = "[" + host + "]"
	}
	return fmt.Sprintf("http://%s:%d", host, p), p, nil
}

// Looks reports whether s could plausibly be a target rather than a subcommand.
func Looks(s string) bool {
	_, _, err := Parse(s)
	return err == nil
}

func parsePort(s string) (int, error) {
	p, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%q is not a valid port number", s)
	}
	if p < 1 || p > 65535 {
		return 0, fmt.Errorf("port %d is out of range (1-65535)", p)
	}
	return p, nil
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
