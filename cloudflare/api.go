// Package cloudflare is a minimal client for the parts of the Cloudflare API
// that linko needs: token verification, zones, Zero Trust tunnels and DNS.
//
// It intentionally uses only net/http so linko stays a small, dependency-light
// binary and so the request/response shapes are visible and testable.
package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBaseURL is the Cloudflare API v4 root.
const DefaultBaseURL = "https://api.cloudflare.com/client/v4"

// UserAgent is sent with every request; main() overrides it with the version.
var UserAgent = "linko"

// Client talks to the Cloudflare API with a scoped API token.
type Client struct {
	Token     string
	AccountID string
	ZoneID    string
	BaseURL   string
	HTTP      *http.Client
}

// New returns a client with sensible defaults.
func New(token string) *Client {
	return &Client{
		Token:   strings.TrimSpace(token),
		BaseURL: DefaultBaseURL,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

// ErrorDetail is a single error entry from a Cloudflare response.
type ErrorDetail struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Error is an unsuccessful Cloudflare API response.
type Error struct {
	StatusCode int
	Method     string
	Path       string
	Errors     []ErrorDetail
	Raw        string
}

func (e *Error) Error() string {
	if len(e.Errors) > 0 {
		msgs := make([]string, 0, len(e.Errors))
		for _, d := range e.Errors {
			msgs = append(msgs, fmt.Sprintf("%s (code %d)", d.Message, d.Code))
		}
		return fmt.Sprintf("cloudflare: %s", strings.Join(msgs, "; "))
	}
	raw := e.Raw
	if len(raw) > 200 {
		raw = raw[:200] + "…"
	}
	if raw == "" {
		raw = http.StatusText(e.StatusCode)
	}
	return fmt.Sprintf("cloudflare: HTTP %d: %s", e.StatusCode, raw)
}

// HasCode reports whether any returned error carries the given Cloudflare code.
func (e *Error) HasCode(code int) bool {
	for _, d := range e.Errors {
		if d.Code == code {
			return true
		}
	}
	return false
}

// IsAuth reports whether the error looks like a bad or under-scoped token.
func (e *Error) IsAuth() bool {
	if e.StatusCode == http.StatusUnauthorized || e.StatusCode == http.StatusForbidden {
		return true
	}
	// 1000 invalid credentials, 9109 unauthorised to access resource,
	// 10000 authentication error.
	return e.HasCode(1000) || e.HasCode(9109) || e.HasCode(10000)
}

// IsNotFound reports whether the resource does not exist.
func (e *Error) IsNotFound() bool {
	return e.StatusCode == http.StatusNotFound || e.HasCode(1049) || e.HasCode(81044)
}

type envelope struct {
	Success bool            `json:"success"`
	Errors  []ErrorDetail   `json:"errors"`
	Result  json.RawMessage `json:"result"`
}

func (c *Client) baseURL() string {
	if c.BaseURL != "" {
		return strings.TrimSuffix(c.BaseURL, "/")
	}
	return DefaultBaseURL
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

// do performs a request and unmarshals the `result` field into out.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL()+path, reader)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", UserAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("calling cloudflare: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return fmt.Errorf("reading cloudflare response: %w", err)
	}

	var env envelope
	if jerr := json.Unmarshal(raw, &env); jerr != nil {
		return &Error{
			StatusCode: resp.StatusCode,
			Method:     method,
			Path:       path,
			Raw:        strings.TrimSpace(string(raw)),
		}
	}
	if !env.Success || resp.StatusCode >= 400 {
		return &Error{
			StatusCode: resp.StatusCode,
			Method:     method,
			Path:       path,
			Errors:     env.Errors,
			Raw:        strings.TrimSpace(string(raw)),
		}
	}
	if out != nil && len(env.Result) > 0 && string(env.Result) != "null" {
		if uerr := json.Unmarshal(env.Result, out); uerr != nil {
			return fmt.Errorf("decoding cloudflare response: %w", uerr)
		}
	}
	return nil
}

// TokenStatus is the result of a token verification call.
type TokenStatus struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// VerifyToken checks the token is live. It falls back to an account-scoped
// check and finally to a cheap authenticated read, because account-owned
// tokens cannot call /user endpoints.
func (c *Client) VerifyToken(ctx context.Context) (*TokenStatus, error) {
	var ts TokenStatus
	err := c.do(ctx, http.MethodGet, "/user/tokens/verify", nil, &ts)
	if err == nil {
		return &ts, nil
	}

	if c.AccountID != "" {
		var accTS TokenStatus
		if e := c.do(ctx, http.MethodGet, "/accounts/"+c.AccountID+"/tokens/verify", nil, &accTS); e == nil {
			return &accTS, nil
		}
	}

	var zones []Zone
	if e := c.do(ctx, http.MethodGet, "/zones?per_page=1", nil, &zones); e == nil {
		return &TokenStatus{Status: "active"}, nil
	}
	return nil, err
}

// Account is a Cloudflare account.
type Account struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ListAccounts returns the accounts the token can see.
func (c *Client) ListAccounts(ctx context.Context) ([]Account, error) {
	var accounts []Account
	if err := c.do(ctx, http.MethodGet, "/accounts?per_page=50", nil, &accounts); err != nil {
		return nil, err
	}
	return accounts, nil
}

// Zone is a DNS zone.
type Zone struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Account struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"account"`
}

// ListZones returns every zone visible to the token.
func (c *Client) ListZones(ctx context.Context) ([]Zone, error) {
	var zones []Zone
	if err := c.do(ctx, http.MethodGet, "/zones?per_page=50", nil, &zones); err != nil {
		return nil, err
	}
	return zones, nil
}

// FindZone looks a zone up by exact name.
func (c *Client) FindZone(ctx context.Context, name string) (*Zone, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	var zones []Zone
	path := "/zones?name=" + url.QueryEscape(name)
	if err := c.do(ctx, http.MethodGet, path, nil, &zones); err != nil {
		return nil, err
	}
	for i := range zones {
		if strings.EqualFold(zones[i].Name, name) {
			return &zones[i], nil
		}
	}
	return nil, fmt.Errorf("no zone named %q on this account — check the domain and that the token has Zone:Read", name)
}

// ZoneNameFor returns the registrable zone name for a hostname by matching it
// against the zones the token can see. Useful when the user types a subdomain.
func ZoneNameFor(hostname string, zones []Zone) (Zone, bool) {
	hostname = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(hostname), "."))
	best := Zone{}
	found := false
	for _, z := range zones {
		zn := strings.ToLower(z.Name)
		if hostname == zn || strings.HasSuffix(hostname, "."+zn) {
			if !found || len(zn) > len(best.Name) {
				best, found = z, true
			}
		}
	}
	return best, found
}
