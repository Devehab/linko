package cloudflare

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// CatchAllService is the mandatory final ingress rule of a tunnel config.
const CatchAllService = "http_status:404"

// Connection is one edge connection of a running tunnel.
type Connection struct {
	ID                 string    `json:"id"`
	ColoName           string    `json:"colo_name"`
	IsPendingReconnect bool      `json:"is_pending_reconnect"`
	OriginIP           string    `json:"origin_ip"`
	OpenedAt           time.Time `json:"opened_at"`
	ClientVersion      string    `json:"client_version"`
}

// Tunnel is a Cloudflare Zero Trust tunnel.
type Tunnel struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	CreatedAt   time.Time    `json:"created_at"`
	DeletedAt   *time.Time   `json:"deleted_at"`
	Status      string       `json:"status"`
	ConfigSrc   string       `json:"config_src"`
	Connections []Connection `json:"connections"`
}

// ActiveConnections counts connections that are not mid-reconnect.
func (t *Tunnel) ActiveConnections() int {
	n := 0
	for _, c := range t.Connections {
		if !c.IsPendingReconnect {
			n++
		}
	}
	return n
}

// Colos returns the unique data centres the tunnel is connected to.
func (t *Tunnel) Colos() []string {
	seen := map[string]bool{}
	out := []string{}
	for _, c := range t.Connections {
		if c.ColoName == "" || seen[c.ColoName] {
			continue
		}
		seen[c.ColoName] = true
		out = append(out, c.ColoName)
	}
	return out
}

func (c *Client) accountPath(suffix string) (string, error) {
	if strings.TrimSpace(c.AccountID) == "" {
		return "", fmt.Errorf("no cloudflare account id configured")
	}
	return "/accounts/" + c.AccountID + suffix, nil
}

// ListTunnels returns non-deleted tunnels, optionally filtered by name.
func (c *Client) ListTunnels(ctx context.Context, name string) ([]Tunnel, error) {
	path, err := c.accountPath("/cfd_tunnel?is_deleted=false&per_page=50")
	if err != nil {
		return nil, err
	}
	if name != "" {
		path += "&name=" + url.QueryEscape(name)
	}
	var tunnels []Tunnel
	if err := c.do(ctx, http.MethodGet, path, nil, &tunnels); err != nil {
		return nil, err
	}
	return tunnels, nil
}

// FindTunnel returns the tunnel with the exact name, or (nil, nil).
func (c *Client) FindTunnel(ctx context.Context, name string) (*Tunnel, error) {
	tunnels, err := c.ListTunnels(ctx, name)
	if err != nil {
		return nil, err
	}
	for i := range tunnels {
		if tunnels[i].Name == name && tunnels[i].DeletedAt == nil {
			return &tunnels[i], nil
		}
	}
	return nil, nil
}

// GetTunnel fetches a single tunnel, including its live connections.
func (c *Client) GetTunnel(ctx context.Context, id string) (*Tunnel, error) {
	path, err := c.accountPath("/cfd_tunnel/" + url.PathEscape(id))
	if err != nil {
		return nil, err
	}
	var t Tunnel
	if err := c.do(ctx, http.MethodGet, path, nil, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// CreateTunnel creates a remotely-managed tunnel.
func (c *Client) CreateTunnel(ctx context.Context, name string) (*Tunnel, error) {
	path, err := c.accountPath("/cfd_tunnel")
	if err != nil {
		return nil, err
	}
	body := map[string]string{
		"name":       name,
		"config_src": "cloudflare",
	}
	var t Tunnel
	if err := c.do(ctx, http.MethodPost, path, body, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// DeleteTunnel removes a tunnel.
func (c *Client) DeleteTunnel(ctx context.Context, id string) error {
	path, err := c.accountPath("/cfd_tunnel/" + url.PathEscape(id))
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodDelete, path, nil, nil)
}

// TunnelToken returns the token cloudflared needs to run the tunnel.
func (c *Client) TunnelToken(ctx context.Context, id string) (string, error) {
	path, err := c.accountPath("/cfd_tunnel/" + url.PathEscape(id) + "/token")
	if err != nil {
		return "", err
	}
	var token string
	if err := c.do(ctx, http.MethodGet, path, nil, &token); err != nil {
		return "", err
	}
	if token == "" {
		return "", fmt.Errorf("cloudflare returned an empty tunnel token")
	}
	return token, nil
}

// IngressRule maps a public hostname to a local service.
type IngressRule struct {
	Hostname      string         `json:"hostname,omitempty"`
	Path          string         `json:"path,omitempty"`
	Service       string         `json:"service"`
	OriginRequest map[string]any `json:"originRequest,omitempty"`
}

// TunnelConfig is the remotely-stored configuration of a tunnel.
type TunnelConfig struct {
	Ingress       []IngressRule  `json:"ingress"`
	OriginRequest map[string]any `json:"originRequest,omitempty"`
	WarpRouting   map[string]any `json:"warp-routing,omitempty"`
}

type tunnelConfigEnvelope struct {
	TunnelID string        `json:"tunnel_id"`
	Version  int           `json:"version"`
	Config   *TunnelConfig `json:"config"`
	Source   string        `json:"source"`
}

// GetTunnelConfig fetches the tunnel's remote configuration.
func (c *Client) GetTunnelConfig(ctx context.Context, id string) (*TunnelConfig, error) {
	path, err := c.accountPath("/cfd_tunnel/" + url.PathEscape(id) + "/configurations")
	if err != nil {
		return nil, err
	}
	var env tunnelConfigEnvelope
	if err := c.do(ctx, http.MethodGet, path, nil, &env); err != nil {
		return nil, err
	}
	if env.Config == nil {
		return &TunnelConfig{}, nil
	}
	return env.Config, nil
}

// PutTunnelConfig replaces the tunnel's remote configuration.
func (c *Client) PutTunnelConfig(ctx context.Context, id string, cfg *TunnelConfig) error {
	path, err := c.accountPath("/cfd_tunnel/" + url.PathEscape(id) + "/configurations")
	if err != nil {
		return err
	}
	if cfg == nil {
		cfg = &TunnelConfig{}
	}
	cfg.Ingress = NormalizeIngress(cfg.Ingress)
	return c.do(ctx, http.MethodPut, path, map[string]any{"config": cfg}, nil)
}

// NormalizeIngress guarantees exactly one catch-all rule, positioned last.
func NormalizeIngress(rules []IngressRule) []IngressRule {
	out := make([]IngressRule, 0, len(rules)+1)
	catchAll := IngressRule{Service: CatchAllService}
	seenCatchAll := false

	for _, r := range rules {
		if r.Hostname == "" && r.Path == "" {
			if !seenCatchAll {
				catchAll = r
				seenCatchAll = true
			}
			continue
		}
		out = append(out, r)
	}
	if strings.TrimSpace(catchAll.Service) == "" {
		catchAll.Service = CatchAllService
	}
	return append(out, catchAll)
}

// FindIngress returns the rule serving a hostname.
func FindIngress(rules []IngressRule, hostname string) (IngressRule, bool) {
	for _, r := range rules {
		if r.Hostname != "" && strings.EqualFold(r.Hostname, hostname) {
			return r, true
		}
	}
	return IngressRule{}, false
}

// UpsertIngress adds or updates the rule for hostname, keeping the catch-all last.
func UpsertIngress(rules []IngressRule, hostname, service string) []IngressRule {
	normalized := NormalizeIngress(rules)
	for i := range normalized {
		if normalized[i].Hostname != "" && strings.EqualFold(normalized[i].Hostname, hostname) {
			normalized[i].Service = service
			return normalized
		}
	}
	last := len(normalized) - 1
	out := make([]IngressRule, 0, len(normalized)+1)
	out = append(out, normalized[:last]...)
	out = append(out, IngressRule{Hostname: hostname, Service: service})
	out = append(out, normalized[last])
	return out
}

// RemoveIngress drops every rule for hostname.
func RemoveIngress(rules []IngressRule, hostname string) []IngressRule {
	normalized := NormalizeIngress(rules)
	out := make([]IngressRule, 0, len(normalized))
	for _, r := range normalized {
		if r.Hostname != "" && strings.EqualFold(r.Hostname, hostname) {
			continue
		}
		out = append(out, r)
	}
	return NormalizeIngress(out)
}

// HostnameRules returns only the rules that publish a hostname.
func HostnameRules(rules []IngressRule) []IngressRule {
	out := make([]IngressRule, 0, len(rules))
	for _, r := range rules {
		if r.Hostname != "" {
			out = append(out, r)
		}
	}
	return out
}
